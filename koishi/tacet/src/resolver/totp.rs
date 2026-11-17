use crate::error::ResolverError;
use totp_rs::{Algorithm, Secret, TOTP};

/// Generates a live 6-digit RFC 6238 TOTP code from an otpauth:// URI or raw base32 secret.
pub fn generate_totp(secret_or_url: &str) -> Result<String, ResolverError> {
    let trimmed = secret_or_url.trim();

    if trimmed.starts_with("otpauth://") {
        let totp = TOTP::from_url(trimmed)
            .map_err(|e| ResolverError::TotpError(format!("Failed to parse otpauth URL: {e}")))?;
        return totp
            .generate_current()
            .map_err(|e| ResolverError::TotpError(format!("Failed to generate TOTP: {e}")));
    }

    // Clean base32 string: strip spaces, dashes, and uppercase
    let cleaned: String = trimmed
        .chars()
        .filter(|c| !c.is_whitespace() && *c != '-')
        .collect::<String>()
        .to_uppercase();

    let secret = Secret::Encoded(cleaned);
    let secret_bytes = secret
        .to_bytes()
        .map_err(|e| ResolverError::TotpError(format!("Invalid base32 secret: {e}")))?;

    let totp = TOTP::new_unchecked(
        Algorithm::SHA1,
        6,
        1,
        30,
        secret_bytes,
        None,
        "tacet".to_string(),
    );

    totp.generate_current()
        .map_err(|e| ResolverError::TotpError(format!("Failed to generate TOTP: {e}")))
}
