use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StatusResponse {
    pub running: bool,
    pub logged_in: bool,
    pub unlocked: bool,
    pub server_url: Option<String>,
    pub item_count: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoginRequest {
    pub server_url: String,
    pub email: String,
    pub master_password: String,
    #[serde(default)]
    pub two_factor_provider: Option<u8>,
    #[serde(default)]
    pub two_factor_token: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoginResponse {
    pub unlocked: bool,
    pub item_count: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnlockRequest {
    pub master_password: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnlockResponse {
    pub unlocked: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LockResponse {
    pub locked: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LogoutResponse {
    pub logged_out: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SyncStatusResponse {
    pub synced: bool,
    pub item_count: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResolveRequest {
    pub uris: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResolveResponse {
    pub secrets: HashMap<String, String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ErrorResponse {
    pub error: String,
}

impl ErrorResponse {
    pub fn new<S: Into<String>>(msg: S) -> Self {
        Self { error: msg.into() }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ListQuery {
    pub scope: Option<String>,
    pub search: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ListResponse {
    pub items: Vec<crate::resolver::VaultItemSummary>,
    pub total: usize,
}
