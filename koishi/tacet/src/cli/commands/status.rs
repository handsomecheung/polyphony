use crate::daemon::DaemonClient;

pub async fn execute_status(client: &DaemonClient) -> Result<(), Box<dyn std::error::Error>> {
    let status = client.status().await?;

    println!("Daemon:     Running (UDS)");
    if status.logged_in {
        let server = status.server_url.as_deref().unwrap_or("unknown");
        println!("Logged in:  Yes ({server})");
    } else {
        println!("Logged in:  No (run 'tacet login')");
    }

    if status.unlocked {
        println!("Status:     Unlocked");
    } else {
        println!("Status:     Locked (run 'tacet unlock')");
    }

    println!("Vault:      {} items cached", status.item_count);
    Ok(())
}
