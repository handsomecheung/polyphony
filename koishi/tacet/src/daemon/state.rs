use crate::crypto::symmetric_key::SymmetricKey;
use crate::storage::paths::StoragePaths;
use crate::storage::session::SessionMetadata;
use crate::storage::vault_cache::EncryptedVaultCache;
use crate::upstream::models::SyncResponse;
use crate::upstream::UpstreamClient;
use chrono::Utc;
use rsa::RsaPrivateKey;
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;

#[derive(Clone)]
pub struct DaemonState {
    pub storage_paths: StoragePaths,
    pub idle_ttl: Duration,
    pub session: Arc<RwLock<Option<SessionMetadata>>>,
    pub vault: Arc<RwLock<Option<SyncResponse>>>,
    pub symmetric_key: Arc<RwLock<Option<SymmetricKey>>>,
    pub rsa_private_key: Arc<RwLock<Option<RsaPrivateKey>>>,
    pub last_activity: Arc<AtomicI64>,
    pub upstream_client: Arc<RwLock<Option<UpstreamClient>>>,
}

impl DaemonState {
    pub fn new(storage_paths: StoragePaths, idle_ttl: Duration) -> Self {
        // Attempt to load existing session from disk
        let session = SessionMetadata::load(&storage_paths).ok();
        let upstream_client = session
            .as_ref()
            .and_then(|s| UpstreamClient::new(&s.server_url).ok());

        // Attempt to load cached vault from disk
        let vault = EncryptedVaultCache::load(&storage_paths)
            .map(|cache| cache.data)
            .ok();

        let now_sec = Utc::now().timestamp();

        Self {
            storage_paths,
            idle_ttl,
            session: Arc::new(RwLock::new(session)),
            vault: Arc::new(RwLock::new(vault)),
            symmetric_key: Arc::new(RwLock::new(None)),
            rsa_private_key: Arc::new(RwLock::new(None)),
            last_activity: Arc::new(AtomicI64::new(now_sec)),
            upstream_client: Arc::new(RwLock::new(upstream_client)),
        }
    }

    pub fn touch_activity(&self) {
        let now_sec = Utc::now().timestamp();
        self.last_activity.store(now_sec, Ordering::Relaxed);
    }

    pub async fn lock(&self) {
        // Dropping SymmetricKey automatically invokes ZeroizeOnDrop to wipe key memory
        let mut key_guard = self.symmetric_key.write().await;
        *key_guard = None;

        let mut rsa_guard = self.rsa_private_key.write().await;
        *rsa_guard = None;
    }

    pub async fn is_unlocked(&self) -> bool {
        self.symmetric_key.read().await.is_some()
    }

    pub async fn is_logged_in(&self) -> bool {
        self.session.read().await.is_some()
    }

    pub async fn item_count(&self) -> u32 {
        self.vault
            .read()
            .await
            .as_ref()
            .map(|v| v.ciphers.len() as u32)
            .unwrap_or(0)
    }

    pub async fn server_url(&self) -> Option<String> {
        self.session
            .read()
            .await
            .as_ref()
            .map(|s| s.server_url.clone())
    }

    /// Background task that auto-locks the vault when idle time exceeds idle_ttl.
    pub fn start_auto_lock_task(&self) -> tokio::task::JoinHandle<()> {
        let state = self.clone();
        tokio::spawn(async move {
            let mut interval = tokio::time::interval(Duration::from_secs(5));
            loop {
                interval.tick().await;

                if state.is_unlocked().await {
                    let last_act = state.last_activity.load(Ordering::Relaxed);
                    let now = Utc::now().timestamp();
                    let elapsed = Duration::from_secs((now - last_act).max(0) as u64);

                    if elapsed >= state.idle_ttl {
                        tracing::info!(
                            "Auto-locking vault due to inactivity ({elapsed:?} elapsed)"
                        );
                        state.lock().await;
                    }
                }
            }
        })
    }
}
