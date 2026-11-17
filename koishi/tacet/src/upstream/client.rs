use crate::error::UpstreamError;
use crate::upstream::models::{
    PreloginRequest, PreloginResponse, SyncResponse, TokenResponse, TwoFactorCredentials,
    TwoFactorErrorResponse,
};
use crate::upstream::urls::BitwardenEndpoints;
use reqwest::{Client, StatusCode};
use std::collections::HashMap;

#[derive(Clone, Debug)]
pub struct UpstreamClient {
    endpoints: BitwardenEndpoints,
    client: Client,
}

impl UpstreamClient {
    pub fn new(server_url: &str) -> Result<Self, UpstreamError> {
        let endpoints = BitwardenEndpoints::resolve(server_url)?;
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()?;
        Ok(Self { endpoints, client })
    }

    pub fn from_endpoints(endpoints: BitwardenEndpoints) -> Self {
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .unwrap_or_default();
        Self { endpoints, client }
    }

    pub fn endpoints(&self) -> &BitwardenEndpoints {
        &self.endpoints
    }

    /// Calls `POST {identity}/accounts/prelogin` to fetch KDF configuration for an email.
    pub async fn prelogin(&self, email: &str) -> Result<PreloginResponse, UpstreamError> {
        let url = self.endpoints.prelogin_url()?;
        let body = PreloginRequest {
            email: email.to_string(),
        };

        let resp = self.client.post(url).json(&body).send().await?;
        let status = resp.status();
        if !status.is_success() {
            let error_text = resp.text().await.unwrap_or_default();
            return Err(UpstreamError::HttpStatusError {
                status: status.as_u16(),
                message: error_text,
            });
        }

        let prelogin = resp.json::<PreloginResponse>().await?;
        Ok(prelogin)
    }

    /// Authenticates with `POST {identity}/connect/token` via Resource Owner Password Credentials flow.
    pub async fn login(
        &self,
        email: &str,
        master_password_hash: &str,
        two_factor: Option<TwoFactorCredentials>,
    ) -> Result<TokenResponse, UpstreamError> {
        let url = self.endpoints.token_url()?;
        let mut form = HashMap::new();
        form.insert("grant_type", "password".to_string());
        form.insert("username", email.to_string());
        form.insert("password", master_password_hash.to_string());
        form.insert("scope", "api offline_access".to_string());
        form.insert("client_id", "web".to_string());

        let provider_str;
        let remember_str;
        if let Some(tf) = two_factor {
            provider_str = tf.provider.to_string();
            remember_str = if tf.remember { "1" } else { "0" }.to_string();
            form.insert("twoFactorProvider", provider_str);
            form.insert("twoFactorToken", tf.token);
            form.insert("twoFactorRemember", remember_str);
        }

        let resp = self.client.post(url).form(&form).send().await?;
        let status = resp.status();

        if status.is_success() {
            let token_resp = resp.json::<TokenResponse>().await?;
            return Ok(token_resp);
        }

        let body_bytes = resp.bytes().await?;
        if status == StatusCode::BAD_REQUEST {
            if let Ok(tf_err) = serde_json::from_slice::<TwoFactorErrorResponse>(&body_bytes) {
                if tf_err.error.as_deref() == Some("invalid_grant")
                    && tf_err
                        .error_description
                        .as_deref()
                        .map(|d| d.to_lowercase().contains("two factor"))
                        .unwrap_or(false)
                {
                    let providers = tf_err
                        .two_factor_providers
                        .unwrap_or_default()
                        .keys()
                        .filter_map(|k| k.parse::<u8>().ok())
                        .collect();
                    return Err(UpstreamError::TwoFactorRequired { providers });
                }
            }
        }

        let error_msg = String::from_utf8_lossy(&body_bytes).to_string();
        Err(UpstreamError::HttpStatusError {
            status: status.as_u16(),
            message: error_msg,
        })
    }

    /// Refreshes access token via `grant_type=refresh_token`.
    pub async fn refresh_token(&self, refresh_token: &str) -> Result<TokenResponse, UpstreamError> {
        let url = self.endpoints.token_url()?;
        let mut form = HashMap::new();
        form.insert("grant_type", "refresh_token".to_string());
        form.insert("refresh_token", refresh_token.to_string());
        form.insert("client_id", "web".to_string());

        let resp = self.client.post(url).form(&form).send().await?;
        let status = resp.status();
        if !status.is_success() {
            let error_text = resp.text().await.unwrap_or_default();
            return Err(UpstreamError::HttpStatusError {
                status: status.as_u16(),
                message: error_text,
            });
        }

        let token_resp = resp.json::<TokenResponse>().await?;
        Ok(token_resp)
    }

    /// Fetches the encrypted vault contents from `GET {api}/sync`.
    pub async fn sync(&self, access_token: &str) -> Result<SyncResponse, UpstreamError> {
        let url = self.endpoints.sync_url()?;
        let resp = self
            .client
            .get(url)
            .bearer_auth(access_token)
            .send()
            .await?;

        let status = resp.status();
        if !status.is_success() {
            let error_text = resp.text().await.unwrap_or_default();
            return Err(UpstreamError::HttpStatusError {
                status: status.as_u16(),
                message: error_text,
            });
        }

        let sync_resp = resp.json::<SyncResponse>().await?;
        Ok(sync_resp)
    }
}
