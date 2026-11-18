use crate::daemon::{DaemonClient, LoginRequest};
use dialoguer::Input;
use std::io::{self, Write};

pub async fn execute_login(
    client: &DaemonClient,
    server: Option<String>,
    email: Option<String>,
) -> Result<(), Box<dyn std::error::Error>> {
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

    let master_password = rpassword::prompt_password("Master Password: ")?;

    let mut req = LoginRequest {
        server_url: server_url.clone(),
        email: email_addr.clone(),
        master_password,
        two_factor_provider: None,
        two_factor_token: None,
    };

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
