use crate::daemon::DaemonClient;

pub async fn execute_sync(client: &DaemonClient) -> Result<(), Box<dyn std::error::Error>> {
    let resp = client.sync().await?;
    if resp.synced {
        println!(
            "✓ Vault synchronized successfully ({} items).",
            resp.item_count
        );
    }
    Ok(())
}
