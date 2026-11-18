use crate::daemon::DaemonClient;
use std::path::{Path, PathBuf};
use std::time::Duration;

#[derive(thiserror::Error, Debug)]
pub enum AutoSpawnError {
    #[error(
        "Could not locate 'tacetd' binary (checked current exe directory, TACETD_BIN, and PATH)"
    )]
    BinaryNotFound,

    #[error("Failed to spawn 'tacetd' process: {0}")]
    SpawnFailed(#[from] std::io::Error),

    #[error("Timed out waiting for tacetd to become ready on socket {0:?}")]
    ReadyTimeout(PathBuf),
}

/// Locates the `tacetd` binary:
/// 1. `TACETD_BIN` environment variable
/// 2. Sibling binary next to the current executable
/// 3. In system `$PATH`
pub fn find_tacetd_binary() -> Option<PathBuf> {
    if let Ok(bin_env) = std::env::var("TACETD_BIN") {
        let p = PathBuf::from(bin_env);
        if p.is_file() {
            return Some(p);
        }
    }

    if let Ok(current_exe) = std::env::current_exe() {
        let sibling = current_exe.with_file_name("tacetd");
        if sibling.is_file() {
            return Some(sibling);
        }
    }

    if let Ok(path_var) = std::env::var("PATH") {
        for dir in std::env::split_paths(&path_var) {
            let candidate = dir.join("tacetd");
            if candidate.is_file() {
                return Some(candidate);
            }
        }
    }

    None
}

/// Checks if `tacetd` is already responding on `socket_path`; if unreachable,
/// auto-spawns it detached in the background and polls `/v1/status` until ready.
pub async fn ensure_daemon_running(socket_path: &Path) -> Result<DaemonClient, AutoSpawnError> {
    let client = DaemonClient::new(socket_path);

    // Fast check: if daemon is already alive, return immediately
    if client.is_alive().await {
        return Ok(client);
    }

    let tacetd_bin = find_tacetd_binary().ok_or(AutoSpawnError::BinaryNotFound)?;

    tracing::debug!("Auto-spawning tacetd daemon from {:?}", tacetd_bin);

    let mut cmd = std::process::Command::new(tacetd_bin);
    cmd.arg("--socket").arg(socket_path);
    cmd.stdin(std::process::Stdio::null())
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null());

    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        cmd.process_group(0);
    }

    cmd.spawn()?;

    // Poll until ready (up to 5 seconds)
    for _ in 0..50 {
        tokio::time::sleep(Duration::from_millis(100)).await;
        if client.is_alive().await {
            return Ok(client);
        }
    }

    Err(AutoSpawnError::ReadyTimeout(socket_path.to_path_buf()))
}
