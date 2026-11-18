use clap::Parser;
use tacet::cli::commands::{
    execute_inject, execute_lock, execute_login, execute_logout, execute_read, execute_run,
    execute_status, execute_sync, execute_unlock,
};
use tacet::cli::{ensure_daemon_running, Cli, Commands};
use tacet::daemon::default_socket_path;

#[tokio::main]
async fn main() {
    let cli = Cli::parse();
    let socket_path = cli.socket.unwrap_or_else(default_socket_path);

    // Auto-spawn daemon if not running per Section 7.1
    let client = match ensure_daemon_running(&socket_path).await {
        Ok(c) => c,
        Err(e) => {
            eprintln!("Error: {e}");
            std::process::exit(1);
        }
    };

    let result = match cli.command {
        Commands::Login { server, email } => execute_login(&client, server, email).await,
        Commands::Unlock => execute_unlock(&client).await,
        Commands::Lock => execute_lock(&client).await,
        Commands::Logout => execute_logout(&client).await,
        Commands::Status => execute_status(&client).await,
        Commands::Sync => execute_sync(&client).await,
        Commands::Read { uri, no_newline } => execute_read(&client, uri, no_newline).await,
        Commands::Inject { input, output } => execute_inject(&client, input, output).await,
        Commands::Run {
            env_file,
            no_mask,
            command,
        } => match execute_run(&client, env_file, no_mask, command).await {
            Ok(code) => std::process::exit(code),
            Err(e) => Err(e),
        },
    };

    if let Err(e) = result {
        eprintln!("Error: {e}");
        std::process::exit(1);
    }
}
