use crate::crypto::cipher_string::parse_rsa_private_key;
use crate::crypto::kdf::{
    derive_master_key, derive_master_password_hash, stretch_master_key, KdfParams,
};
use crate::crypto::symmetric_key::SymmetricKey;
use crate::daemon::models::{
    ErrorResponse, ListQuery, ListResponse, LockResponse, LoginRequest, LoginResponse,
    LogoutResponse, ResolveRequest, ResolveResponse, StatusResponse, SyncStatusResponse,
    UnlockRequest, UnlockResponse,
};
use crate::daemon::state::DaemonState;
use crate::error::ResolverError;
use crate::resolver::VaultResolver;
use crate::storage::session::SessionMetadata;
use crate::storage::vault_cache::EncryptedVaultCache;
use crate::upstream::models::TwoFactorCredentials;
use crate::upstream::UpstreamClient;
use crate::uri::parser::TacetUri;
use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::routing::{get, post};
use axum::{Json, Router};
use chrono::{Duration, Utc};
use std::collections::HashMap;

pub fn create_router(state: DaemonState) -> Router {
    Router::new()
        .route("/v1/status", get(status_handler))
        .route("/v1/login", post(login_handler))
        .route("/v1/unlock", post(unlock_handler))
        .route("/v1/lock", post(lock_handler))
        .route("/v1/logout", post(logout_handler))
        .route("/v1/sync", post(sync_handler))
        .route("/v1/resolve", post(resolve_handler))
        .route("/v1/list", get(list_handler))
        .with_state(state)
}

async fn status_handler(State(state): State<DaemonState>) -> impl IntoResponse {
    state.touch_activity();

    let logged_in = state.is_logged_in().await;
    let unlocked = state.is_unlocked().await;
    let server_url = state.server_url().await;
    let item_count = state.item_count().await;

    Json(StatusResponse {
        running: true,
        logged_in,
        unlocked,
        server_url,
        item_count,
    })
}

