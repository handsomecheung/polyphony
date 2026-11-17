use crate::error::UpstreamError;
use url::Url;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BitwardenEndpoints {
    pub server_url: String,
    pub identity_url: Url,
    pub api_url: Url,
}

impl BitwardenEndpoints {
    /// Resolves the Identity and API URLs for Bitwarden cloud or custom Vaultwarden instances.
    pub fn resolve(server: &str) -> Result<Self, UpstreamError> {
        let trimmed = server.trim();
        let normalized = if trimmed.is_empty() {
            "https://vault.bitwarden.com"
        } else if !trimmed.starts_with("http://") && !trimmed.starts_with("https://") {
            // Default to https if scheme is omitted
            &format!("https://{trimmed}")
        } else {
            trimmed
        };

        let mut parsed = Url::parse(normalized)?;
        let host_str = parsed.host_str().unwrap_or("").to_lowercase();

        // 1. Bitwarden US Cloud
        if host_str == "bitwarden.com"
            || host_str == "vault.bitwarden.com"
            || host_str == "api.bitwarden.com"
            || host_str == "identity.bitwarden.com"
        {
            return Ok(Self {
                server_url: "https://vault.bitwarden.com".to_string(),
                identity_url: Url::parse("https://identity.bitwarden.com")?,
                api_url: Url::parse("https://api.bitwarden.com")?,
            });
        }

        // 2. Bitwarden EU Cloud
        if host_str == "bitwarden.eu"
            || host_str == "vault.bitwarden.eu"
            || host_str == "api.bitwarden.eu"
            || host_str == "identity.bitwarden.eu"
        {
            return Ok(Self {
                server_url: "https://vault.bitwarden.eu".to_string(),
                identity_url: Url::parse("https://identity.bitwarden.eu")?,
                api_url: Url::parse("https://api.bitwarden.eu")?,
            });
        }

        // 3. Custom Server (e.g. Vaultwarden or self-hosted Bitwarden)
        // Clean trailing slashes or subpaths like /api or /identity
        let mut path = parsed.path().trim_end_matches('/').to_string();
        if path.ends_with("/api") {
            path = path[..path.len() - 4].to_string();
        } else if path.ends_with("/identity") {
            path = path[..path.len() - 9].to_string();
        }
        parsed.set_path(&path);

        let base_str = parsed.as_str().trim_end_matches('/');

        let identity_url = Url::parse(&format!("{base_str}/identity"))?;
        let api_url = Url::parse(&format!("{base_str}/api"))?;

        Ok(Self {
            server_url: base_str.to_string(),
            identity_url,
            api_url,
        })
    }

    pub fn prelogin_url(&self) -> Result<Url, UpstreamError> {
        let base = self.identity_url.as_str().trim_end_matches('/');
        Url::parse(&format!("{base}/accounts/prelogin")).map_err(Into::into)
    }

    pub fn token_url(&self) -> Result<Url, UpstreamError> {
        let base = self.identity_url.as_str().trim_end_matches('/');
        Url::parse(&format!("{base}/connect/token")).map_err(Into::into)
    }

    pub fn sync_url(&self) -> Result<Url, UpstreamError> {
        let base = self.api_url.as_str().trim_end_matches('/');
        Url::parse(&format!("{base}/sync")).map_err(Into::into)
    }
}
