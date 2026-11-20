use serde_json::json;
use std::os::unix::fs::PermissionsExt;
use std::time::Duration;
use tacet::crypto::cipher_string::CipherString;
use tacet::crypto::kdf::{derive_master_key, stretch_master_key, KdfParams};
use tacet::crypto::symmetric_key::SymmetricKey;
use tacet::daemon::{bind_unix_socket, run_server, DaemonClient, DaemonState, LoginRequest};
use tacet::storage::StoragePaths;
use tempfile::tempdir;
use tokio::sync::broadcast;
use wiremock::matchers::{method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[tokio::test]
async fn test_socket_creation_and_permissions() {
    let tmp = tempdir().unwrap();
    let socket_path = tmp.path().join("tacet_test").join("tacetd.sock");

    let listener = bind_unix_socket(&socket_path).await.unwrap();

    // Check directory mode 0700
    let dir_meta = std::fs::metadata(socket_path.parent().unwrap()).unwrap();
    assert_eq!(dir_meta.permissions().mode() & 0o777, 0o700);

    // Check socket file mode 0600
    let sock_meta = std::fs::metadata(&socket_path).unwrap();
    assert_eq!(sock_meta.permissions().mode() & 0o777, 0o600);

    // Attempting to bind another listener to the active socket must fail with AddrInUse
    let second_bind = bind_unix_socket(&socket_path).await;
    assert!(second_bind.is_err());
    assert_eq!(
        second_bind.unwrap_err().kind(),
        std::io::ErrorKind::AddrInUse
    );

    drop(listener);
}

#[tokio::test]
async fn test_stale_socket_cleanup() {
    let tmp = tempdir().unwrap();
    let socket_path = tmp.path().join("stale.sock");

    // Create a dead socket / dummy file
    std::fs::write(&socket_path, b"stale socket").unwrap();
    assert!(socket_path.exists());

    // bind_unix_socket should detect it's not live, remove it, and bind successfully
    let listener = bind_unix_socket(&socket_path).await.unwrap();
    assert!(socket_path.exists());
    drop(listener);
}

#[tokio::test]
async fn test_daemon_full_rest_lifecycle() {
    let tmp = tempdir().unwrap();
    let socket_path = tmp.path().join("tacetd.sock");
    let config_dir = tmp.path().join("config");
    let cache_dir = tmp.path().join("cache");
    let storage_paths = StoragePaths::custom(config_dir, cache_dir);

    // Setup mock Bitwarden server
    let mock_server = MockServer::start().await;

    // 1. Mock Prelogin
    Mock::given(method("POST"))
        .and(path("/identity/accounts/prelogin"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "kdf": 0,
            "kdfIterations": 100000,
            "kdfMemory": null,
            "kdfParallelism": null
        })))
        .mount(&mock_server)
        .await;

    // Derive test keys to build valid Token and Sync responses
    let email = "user@example.com";
    let password = "SuperMasterPassword123!";
    let kdf_params = KdfParams::pbkdf2(100000);
    let master_key = derive_master_key(password, email, &kdf_params).unwrap();
    let stretched = stretch_master_key(&master_key).unwrap();

    let user_sym_key = SymmetricKey::new([42u8; 64]);
    // Encrypt the user_sym_key with stretched master key for token response
    let mut str_enc = [0u8; 32];
    let mut str_mac = [0u8; 32];
    str_enc.copy_from_slice(&stretched[..32]);
    str_mac.copy_from_slice(&stretched[32..]);
    let server_key_type2 =
        CipherString::encrypt_type2(&str_enc, &str_mac, user_sym_key.as_bytes()).unwrap();

    // 2. Mock Token
    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "access_token": "valid_oauth_access_token",
            "expires_in": 3600,
            "token_type": "Bearer",
            "refresh_token": "valid_refresh_token",
            "Key": server_key_type2,
            "PrivateKey": null,
            "Kdf": 0,
            "KdfIterations": 100000
        })))
        .mount(&mock_server)
        .await;

    // 3. Mock Sync
    let enc_cipher_name = user_sym_key
        .encrypt_type2_string("Production API Key")
        .unwrap();
    let enc_cipher_pass = user_sym_key
        .encrypt_type2_string("sk-live-secret-42")
        .unwrap();
    let enc_cipher_totp = user_sym_key
        .encrypt_type2_string("JBSWY3DPEHPK3PXP")
        .unwrap();

    Mock::given(method("GET"))
        .and(path("/api/sync"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "profile": {
                "id": "user-uuid",
                "email": email,
                "key": null,
                "privateKey": null,
                "organizations": []
            },
            "folders": [],
            "collections": [],
            "ciphers": [
                {
                    "id": "11111111-1111-1111-1111-111111111111",
                    "organizationId": null,
                    "folderId": null,
                    "type": 1,
                    "name": enc_cipher_name,
                    "notes": null,
                    "login": {
                        "username": null,
                        "password": enc_cipher_pass,
                        "totp": enc_cipher_totp,
                        "uris": null
                    },
                    "fields": null
                }
            ]
        })))
        .mount(&mock_server)
        .await;

    // Start daemon server in background
    let listener = bind_unix_socket(&socket_path).await.unwrap();
    let state = DaemonState::new(storage_paths.clone(), Duration::from_secs(1800));
    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);

    let server_handle = tokio::spawn(async move {
        run_server(listener, state, shutdown_rx).await.unwrap();
    });

    let client = DaemonClient::new(&socket_path);

    // A. Initial status: running, but not logged in and not unlocked
    let status = client.status().await.expect("Status failed");
    assert!(status.running);
    assert!(!status.logged_in);
    assert!(!status.unlocked);
    assert_eq!(status.item_count, 0);

    // B. Attempting resolve before login/unlock returns 401 Unauthorized
    let resolve_unauth = client
        .resolve(vec!["tacet://personal/Production API Key/password".into()])
        .await;
    assert!(resolve_unauth.is_err());

    // C. Login
    let login_req = LoginRequest::password(mock_server.uri(), email, password);
    let login_resp = client.login(&login_req).await.expect("Login failed");
    assert!(login_resp.unlocked);
    assert_eq!(login_resp.item_count, 1);

    // Verify status after login
    let status_logged_in = client.status().await.unwrap();
    assert!(status_logged_in.logged_in);
    assert!(status_logged_in.unlocked);
    assert_eq!(status_logged_in.item_count, 1);

    // D. Resolve secret while unlocked
    let resolve_resp = client
        .resolve(vec![
            "tacet://personal/Production API Key/password".into(),
            "tacet://personal/Production API Key/totp".into(),
        ])
        .await
        .expect("Resolve failed");

    assert_eq!(
        resolve_resp
            .secrets
            .get("tacet://personal/Production API Key/password")
            .unwrap(),
        "sk-live-secret-42"
    );
    let totp_code = resolve_resp
        .secrets
        .get("tacet://personal/Production API Key/totp")
        .unwrap();
    assert_eq!(totp_code.len(), 6);

    // E. Lock daemon
    let lock_resp = client.lock().await.expect("Lock failed");
    assert!(lock_resp.locked);

    let status_locked = client.status().await.unwrap();
    assert!(status_locked.logged_in);
    assert!(!status_locked.unlocked);

    // Resolve now fails with 401
    assert!(client
        .resolve(vec!["tacet://personal/Production API Key/password".into()])
        .await
        .is_err());

    // F. Unlock with wrong password fails
    let bad_unlock = client.unlock("WrongPassword!").await;
    assert!(bad_unlock.is_err());

    // Unlock with correct password succeeds
    let unlock_resp = client.unlock(password).await.expect("Unlock failed");
    assert!(unlock_resp.unlocked);

    // Resolve succeeds again
    let resolve_after_unlock = client
        .resolve(vec!["tacet://personal/Production API Key/password".into()])
        .await
        .unwrap();
    assert_eq!(
        resolve_after_unlock
            .secrets
            .get("tacet://personal/Production API Key/password")
            .unwrap(),
        "sk-live-secret-42"
    );

    // G. Sync
    let sync_resp = client.sync().await.expect("Sync failed");
    assert!(sync_resp.synced);
    assert_eq!(sync_resp.item_count, 1);

    // Test list endpoint
    let list_resp = client.list(None, None).await.expect("List failed");
    assert_eq!(list_resp.total, 1);
    assert_eq!(list_resp.items[0].name, "Production API Key");
    assert_eq!(list_resp.items[0].cipher_type, "Login");

    // H. Logout
    let logout_resp = client.logout().await.expect("Logout failed");
    assert!(logout_resp.logged_out);

    let status_after_logout = client.status().await.unwrap();
    assert!(!status_after_logout.logged_in);
    assert!(!status_after_logout.unlocked);

    // Session file on disk must be gone
    assert!(!storage_paths.session_file().exists());

    // Stop daemon
    let _ = shutdown_tx.send(());
    let _ = server_handle.await;
}

