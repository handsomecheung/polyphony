use clap::Parser;
use std::path::PathBuf;
use std::time::Duration;
use tacet::daemon::{bind_unix_socket, default_socket_path, run_server, DaemonState};
use tacet::storage::StoragePaths;
use tokio::sync::broadcast;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[derive(Parser, Debug)]
#[command(
    name = "tacetd",
    about = "Tacet background secret custody daemon (Axum HTTP over UDS)"
)]
struct Args {
    /// Custom Unix Domain Socket path
    #[arg(short, long)]
    socket: Option<PathBuf>,

    /// Inactivity idle timeout before auto-locking keys in seconds (default: 1800s / 30m)
    #[arg(long, default_value_t = 1800)]
    idle_ttl_secs: u64,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "tacetd=info,tower_http=info".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    let args = Args::parse();
    let socket_path = args.socket.unwrap_or_else(default_socket_path);

    tracing::info!("Starting tacetd on UDS socket: {:?}", socket_path);

    let storage_paths = StoragePaths::default_paths()?;
    let idle_ttl = Duration::from_secs(args.idle_ttl_secs);
    let state = DaemonState::new(storage_paths, idle_ttl);

    let listener = bind_unix_socket(&socket_path).await?;
    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);

    // Setup graceful shutdown on Ctrl-C / SIGTERM
    let shutdown_tx_clone = shutdown_tx.clone();
    tokio::spawn(async move {
        let _ = tokio::signal::ctrl_c().await;
        tracing::info!("Received SIGINT, initiating shutdown...");
        let _ = shutdown_tx_clone.send(());
    });

    run_server(listener, state, shutdown_rx).await?;

    // Cleanup socket on exit
    if socket_path.exists() {
        let _ = std::fs::remove_file(&socket_path);
    }

    tracing::info!("tacetd stopped gracefully");
    Ok(())
}
