use crate::crypto::cipher_string::CipherString;
use crate::error::CryptoError;
use zeroize::{Zeroize, ZeroizeOnDrop};

/// A 64-byte symmetric key consisting of:
/// - EncKey: 32 bytes (for AES-256-CBC)
/// - MacKey: 32 bytes (for HMAC-SHA256)
///
/// Guaranteed to be zeroized from memory on drop.
#[derive(Clone, Zeroize, ZeroizeOnDrop)]
pub struct SymmetricKey {
    key: [u8; 64],
}

impl SymmetricKey {
    /// Creates a SymmetricKey from a 64-byte array.
    pub fn new(key: [u8; 64]) -> Self {
        Self { key }
    }

    /// Creates a SymmetricKey from a slice, returning an error if length != 64.
    pub fn from_slice(slice: &[u8]) -> Result<Self, CryptoError> {
        if slice.len() != 64 {
            return Err(CryptoError::InvalidKeyLength {
                expected: 64,
                got: slice.len(),
            });
        }
        let mut key = [0u8; 64];
        key.copy_from_slice(slice);
        Ok(Self { key })
    }

    /// Creates a SymmetricKey from the 64-byte stretched master key.
    pub fn from_stretched_master_key(stretched: &[u8; 64]) -> Self {
        let mut key = [0u8; 64];
        key.copy_from_slice(stretched);
        Self { key }
    }

    /// Decrypts the user's protected symmetric key (login response's `Key`, a Type 2 CipherString)
    /// using the StretchedMasterKey (64B: 32B enc + 32B mac).
    pub fn from_protected_key(
        protected_key_str: &str,
        stretched_master_key: &[u8; 64],
    ) -> Result<Self, CryptoError> {
        let cs = CipherString::parse(protected_key_str)?;
        let mut enc_key = [0u8; 32];
        let mut mac_key = [0u8; 32];
        enc_key.copy_from_slice(&stretched_master_key[..32]);
        mac_key.copy_from_slice(&stretched_master_key[32..]);

        let raw_bytes = cs.decrypt_type2(&enc_key, &mac_key)?;
        Self::from_slice(&raw_bytes)
    }

    /// Reference to the 32-byte AES encryption key.
    pub fn enc_key(&self) -> &[u8; 32] {
        let ptr = self.key[..32].as_ptr() as *const [u8; 32];
        unsafe { &*ptr }
    }

    /// Reference to the 32-byte HMAC key.
    pub fn mac_key(&self) -> &[u8; 32] {
        let ptr = self.key[32..].as_ptr() as *const [u8; 32];
        unsafe { &*ptr }
    }

    /// Reference to the full 64-byte key material.
    pub fn as_bytes(&self) -> &[u8; 64] {
        &self.key
    }

    /// Decrypts a Type 2 CipherString into raw plaintext bytes.
    pub fn decrypt_type2(&self, cipher_string: &str) -> Result<Vec<u8>, CryptoError> {
        let cs = CipherString::parse(cipher_string)?;
        cs.decrypt_type2(self.enc_key(), self.mac_key())
    }

    /// Decrypts a Type 2 CipherString into a UTF-8 String.
    pub fn decrypt_type2_string(&self, cipher_string: &str) -> Result<String, CryptoError> {
        let bytes = self.decrypt_type2(cipher_string)?;
        String::from_utf8(bytes).map_err(CryptoError::Utf8Error)
    }

    /// Encrypts raw plaintext bytes into a Type 2 CipherString: "2.<iv>|<ct>|<mac>"
    pub fn encrypt_type2(&self, plaintext: &[u8]) -> Result<String, CryptoError> {
        CipherString::encrypt_type2(self.enc_key(), self.mac_key(), plaintext)
    }

    /// Encrypts a UTF-8 string into a Type 2 CipherString.
    pub fn encrypt_type2_string(&self, plaintext: &str) -> Result<String, CryptoError> {
        self.encrypt_type2(plaintext.as_bytes())
    }
}