async fn login_handler(
    State(state): State<DaemonState>,
    Json(req): Json<LoginRequest>,
) -> Result<Json<LoginResponse>, (StatusCode, Json<ErrorResponse>)> {
    state.touch_activity();

    // 1. Build upstream client
    let client = UpstreamClient::new(&req.server_url).map_err(|e| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let device_id = state.storage_paths.get_or_create_device_id().map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let is_api_key_login = req.client_id.is_some() && req.client_secret.is_some();

    if is_api_key_login {
        let client_id = req.client_id.as_deref().unwrap();
        let client_secret = req.client_secret.as_deref().unwrap();

        let token_resp = client
            .login_api_key(client_id, client_secret, Some(&device_id))
            .await
            .map_err(|e| {
                (
                    StatusCode::UNAUTHORIZED,
                    Json(ErrorResponse::new(e.to_string())),
                )
            })?;

        // Initial Sync to retrieve profile (email, protected keys) and vault ciphers
        let sync_resp = client.sync(&token_resp.access_token).await.map_err(|e| {
            (
                StatusCode::BAD_GATEWAY,
                Json(ErrorResponse::new(e.to_string())),
            )
        })?;

        let email = match req
            .email
            .as_deref()
            .or_else(|| sync_resp.profile.as_ref().map(|p| p.email.as_str()))
        {
            Some(e) if !e.trim().is_empty() => e.trim().to_string(),
            _ => {
                return Err((
                    StatusCode::BAD_GATEWAY,
                    Json(ErrorResponse::new(
                        "API key authentication succeeded, but could not determine user email from vault profile. Please provide email explicitly.",
                    )),
                ));
            }
        };

        let prelogin = client.prelogin(&email).await.map_err(|e| {
            (
                StatusCode::BAD_GATEWAY,
                Json(ErrorResponse::new(e.to_string())),
            )
        })?;

        let kdf_params = match prelogin.kdf {
            1 => KdfParams::argon2id(
                prelogin.kdf_iterations,
                prelogin.kdf_memory.unwrap_or(64),
                prelogin.kdf_parallelism.unwrap_or(4),
            ),
            _ => KdfParams::pbkdf2(prelogin.kdf_iterations),
        };

        let protected_key = sync_resp.profile.as_ref().and_then(|p| p.key.clone());
        let protected_rsa_private_key = sync_resp
            .profile
            .as_ref()
            .and_then(|p| p.private_key.clone());

        let (sym_key, rsa_priv, unlocked) = if let Some(master_pwd) = &req.master_password {
            if !master_pwd.is_empty() {
                let prot_key_str = protected_key.as_deref().ok_or_else(|| {
                    (
                        StatusCode::BAD_GATEWAY,
                        Json(ErrorResponse::new(
                            "Server did not return account key in profile",
                        )),
                    )
                })?;

                let master_key =
                    derive_master_key(master_pwd, &email, &kdf_params).map_err(|e| {
                        (
                            StatusCode::INTERNAL_SERVER_ERROR,
                            Json(ErrorResponse::new(e.to_string())),
                        )
                    })?;

                let stretched = stretch_master_key(&master_key).map_err(|e| {
                    (
                        StatusCode::INTERNAL_SERVER_ERROR,
                        Json(ErrorResponse::new(e.to_string())),
                    )
                })?;

                let sk =
                    SymmetricKey::from_protected_key(prot_key_str, &stretched).map_err(|_| {
                        (
                            StatusCode::UNAUTHORIZED,
                            Json(ErrorResponse::new("Invalid master password")),
                        )
                    })?;

                let rsa = if let Some(pk_str) = &protected_rsa_private_key {
                    if let Ok(raw_pk) = sk.decrypt_type2(pk_str) {
                        parse_rsa_private_key(&raw_pk).ok()
                    } else {
                        None
                    }
                } else {
                    None
                };

                (Some(sk), rsa, true)
            } else {
                (None, None, false)
            }
        } else {
            (None, None, false)
        };

        let expires_at = Utc::now() + Duration::seconds(token_resp.expires_in as i64);

        let session_meta = SessionMetadata {
            server_url: req.server_url.clone(),
            email: email.clone(),
            access_token: token_resp.access_token.clone(),
            refresh_token: token_resp.refresh_token.clone(),
            expires_at: Some(expires_at),
            kdf_params,
            protected_symmetric_key: protected_key,
            protected_rsa_private_key,
            user_id: sync_resp.profile.as_ref().map(|p| p.id.clone()),
            last_synced: Some(Utc::now()),
        };

        session_meta.save(&state.storage_paths).map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse::new(format!(
                    "Failed to persist session: {e}"
                ))),
            )
        })?;

        let vault_cache = EncryptedVaultCache::new(sync_resp.clone());
        vault_cache.save(&state.storage_paths).map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse::new(format!(
                    "Failed to persist vault cache: {e}"
                ))),
            )
        })?;

        let item_count = sync_resp.ciphers.len() as u32;

        *state.session.write().await = Some(session_meta);
        *state.vault.write().await = Some(sync_resp);
        *state.symmetric_key.write().await = sym_key;
        *state.rsa_private_key.write().await = rsa_priv;
        *state.upstream_client.write().await = Some(client);

        return Ok(Json(LoginResponse {
            unlocked,
            item_count,
        }));
    }

    // Password login flow
    let email = req.email.as_deref().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new("Password login requires 'email'")),
        )
    })?;

    let master_password = req.master_password.as_deref().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new(
                "Password login requires 'master_password'",
            )),
        )
    })?;

    // 2. Prelogin to obtain KDF parameters
    let prelogin = client.prelogin(email).await.map_err(|e| {
        (
            StatusCode::BAD_GATEWAY,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let kdf_params = match prelogin.kdf {
        1 => KdfParams::argon2id(
            prelogin.kdf_iterations,
            prelogin.kdf_memory.unwrap_or(64),
            prelogin.kdf_parallelism.unwrap_or(4),
        ),
        _ => KdfParams::pbkdf2(prelogin.kdf_iterations),
    };

    // 3. Derive keys
    let master_key = derive_master_key(master_password, email, &kdf_params).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let master_password_hash =
        derive_master_password_hash(&master_key, master_password).map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse::new(e.to_string())),
            )
        })?;

    let two_factor = req.two_factor_token.map(|tok| TwoFactorCredentials {
        provider: req.two_factor_provider.unwrap_or(0),
        token: tok,
        remember: true,
    });

    // 4. Authenticate upstream
    let token_resp = client
        .login(email, &master_password_hash, two_factor, Some(&device_id))
        .await
        .map_err(|e| {
            (
                StatusCode::UNAUTHORIZED,
                Json(ErrorResponse::new(e.to_string())),
            )
        })?;

    // 5. Derive symmetric key
    let stretched = stretch_master_key(&master_key).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let protected_key = token_resp.key.as_deref().ok_or_else(|| {
        (
            StatusCode::BAD_GATEWAY,
            Json(ErrorResponse::new("Server did not return account key")),
        )
    })?;

    let sym_key = SymmetricKey::from_protected_key(protected_key, &stretched).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    // Decrypt RSA private key if available
    let rsa_priv = if let Some(pk_str) = &token_resp.private_key {
        if let Ok(raw_pk) = sym_key.decrypt_type2(pk_str) {
            parse_rsa_private_key(&raw_pk).ok()
        } else {
            None
        }
    } else {
        None
    };

    // 6. Initial Sync
    let sync_resp = client.sync(&token_resp.access_token).await.map_err(|e| {
        (
            StatusCode::BAD_GATEWAY,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let expires_at = Utc::now() + Duration::seconds(token_resp.expires_in as i64);

    let session_meta = SessionMetadata {
        server_url: req.server_url.clone(),
        email: email.to_string(),
        access_token: token_resp.access_token.clone(),
        refresh_token: token_resp.refresh_token.clone(),
        expires_at: Some(expires_at),
        kdf_params,
        protected_symmetric_key: token_resp.key.clone(),
        protected_rsa_private_key: token_resp.private_key.clone(),
        user_id: sync_resp.profile.as_ref().map(|p| p.id.clone()),
        last_synced: Some(Utc::now()),
    };

    // 7. Persist to storage
    session_meta.save(&state.storage_paths).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(format!(
                "Failed to persist session: {e}"
            ))),
        )
    })?;

    let vault_cache = EncryptedVaultCache::new(sync_resp.clone());
    vault_cache.save(&state.storage_paths).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(format!(
                "Failed to persist vault cache: {e}"
            ))),
        )
    })?;

    let item_count = sync_resp.ciphers.len() as u32;

    // 8. Update in-memory state
    *state.session.write().await = Some(session_meta);
    *state.vault.write().await = Some(sync_resp);
    *state.symmetric_key.write().await = Some(sym_key);
    *state.rsa_private_key.write().await = rsa_priv;
    *state.upstream_client.write().await = Some(client);

    Ok(Json(LoginResponse {
        unlocked: true,
        item_count,
    }))
}

