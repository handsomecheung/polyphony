use crate::daemon::DaemonClient;

pub async fn execute_logout(client: &DaemonClient) -> Result<(), Box<dyn std::error::Error>> {
    let resp = client.logout().await?;
    if resp.logged_out {
        println!("✓ Logged out. In-memory keys wiped and local session/cache removed.");
    }
    Ok(())
}
