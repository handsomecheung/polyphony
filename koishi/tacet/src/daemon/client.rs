use crate::daemon::models::{
    ErrorResponse, ListResponse, LockResponse, LoginRequest, LoginResponse, LogoutResponse,
    ResolveRequest, ResolveResponse, StatusResponse, SyncStatusResponse, UnlockRequest,
    UnlockResponse,
};
use bytes::Bytes;
use http_body_util::{BodyExt, Full};
use hyper::client::conn::http1;
use hyper::{Method, Request, StatusCode};
use hyper_util::rt::TokioIo;
use std::path::{Path, PathBuf};
use thiserror::Error;
use tokio::net::UnixStream;

#[derive(Error, Debug)]
pub enum DaemonClientError {
    #[error("IO error connecting to daemon socket: {0}")]
    Io(#[from] std::io::Error),

    #[error("Hyper error: {0}")]
    Hyper(#[from] hyper::Error),

    #[error("HTTP error: {0}")]
    Http(#[from] hyper::http::Error),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Daemon returned status {status}: {message}")]
    HttpStatus { status: StatusCode, message: String },
}

#[derive(Clone, Debug)]
pub struct DaemonClient {
    socket_path: PathBuf,
}

impl DaemonClient {
    pub fn new<P: AsRef<Path>>(socket_path: P) -> Self {
        Self {
            socket_path: socket_path.as_ref().to_path_buf(),
        }
    }

    pub fn socket_path(&self) -> &Path {
        &self.socket_path
    }

    pub async fn is_alive(&self) -> bool {
        self.status().await.is_ok()
    }

    async fn send_json<Req: serde::Serialize, Resp: serde::de::DeserializeOwned>(
        &self,
        method: Method,
        path: &str,
        body: Option<&Req>,
    ) -> Result<Resp, DaemonClientError> {
        let stream = UnixStream::connect(&self.socket_path).await?;
        let io = TokioIo::new(stream);
        let (mut sender, conn) = http1::handshake(io).await?;

        tokio::spawn(async move {
            if let Err(err) = conn.await {
                tracing::debug!("Client connection closed: {err}");
            }
        });

        let body_bytes = if let Some(b) = body {
            Bytes::from(serde_json::to_vec(b)?)
        } else {
            Bytes::new()
        };

        let req = Request::builder()
            .method(method)
            .uri(path)
            .header("Host", "localhost")
            .header("Content-Type", "application/json")
            .body(Full::new(body_bytes))?;

        let resp = sender.send_request(req).await?;
        let status = resp.status();
        let resp_bytes = resp.into_body().collect().await?.to_bytes();

        if status.is_success() {
            let data: Resp = serde_json::from_slice(&resp_bytes)?;
            Ok(data)
        } else {
            let message = if let Ok(err_resp) = serde_json::from_slice::<ErrorResponse>(&resp_bytes)
            {
                err_resp.error
            } else {
                String::from_utf8_lossy(&resp_bytes).to_string()
            };

            Err(DaemonClientError::HttpStatus { status, message })
        }
    }

    pub async fn status(&self) -> Result<StatusResponse, DaemonClientError> {
        self.send_json::<(), StatusResponse>(Method::GET, "/v1/status", None)
            .await
    }

    pub async fn login(&self, req: &LoginRequest) -> Result<LoginResponse, DaemonClientError> {
        self.send_json(Method::POST, "/v1/login", Some(req)).await
    }

    pub async fn unlock(&self, master_password: &str) -> Result<UnlockResponse, DaemonClientError> {
        let req = UnlockRequest {
            master_password: master_password.to_string(),
        };
        self.send_json(Method::POST, "/v1/unlock", Some(&req)).await
    }

    pub async fn lock(&self) -> Result<LockResponse, DaemonClientError> {
        self.send_json::<(), LockResponse>(Method::POST, "/v1/lock", None)
            .await
    }

    pub async fn logout(&self) -> Result<LogoutResponse, DaemonClientError> {
        self.send_json::<(), LogoutResponse>(Method::POST, "/v1/logout", None)
            .await
    }

    pub async fn sync(&self) -> Result<SyncStatusResponse, DaemonClientError> {
        self.send_json::<(), SyncStatusResponse>(Method::POST, "/v1/sync", None)
            .await
    }

    pub async fn resolve(&self, uris: Vec<String>) -> Result<ResolveResponse, DaemonClientError> {
        let req = ResolveRequest { uris };
        self.send_json(Method::POST, "/v1/resolve", Some(&req))
            .await
    }

    pub async fn list(
        &self,
        scope: Option<&str>,
        search: Option<&str>,
    ) -> Result<ListResponse, DaemonClientError> {
        let mut params = Vec::new();
        if let Some(s) = scope {
            params.push(format!(
                "scope={}",
                percent_encoding::utf8_percent_encode(s, percent_encoding::NON_ALPHANUMERIC)
            ));
        }
        if let Some(q) = search {
            params.push(format!(
                "search={}",
                percent_encoding::utf8_percent_encode(q, percent_encoding::NON_ALPHANUMERIC)
            ));
        }

        let path = if params.is_empty() {
            "/v1/list".to_string()
        } else {
            format!("/v1/list?{}", params.join("&"))
        };

        self.send_json::<(), ListResponse>(Method::GET, &path, None)
            .await
    }
}
