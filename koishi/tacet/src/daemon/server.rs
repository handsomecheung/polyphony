use crate::daemon::routes::create_router;
use crate::daemon::state::DaemonState;
use hyper::server::conn::http1;
use hyper_util::rt::TokioIo;
use hyper_util::service::TowerToHyperService;
use tokio::io::AsyncWriteExt;
use tokio::net::UnixListener;
use tokio::sync::broadcast;

/// Runs the `tacetd` HTTP-over-UDS server loop.
/// - Validates peer credentials (UID check).
/// - Rejects non-matching UID connections with 403 Forbidden.
/// - Serves HTTP/1.1 requests using Axum router.
pub async fn run_server(
    listener: UnixListener,
    state: DaemonState,
    mut shutdown_rx: broadcast::Receiver<()>,
) -> std::io::Result<()> {
    // Start background auto-lock task
    let _auto_lock_handle = state.start_auto_lock_task();

    let router = create_router(state);
    let daemon_uid = unsafe { libc::getuid() };

    loop {
        tokio::select! {
            accept_res = listener.accept() => {
                let (mut stream, _) = match accept_res {
                    Ok(res) => res,
                    Err(err) => {
                        tracing::warn!("Failed to accept UDS connection: {err}");
                        continue;
                    }
                };

                // Peer UID verification per Section 6.1
                match stream.peer_cred() {
                    Ok(cred) => {
                        if cred.uid() != daemon_uid {
                            tracing::warn!(
                                "Unauthorized peer connection rejected: peer UID {} != daemon UID {}",
                                cred.uid(),
                                daemon_uid
                            );
                            let forbidden_resp = b"HTTP/1.1 403 Forbidden\r\nContent-Type: text/plain\r\nContent-Length: 17\r\nConnection: close\r\n\r\n403 UID Forbidden";
                            let _ = stream.write_all(forbidden_resp).await;
                            let _ = stream.flush().await;
                            continue;
                        }
                    }
                    Err(e) => {
                        tracing::warn!("Could not read peer credentials: {e}");
                        continue;
                    }
                }

                let router_clone = router.clone();
                tokio::spawn(async move {
                    let io = TokioIo::new(stream);
                    let service = TowerToHyperService::new(router_clone);
                    if let Err(err) = http1::Builder::new().serve_connection(io, service).await {
                        tracing::debug!("Connection terminated: {err}");
                    }
                });
            }
            _ = shutdown_rx.recv() => {
                tracing::info!("Received shutdown signal, terminating daemon server loop");
                break;
            }
        }
    }

    Ok(())
}
