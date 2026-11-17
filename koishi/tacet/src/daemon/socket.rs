use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use tokio::net::{UnixListener, UnixStream};

/// Resolves the default socket path per Section 6.1:
/// `$XDG_RUNTIME_DIR/tacet/tacetd.sock` (fallback `/tmp/tacet-$UID/tacetd.sock`).
pub fn default_socket_path() -> PathBuf {
    let uid = unsafe { libc::getuid() };
    if let Ok(runtime_dir) = std::env::var("XDG_RUNTIME_DIR") {
        if !runtime_dir.trim().is_empty() {
            return PathBuf::from(runtime_dir).join("tacet").join("tacetd.sock");
        }
    }
    PathBuf::from(format!("/tmp/tacet-{uid}")).join("tacetd.sock")
}

/// Prepares the socket directory and binds the UnixListener.
/// - Sets directory mode to 0700.
/// - Cleans up stale socket files if no live daemon is listening.
/// - Sets socket file mode to 0600.
pub async fn bind_unix_socket<P: AsRef<Path>>(socket_path: P) -> std::io::Result<UnixListener> {
    let path = socket_path.as_ref();

    // 1. Ensure parent directory exists with mode 0700
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
        let mut perms = fs::metadata(parent)?.permissions();
        perms.set_mode(0o700);
        fs::set_permissions(parent, perms)?;
    }

    // 2. Stale socket check
    if path.exists() {
        match UnixStream::connect(path).await {
            Ok(_) => {
                // Another live daemon is listening on this socket
                return Err(std::io::Error::new(
                    std::io::ErrorKind::AddrInUse,
                    format!("Another tacetd daemon is already running on {path:?}"),
                ));
            }
            Err(_) => {
                // Stale socket, remove it
                let _ = fs::remove_file(path);
            }
        }
    }

    // 3. Bind UnixListener
    let listener = UnixListener::bind(path)?;

    // 4. Set socket file permissions to 0600
    if path.exists() {
        let mut perms = fs::metadata(path)?.permissions();
        perms.set_mode(0o600);
        fs::set_permissions(path, perms)?;
    }

    Ok(listener)
}