async fn unlock_handler(
    State(state): State<DaemonState>,
    Json(req): Json<UnlockRequest>,
) -> Result<Json<UnlockResponse>, (StatusCode, Json<ErrorResponse>)> {
    state.touch_activity();

    // 1. Ensure logged in (from memory or disk)
    let session = {
        let guard = state.session.read().await;
        if let Some(s) = guard.as_ref() {
            s.clone()
        } else {
            SessionMetadata::load(&state.storage_paths).map_err(|_| {
                (
                    StatusCode::BAD_REQUEST,
                    Json(ErrorResponse::new(
                        "Not logged in. Run 'tacet login' first.",
                    )),
                )
            })?
        }
    };

    let protected_key = session.protected_symmetric_key.as_deref().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new("Session metadata missing protected key")),
        )
    })?;

    // 2. Derive master key from cached KDF parameters
    let master_key = derive_master_key(&req.master_password, &session.email, &session.kdf_params)
        .map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    let stretched = stretch_master_key(&master_key).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorResponse::new(e.to_string())),
        )
    })?;

    // 3. Decrypt protected symmetric key (constant-time verification)
    let sym_key = SymmetricKey::from_protected_key(protected_key, &stretched).map_err(|_| {
        (
            StatusCode::UNAUTHORIZED,
            Json(ErrorResponse::new("Invalid master password")),
        )
    })?;

    // Decrypt RSA private key if present
    let rsa_priv = if let Some(pk_str) = &session.protected_rsa_private_key {
        if let Ok(raw_pk) = sym_key.decrypt_type2(pk_str) {
            parse_rsa_private_key(&raw_pk).ok()
        } else {
            None
        }
    } else {
        None
    };

    // Ensure vault cache is loaded
    {
        let mut vault_guard = state.vault.write().await;
        if vault_guard.is_none() {
            if let Ok(cache) = EncryptedVaultCache::load(&state.storage_paths) {
                *vault_guard = Some(cache.data);
            }
        }
    }

    *state.session.write().await = Some(session);
    *state.symmetric_key.write().await = Some(sym_key);
    *state.rsa_private_key.write().await = rsa_priv;

    Ok(Json(UnlockResponse { unlocked: true }))
}

