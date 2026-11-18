use crate::daemon::DaemonClient;

pub async fn execute_unlock(client: &DaemonClient) -> Result<(), Box<dyn std::error::Error>> {
    let password = rpassword::prompt_password("Master Password: ")?;
    let resp = client.unlock(&password).await?;
    if resp.unlocked {
        println!("✓ Vault unlocked.");
    }
    Ok(())
}
