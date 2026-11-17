use rsa::pkcs8::EncodePrivateKey;
use rsa::{RsaPrivateKey, RsaPublicKey};
use tacet::crypto::cipher_string::{parse_rsa_private_key, CipherString};
use tacet::crypto::kdf::{
    derive_master_key, derive_master_password_hash, stretch_master_key, KdfParams,
};
use tacet::crypto::symmetric_key::SymmetricKey;
use tacet::error::CryptoError;

#[test]
fn test_pbkdf2_master_key_derivation() {
    let password = "MySuperSecretPassword123!";
    let email = " User@Example.COM "; // Should be trimmed and lowercased to "user@example.com"
    let params = KdfParams::pbkdf2(100_000);

    let key1 = derive_master_key(password, email, &params).expect("Derivation failed");
    let key2 = derive_master_key(password, "user@example.com", &params).expect("Derivation failed");

    assert_eq!(key1.as_ref(), key2.as_ref());
    assert_eq!(key1.len(), 32);

    // Different email must produce different key
    let key3 =
        derive_master_key(password, "other@example.com", &params).expect("Derivation failed");
    assert_ne!(key1.as_ref(), key3.as_ref());

    // 0 iterations should fail
    let bad_params = KdfParams::pbkdf2(0);
    assert!(derive_master_key(password, email, &bad_params).is_err());
}

#[test]
fn test_argon2id_master_key_derivation() {
    let password = "MySuperSecretPassword123!";
    let email = "  TestUser@Domain.org  ";
    let params = KdfParams::argon2id(3, 64, 4);

    let key1 = derive_master_key(password, email, &params).expect("Argon2id failed");
    let key2 =
        derive_master_key(password, "testuser@domain.org", &params).expect("Argon2id failed");

    assert_eq!(key1.as_ref(), key2.as_ref());
    assert_eq!(key1.len(), 32);

    let key3 = derive_master_key("different-password", "testuser@domain.org", &params).unwrap();
    assert_ne!(key1.as_ref(), key3.as_ref());
}

#[test]
fn test_master_password_hash() {
    let master_key = [42u8; 32];
    let password = "master_password";

    let hash1 = derive_master_password_hash(&master_key, password).expect("Hash failed");
    let hash2 = derive_master_password_hash(&master_key, password).expect("Hash failed");

    assert_eq!(hash1, hash2);
    assert!(!hash1.is_empty());

    // Different password produces different hash
    let hash3 = derive_master_password_hash(&master_key, "different").unwrap();
    assert_ne!(hash1, hash3);
}

#[test]
fn test_hkdf_master_key_stretching() {
    let master_key = [7u8; 32];
    let stretched = stretch_master_key(&master_key).expect("Stretching failed");

    assert_eq!(stretched.len(), 64);
    let enc_key = &stretched[..32];
    let mac_key = &stretched[32..];

    // Encryption key and MAC key must be distinct
    assert_ne!(enc_key, mac_key);
}

#[test]
fn test_type2_cipher_string_encrypt_decrypt() {
    let mut enc_key = [0u8; 32];
    let mut mac_key = [0u8; 32];
    for i in 0..32 {
        enc_key[i] = i as u8;
        mac_key[i] = (i + 100) as u8;
    }

    let plaintext = b"Hello, Tacet Bitwarden Secret Manager!";
    let cipher_str =
        CipherString::encrypt_type2(&enc_key, &mac_key, plaintext).expect("Encryption failed");

    assert!(cipher_str.starts_with("2."));

    let parsed = CipherString::parse(&cipher_str).expect("Parse failed");
    let decrypted = parsed
        .decrypt_type2(&enc_key, &mac_key)
        .expect("Decryption failed");

    assert_eq!(decrypted, plaintext);
}

