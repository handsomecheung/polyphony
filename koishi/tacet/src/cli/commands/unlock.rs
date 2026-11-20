use crate::daemon::DaemonClient;

pub async fn execute_unlock(
    client: &DaemonClient,
    password: Option<String>,
) -> Result<(), Box<dyn std::error::Error>> {
    let master_password = match password
        .or_else(|| std::env::var("TACET_PASSWORD").ok())
        .or_else(|| std::env::var("BW_PASSWORD").ok())
    {
        Some(p) => p,
        None => rpassword::prompt_password("Master Password: ")?,
    };

    let resp = client.unlock(&master_password).await?;
    if resp.unlocked {
        println!("✓ Vault unlocked.");
    }
    Ok(())
}
