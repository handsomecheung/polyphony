use serde_json::json;
use tacet::error::UpstreamError;
use tacet::upstream::models::TwoFactorCredentials;
use tacet::upstream::urls::BitwardenEndpoints;
use tacet::upstream::UpstreamClient;
use wiremock::matchers::{bearer_token, header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

#[test]
fn test_url_resolution() {
    // 1. Bitwarden Cloud US
    let ep = BitwardenEndpoints::resolve("https://vault.bitwarden.com").unwrap();
    assert_eq!(ep.identity_url.as_str(), "https://identity.bitwarden.com/");
    assert_eq!(ep.api_url.as_str(), "https://api.bitwarden.com/");
    assert_eq!(
        ep.prelogin_url().unwrap().as_str(),
        "https://identity.bitwarden.com/accounts/prelogin"
    );
    assert_eq!(
        ep.token_url().unwrap().as_str(),
        "https://identity.bitwarden.com/connect/token"
    );
    assert_eq!(
        ep.sync_url().unwrap().as_str(),
        "https://api.bitwarden.com/sync"
    );

    // Bare domain
    let ep_bare = BitwardenEndpoints::resolve("bitwarden.com").unwrap();
    assert_eq!(
        ep_bare.identity_url.as_str(),
        "https://identity.bitwarden.com/"
    );

    // 2. Bitwarden Cloud EU
    let ep_eu = BitwardenEndpoints::resolve("https://vault.bitwarden.eu").unwrap();
    assert_eq!(
        ep_eu.identity_url.as_str(),
        "https://identity.bitwarden.eu/"
    );
    assert_eq!(ep_eu.api_url.as_str(), "https://api.bitwarden.eu/");

    // 3. Custom Vaultwarden / Self-hosted instance
    let ep_custom = BitwardenEndpoints::resolve("https://vault.home.local").unwrap();
    assert_eq!(
        ep_custom.identity_url.as_str(),
        "https://vault.home.local/identity"
    );
    assert_eq!(ep_custom.api_url.as_str(), "https://vault.home.local/api");
    assert_eq!(
        ep_custom.prelogin_url().unwrap().as_str(),
        "https://vault.home.local/identity/accounts/prelogin"
    );
    assert_eq!(
        ep_custom.token_url().unwrap().as_str(),
        "https://vault.home.local/identity/connect/token"
    );
    assert_eq!(
        ep_custom.sync_url().unwrap().as_str(),
        "https://vault.home.local/api/sync"
    );

    // Custom instance with trailing slash or /api subpath
    let ep_custom_subpath = BitwardenEndpoints::resolve("https://vault.home.local/api/").unwrap();
    assert_eq!(
        ep_custom_subpath.identity_url.as_str(),
        "https://vault.home.local/identity"
    );
    assert_eq!(
        ep_custom_subpath.api_url.as_str(),
        "https://vault.home.local/api"
    );
}

#[tokio::test]
async fn test_upstream_prelogin() {
    let mock_server = MockServer::start().await;

    let response_body = json!({
        "kdf": 0,
        "kdfIterations": 600000,
        "kdfMemory": null,
        "kdfParallelism": null
    });

    Mock::given(method("POST"))
        .and(path("/identity/accounts/prelogin"))
        .and(header("content-type", "application/json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(response_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let prelogin = client
        .prelogin("user@example.com")
        .await
        .expect("Prelogin failed");

    assert_eq!(prelogin.kdf, 0);
    assert_eq!(prelogin.kdf_iterations, 600000);
}

#[tokio::test]
async fn test_upstream_login_success() {
    let mock_server = MockServer::start().await;

    let response_body = json!({
        "access_token": "mock_jwt_access_token_xyz",
        "expires_in": 3600,
        "token_type": "Bearer",
        "refresh_token": "mock_refresh_token_abc",
        "Key": "2.ivvvvvvvvvvvvvvvv|ctttttttttttttttt|maccccccccccccccc",
        "PrivateKey": null,
        "Kdf": 0,
        "KdfIterations": 600000
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(response_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let token_resp = client
        .login("user@example.com", "hashed_pw", None, None)
        .await
        .expect("Login failed");

    assert_eq!(token_resp.access_token, "mock_jwt_access_token_xyz");
    assert_eq!(token_resp.expires_in, 3600);
    assert_eq!(
        token_resp.refresh_token.as_deref(),
        Some("mock_refresh_token_abc")
    );
    assert_eq!(
        token_resp.key.as_deref(),
        Some("2.ivvvvvvvvvvvvvvvv|ctttttttttttttttt|maccccccccccccccc")
    );
}

#[tokio::test]
async fn test_upstream_login_2fa_required() {
    let mock_server = MockServer::start().await;

    let error_body = json!({
        "error": "invalid_grant",
        "error_description": "Two factor required.",
        "TwoFactorProviders": {
            "0": null
        }
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(400).set_body_json(error_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let err = client
        .login("user@example.com", "hashed_pw", None, None)
        .await
        .unwrap_err();

    match err {
        UpstreamError::TwoFactorRequired { providers } => {
            assert_eq!(providers, vec![0]);
        }
        other => panic!("Expected TwoFactorRequired, got {other:?}"),
    }
}

#[tokio::test]
async fn test_upstream_login_with_2fa_token() {
    let mock_server = MockServer::start().await;

    let response_body = json!({
        "access_token": "2fa_authenticated_token",
        "expires_in": 3600,
        "token_type": "Bearer"
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(response_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let tf = TwoFactorCredentials {
        provider: 0,
        token: "123456".to_string(),
        remember: true,
    };

    let token_resp = client
        .login("user@example.com", "hashed_pw", Some(tf), None)
        .await
        .expect("Login with 2FA failed");

    assert_eq!(token_resp.access_token, "2fa_authenticated_token");
}

#[tokio::test]
async fn test_upstream_refresh_token() {
    let mock_server = MockServer::start().await;

    let response_body = json!({
        "access_token": "new_refreshed_access_token",
        "expires_in": 3600,
        "token_type": "Bearer",
        "refresh_token": "new_refreshed_refresh_token"
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(response_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let token_resp = client
        .refresh_token("old_refresh_token")
        .await
        .expect("Refresh failed");

    assert_eq!(token_resp.access_token, "new_refreshed_access_token");
    assert_eq!(
        token_resp.refresh_token.as_deref(),
        Some("new_refreshed_refresh_token")
    );
}

#[tokio::test]
async fn test_upstream_sync() {
    let mock_server = MockServer::start().await;

    let sync_body = json!({
        "profile": {
            "id": "user-uuid-1",
            "name": "Alice",
            "email": "alice@example.com",
            "key": "2.symmkey",
            "privateKey": null
        },
        "folders": [
            { "id": "folder-1", "name": "2.iv|folder1_ct|mac" }
        ],
        "collections": [
            { "id": "col-1", "organizationId": "org-1", "name": "2.iv|col1_ct|mac" }
        ],
        "ciphers": [
            {
                "id": "cipher-1",
                "organizationId": null,
                "folderId": "folder-1",
                "type": 1,
                "name": "2.iv|github_ct|mac",
                "notes": null,
                "favorite": true,
                "login": {
                    "username": "2.iv|alice_user_ct|mac",
                    "password": "2.iv|secret_pwd_ct|mac",
                    "totp": "2.iv|otpauth_seed_ct|mac",
                    "uris": [
                        { "uri": "2.iv|https_github_com_ct|mac", "match": null }
                    ]
                },
                "fields": [
                    { "name": "2.iv|api_key_ct|mac", "value": "2.iv|secret_val_ct|mac", "type": 0 }
                ]
            }
        ]
    });

    Mock::given(method("GET"))
        .and(path("/api/sync"))
        .and(bearer_token("valid_access_token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(sync_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let sync_resp = client
        .sync("valid_access_token")
        .await
        .expect("Sync failed");

    assert_eq!(
        sync_resp.profile.as_ref().unwrap().email,
        "alice@example.com"
    );
    assert_eq!(sync_resp.folders.len(), 1);
    assert_eq!(sync_resp.collections.len(), 1);
    assert_eq!(sync_resp.ciphers.len(), 1);

    let cipher = &sync_resp.ciphers[0];
    assert_eq!(cipher.id, "cipher-1");
    assert_eq!(cipher.name, "2.iv|github_ct|mac");
    assert_eq!(
        cipher.login.as_ref().unwrap().password.as_deref(),
        Some("2.iv|secret_pwd_ct|mac")
    );
}

#[tokio::test]
async fn test_upstream_http_status_error() {
    let mock_server = MockServer::start().await;

    Mock::given(method("GET"))
        .and(path("/api/sync"))
        .respond_with(ResponseTemplate::new(401).set_body_string("Unauthorized"))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let err = client.sync("bad_token").await.unwrap_err();

    match err {
        UpstreamError::HttpStatusError { status, message } => {
            assert_eq!(status, 401);
            assert!(message.contains("Unauthorized"));
        }
        other => panic!("Expected HttpStatusError, got {other:?}"),
    }
}

#[tokio::test]
async fn test_upstream_login_api_key_success() {
    let mock_server = MockServer::start().await;

    let token_body = json!({
        "access_token": "api_key_oauth_access_token",
        "expires_in": 3600,
        "token_type": "Bearer"
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(token_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    let token_resp = client
        .login_api_key(
            "user.11111111-2222-3333-4444-555555555555",
            "super_secret_client_key_123",
            Some("device-id-test"),
        )
        .await
        .expect("API key login failed");

    assert_eq!(token_resp.access_token, "api_key_oauth_access_token");
    assert_eq!(token_resp.expires_in, 3600);
    assert_eq!(token_resp.token_type, "Bearer");
}

#[tokio::test]
async fn test_upstream_login_api_key_normalization() {
    let mock_server = MockServer::start().await;

    let token_body = json!({
        "access_token": "normalized_token",
        "expires_in": 3600,
        "token_type": "Bearer"
    });

    Mock::given(method("POST"))
        .and(path("/identity/connect/token"))
        .respond_with(ResponseTemplate::new(200).set_body_json(token_body))
        .mount(&mock_server)
        .await;

    let client = UpstreamClient::new(&mock_server.uri()).unwrap();
    // Test raw UUID without user. prefix and with surrounding spaces and quotes
    let token_resp = client
        .login_api_key(
            "  \"43b602af-98fe-4320-b446-4fa469ea6d04\"  ",
            "  client_secret_xyz  ",
            Some("device-id-test"),
        )
        .await
        .expect("Normalized API key login failed");

    assert_eq!(token_resp.access_token, "normalized_token");
}
