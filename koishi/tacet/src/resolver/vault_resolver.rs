use crate::crypto::cipher_string::CipherString;
use crate::crypto::symmetric_key::SymmetricKey;
use crate::error::ResolverError;
use crate::resolver::json_path::extract_json_path;
use crate::resolver::totp::generate_totp;
use crate::upstream::models::{CipherResponse, SyncResponse};
use crate::uri::parser::{CipherFieldTarget, Encoding, Scope, TacetUri};
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use base64::Engine;
use rsa::RsaPrivateKey;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct VaultItemSummary {
    pub id: String,
    pub name: String,
    pub scope: String,
    pub folder: Option<String>,
    pub cipher_type: String,
    pub uri_prefix: String,
}

pub struct VaultResolver<'a> {
    vault: &'a SyncResponse,
    user_key: &'a SymmetricKey,
    user_rsa_key: Option<&'a RsaPrivateKey>,
    org_keys: HashMap<String, SymmetricKey>,
}

impl<'a> VaultResolver<'a> {
    pub fn new(
        vault: &'a SyncResponse,
        user_key: &'a SymmetricKey,
        user_rsa_key: Option<&'a RsaPrivateKey>,
    ) -> Self {
        let mut org_keys = HashMap::new();

        // Automatically decrypt organization keys from profile if available
        if let Some(profile) = &vault.profile {
            for org in &profile.organizations {
                if let Some(key_str) = &org.key {
                    if let Ok(cs) = CipherString::parse(key_str) {
                        match cs {
                            CipherString::Type4 { .. } => {
                                if let Some(rsa_key) = user_rsa_key {
                                    if let Ok(raw_key) = cs.decrypt_type4(rsa_key) {
                                        if let Ok(sym) = SymmetricKey::from_slice(&raw_key) {
                                            org_keys.insert(org.id.clone(), sym);
                                        }
                                    }
                                }
                            }
                            CipherString::Type2 { .. } => {
                                if let Ok(sym) =
                                    SymmetricKey::from_protected_key(key_str, user_key.as_bytes())
                                {
                                    org_keys.insert(org.id.clone(), sym);
                                }
                            }
                            _ => {}
                        }
                    }
                }
            }
        }

        Self {
            vault,
            user_key,
            user_rsa_key,
            org_keys,
        }
    }

    /// Explicitly registers or overrides an organization's symmetric key.
    pub fn with_org_key(mut self, org_id: &str, key: SymmetricKey) -> Self {
        self.org_keys.insert(org_id.to_string(), key);
        self
    }