async fn lock_handler(State(state): State<DaemonState>) -> impl IntoResponse {
    state.lock().await;
    Json(LockResponse { locked: true })
}

async fn logout_handler(State(state): State<DaemonState>) -> impl IntoResponse {
    state.lock().await;

    *state.session.write().await = None;
    *state.vault.write().await = None;
    *state.upstream_client.write().await = None;

    let _ = SessionMetadata::clear(&state.storage_paths);
    let _ = EncryptedVaultCache::clear(&state.storage_paths);

    Json(LogoutResponse { logged_out: true })
}

async fn sync_handler(
    State(state): State<DaemonState>,
) -> Result<Json<SyncStatusResponse>, (StatusCode, Json<ErrorResponse>)> {
    state.touch_activity();

    // 1. Get session
    let mut session = {
        let guard = state.session.read().await;
        if let Some(s) = guard.as_ref() {
            s.clone()
        } else {
            SessionMetadata::load(&state.storage_paths).map_err(|_| {
                (
                    StatusCode::BAD_REQUEST,
                    Json(ErrorResponse::new("Not logged in")),
                )
            })?
        }
    };

    let client = {
        let mut client_guard = state.upstream_client.write().await;
        if client_guard.is_none() {
            let c = UpstreamClient::new(&session.server_url).map_err(|e| {
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Json(ErrorResponse::new(e.to_string())),
                )
            })?;
            *client_guard = Some(c.clone());
            c
        } else {
            client_guard.as_ref().unwrap().clone()
        }
    };

    // 2. Check if token needs refresh
    let now = Utc::now();
    let is_expired = session
        .expires_at
        .map(|exp| exp <= now + Duration::seconds(60))
        .unwrap_or(false);

    if is_expired && session.refresh_token.is_some() {
        let refresh_tok = session.refresh_token.as_ref().unwrap();
        if let Ok(token_resp) = client.refresh_token(refresh_tok).await {
            session.access_token = token_resp.access_token;
            if let Some(rt) = token_resp.refresh_token {
                session.refresh_token = Some(rt);
            }
            session.expires_at = Some(now + Duration::seconds(token_resp.expires_in as i64));
            let _ = session.save(&state.storage_paths);
        }
    }

    // 3. Fetch sync
    let sync_resp = client.sync(&session.access_token).await.map_err(|e| {
        (
            StatusCode::BAD_GATEWAY,
            Json(ErrorResponse::new(format!("Sync failed: {e}"))),
        )
    })?;

    let item_count = sync_resp.ciphers.len() as u32;

    // 4. Persist cache and update memory
    let cache = EncryptedVaultCache::new(sync_resp.clone());
    let _ = cache.save(&state.storage_paths);

    session.last_synced = Some(Utc::now());
    let _ = session.save(&state.storage_paths);

    *state.session.write().await = Some(session);
    *state.vault.write().await = Some(sync_resp);

    Ok(Json(SyncStatusResponse {
        synced: true,
        item_count,
    }))
}

