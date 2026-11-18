use crate::daemon::DaemonClient;
use std::io::{self, Write};

pub async fn execute_read(
    client: &DaemonClient,
    uri: String,
    no_newline: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    let resp = client.resolve(vec![uri.clone()]).await?;
    let secret = resp
        .secrets
        .get(&uri)
        .ok_or_else(|| format!("Secret '{uri}' was not returned by daemon"))?;

    if no_newline {
        print!("{secret}");
        io::stdout().flush()?;
    } else {
        println!("{secret}");
    }

    Ok(())
}