    pub fn user_rsa_key(&self) -> Option<&'a RsaPrivateKey> {
        self.user_rsa_key
    }

    /// Resolves a parsed `TacetUri` to its plaintext secret string.
    pub fn resolve(&self, uri: &TacetUri) -> Result<String, ResolverError> {
        // Step 1: Filter ciphers and assign appropriate decryption key
        let candidates = self.filter_scope(&uri.scope)?;

        // Step 2: Match item by 36-char UUID or case-insensitive decrypted name
        let matched = self.match_item(&candidates, &uri.item, &uri.scope)?;

        // Step 3: Extract field value
        let raw_val = self.extract_field(matched.0, matched.1, &uri.field)?;

        // Step 4: Apply modifiers (json path, trim, encoding)
        self.apply_modifiers(raw_val, uri)
    }

    /// Helper to resolve a raw URI string.
    pub fn resolve_str(&self, uri_str: &str) -> Result<String, crate::error::TacetError> {
        let uri = TacetUri::parse(uri_str)?;
        let secret = self.resolve(&uri)?;
        Ok(secret)
    }

    /// Lists vault item summaries with optional scope and search filters.
    pub fn list_items(
        &self,
        scope_filter: Option<&str>,
        search_filter: Option<&str>,
    ) -> Result<Vec<VaultItemSummary>, ResolverError> {
        // Pre-decrypt all personal folder names
        let mut folder_names: HashMap<String, String> = HashMap::new();
        for f in &self.vault.folders {
            if let Ok(name) = self.user_key.decrypt_type2_string(&f.name) {
                folder_names.insert(f.id.clone(), name);
            }
        }

        // Map organization names
        let mut org_names: HashMap<String, String> = HashMap::new();
        if let Some(profile) = &self.vault.profile {
            for org in &profile.organizations {
                org_names.insert(org.id.clone(), org.name.clone());
            }
        }

        // Pre-decrypt collection names
        let mut col_names: HashMap<String, String> = HashMap::new();
        for col in &self.vault.collections {
            let org_key = self
                .org_keys
                .get(&col.organization_id)
                .unwrap_or(self.user_key);
            if let Ok(name) = org_key
                .decrypt_type2_string(&col.name)
                .or_else(|_| self.user_key.decrypt_type2_string(&col.name))
            {
                col_names.insert(col.id.clone(), name);
            }
        }

        let mut items = Vec::new();

        for cipher in &self.vault.ciphers {
            let is_personal = cipher.organization_id.is_none();
            let key = if let Some(org_id) = &cipher.organization_id {
                match self.org_keys.get(org_id) {
                    Some(k) => k,
                    None => continue,
                }
            } else {
                self.user_key
            };

            let dec_name = match key.decrypt_type2_string(&cipher.name) {
                Ok(n) => n,
                Err(_) => continue,
            };

            let folder_name = cipher
                .folder_id
                .as_ref()
                .and_then(|fid| folder_names.get(fid).cloned());

            let (scope_str, uri_prefix) = if is_personal {
                let enc_name = dec_name.replace('/', "%2F");
                let scope = match &folder_name {
                    Some(f) => format!("personal/{f}"),
                    None => "personal".to_string(),
                };
                let prefix = match &folder_name {
                    Some(f) => format!("personal/{f}/{enc_name}"),
                    None => format!("tacet://personal/{}", enc_name),
                };
                let uri_prefix = if prefix.starts_with("tacet://") {
                    prefix
                } else {
                    format!("tacet://{prefix}")
                };
                (scope, uri_prefix)
            } else {
                let org_id = cipher.organization_id.as_deref().unwrap_or_default();
                let org_name = org_names
                    .get(org_id)
                    .cloned()
                    .unwrap_or_else(|| org_id.to_string());

                let col_name = cipher
                    .collection_ids
                    .first()
                    .and_then(|cid| col_names.get(cid))
                    .cloned()
                    .unwrap_or_else(|| "default".to_string());

                let scope = format!("org/{org_name}/{col_name}");
                let enc_org = org_name.replace('/', "%2F");
                let enc_name = dec_name.replace('/', "%2F");
                let prefix = format!("tacet://org/{enc_org}/{col_name}/{enc_name}");
                (scope, prefix)
            };

            // Apply scope filter if specified
            if let Some(scope_req) = scope_filter {
                let trimmed = scope_req.trim();
                if !trimmed.is_empty() && trimmed != "*" {
                    if trimmed.eq_ignore_ascii_case("personal") {
                        if !is_personal {
                            continue;
                        }
                    } else if let Some(subfolder) = trimmed.strip_prefix("personal/") {
                        if !is_personal {
                            continue;
                        }
                        match &folder_name {
                            Some(fn_str) if fn_str.eq_ignore_ascii_case(subfolder) => {}
                            _ => continue,
                        }
                    } else if let Some(org_filter) = trimmed.strip_prefix("org/") {
                        if is_personal {
                            continue;
                        }
                        if let Some((target_org, target_col)) = org_filter.split_once('/') {
                            if !scope_str
                                .eq_ignore_ascii_case(&format!("org/{target_org}/{target_col}"))
                            {
                                continue;
                            }
                        } else if !scope_str
                            .to_lowercase()
                            .starts_with(&format!("org/{}", org_filter.to_lowercase()))
                        {
                            continue;
                        }
                    } else {
                        let matches_folder = folder_name
                            .as_ref()
                            .map(|f| f.eq_ignore_ascii_case(trimmed))
                            .unwrap_or(false);
                        let matches_scope = scope_str.eq_ignore_ascii_case(trimmed);
                        if !matches_folder && !matches_scope {
                            continue;
                        }
                    }
                }
            }

            // Apply search filter if specified
            if let Some(query) = search_filter {
                let q = query.trim().to_lowercase();
                if !q.is_empty() {
                    let name_match = dec_name.to_lowercase().contains(&q);
                    let id_match = cipher.id.to_lowercase().contains(&q);
                    let folder_match = folder_name
                        .as_ref()
                        .map(|f| f.to_lowercase().contains(&q))
                        .unwrap_or(false);
                    let scope_match = scope_str.to_lowercase().contains(&q);

                    if !name_match && !id_match && !folder_match && !scope_match {
                        continue;
                    }
                }
            }

            let type_str = match cipher.cipher_type {
                1 => "Login",
                2 => "SecureNote",
                3 => "Card",
                4 => "Identity",
                _ => "Other",
            }
            .to_string();

            items.push(VaultItemSummary {
                id: cipher.id.clone(),
                name: dec_name,
                scope: scope_str,
                folder: folder_name,
                cipher_type: type_str,
                uri_prefix,
            });
        }

        items.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
        Ok(items)
    }

    fn filter_scope<'s>(
        &'s self,
        scope: &Scope,
    ) -> Result<Vec<(&'a CipherResponse, &'s SymmetricKey)>, ResolverError> {
        match scope {
            Scope::Personal { folder } => {
                let personal_ciphers: Vec<&'a CipherResponse> = self
                    .vault
                    .ciphers
                    .iter()
                    .filter(|c| c.organization_id.is_none())
                    .collect();

                if let Some(folder_name) = folder {
                    // Match folder by decrypted name
                    let target_folder_id = self.find_folder_id(folder_name)?;
                    let filtered = personal_ciphers
                        .into_iter()
                        .filter(|c| c.folder_id.as_deref() == Some(&target_folder_id))
                        .map(|c| (c, self.user_key))
                        .collect();
                    Ok(filtered)
                } else {
                    Ok(personal_ciphers
                        .into_iter()
                        .map(|c| (c, self.user_key))
                        .collect())
                }
            }
            Scope::Organization {
                org_name,
                collection,
            } => {
                let (org_id, org_key) = self.find_org_and_key(org_name)?;
                let col_id = self.find_collection_id(&org_id, org_key, collection, org_name)?;

                let filtered = self
                    .vault
                    .ciphers
                    .iter()
                    .filter(|c| {
                        c.organization_id.as_deref() == Some(&org_id)
                            && c.collection_ids.contains(&col_id)
                    })
                    .map(|c| (c, org_key))
                    .collect();

                Ok(filtered)
            }
            Scope::All => {
                let mut all = Vec::new();
                for c in &self.vault.ciphers {
                    if let Some(org_id) = &c.organization_id {
                        if let Some(key) = self.org_keys.get(org_id) {
                            all.push((c, key));
                        }
                    } else {
                        all.push((c, self.user_key));
                    }
                }
                Ok(all)
            }
        }
    }

    fn find_folder_id(&self, folder_name: &str) -> Result<String, ResolverError> {
        for f in &self.vault.folders {
            if let Ok(dec_name) = self.user_key.decrypt_type2_string(&f.name) {
                if dec_name.eq_ignore_ascii_case(folder_name) {
                    return Ok(f.id.clone());
                }
            }
        }
        Err(ResolverError::FolderNotFound {
            folder: folder_name.to_string(),
        })
    }

    fn find_org_and_key(&self, org_name: &str) -> Result<(String, &SymmetricKey), ResolverError> {
        let orgs = self
            .vault
            .profile
            .as_ref()
            .map(|p| &p.organizations)
            .ok_or_else(|| ResolverError::OrganizationNotFound {
                org: org_name.to_string(),
            })?;

        for org in orgs {
            if org.name.eq_ignore_ascii_case(org_name) || org.id.eq_ignore_ascii_case(org_name) {
                let key = self
                    .org_keys
                    .get(&org.id)
                    .ok_or_else(|| ResolverError::MissingOrgKey(org_name.to_string()))?;
                return Ok((org.id.clone(), key));
            }
        }

        Err(ResolverError::OrganizationNotFound {
            org: org_name.to_string(),
        })
    }

    fn find_collection_id(
        &self,
        org_id: &str,
        org_key: &SymmetricKey,
        col_name: &str,
        org_name: &str,
    ) -> Result<String, ResolverError> {
        for col in &self.vault.collections {
            if col.organization_id == org_id {
                // Collections are encrypted with organization key or user key
                let dec_name = org_key
                    .decrypt_type2_string(&col.name)
                    .or_else(|_| self.user_key.decrypt_type2_string(&col.name));

                if let Ok(name) = dec_name {
                    if name.eq_ignore_ascii_case(col_name) {
                        return Ok(col.id.clone());
                    }
                }
            }
        }

        Err(ResolverError::CollectionNotFound {
            collection: col_name.to_string(),
            org: org_name.to_string(),
        })
    }

    fn match_item<'s>(
        &'s self,
        candidates: &[(&'a CipherResponse, &'s SymmetricKey)],
        item_query: &str,
        scope: &Scope,
    ) -> Result<(&'a CipherResponse, &'s SymmetricKey), ResolverError> {
        let is_uuid = item_query.len() == 36 && uuid::Uuid::parse_str(item_query).is_ok();

        let mut matches = Vec::new();
        for &(cipher, key) in candidates {
            if is_uuid && cipher.id.eq_ignore_ascii_case(item_query) {
                matches.push((cipher, key, item_query.to_string()));
            } else if let Ok(dec_name) = key.decrypt_type2_string(&cipher.name) {
                if dec_name.eq_ignore_ascii_case(item_query) {
                    matches.push((cipher, key, dec_name));
                }
            }
        }

        match matches.len() {
            0 => Err(ResolverError::ItemNotFound {
                item: item_query.to_string(),
                scope: scope.to_string(),
            }),
            1 => {
                let (c, k, _) = matches.remove(0);
                Ok((c, k))
            }
            _ => {
                let descriptions = matches
                    .iter()
                    .map(|(c, _, name)| {
                        format!(
                            "UUID: {} (Name: '{}', Folder: {:?}, Org: {:?})",
                            c.id, name, c.folder_id, c.organization_id
                        )
                    })
                    .collect::<Vec<_>>()
                    .join(", ");

                Err(ResolverError::AmbiguousMatch {
                    item: item_query.to_string(),
                    matches: descriptions,
                })
            }
        }
    }

    fn extract_field(
        &self,
        cipher: &CipherResponse,
        key: &SymmetricKey,
        target: &CipherFieldTarget,
    ) -> Result<String, ResolverError> {
        match target {
            CipherFieldTarget::Username => cipher
                .login
                .as_ref()
                .and_then(|l| l.username.as_deref())
                .ok_or_else(|| ResolverError::FieldNotFound {
                    field: "username".to_string(),
                    item: cipher.id.clone(),
                })
                .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto)),
            CipherFieldTarget::Password => cipher
                .login
                .as_ref()
                .and_then(|l| l.password.as_deref())
                .ok_or_else(|| ResolverError::FieldNotFound {
                    field: "password".to_string(),
                    item: cipher.id.clone(),
                })
                .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto)),
            CipherFieldTarget::Totp => {
                let raw_secret = cipher
                    .login
                    .as_ref()
                    .and_then(|l| l.totp.as_deref())
                    .ok_or_else(|| ResolverError::FieldNotFound {
                        field: "totp".to_string(),
                        item: cipher.id.clone(),
                    })
                    .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto))?;
                generate_totp(&raw_secret)
            }
            CipherFieldTarget::TotpSecret => cipher
                .login
                .as_ref()
                .and_then(|l| l.totp.as_deref())
                .ok_or_else(|| ResolverError::FieldNotFound {
                    field: "totp_secret".to_string(),
                    item: cipher.id.clone(),
                })
                .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto)),
            CipherFieldTarget::Uri(idx) => cipher
                .login
                .as_ref()
                .and_then(|l| l.uris.as_ref())
                .and_then(|uris| uris.get(*idx))
                .and_then(|u| u.uri.as_deref())
                .ok_or_else(|| ResolverError::FieldNotFound {
                    field: format!("uri.{idx}"),
                    item: cipher.id.clone(),
                })
                .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto)),
            CipherFieldTarget::Notes => cipher
                .notes
                .as_deref()
                .ok_or_else(|| ResolverError::FieldNotFound {
                    field: "notes".to_string(),
                    item: cipher.id.clone(),
                })
                .and_then(|cs| key.decrypt_type2_string(cs).map_err(ResolverError::Crypto)),
            CipherFieldTarget::CustomField(name) => self.find_custom_field(cipher, key, name),
            CipherFieldTarget::Named(name) => {
                // Checks built-in canonical fields first, then falls back to custom field
                match name.to_lowercase().as_str() {
                    "username" => self.extract_field(cipher, key, &CipherFieldTarget::Username),
                    "password" => self.extract_field(cipher, key, &CipherFieldTarget::Password),
                    "totp" => self.extract_field(cipher, key, &CipherFieldTarget::Totp),
                    "totp_secret" => {
                        self.extract_field(cipher, key, &CipherFieldTarget::TotpSecret)
                    }
                    "notes" => self.extract_field(cipher, key, &CipherFieldTarget::Notes),
                    "uri" => self.extract_field(cipher, key, &CipherFieldTarget::Uri(0)),
                    _ => self.find_custom_field(cipher, key, name),
                }
            }
        }
    }

    fn find_custom_field(
        &self,
        cipher: &CipherResponse,
        key: &SymmetricKey,
        field_name: &str,
    ) -> Result<String, ResolverError> {
        if let Some(fields) = &cipher.fields {
            for f in fields {
                if let (Some(enc_name), Some(enc_val)) = (&f.name, &f.value) {
                    if let Ok(dec_name) = key.decrypt_type2_string(enc_name) {
                        if dec_name.eq_ignore_ascii_case(field_name) {
                            return key
                                .decrypt_type2_string(enc_val)
                                .map_err(ResolverError::Crypto);
                        }
                    }
                }
            }
        }

        Err(ResolverError::FieldNotFound {
            field: field_name.to_string(),
            item: cipher.id.clone(),
        })
    }

    fn apply_modifiers(&self, mut val: String, uri: &TacetUri) -> Result<String, ResolverError> {
        // 1. JSON extraction
        if let Some(dot_path) = &uri.modifiers.json {
            val = extract_json_path(&val, dot_path)?;
        }

        // 2. Whitespace trimming (default true)
        if uri.modifiers.trim {
            val = val.trim().to_string();
        }

        // 3. Encoding modifier
        match uri.modifiers.encoding {
            Some(Encoding::Base64) => {
                val = BASE64_STANDARD.encode(val.as_bytes());
            }
            Some(Encoding::Hex) => {
                val = val
                    .as_bytes()
                    .iter()
                    .map(|b| format!("{b:02x}"))
                    .collect::<String>();
            }
            None => {}
        }

        Ok(val)
    }
}
