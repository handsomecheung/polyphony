use tacet::error::UriError;
use tacet::uri::parser::{CipherFieldTarget, Encoding, Scope, TacetUri};

#[test]
fn test_parse_personal_scope() {
    let uri1 = TacetUri::parse("tacet://personal/github/password").unwrap();
    assert_eq!(uri1.scope, Scope::Personal { folder: None });
    assert_eq!(uri1.item, "github");
    assert_eq!(uri1.field, CipherFieldTarget::Password);

    let uri2 = TacetUri::parse("tacet://personal/work/aws_root/totp").unwrap();
    assert_eq!(
        uri2.scope,
        Scope::Personal {
            folder: Some("work".to_string())
        }
    );
    assert_eq!(uri2.item, "aws_root");
    assert_eq!(uri2.field, CipherFieldTarget::Totp);

    let uri3 = TacetUri::parse("tacet://_/stripe/user").unwrap();
    assert_eq!(uri3.scope, Scope::Personal { folder: None });
    assert_eq!(uri3.item, "stripe");
    assert_eq!(uri3.field, CipherFieldTarget::Username);
}

#[test]
fn test_parse_organization_scope() {
    let uri = TacetUri::parse("tacet://org/Engineering/infrastructure/prod-db/password").unwrap();
    assert_eq!(
        uri.scope,
        Scope::Organization {
            org_name: "Engineering".to_string(),
            collection: "infrastructure".to_string(),
        }
    );
    assert_eq!(uri.item, "prod-db");
    assert_eq!(uri.field, CipherFieldTarget::Password);
}

#[test]
fn test_parse_all_scope() {
    let uri = TacetUri::parse("tacet://*/OpenAI%20API/api_key").unwrap();
    assert_eq!(uri.scope, Scope::All);
    assert_eq!(uri.item, "OpenAI API");
    assert_eq!(uri.field, CipherFieldTarget::Named("api_key".to_string()));
}

#[test]
fn test_parse_escaped_characters() {
    // Escaped spaces and literal %2F slashes
    let uri = TacetUri::parse("tacet://personal/prod%2Fcluster/k8s%20token/pass").unwrap();
    assert_eq!(
        uri.scope,
        Scope::Personal {
            folder: Some("prod/cluster".to_string())
        }
    );
    assert_eq!(uri.item, "k8s token");
    assert_eq!(uri.field, CipherFieldTarget::Password);
}

#[test]
fn test_parse_field_variants() {
    assert_eq!(
        TacetUri::parse("tacet://_/item/username").unwrap().field,
        CipherFieldTarget::Username
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/user").unwrap().field,
        CipherFieldTarget::Username
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/password").unwrap().field,
        CipherFieldTarget::Password
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/pass").unwrap().field,
        CipherFieldTarget::Password
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/totp").unwrap().field,
        CipherFieldTarget::Totp
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/otp").unwrap().field,
        CipherFieldTarget::Totp
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/totp_secret").unwrap().field,
        CipherFieldTarget::TotpSecret
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/notes").unwrap().field,
        CipherFieldTarget::Notes
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/note").unwrap().field,
        CipherFieldTarget::Notes
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/uri").unwrap().field,
        CipherFieldTarget::Uri(0)
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/url").unwrap().field,
        CipherFieldTarget::Uri(0)
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/uri.2").unwrap().field,
        CipherFieldTarget::Uri(2)
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/fields/custom_key")
            .unwrap()
            .field,
        CipherFieldTarget::CustomField("custom_key".to_string())
    );
    assert_eq!(
        TacetUri::parse("tacet://_/item/custom_key").unwrap().field,
        CipherFieldTarget::Named("custom_key".to_string())
    );
}

#[test]
fn test_parse_modifiers() {
    let uri1 = TacetUri::parse("tacet://_/item/notes?encoding=base64").unwrap();
    assert_eq!(uri1.modifiers.encoding, Some(Encoding::Base64));
    assert!(uri1.modifiers.trim);

    let uri2 = TacetUri::parse("tacet://_/item/notes?encoding=hex&trim=false").unwrap();
    assert_eq!(uri2.modifiers.encoding, Some(Encoding::Hex));
    assert!(!uri2.modifiers.trim);

    let uri3 = TacetUri::parse("tacet://_/item/notes?json=database.password").unwrap();
    assert_eq!(uri3.modifiers.json.as_deref(), Some("database.password"));
    assert!(uri3.modifiers.trim);
}

#[test]
fn test_invalid_uris() {
    assert_eq!(
        TacetUri::parse("https://personal/item/pass").unwrap_err(),
        UriError::InvalidScheme
    );
    assert_eq!(
        TacetUri::parse("tacet://unknown_scope/item/pass").unwrap_err(),
        UriError::InvalidScope("unknown_scope".to_string())
    );
    assert!(matches!(
        TacetUri::parse("tacet://personal").unwrap_err(),
        UriError::MissingComponent(_)
    ));
    assert!(matches!(
        TacetUri::parse("tacet://_/item/pass?encoding=invalid").unwrap_err(),
        UriError::InvalidModifier(_)
    ));
}
