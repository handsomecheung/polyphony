use rsa::{RsaPrivateKey, RsaPublicKey};
use tacet::crypto::cipher_string::CipherString;
use tacet::crypto::symmetric_key::SymmetricKey;
use tacet::error::ResolverError;
use tacet::resolver::VaultResolver;
use tacet::upstream::models::{
    CipherFieldResponse, CipherLoginResponse, CipherResponse, CipherUriResponse,
    CollectionResponse, FolderResponse, ProfileOrganizationResponse, ProfileResponse, SyncResponse,
};

fn setup_test_vault() -> (
    SyncResponse,
    SymmetricKey,
    RsaPrivateKey,
    SymmetricKey, // org_key
) {
    let mut rng = rand::rngs::OsRng;
    let user_rsa_priv = RsaPrivateKey::new(&mut rng, 2048).unwrap();
    let user_rsa_pub = RsaPublicKey::from(&user_rsa_priv);

    let user_key = SymmetricKey::new([11u8; 64]);
    let org_key = SymmetricKey::new([22u8; 64]);

    // Encrypt org_key using user's RSA public key (Type 4 CipherString)
    let org_key_encrypted = CipherString::encrypt_type4(&user_rsa_pub, org_key.as_bytes()).unwrap();

    let profile = ProfileResponse {
        id: "user-uuid".to_string(),
        name: Some("Alice".to_string()),
        email: "alice@example.com".to_string(),
        key: None,
        private_key: None,
        organizations: vec![ProfileOrganizationResponse {
            id: "org-uuid-1".to_string(),
            name: "AcmeCorp".to_string(),
            key: Some(org_key_encrypted),
        }],
    };

    let folders = vec![FolderResponse {
        id: "folder-dev".to_string(),
        name: user_key.encrypt_type2_string("Development").unwrap(),
    }];

    let collections = vec![CollectionResponse {
        id: "col-backend".to_string(),
        organization_id: "org-uuid-1".to_string(),
        name: org_key.encrypt_type2_string("Backend").unwrap(),
    }];

    let json_notes = r#"{
  "db": {
    "user": "db_admin",
    "pass": "db_secret_123"
  }
}
"#;

    let ciphers = vec![
        // p1: GitHub Personal
        CipherResponse {
            id: "00000000-0000-0000-0000-000000000001".to_string(),
            organization_id: None,
            folder_id: None,
            collection_ids: vec![],
            cipher_type: 1,
            name: user_key.encrypt_type2_string("GitHub Personal").unwrap(),
            notes: Some(user_key.encrypt_type2_string(json_notes).unwrap()),
            favorite: Some(true),
            login: Some(CipherLoginResponse {
                username: Some(user_key.encrypt_type2_string("alice").unwrap()),
                password: Some(
                    user_key
                        .encrypt_type2_string("ghp_secret_password")
                        .unwrap(),
                ),
                totp: Some(user_key.encrypt_type2_string("JBSWY3DPEHPK3PXP").unwrap()),
                uris: Some(vec![
                    CipherUriResponse {
                        uri: Some(
                            user_key
                                .encrypt_type2_string("https://github.com/login")
                                .unwrap(),
                        ),
                        r#match: None,
                    },
                    CipherUriResponse {
                        uri: Some(
                            user_key
                                .encrypt_type2_string("https://api.github.com")
                                .unwrap(),
                        ),
                        r#match: None,
                    },
                ]),
            }),
            fields: Some(vec![CipherFieldResponse {
                name: Some(user_key.encrypt_type2_string("api_token").unwrap()),
                value: Some(user_key.encrypt_type2_string("custom_token_abc").unwrap()),
                field_type: Some(0),
            }]),
        },
        // p2: Postgres Dev inside Development folder
        CipherResponse {
            id: "00000000-0000-0000-0000-000000000002".to_string(),
            organization_id: None,
            folder_id: Some("folder-dev".to_string()),
            collection_ids: vec![],
            cipher_type: 1,
            name: user_key.encrypt_type2_string("Postgres Dev").unwrap(),
            notes: None,
            favorite: None,
            login: Some(CipherLoginResponse {
                password: Some(user_key.encrypt_type2_string("dev_pg_password").unwrap()),
                ..Default::default()
            }),
            fields: None,
        },
        // p3 & p4: Duplicate items for disambiguation test
        CipherResponse {
            id: "00000000-0000-0000-0000-000000000003".to_string(),
            organization_id: None,
            folder_id: None,
            collection_ids: vec![],
            cipher_type: 1,
            name: user_key.encrypt_type2_string("DuplicateItem").unwrap(),
            notes: None,
            favorite: None,
            login: Some(CipherLoginResponse {
                password: Some(user_key.encrypt_type2_string("dup1_pass").unwrap()),
                ..Default::default()
            }),
            fields: None,
        },
        CipherResponse {
            id: "00000000-0000-0000-0000-000000000004".to_string(),
            organization_id: None,
            folder_id: None,
            collection_ids: vec![],
            cipher_type: 1,
            name: user_key.encrypt_type2_string("DuplicateItem").unwrap(),
            notes: None,
            favorite: None,
            login: Some(CipherLoginResponse {
                password: Some(user_key.encrypt_type2_string("dup2_pass").unwrap()),
                ..Default::default()
            }),
            fields: None,
        },
        // o1: Organization cipher
        CipherResponse {
            id: "00000000-0000-0000-0000-000000000005".to_string(),
            organization_id: Some("org-uuid-1".to_string()),
            folder_id: None,
            collection_ids: vec!["col-backend".to_string()],
            cipher_type: 1,
            name: org_key.encrypt_type2_string("Prod Database").unwrap(),
            notes: Some(
                org_key
                    .encrypt_type2_string("-----BEGIN CERTIFICATE-----\nMIIB...\n")
                    .unwrap(),
            ),
            favorite: None,
            login: Some(CipherLoginResponse {
                username: Some(org_key.encrypt_type2_string("postgres_admin").unwrap()),
                password: Some(
                    org_key
                        .encrypt_type2_string("ultra_secure_prod_password")
                        .unwrap(),
                ),
                ..Default::default()
            }),
            fields: None,
        },
    ];

    let sync = SyncResponse {
        profile: Some(profile),
        folders,
        collections,
        ciphers,
    };

    (sync, user_key, user_rsa_priv, org_key)
}

