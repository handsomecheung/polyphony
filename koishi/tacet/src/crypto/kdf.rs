use crate::error::CryptoError;
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use base64::Engine;
use sha2::{Digest, Sha256};
use zeroize::Zeroizing;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum KdfType {
    #[default]
    Pbkdf2 = 0,
    Argon2id = 1,
}

impl From<u8> for KdfType {
    fn from(value: u8) -> Self {
        match value {
            1 => Self::Argon2id,
            _ => Self::Pbkdf2,
        }
    }
}

impl From<KdfType> for u8 {
    fn from(kdf: KdfType) -> Self {
        match kdf {
            KdfType::Pbkdf2 => 0,
            KdfType::Argon2id => 1,
        }
    }
}

impl serde::Serialize for KdfType {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        serializer.serialize_u8((*self).into())
    }
}

impl<'de> serde::Deserialize<'de> for KdfType {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let val = u8::deserialize(deserializer)?;
        Ok(KdfType::from(val))
    }
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct KdfParams {
    pub kdf_type: KdfType,
    pub iterations: u32,
    pub memory_mb: Option<u32>,
    pub parallelism: Option<u32>,
}

impl Default for KdfParams {
    fn default() -> Self {
        Self {
            kdf_type: KdfType::Pbkdf2,
            iterations: 600_000,
            memory_mb: None,
            parallelism: None,
        }
    }
}

impl KdfParams {
    pub fn pbkdf2(iterations: u32) -> Self {
        Self {
            kdf_type: KdfType::Pbkdf2,
            iterations,
            memory_mb: None,
            parallelism: None,
        }
    }

    pub fn argon2id(iterations: u32, memory_mb: u32, parallelism: u32) -> Self {
        Self {
            kdf_type: KdfType::Argon2id,
            iterations,
            memory_mb: Some(memory_mb),
            parallelism: Some(parallelism),
        }
    }
}

/// Derives the 32-byte MasterKey from the master password and email.
pub fn derive_master_key(
    master_password: &str,
    email: &str,
    params: &KdfParams,
) -> Result<Zeroizing<[u8; 32]>, CryptoError> {
    let normalized_email = email.trim().to_lowercase();
    let mut key = [0u8; 32];

    match params.kdf_type {
        KdfType::Pbkdf2 => {
            if params.iterations == 0 {
                return Err(CryptoError::InvalidKdfParams(
                    "PBKDF2 iterations must be greater than 0".to_string(),
                ));
            }
            pbkdf2::pbkdf2_hmac::<Sha256>(
                master_password.as_bytes(),
                normalized_email.as_bytes(),
                params.iterations,
                &mut key,
            );
        }
        KdfType::Argon2id => {
            let memory_kib = params.memory_mb.unwrap_or(64) * 1024;
            let parallelism = params.parallelism.unwrap_or(4);
            let iterations = params.iterations;

            let argon_params = argon2::Params::new(memory_kib, iterations, parallelism, Some(32))
                .map_err(|e| CryptoError::InvalidKdfParams(e.to_string()))?;

            let salt = Sha256::digest(normalized_email.as_bytes());

            let argon2 = argon2::Argon2::new(
                argon2::Algorithm::Argon2id,
                argon2::Version::V0x13,
                argon_params,
            );

            argon2
                .hash_password_into(master_password.as_bytes(), &salt, &mut key)
                .map_err(|e| CryptoError::KeyDerivationFailed(e.to_string()))?;
        }
    }

    Ok(Zeroizing::new(key))
}

/// Derives the MasterPasswordHash for upstream authentication:
/// Base64(PBKDF2-SHA256(password = MasterKey, salt = master_password, iterations = 1, dkLen = 32))
pub fn derive_master_password_hash(
    master_key: &[u8; 32],
    master_password: &str,
) -> Result<String, CryptoError> {
    let mut hash_bytes = [0u8; 32];
    pbkdf2::pbkdf2_hmac::<Sha256>(master_key, master_password.as_bytes(), 1, &mut hash_bytes);

    Ok(BASE64_STANDARD.encode(hash_bytes))
}

/// Stretches the 32-byte MasterKey into a 64-byte key using HKDF-Expand:
/// - EncKey (32B): HKDF-Expand(PRK = MasterKey, info = b"enc", L = 32)
/// - MacKey (32B): HKDF-Expand(PRK = MasterKey, info = b"mac", L = 32)
pub fn stretch_master_key(master_key: &[u8; 32]) -> Result<Zeroizing<[u8; 64]>, CryptoError> {
    let hkdf = hkdf::Hkdf::<Sha256>::from_prk(master_key)
        .map_err(|e| CryptoError::KeyDerivationFailed(format!("HKDF PRK error: {e:?}")))?;

    let mut enc_key = [0u8; 32];
    hkdf.expand(b"enc", &mut enc_key)
        .map_err(|e| CryptoError::KeyDerivationFailed(format!("HKDF expand enc error: {e:?}")))?;

    let mut mac_key = [0u8; 32];
    hkdf.expand(b"mac", &mut mac_key)
        .map_err(|e| CryptoError::KeyDerivationFailed(format!("HKDF expand mac error: {e:?}")))?;

    let mut stretched = [0u8; 64];
    stretched[..32].copy_from_slice(&enc_key);
    stretched[32..].copy_from_slice(&mac_key);

    Ok(Zeroizing::new(stretched))
}
