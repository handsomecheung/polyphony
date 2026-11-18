use crate::daemon::DaemonClient;
use regex::Regex;
use std::collections::HashSet;
use std::fs;
use std::io::{self, Read, Write};
use std::path::PathBuf;

pub async fn execute_inject(
    client: &DaemonClient,
    input: Option<PathBuf>,
    output: Option<PathBuf>,
) -> Result<(), Box<dyn std::error::Error>> {
    // 1. Read input template
    let content = match input {
        Some(path) => fs::read_to_string(&path)?,
        None => {
            let mut buf = String::new();
            io::stdin().read_to_string(&mut buf)?;
            buf
        }
    };

    // 2. Extract placeholders: {{ tacet://... }}
    let re = Regex::new(r"\{\{\s*(tacet://[^\s}]+)\s*\}\}")?;
    let mut uris_set = HashSet::new();
    for cap in re.captures_iter(&content) {
        if let Some(matched_uri) = cap.get(1) {
            uris_set.insert(matched_uri.as_str().to_string());
        }
    }

    let rendered = if uris_set.is_empty() {
        content
    } else {
        let uris: Vec<String> = uris_set.into_iter().collect();

        // 3. Fail-Fast Guarantee: Batch resolve all URIs; abort if any fail
        let resolve_resp = client.resolve(uris).await?;

        // 4. Replace placeholders with resolved secrets
        re.replace_all(&content, |caps: &regex::Captures| {
            let uri = &caps[1];
            resolve_resp
                .secrets
                .get(uri)
                .cloned()
                .unwrap_or_else(|| caps[0].to_string())
        })
        .to_string()
    };

    // 5. Write output
    match output {
        Some(path) => {
            fs::write(path, rendered)?;
        }
        None => {
            print!("{rendered}");
            io::stdout().flush()?;
        }
    }

    Ok(())
}
