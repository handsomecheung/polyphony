use chrono::Utc;
use std::fs;
use tacet::crypto::kdf::KdfParams;
use tacet::error::StorageError;
use tacet::storage::paths::StoragePaths;
use tacet::storage::session::SessionMetadata;
use tacet::storage::vault_cache::EncryptedVaultCache;
use tacet::upstream::models::{CipherResponse, SyncResponse};
use tempfile::tempdir;

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

#[test]
fn test_session_lifecycle_and_permissions() {
    let tmp = tempdir().expect("Failed to create tempdir");
    let config_dir = tmp.path().join("config");
    let cache_dir = tmp.path().join("cache");

    let paths = StoragePaths::custom(config_dir.clone(), cache_dir.clone());

    // Initially not exists
    assert!(!SessionMetadata::exists(&paths));
    assert!(matches!(
        SessionMetadata::load(&paths).unwrap_err(),
        StorageError::SessionNotFound
    ));

    let session = SessionMetadata {
        server_url: "https://vault.example.com".to_string(),
        email: "alice@example.com".to_string(),
        access_token: "jwt_token_123".to_string(),
        refresh_token: Some("refresh_abc".to_string()),
        expires_at: Some(Utc::now()),
        kdf_params: KdfParams::pbkdf2(600_000),
        protected_symmetric_key: Some("2.iv|key_ct|mac".to_string()),
        protected_rsa_private_key: None,
        user_id: Some("user-uuid".to_string()),
        last_synced: Some(Utc::now()),
    };

    // Save session
    session.save(&paths).expect("Save failed");
    assert!(SessionMetadata::exists(&paths));

    // Verify Unix permissions
    #[cfg(unix)]
    {
        let dir_meta = fs::metadata(&config_dir).unwrap();
        assert_eq!(dir_meta.permissions().mode() & 0o777, 0o700);

        let file_meta = fs::metadata(paths.session_file()).unwrap();
        assert_eq!(file_meta.permissions().mode() & 0o777, 0o600);
    }

    // Load and verify fields
    let loaded = SessionMetadata::load(&paths).expect("Load failed");
    assert_eq!(loaded.server_url, session.server_url);
    assert_eq!(loaded.email, session.email);
    assert_eq!(loaded.access_token, session.access_token);
    assert_eq!(loaded.refresh_token, session.refresh_token);
    assert_eq!(
        loaded.protected_symmetric_key,
        session.protected_symmetric_key
    );

    // Clear session
    SessionMetadata::clear(&paths).expect("Clear failed");
    assert!(!SessionMetadata::exists(&paths));
}

#[test]
fn test_vault_cache_lifecycle_and_permissions() {
    let tmp = tempdir().expect("Failed to create tempdir");
    let config_dir = tmp.path().join("config");
    let cache_dir = tmp.path().join("cache");

    let paths = StoragePaths::custom(config_dir, cache_dir.clone());

    assert!(!EncryptedVaultCache::exists(&paths));
    assert!(matches!(
        EncryptedVaultCache::load(&paths).unwrap_err(),
        StorageError::VaultCacheNotFound
    ));

    let sync_data = SyncResponse {
        profile: None,
        folders: vec![],
        collections: vec![],
        ciphers: vec![
            CipherResponse {
                id: "c1".to_string(),
                organization_id: None,
                folder_id: None,
                cipher_type: 1,
                name: "2.iv|name1|mac".to_string(),
                notes: None,
                favorite: Some(true),
                login: None,
                fields: None,
            },
            CipherResponse {
                id: "c2".to_string(),
                organization_id: None,
                folder_id: None,
                cipher_type: 1,
                name: "2.iv|name2|mac".to_string(),
                notes: None,
                favorite: None,
                login: None,
                fields: None,
            },
        ],
    };

    let cache = EncryptedVaultCache::new(sync_data);
    assert_eq!(cache.item_count(), 2);

    // Save cache
    cache.save(&paths).expect("Save cache failed");
    assert!(EncryptedVaultCache::exists(&paths));

    // Verify Unix permissions
    #[cfg(unix)]
    {
        let dir_meta = fs::metadata(&cache_dir).unwrap();
        assert_eq!(dir_meta.permissions().mode() & 0o777, 0o700);

        let file_meta = fs::metadata(paths.vault_cache_file()).unwrap();
        assert_eq!(file_meta.permissions().mode() & 0o777, 0o600);
    }

    // Load and verify
    let loaded = EncryptedVaultCache::load(&paths).expect("Load cache failed");
    assert_eq!(loaded.item_count(), 2);
    assert_eq!(loaded.data.ciphers[0].id, "c1");
    assert_eq!(loaded.data.ciphers[0].name, "2.iv|name1|mac");

    // Clear cache
    EncryptedVaultCache::clear(&paths).expect("Clear cache failed");
    assert!(!EncryptedVaultCache::exists(&paths));
}
