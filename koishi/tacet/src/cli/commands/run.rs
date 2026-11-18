use crate::cli::masker::StreamMasker;
use crate::daemon::DaemonClient;
use regex::Regex;
use std::collections::{HashMap, HashSet};
use std::path::{Path, PathBuf};
use std::process::Stdio;
use tokio::process::Command;

pub async fn execute_run(
    client: &DaemonClient,
    env_file: Option<PathBuf>,
    no_mask: bool,
    command: Vec<String>,
) -> Result<i32, Box<dyn std::error::Error>> {
    if command.is_empty() {
        return Err("No command specified to run".into());
    }

    let mut raw_env_vars: HashMap<String, String> = HashMap::new();

    // 1. Read env file if specified or if .env exists
    let target_env_file = env_file.or_else(|| {
        let p = Path::new(".env");
        if p.is_file() {
            Some(p.to_path_buf())
        } else {
            None
        }
    });

    if let Some(file_path) = target_env_file {
        if file_path.is_file() {
            match dotenvy::from_path_iter(&file_path) {
                Ok(iter) => {
                    for (k, v) in iter.flatten() {
                        raw_env_vars.insert(k, v);
                    }
                }
                Err(e) => {
                    tracing::warn!("Could not parse env file {file_path:?}: {e}");
                }
            }
        }
    }

    // 2. Also check current process environment for tacet:// values
    for (k, v) in std::env::vars() {
        if v.contains("tacet://") {
            raw_env_vars.insert(k, v);
        }
    }

    // 3. Collect all tacet:// URIs from the env vars
    let placeholder_re = Regex::new(r"\{\{\s*(tacet://[^\s}]+)\s*\}\}")?;
    let mut uris_to_resolve = HashSet::new();

    for val in raw_env_vars.values() {
        if val.starts_with("tacet://") {
            uris_to_resolve.insert(val.trim().to_string());
        } else {
            for cap in placeholder_re.captures_iter(val) {
                if let Some(m) = cap.get(1) {
                    uris_to_resolve.insert(m.as_str().to_string());
                }
            }
        }
    }

    // 4. Fail-Fast Guarantee: Resolve all URIs before spawning command
    let resolved_map = if uris_to_resolve.is_empty() {
        HashMap::new()
    } else {
        let uris_vec: Vec<String> = uris_to_resolve.into_iter().collect();
        let resp = client.resolve(uris_vec).await?;
        resp.secrets
    };

    // 5. Render final environment variables
    let mut injected_env = HashMap::new();
    for (k, v) in raw_env_vars {
        if v.starts_with("tacet://") {
            let uri = v.trim();
            if let Some(resolved) = resolved_map.get(uri) {
                injected_env.insert(k, resolved.clone());
            }
        } else if placeholder_re.is_match(&v) {
            let rendered = placeholder_re.replace_all(&v, |caps: &regex::Captures| {
                let uri = &caps[1];
                resolved_map
                    .get(uri)
                    .cloned()
                    .unwrap_or_else(|| caps[0].to_string())
            });
            injected_env.insert(k, rendered.to_string());
        } else {
            injected_env.insert(k, v);
        }
    }

    // 6. Build subprocess
    let mut cmd = Command::new(&command[0]);
    if command.len() > 1 {
        cmd.args(&command[1..]);
    }
    for (k, v) in injected_env {
        cmd.env(k, v);
    }

    if no_mask {
        cmd.stdout(Stdio::inherit());
        cmd.stderr(Stdio::inherit());
        let mut child = cmd.spawn()?;
        let status = child.wait().await?;
        return Ok(status.code().unwrap_or(1));
    }

    // Output Masking: collect secrets >= 4 bytes and stream child output
    let masker = StreamMasker::new(resolved_map.values());

    cmd.stdout(Stdio::piped());
    cmd.stderr(Stdio::piped());

    let mut child = cmd.spawn()?;
    let stdout = child.stdout.take().expect("Child stdout piped");
    let stderr = child.stderr.take().expect("Child stderr piped");

    let masker_out = masker.clone();
    let out_task = tokio::spawn(async move {
        let _ = masker_out.pipe_masked(stdout, tokio::io::stdout()).await;
    });

    let masker_err = masker.clone();
    let err_task = tokio::spawn(async move {
        let _ = masker_err.pipe_masked(stderr, tokio::io::stderr()).await;
    });

    let status = child.wait().await?;
    let _ = out_task.await;
    let _ = err_task.await;

    Ok(status.code().unwrap_or(1))
}
