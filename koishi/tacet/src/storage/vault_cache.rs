use crate::error::StorageError;
use crate::storage::paths::StoragePaths;
use crate::upstream::models::SyncResponse;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fs;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EncryptedVaultCache {
    pub updated_at: DateTime<Utc>,
    pub data: SyncResponse,
}

impl EncryptedVaultCache {
    pub fn new(data: SyncResponse) -> Self {
        Self {
            updated_at: Utc::now(),
            data,
        }
    }

    pub fn item_count(&self) -> usize {
        self.data.ciphers.len()
    }

    pub fn save(&self, paths: &StoragePaths) -> Result<(), StorageError> {
        paths.ensure_dirs()?;
        let json = serde_json::to_vec_pretty(self)?;
        StoragePaths::write_secure_file(paths.vault_cache_file(), &json)?;
        Ok(())
    }

    pub fn load(paths: &StoragePaths) -> Result<Self, StorageError> {
        let file = paths.vault_cache_file();
        if !file.exists() {
            return Err(StorageError::VaultCacheNotFound);
        }
        let bytes = fs::read(file)?;
        let cache = serde_json::from_slice(&bytes)?;
        Ok(cache)
    }

    pub fn exists(paths: &StoragePaths) -> bool {
        paths.vault_cache_file().exists()
    }

    pub fn clear(paths: &StoragePaths) -> Result<(), StorageError> {
        let file = paths.vault_cache_file();
        if file.exists() {
            fs::remove_file(file)?;
        }
        Ok(())
    }
}