#[test]
fn test_type2_tamper_detection() {
    let enc_key = [15u8; 32];
    let mac_key = [25u8; 32];
    let plaintext = b"Confidential API Key: sk-live-123456789";

    let cipher_str = CipherString::encrypt_type2(&enc_key, &mac_key, plaintext).unwrap();
    let parts: Vec<&str> = cipher_str.split('.').collect();
    let subparts: Vec<&str> = parts[1].split('|').collect();

    let iv = subparts[0];
    let ct = subparts[1];
    let mac = subparts[2];

    // Tampered ciphertext: modify a character
    let mut tampered_ct = ct.to_string();
    let last_char = tampered_ct.pop().unwrap();
    let replacement_char = if last_char == 'A' { 'B' } else { 'A' };
    tampered_ct.push(replacement_char);

    let tampered_cipher_str = format!("2.{iv}|{tampered_ct}|{mac}");
    let parsed = CipherString::parse(&tampered_cipher_str).unwrap();
    let err = parsed.decrypt_type2(&enc_key, &mac_key).unwrap_err();
    match err {
        CryptoError::MacVerificationFailed => (),
        other => panic!("Expected MacVerificationFailed, got {other:?}"),
    }

    // Tampered MAC
    let mut tampered_mac = mac.to_string();
    let last_mac_char = tampered_mac.pop().unwrap();
    tampered_mac.push(if last_mac_char == 'X' { 'Y' } else { 'X' });
    let tampered_mac_str = format!("2.{iv}|{ct}|{tampered_mac}");
    let parsed_mac = CipherString::parse(&tampered_mac_str).unwrap();
    assert!(matches!(
        parsed_mac.decrypt_type2(&enc_key, &mac_key).unwrap_err(),
        CryptoError::MacVerificationFailed
    ));

    // Wrong Key
    let wrong_enc_key = [99u8; 32];
    let wrong_mac_key = [88u8; 32];
    let parsed_original = CipherString::parse(&cipher_str).unwrap();
    assert!(matches!(
        parsed_original
            .decrypt_type2(&wrong_enc_key, &wrong_mac_key)
            .unwrap_err(),
        CryptoError::MacVerificationFailed
    ));
}

#[test]
fn test_symmetric_key_lifecycle_and_protected_key() {
    // 1. Simulate MasterKey and StretchedMasterKey
    let master_key = [123u8; 32];
    let stretched = stretch_master_key(&master_key).unwrap();

    // 2. Simulate User's SymmetricKey (64 bytes generated during account registration)
    let original_sym_key_bytes = [77u8; 64];
    let original_sym_key = SymmetricKey::new(original_sym_key_bytes);

    // 3. Encrypt the SymmetricKey using StretchedMasterKey (like Bitwarden server does for login response's `Key`)
    let mut stretch_enc = [0u8; 32];
    let mut stretch_mac = [0u8; 32];
    stretch_enc.copy_from_slice(&stretched[..32]);
    stretch_mac.copy_from_slice(&stretched[32..]);

    let protected_key_str =
        CipherString::encrypt_type2(&stretch_enc, &stretch_mac, original_sym_key.as_bytes())
            .unwrap();

    // 4. Decrypt via SymmetricKey::from_protected_key
    let restored_sym_key =
        SymmetricKey::from_protected_key(&protected_key_str, &stretched).expect("Restore failed");

    assert_eq!(restored_sym_key.as_bytes(), original_sym_key.as_bytes());

    // 5. Test encryption & decryption of vault secret using restored SymmetricKey
    let secret = "ghp_0123456789abcdefghijklmnopqrstuvwxyz";
    let encrypted_secret = restored_sym_key.encrypt_type2_string(secret).unwrap();
    let decrypted_secret = restored_sym_key
        .decrypt_type2_string(&encrypted_secret)
        .unwrap();

    assert_eq!(decrypted_secret, secret);
}

#[test]
fn test_type4_rsa_oaep_sha1() {
    let mut rng = rand::rngs::OsRng;
    let private_key = RsaPrivateKey::new(&mut rng, 2048).expect("Failed to generate RSA key");
    let public_key = RsaPublicKey::from(&private_key);

    let secret = b"Organization shared collection key!";
    let cipher_str =
        CipherString::encrypt_type4(&public_key, secret).expect("Type 4 encrypt failed");

    assert!(cipher_str.starts_with("4."));

    let parsed = CipherString::parse(&cipher_str).expect("Parse failed");
    let decrypted = parsed
        .decrypt_type4(&private_key)
        .expect("Type 4 decrypt failed");

    assert_eq!(decrypted, secret);

    // Test PKCS#8 DER export and parsing
    let pkcs8_der = private_key
        .to_pkcs8_der()
        .expect("PKCS#8 DER export failed");
    let parsed_key = parse_rsa_private_key(pkcs8_der.as_bytes()).expect("PKCS#8 parsing failed");

    let decrypted2 = parsed
        .decrypt_type4(&parsed_key)
        .expect("Decrypt with parsed key failed");
    assert_eq!(decrypted2, secret);
}
