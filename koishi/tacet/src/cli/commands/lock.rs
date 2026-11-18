use crate::daemon::DaemonClient;

pub async fn execute_lock(client: &DaemonClient) -> Result<(), Box<dyn std::error::Error>> {
    let resp = client.lock().await?;
    if resp.locked {
        println!("✓ Vault locked. Keys wiped from memory.");
    }
    Ok(())
}