#[tokio::test]
async fn test_auto_lock_timer() {
    let tmp = tempdir().unwrap();
    let socket_path = tmp.path().join("autolock.sock");
    let storage_paths = StoragePaths::custom(tmp.path().join("cfg"), tmp.path().join("cache"));

    let listener = bind_unix_socket(&socket_path).await.unwrap();
    // Configure idle_ttl of 1 second
    let state = DaemonState::new(storage_paths, Duration::from_secs(1));

    // Manually set symmetric key (simulating unlocked state)
    let key = SymmetricKey::new([99u8; 64]);
    *state.symmetric_key.write().await = Some(key);
    assert!(state.is_unlocked().await);

    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);
    let server_handle = tokio::spawn(async move {
        run_server(listener, state, shutdown_rx).await.unwrap();
    });

    let client = DaemonClient::new(&socket_path);

    // Status shows unlocked initially
    let status_init = client.status().await.unwrap();
    assert!(status_init.unlocked);

    // Wait 6 seconds for the background auto-lock check (which checks every 5s)
    tokio::time::sleep(Duration::from_millis(6000)).await;

    let _ = shutdown_tx.send(());
    let _ = server_handle.await;
}

#[tokio::test]
async fn test_daemon_api_key_login_and_unlock() {
    let tmp = tempdir().unwrap();
    let socket_path = tmp.path().join("tacetd_apikey.sock");
    let config_dir = tmp.path().join("config");
    let cache_dir = tmp.path().join("cache");
    let storage_paths = StoragePaths::custom(config_dir, cache_dir);

    // Setup mock Bitwarden server
    let mock_server = MockServer::start().await;

    let email = "apikey_user@example.com";
    let password = "SuperSecretMasterPassword123!";
    let kdf_params = KdfParams::pbkdf2(100000);
    let master_key = derive_master_key(password, email, &kdf_params).unwrap();
    let stretched = stretch_master_key(&master_key).unwrap();

    let user_sym_key = SymmetricKey::new([88u8; 64]);
    let mut str_enc = [0u8; 32];
    let mut str_mac = [0u8; 32];
    str_enc.copy_from_slice(&stretched[..32]);
    str_mac.copy_from_slice(&stretched[32..]);
    let server_key_type2 =
        CipherString::encrypt_type2(&str_enc, &str_mac, user_sym_key.as_bytes()).unwrap();

    // 1. Mock Prelogin (called during API key login or unlock)
    Mock::given(method("POST"))
        .and(path("/identity/accounts/prelogin"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "kdf": 0,
            "kdfIterations": 100000,
            "kdfMemory": null,
            "kdfParallelism": null
        })))
        .mount(&mock_server)
        .await;

    // 2. Mock Token for client_credentials
    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "access_token": "apikey_access_token_123",
            "expires_in": 3600,
            "token_type": "Bearer"
        })))
        .mount(&mock_server)
        .await;

    // 3. Mock Sync returning profile with key
    let enc_cipher_name = user_sym_key.encrypt_type2_string("Stripe API").unwrap();
    let enc_cipher_pass = user_sym_key
        .encrypt_type2_string("sk_live_stripe_999")
        .unwrap();
    let enc_cipher_totp = user_sym_key
        .encrypt_type2_string("JBSWY3DPEHPK3PXP")
        .unwrap();

    Mock::given(method("GET"))
        .and(path("/api/sync"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "profile": {
                "id": "user-uuid-apikey",
                "email": email,
                "key": server_key_type2,
                "privateKey": null,
                "organizations": []
            },
            "folders": [],
            "collections": [],
            "ciphers": [
                {
                    "id": "22222222-2222-2222-2222-222222222222",
                    "organizationId": null,
                    "folderId": null,
                    "type": 1,
                    "name": enc_cipher_name,
                    "notes": null,
                    "login": {
                        "username": null,
                        "password": enc_cipher_pass,
                        "totp": enc_cipher_totp,
                        "uris": null
                    },
                    "fields": null
                }
            ]
        })))
        .mount(&mock_server)
        .await;

    // Start daemon
    let listener = bind_unix_socket(&socket_path).await.unwrap();
    let state = DaemonState::new(storage_paths.clone(), Duration::from_secs(1800));
    let (shutdown_tx, shutdown_rx) = broadcast::channel(1);

    let server_handle = tokio::spawn(async move {
        run_server(listener, state, shutdown_rx).await.unwrap();
    });

    let client = DaemonClient::new(&socket_path);

    // Step A: Login with API key without password (locked)
    let login_req = LoginRequest::api_key(
        mock_server.uri(),
        "user.22222222-3333-4444-5555-666666666666",
        "secret_client_key_999",
        None,
        None,
    );
    let login_resp = client
        .login(&login_req)
        .await
        .expect("API key login failed");
    assert!(!login_resp.unlocked);
    assert_eq!(login_resp.item_count, 1);

    let status = client.status().await.unwrap();
    assert!(status.logged_in);
    assert!(!status.unlocked);
    assert_eq!(status.item_count, 1);

    // Resolving secrets should fail while locked
    let resolve_res = client
        .resolve(vec!["tacet://personal/Stripe API/password".into()])
        .await;
    assert!(resolve_res.is_err());

    // Step B: Unlock using Master Password
    let unlock_resp = client.unlock(password).await.expect("Unlock failed");
    assert!(unlock_resp.unlocked);

    let status_unlocked = client.status().await.unwrap();
    assert!(status_unlocked.unlocked);

    // Resolving secrets should now succeed
    let resolve_success = client
        .resolve(vec!["tacet://personal/Stripe API/password".into()])
        .await
        .expect("Resolve after unlock failed");
    assert_eq!(
        resolve_success
            .secrets
            .get("tacet://personal/Stripe API/password")
            .unwrap(),
        "sk_live_stripe_999"
    );

    let _ = shutdown_tx.send(());
    let _ = server_handle.await;
}
