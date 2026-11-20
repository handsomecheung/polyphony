use crate::daemon::{DaemonClient, LoginRequest};
use dialoguer::Input;
use std::io::{self, Write};

pub async fn execute_login(
    client: &DaemonClient,
    server: Option<String>,
    email: Option<String>,
    apikey: bool,
    client_id: Option<String>,
    client_secret: Option<String>,
    password: Option<String>,
) -> Result<(), Box<dyn std::error::Error>> {
    let client_id = client_id.or_else(|| std::env::var("BW_CLIENTID").ok());
    let client_secret = client_secret.or_else(|| std::env::var("BW_CLIENTSECRET").ok());
    let password = password.or_else(|| std::env::var("BW_PASSWORD").ok());

    let is_apikey_mode = apikey || client_id.is_some() || client_secret.is_some();

    if is_apikey_mode {
        let server_url = match server {
            Some(s) => s,
            None => Input::<String>::new()
                .with_prompt("Bitwarden / Vaultwarden Server URL")
                .default("https://vault.bitwarden.com".to_string())
                .interact_text()?,
        };

        let cid = match client_id {
            Some(id) => id,
            None => Input::<String>::new()
                .with_prompt("API Client ID (e.g. user.43b602af-... or UUID)")
                .interact_text()?,
        };

        let csec = match client_secret {
            Some(sec) => sec,
            None => rpassword::prompt_password("API Client Secret: ")?,
        };

        let cid = cid.trim().trim_matches('"').trim_matches('\'').to_string();
        let csec = csec.trim().trim_matches('"').trim_matches('\'').to_string();

        let master_password = match password {
            Some(p) => Some(p),
            None => {
                let p = rpassword::prompt_password(
                    "Master Password (optional, leave blank to leave locked): ",
                )?;
                if p.trim().is_empty() {
                    None
                } else {
                    Some(p)
                }
            }
        };

        let req = LoginRequest::api_key(server_url.clone(), cid, csec, master_password, email);
        let resp = client.login(&req).await?;

        if resp.unlocked {
            println!(
                "✓ Logged in via API key to {server_url}. Vault unlocked ({} items).",
                resp.item_count
            );
        } else {
            println!(
                "✓ Logged in via API key to {server_url}. Vault synced ({} items, locked).",
                resp.item_count
            );
            println!("  Run 'tacet unlock' to unlock key custody.");
        }

        Ok(())
    } else {
        let server_url = match server {
            Some(s) => s,
            None => Input::<String>::new()
                .with_prompt("Bitwarden / Vaultwarden Server URL")
                .default("https://vault.bitwarden.com".to_string())
                .interact_text()?,
        };

        let email_addr = match email {
            Some(e) => e,
            None => Input::<String>::new()
                .with_prompt("Account Email")
                .interact_text()?,
        };

        let master_password = match password {
            Some(p) => p,
            None => rpassword::prompt_password("Master Password: ")?,
        };

        let mut req =
            LoginRequest::password(server_url.clone(), email_addr.clone(), master_password);

        match client.login(&req).await {
            Ok(resp) => {
                println!(
                    "✓ Logged in to {server_url} as {email_addr}. Vault unlocked ({} items).",
                    resp.item_count
                );
                Ok(())
            }
            Err(e) => {
                let err_msg = e.to_string();
                if err_msg.to_lowercase().contains("two factor")
                    || err_msg.to_lowercase().contains("two-factor")
                {
                    print!("Two-Factor Authentication Code: ");
                    io::stdout().flush()?;
                    let mut token_input = String::new();
                    io::stdin().read_line(&mut token_input)?;
                    let token = token_input.trim().to_string();

                    req.two_factor_provider = Some(0); // Authenticator TOTP
                    req.two_factor_token = Some(token);

                    let retry_resp = client.login(&req).await?;
                    println!(
                        "✓ Logged in to {server_url} as {email_addr}. Vault unlocked ({} items).",
                        retry_resp.item_count
                    );
                    Ok(())
                } else {
                    Err(e.into())
                }
            }
        }
    }
}