#[test]
fn test_resolve_personal_basic_fields() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/username")
            .unwrap(),
        "alice"
    );
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/password")
            .unwrap(),
        "ghp_secret_password"
    );
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/uri")
            .unwrap(),
        "https://github.com/login"
    );
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/uri.1")
            .unwrap(),
        "https://api.github.com"
    );
}

#[test]
fn test_resolve_custom_fields() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    // Via named fallback
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/api_token")
            .unwrap(),
        "custom_token_abc"
    );
    // Via explicit fields/ prefix
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/fields/api_token")
            .unwrap(),
        "custom_token_abc"
    );
}

#[test]
fn test_resolve_by_uuid() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    assert_eq!(
        resolver
            .resolve_str("tacet://personal/00000000-0000-0000-0000-000000000001/password")
            .unwrap(),
        "ghp_secret_password"
    );
}

#[test]
fn test_resolve_folder_scoping() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    // Item inside folder
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/Development/Postgres Dev/password")
            .unwrap(),
        "dev_pg_password"
    );

    // Item not inside this folder
    let err = resolver
        .resolve_str("tacet://personal/Development/GitHub Personal/password")
        .unwrap_err();
    assert!(matches!(
        err,
        tacet::error::TacetError::Resolver(ResolverError::ItemNotFound { .. })
    ));

    // Non-existent folder
    let err2 = resolver
        .resolve_str("tacet://personal/NonExistent/Postgres Dev/password")
        .unwrap_err();
    assert!(matches!(
        err2,
        tacet::error::TacetError::Resolver(ResolverError::FolderNotFound { .. })
    ));
}

