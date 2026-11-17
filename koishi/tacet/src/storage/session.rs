use crate::crypto::kdf::KdfParams;
use crate::error::StorageError;
use crate::storage::paths::StoragePaths;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionMetadata {
    pub server_url: String,
    pub email: String,
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub expires_at: Option<DateTime<Utc>>,
    pub kdf_params: KdfParams,
    pub protected_symmetric_key: Option<String>,
    pub protected_rsa_private_key: Option<String>,
    pub user_id: Option<String>,
    pub last_synced: Option<DateTime<Utc>>,
}

impl SessionMetadata {
    pub fn save(&self, paths: &StoragePaths) -> Result<(), StorageError> {
        paths.ensure_dirs()?;
        let json = serde_json::to_vec_pretty(self)?;
        StoragePaths::write_secure_file(paths.session_file(), &json)?;
        Ok(())
    }

    pub fn load(paths: &StoragePaths) -> Result<Self, StorageError> {
        let file = paths.session_file();
        if !file.exists() {
            return Err(StorageError::SessionNotFound);
        }
        let bytes = fs::read(file)?;
        let session = serde_json::from_slice(&bytes)?;
        Ok(session)
    }

    pub fn exists(paths: &StoragePaths) -> bool {
        paths.session_file().exists()
    }

    pub fn clear(paths: &StoragePaths) -> Result<(), StorageError> {
        let file = paths.session_file();
        if file.exists() {
            fs::remove_file(file)?;
        }
        Ok(())
    }
}
