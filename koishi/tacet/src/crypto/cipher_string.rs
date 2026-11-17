use crate::error::CryptoError;
use aes::cipher::{block_padding::Pkcs7, BlockDecryptMut, BlockEncryptMut, KeyIvInit};
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use base64::Engine;
use hmac::{Hmac, Mac};
use rand::RngCore;
use rsa::pkcs1::DecodeRsaPrivateKey;
use rsa::pkcs8::DecodePrivateKey;
use rsa::{Oaep, RsaPrivateKey, RsaPublicKey};
use sha1::Sha1;
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;
type Aes256CbcDec = cbc::Decryptor<aes::Aes256>;
type Aes256CbcEnc = cbc::Encryptor<aes::Aes256>;

/// Parses an RSA private key from PKCS#8 / PKCS#1 DER or PEM bytes.
pub fn parse_rsa_private_key(data: &[u8]) -> Result<RsaPrivateKey, CryptoError> {
    if let Ok(key) = RsaPrivateKey::from_pkcs8_der(data) {
        return Ok(key);
    }
    if let Ok(key) = RsaPrivateKey::from_pkcs1_der(data) {
        return Ok(key);
    }
    if let Ok(s) = std::str::from_utf8(data) {
        if let Ok(key) = RsaPrivateKey::from_pkcs8_pem(s) {
            return Ok(key);
        }
        if let Ok(key) = RsaPrivateKey::from_pkcs1_pem(s) {
            return Ok(key);
        }
    }
    Err(CryptoError::DecryptionFailed(
        "Failed to parse RSA private key as PKCS#8 or PKCS#1 (DER/PEM)".to_string(),
    ))
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CipherString {
    /// Type 0: AesCbc256_B64 (iv | ct)
    Type0 { iv: Vec<u8>, ct: Vec<u8> },
    /// Type 2: AesCbc256_HmacSha256_B64 (iv | ct | mac)
    Type2 {
        iv: Vec<u8>,
        ct: Vec<u8>,
        mac: Vec<u8>,
    },
    /// Type 4: Rsa2048_OaepSha1_B64 (ct)
    Type4 { ct: Vec<u8> },
}

impl CipherString {
    /// Parses a Bitwarden CipherString, e.g. "2.<iv>|<ct>|<mac>" or "4.<ct>"
    pub fn parse(s: &str) -> Result<Self, CryptoError> {
        let trimmed = s.trim();
        let (type_str, payload) = trimmed.split_once('.').ok_or_else(|| {
            CryptoError::InvalidCipherString("Missing type prefix '.'".to_string())
        })?;

        let enc_type: u8 = type_str
            .parse()
            .map_err(|_| CryptoError::InvalidCipherString(format!("Invalid type '{type_str}'")))?;

        match enc_type {
            0 => {
                let parts: Vec<&str> = payload.split('|').collect();
                if parts.len() != 2 {
                    return Err(CryptoError::InvalidCipherString(
                        "Type 0 requires 2 parts: iv|ct".to_string(),
                    ));
                }
                let iv = BASE64_STANDARD.decode(parts[0])?;
                let ct = BASE64_STANDARD.decode(parts[1])?;
                Ok(CipherString::Type0 { iv, ct })
            }
            2 => {
                let parts: Vec<&str> = payload.split('|').collect();
                if parts.len() != 3 {
                    return Err(CryptoError::InvalidCipherString(
                        "Type 2 requires 3 parts: iv|ct|mac".to_string(),
                    ));
                }
                let iv = BASE64_STANDARD.decode(parts[0])?;
                let ct = BASE64_STANDARD.decode(parts[1])?;
                let mac = BASE64_STANDARD.decode(parts[2])?;
                Ok(CipherString::Type2 { iv, ct, mac })
            }
            4 => {
                let ct = BASE64_STANDARD.decode(payload)?;
                Ok(CipherString::Type4 { ct })
            }
            other => Err(CryptoError::InvalidCipherString(format!(
                "Unsupported cipher type {other}"
            ))),
        }
    }

    /// Decrypts a Type 2 CipherString using 32-byte enc_key and 32-byte mac_key.
    pub fn decrypt_type2(
        &self,
        enc_key: &[u8; 32],
        mac_key: &[u8; 32],
    ) -> Result<Vec<u8>, CryptoError> {
        match self {
            CipherString::Type2 { iv, ct, mac } => {
                if iv.len() != 16 {
                    return Err(CryptoError::DecryptionFailed(format!(
                        "Invalid IV length: expected 16, got {}",
                        iv.len()
                    )));
                }

                // 1. Verify HMAC-SHA256 over (iv || ct) in constant time
                let mut hmac = HmacSha256::new_from_slice(mac_key)
                    .map_err(|e| CryptoError::DecryptionFailed(format!("HMAC init error: {e}")))?;
                hmac.update(iv);
                hmac.update(ct);
                hmac.verify_slice(mac)
                    .map_err(|_| CryptoError::MacVerificationFailed)?;

                // 2. AES-256-CBC Decryption
                let cipher = Aes256CbcDec::new(enc_key.into(), iv.as_slice().into());
                let mut buf = ct.clone();
                let decrypted = cipher.decrypt_padded_mut::<Pkcs7>(&mut buf).map_err(|e| {
                    CryptoError::DecryptionFailed(format!("AES CBC decrypt error: {e:?}"))
                })?;

                Ok(decrypted.to_vec())
            }
            CipherString::Type0 { iv, ct } => {
                if iv.len() != 16 {
                    return Err(CryptoError::DecryptionFailed(format!(
                        "Invalid IV length: expected 16, got {}",
                        iv.len()
                    )));
                }
                let cipher = Aes256CbcDec::new(enc_key.into(), iv.as_slice().into());
                let mut buf = ct.clone();
                let decrypted = cipher.decrypt_padded_mut::<Pkcs7>(&mut buf).map_err(|e| {
                    CryptoError::DecryptionFailed(format!("AES CBC decrypt error: {e:?}"))
                })?;

                Ok(decrypted.to_vec())
            }
            _ => Err(CryptoError::DecryptionFailed(
                "Expected Type 2 (or Type 0) CipherString".to_string(),
            )),
        }
    }

    /// Decrypts a Type 4 CipherString using RSA private key and OAEP-SHA1 padding.
    pub fn decrypt_type4(&self, private_key: &RsaPrivateKey) -> Result<Vec<u8>, CryptoError> {
        match self {
            CipherString::Type4 { ct } => {
                let padding = Oaep::new::<Sha1>();
                let decrypted = private_key
                    .decrypt(padding, ct)
                    .map_err(CryptoError::RsaError)?;
                Ok(decrypted)
            }
            _ => Err(CryptoError::DecryptionFailed(
                "Expected Type 4 CipherString".to_string(),
            )),
        }
    }

    /// Encrypts plaintext into a Type 2 CipherString: "2.<iv>|<ct>|<mac>"
    pub fn encrypt_type2(
        enc_key: &[u8; 32],
        mac_key: &[u8; 32],
        plaintext: &[u8],
    ) -> Result<String, CryptoError> {
        // Generate random 16-byte IV
        let mut iv = [0u8; 16];
        rand::rngs::OsRng.fill_bytes(&mut iv);

        // AES-256-CBC Encrypt
        let cipher = Aes256CbcEnc::new(enc_key.into(), (&iv).into());
        let ct = cipher.encrypt_padded_vec_mut::<Pkcs7>(plaintext);

        // Compute HMAC-SHA256 over (iv || ct)
        let mut hmac = HmacSha256::new_from_slice(mac_key)
            .map_err(|e| CryptoError::EncryptionFailed(format!("HMAC init error: {e}")))?;
        hmac.update(&iv);
        hmac.update(&ct);
        let mac = hmac.finalize().into_bytes();

        let iv_b64 = BASE64_STANDARD.encode(iv);
        let ct_b64 = BASE64_STANDARD.encode(&ct);
        let mac_b64 = BASE64_STANDARD.encode(mac);

        Ok(format!("2.{iv_b64}|{ct_b64}|{mac_b64}"))
    }

    /// Encrypts plaintext into a Type 4 CipherString using RSA public key and OAEP-SHA1 padding.
    pub fn encrypt_type4(
        public_key: &RsaPublicKey,
        plaintext: &[u8],
    ) -> Result<String, CryptoError> {
        let mut rng = rand::rngs::OsRng;
        let padding = Oaep::new::<Sha1>();
        let ct = public_key
            .encrypt(&mut rng, padding, plaintext)
            .map_err(CryptoError::RsaError)?;
        let ct_b64 = BASE64_STANDARD.encode(ct);
        Ok(format!("4.{ct_b64}"))
    }
}
