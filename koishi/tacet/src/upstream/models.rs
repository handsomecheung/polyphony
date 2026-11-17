use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PreloginRequest {
    pub email: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PreloginResponse {
    pub kdf: u8,
    pub kdf_iterations: u32,
    pub kdf_memory: Option<u32>,
    pub kdf_parallelism: Option<u32>,
}

#[derive(Debug, Clone)]
pub struct TwoFactorCredentials {
    pub provider: u8,
    pub token: String,
    pub remember: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenResponse {
    pub access_token: String,
    pub expires_in: u64,
    pub token_type: String,
    pub refresh_token: Option<String>,
    #[serde(rename = "Key", alias = "key")]
    pub key: Option<String>,
    #[serde(rename = "PrivateKey", alias = "privateKey", alias = "private_key")]
    pub private_key: Option<String>,
    #[serde(rename = "Kdf", alias = "kdf")]
    pub kdf: Option<u8>,
    #[serde(
        rename = "KdfIterations",
        alias = "kdfIterations",
        alias = "kdf_iterations"
    )]
    pub kdf_iterations: Option<u32>,
    #[serde(rename = "KdfMemory", alias = "kdfMemory", alias = "kdf_memory")]
    pub kdf_memory: Option<u32>,
    #[serde(
        rename = "KdfParallelism",
        alias = "kdfParallelism",
        alias = "kdf_parallelism"
    )]
    pub kdf_parallelism: Option<u32>,
    #[serde(rename = "ResetMasterPassword", alias = "resetMasterPassword")]
    pub reset_master_password: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TwoFactorErrorResponse {
    pub error: Option<String>,
    pub error_description: Option<String>,
    #[serde(rename = "TwoFactorProviders", alias = "twoFactorProviders")]
    pub two_factor_providers: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct SyncResponse {
    pub profile: Option<ProfileResponse>,
    #[serde(default)]
    pub folders: Vec<FolderResponse>,
    #[serde(default)]
    pub collections: Vec<CollectionResponse>,
    #[serde(default)]
    pub ciphers: Vec<CipherResponse>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileOrganizationResponse {
    pub id: String,
    pub name: String,
    pub key: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProfileResponse {
    pub id: String,
    pub name: Option<String>,
    pub email: String,
    pub key: Option<String>,
    pub private_key: Option<String>,
    #[serde(default)]
    pub organizations: Vec<ProfileOrganizationResponse>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct FolderResponse {
    pub id: String,
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CollectionResponse {
    pub id: String,
    pub organization_id: String,
    pub name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CipherResponse {
    pub id: String,
    pub organization_id: Option<String>,
    pub folder_id: Option<String>,
    #[serde(default)]
    pub collection_ids: Vec<String>,
    #[serde(rename = "type")]
    pub cipher_type: u8,
    pub name: String,
    pub notes: Option<String>,
    pub favorite: Option<bool>,
    pub login: Option<CipherLoginResponse>,
    pub fields: Option<Vec<CipherFieldResponse>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct CipherLoginResponse {
    pub username: Option<String>,
    pub password: Option<String>,
    pub totp: Option<String>,
    pub uris: Option<Vec<CipherUriResponse>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CipherUriResponse {
    pub uri: Option<String>,
    pub r#match: Option<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CipherFieldResponse {
    pub name: Option<String>,
    pub value: Option<String>,
    #[serde(rename = "type")]
    pub field_type: Option<u8>,
}
