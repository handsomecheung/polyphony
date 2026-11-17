use thiserror::Error;

#[derive(Error, Debug)]
pub enum TacetError {
    #[error("Cryptographic error: {0}")]
    Crypto(#[from] CryptoError),

    #[error("Upstream server error: {0}")]
    Upstream(#[from] UpstreamError),

    #[error("Storage error: {0}")]
    Storage(#[from] StorageError),

    #[error("URI error: {0}")]
    Uri(#[from] UriError),

    #[error("Resolver error: {0}")]
    Resolver(#[from] ResolverError),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

#[derive(Error, Debug)]
pub enum CryptoError {
    #[error("Invalid KDF parameter: {0}")]
    InvalidKdfParams(String),

    #[error("Key derivation failed: {0}")]
    KeyDerivationFailed(String),

    #[error("Invalid cipher string format: {0}")]
    InvalidCipherString(String),

    #[error("Base64 decode error: {0}")]
    Base64DecodeError(#[from] base64::DecodeError),

    #[error("HMAC verification failed: data integrity compromised or wrong key")]
    MacVerificationFailed,

    #[error("Decryption failed: {0}")]
    DecryptionFailed(String),

    #[error("Encryption failed: {0}")]
    EncryptionFailed(String),

    #[error("RSA error: {0}")]
    RsaError(#[from] rsa::Error),

    #[error("Invalid key length: expected {expected}, got {got}")]
    InvalidKeyLength { expected: usize, got: usize },

    #[error("UTF-8 decoding error: {0}")]
    Utf8Error(#[from] std::string::FromUtf8Error),
}

#[derive(Error, Debug)]
pub enum UpstreamError {
    #[error("HTTP request failed: {0}")]
    RequestFailed(#[from] reqwest::Error),

    #[error("URL parse error: {0}")]
    UrlParseError(#[from] url::ParseError),

    #[error("Upstream authentication error: {0}")]
    AuthenticationFailed(String),

    #[error("Two-factor authentication required")]
    TwoFactorRequired { providers: Vec<u8> },

    #[error("JSON serialization/deserialization failed: {0}")]
    JsonError(#[from] serde_json::Error),

    #[error("Upstream server returned error status {status}: {message}")]
    HttpStatusError { status: u16, message: String },
}

#[derive(Error, Debug)]
pub enum StorageError {
    #[error("IO error in storage operation: {0}")]
    Io(#[from] std::io::Error),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Failed to determine home or config directory")]
    DirectoryNotFound,

    #[error("Session metadata not found")]
    SessionNotFound,

    #[error("Vault cache not found")]
    VaultCacheNotFound,
}

#[derive(Error, Debug, PartialEq, Eq)]
pub enum UriError {
    #[error("Invalid scheme: URI must start with 'tacet://'")]
    InvalidScheme,

    #[error("Invalid scope '{0}'")]
    InvalidScope(String),

    #[error("Missing URI component: {0}")]
    MissingComponent(&'static str),

    #[error("Invalid modifier: {0}")]
    InvalidModifier(String),
}

#[derive(Error, Debug)]
pub enum ResolverError {
    #[error("Item '{item}' not found in scope '{scope}'")]
    ItemNotFound { item: String, scope: String },

    #[error("Field '{field}' not found in item '{item}'")]
    FieldNotFound { field: String, item: String },

    #[error("Folder '{folder}' not found")]
    FolderNotFound { folder: String },

    #[error("Organization '{org}' not found")]
    OrganizationNotFound { org: String },

    #[error("Collection '{collection}' not found in organization '{org}'")]
    CollectionNotFound { collection: String, org: String },

    #[error("Ambiguous match for '{item}': multiple items matched [{matches}]")]
    AmbiguousMatch { item: String, matches: String },

    #[error("TOTP generation error: {0}")]
    TotpError(String),

    #[error("JSON modifier path error: {0}")]
    JsonPathError(String),

    #[error("Cryptographic operation failed during resolution: {0}")]
    Crypto(#[from] CryptoError),

    #[error("Organization key not available for '{0}'")]
    MissingOrgKey(String),
}