#[test]
fn test_resolve_organization_with_rsa_key() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    // Resolver automatically decrypts org key via RSA private key!
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    assert_eq!(
        resolver
            .resolve_str("tacet://org/AcmeCorp/Backend/Prod Database/password")
            .unwrap(),
        "ultra_secure_prod_password"
    );
    assert_eq!(
        resolver
            .resolve_str("tacet://org/AcmeCorp/Backend/Prod Database/username")
            .unwrap(),
        "postgres_admin"
    );
}

#[test]
fn test_disambiguation_error() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    let err = resolver
        .resolve_str("tacet://personal/DuplicateItem/password")
        .unwrap_err();

    match err {
        tacet::error::TacetError::Resolver(ResolverError::AmbiguousMatch { item, matches }) => {
            assert_eq!(item, "DuplicateItem");
            assert!(matches.contains("00000000-0000-0000-0000-000000000003"));
            assert!(matches.contains("00000000-0000-0000-0000-000000000004"));
        }
        other => panic!("Expected AmbiguousMatch, got {other:?}"),
    }
}

#[test]
fn test_totp_generation() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    let code = resolver
        .resolve_str("tacet://personal/GitHub Personal/totp")
        .unwrap();
    assert_eq!(code.len(), 6);
    assert!(code.chars().all(|c| c.is_ascii_digit()));

    let raw_secret = resolver
        .resolve_str("tacet://personal/GitHub Personal/totp_secret")
        .unwrap();
    assert_eq!(raw_secret, "JBSWY3DPEHPK3PXP");
}

#[test]
fn test_modifiers() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    // 1. JSON modifier
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/notes?json=db.pass")
            .unwrap(),
        "db_secret_123"
    );
    assert_eq!(
        resolver
            .resolve_str("tacet://personal/GitHub Personal/notes?json=db.user")
            .unwrap(),
        "db_admin"
    );

    // 2. Base64 modifier
    let b64 = resolver
        .resolve_str("tacet://personal/GitHub Personal/password?encoding=base64")
        .unwrap();
    assert_eq!(b64, "Z2hwX3NlY3JldF9wYXNzd29yZA==");

    // 3. Hex modifier
    let hex = resolver
        .resolve_str("tacet://personal/GitHub Personal/username?encoding=hex")
        .unwrap();
    assert_eq!(hex, "616c696365"); // "alice" in hex

    // 4. Trim modifier
    let untrimmed = resolver
        .resolve_str("tacet://org/AcmeCorp/Backend/Prod Database/notes?trim=false")
        .unwrap();
    assert!(untrimmed.ends_with('\n'));

    let trimmed = resolver
        .resolve_str("tacet://org/AcmeCorp/Backend/Prod Database/notes?trim=true")
        .unwrap();
    assert!(!trimmed.ends_with('\n'));
}

#[test]
fn test_list_items() {
    let (vault, user_key, rsa_priv, _) = setup_test_vault();
    let resolver = VaultResolver::new(&vault, &user_key, Some(&rsa_priv));

    // 1. List all items
    let all = resolver.list_items(None, None).unwrap();
    assert_eq!(all.len(), 5);

    // 2. Filter by personal scope
    let personal = resolver.list_items(Some("personal"), None).unwrap();
    assert_eq!(personal.len(), 4);
    for item in &personal {
        assert!(item.scope.starts_with("personal"));
    }

    // 3. Filter by folder
    let dev = resolver
        .list_items(Some("personal/Development"), None)
        .unwrap();
    assert_eq!(dev.len(), 1);
    assert_eq!(dev[0].name, "Postgres Dev");

    // 4. Filter by org
    let org = resolver.list_items(Some("org/AcmeCorp"), None).unwrap();
    assert_eq!(org.len(), 1);
    assert_eq!(org[0].name, "Prod Database");

    // 5. Filter by search query
    let searched_github = resolver.list_items(None, Some("GitHub")).unwrap();
    assert_eq!(searched_github.len(), 1);

    let searched_dup = resolver.list_items(None, Some("Duplicate")).unwrap();
    assert_eq!(searched_dup.len(), 2);
}