async fn resolve_handler(
    State(state): State<DaemonState>,
    Json(req): Json<ResolveRequest>,
) -> Result<Json<ResolveResponse>, (StatusCode, Json<ErrorResponse>)> {
    state.touch_activity();

    // 1. Vault must be unlocked
    let sym_key_guard = state.symmetric_key.read().await;
    let sym_key = sym_key_guard.as_ref().ok_or_else(|| {
        (
            StatusCode::UNAUTHORIZED,
            Json(ErrorResponse::new(
                "Vault is locked. Run 'tacet unlock' first.",
            )),
        )
    })?;

    // 2. Vault data must be present
    let vault_guard = state.vault.read().await;
    let vault = vault_guard.as_ref().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new("Vault cache is empty. Please run sync.")),
        )
    })?;

    let rsa_guard = state.rsa_private_key.read().await;
    let resolver = VaultResolver::new(vault, sym_key, rsa_guard.as_ref());

    let mut secrets = HashMap::new();
    for uri_str in &req.uris {
        let parsed_uri = TacetUri::parse(uri_str).map_err(|e| {
            (
                StatusCode::BAD_REQUEST,
                Json(ErrorResponse::new(format!("Invalid URI '{uri_str}': {e}"))),
            )
        })?;

        match resolver.resolve(&parsed_uri) {
            Ok(value) => {
                secrets.insert(uri_str.clone(), value);
            }
            Err(ResolverError::ItemNotFound { .. })
            | Err(ResolverError::FieldNotFound { .. })
            | Err(ResolverError::FolderNotFound { .. })
            | Err(ResolverError::CollectionNotFound { .. })
            | Err(ResolverError::OrganizationNotFound { .. }) => {
                return Err((
                    StatusCode::NOT_FOUND,
                    Json(ErrorResponse::new(format!(
                        "Secret resolution failed for '{uri_str}': {e}",
                        e = ResolverError::ItemNotFound {
                            item: parsed_uri.item,
                            scope: parsed_uri.scope.to_string(),
                        }
                    ))),
                ));
            }
            Err(err @ ResolverError::AmbiguousMatch { .. }) => {
                return Err((
                    StatusCode::CONFLICT,
                    Json(ErrorResponse::new(err.to_string())),
                ));
            }
            Err(other) => {
                return Err((
                    StatusCode::INTERNAL_SERVER_ERROR,
                    Json(ErrorResponse::new(other.to_string())),
                ));
            }
        }
    }

    Ok(Json(ResolveResponse { secrets }))
}

async fn list_handler(
    State(state): State<DaemonState>,
    Query(query): Query<ListQuery>,
) -> Result<Json<ListResponse>, (StatusCode, Json<ErrorResponse>)> {
    state.touch_activity();

    // 1. Vault must be unlocked
    let sym_key_guard = state.symmetric_key.read().await;
    let sym_key = sym_key_guard.as_ref().ok_or_else(|| {
        (
            StatusCode::UNAUTHORIZED,
            Json(ErrorResponse::new(
                "Vault is locked. Run 'tacet unlock' first.",
            )),
        )
    })?;

    // 2. Vault data must be present
    let vault_guard = state.vault.read().await;
    let vault = vault_guard.as_ref().ok_or_else(|| {
        (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new("Vault cache is empty. Please run sync.")),
        )
    })?;

    let rsa_guard = state.rsa_private_key.read().await;
    let resolver = VaultResolver::new(vault, sym_key, rsa_guard.as_ref());

    let items = resolver
        .list_items(query.scope.as_deref(), query.search.as_deref())
        .map_err(|e| {
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse::new(format!("Failed to list items: {e}"))),
            )
        })?;

    let total = items.len();
    Ok(Json(ListResponse { items, total }))
}
