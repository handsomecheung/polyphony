use crate::error::UriError;
use percent_encoding::percent_decode_str;
use std::fmt;
use std::str::FromStr;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Scope {
    /// `personal`: Personal vault (organizationId == null)
    Personal { folder: Option<String> },
    /// `org/<org_name>/<collection>`: Shared items inside an Organization's collection
    Organization {
        org_name: String,
        collection: String,
    },
    /// `*`: Search all accessible vaults (fails if ambiguous)
    All,
}

impl fmt::Display for Scope {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Scope::Personal { folder: None } => write!(f, "personal"),
            Scope::Personal {
                folder: Some(folder),
            } => write!(f, "personal/{folder}"),
            Scope::Organization {
                org_name,
                collection,
            } => write!(f, "org/{org_name}/{collection}"),
            Scope::All => write!(f, "*"),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CipherFieldTarget {
    Username,
    Password,
    Totp,
    TotpSecret,
    Uri(usize),
    Notes,
    CustomField(String),
    Named(String),
}

impl FromStr for CipherFieldTarget {
    type Err = std::convert::Infallible;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let trimmed = s.trim();
        if let Some(field_name) = trimmed.strip_prefix("fields/") {
            return Ok(CipherFieldTarget::CustomField(field_name.to_string()));
        }

        let lower = trimmed.to_lowercase();
        match lower.as_str() {
            "username" => Ok(CipherFieldTarget::Username),
            "password" => Ok(CipherFieldTarget::Password),
            "totp" => Ok(CipherFieldTarget::Totp),
            "totp_secret" => Ok(CipherFieldTarget::TotpSecret),
            "notes" => Ok(CipherFieldTarget::Notes),
            "uri" | "uri.0" => Ok(CipherFieldTarget::Uri(0)),
            other => {
                if let Some(idx_str) = other.strip_prefix("uri.") {
                    if let Ok(idx) = idx_str.parse::<usize>() {
                        return Ok(CipherFieldTarget::Uri(idx));
                    }
                }
                Ok(CipherFieldTarget::Named(trimmed.to_string()))
            }
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Encoding {
    Base64,
    Hex,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Modifiers {
    pub encoding: Option<Encoding>,
    pub json: Option<String>,
    pub trim: bool,
}

impl Default for Modifiers {
    fn default() -> Self {
        Self {
            encoding: None,
            json: None,
            trim: true,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TacetUri {
    pub raw: String,
    pub scope: Scope,
    pub item: String,
    pub field: CipherFieldTarget,
    pub modifiers: Modifiers,
}

impl FromStr for TacetUri {
    type Err = UriError;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        Self::parse(s)
    }
}

impl TacetUri {
    pub fn parse(input: &str) -> Result<Self, UriError> {
        let trimmed = input.trim();
        let rest = trimmed
            .strip_prefix("tacet://")
            .ok_or(UriError::InvalidScheme)?;

        let (path_str, query_opt) = match rest.split_once('?') {
            Some((p, q)) => (p, Some(q)),
            None => (rest, None),
        };

        // Parse modifiers
        let mut modifiers = Modifiers::default();
        if let Some(query) = query_opt {
            for param in query.split('&') {
                if param.is_empty() {
                    continue;
                }
                let (k, v) = param
                    .split_once('=')
                    .ok_or_else(|| UriError::InvalidModifier(param.to_string()))?;

                match k.to_lowercase().as_str() {
                    "encoding" => match v.to_lowercase().as_str() {
                        "base64" => modifiers.encoding = Some(Encoding::Base64),
                        "hex" => modifiers.encoding = Some(Encoding::Hex),
                        other => {
                            return Err(UriError::InvalidModifier(format!(
                                "Unknown encoding: {other}"
                            )))
                        }
                    },
                    "json" => {
                        modifiers.json = Some(v.to_string());
                    }
                    "trim" => match v.to_lowercase().as_str() {
                        "true" | "1" => modifiers.trim = true,
                        "false" | "0" => modifiers.trim = false,
                        other => {
                            return Err(UriError::InvalidModifier(format!(
                                "Invalid trim value: {other}"
                            )))
                        }
                    },
                    other => {
                        return Err(UriError::InvalidModifier(format!(
                            "Unknown modifier: {other}"
                        )))
                    }
                }
            }
        }

        // Split path segments by '/' and percent-decode each segment
        let raw_segments: Vec<&str> = path_str
            .trim_start_matches('/')
            .split('/')
            .filter(|s| !s.is_empty())
            .collect();

        if raw_segments.is_empty() {
            return Err(UriError::MissingComponent("Empty URI path"));
        }

        let segments: Vec<String> = raw_segments
            .into_iter()
            .map(|s| percent_decode_str(s).decode_utf8_lossy().to_string())
            .collect();

        let (scope, item, field_str) = match segments[0].as_str() {
            "org" => {
                if segments.len() < 5 {
                    return Err(UriError::MissingComponent(
                        "Organization URI requires: org/<org_name>/<collection>/<item>/<field>",
                    ));
                }
                let scope = Scope::Organization {
                    org_name: segments[1].clone(),
                    collection: segments[2].clone(),
                };
                let item = segments[3].clone();
                let field_str = if segments[4] == "fields" && segments.len() >= 6 {
                    format!("fields/{}", segments[5..].join("/"))
                } else {
                    segments[4..].join("/")
                };
                (scope, item, field_str)
            }
            "personal" => {
                if segments.len() < 3 {
                    return Err(UriError::MissingComponent(
                        "Personal URI requires at least item and field",
                    ));
                }
                if segments.len() == 3 {
                    let scope = Scope::Personal { folder: None };
                    let item = segments[1].clone();
                    let field_str = segments[2].clone();
                    (scope, item, field_str)
                } else if segments.len() == 4 {
                    if segments[2] == "fields" {
                        let scope = Scope::Personal { folder: None };
                        let item = segments[1].clone();
                        let field_str = format!("fields/{}", segments[3]);
                        (scope, item, field_str)
                    } else {
                        let scope = Scope::Personal {
                            folder: Some(segments[1].clone()),
                        };
                        let item = segments[2].clone();
                        let field_str = segments[3].clone();
                        (scope, item, field_str)
                    }
                } else {
                    let folder = segments[1].clone();
                    let item = segments[2].clone();
                    let field_str = if segments[3] == "fields" && segments.len() >= 5 {
                        format!("fields/{}", segments[4..].join("/"))
                    } else {
                        segments[3..].join("/")
                    };
                    (
                        Scope::Personal {
                            folder: Some(folder),
                        },
                        item,
                        field_str,
                    )
                }
            }
            "*" => {
                if segments.len() < 3 {
                    return Err(UriError::MissingComponent("URI requires item and field"));
                }
                let scope = Scope::All;
                let item = segments[1].clone();
                let field_str = if segments[2] == "fields" && segments.len() >= 4 {
                    format!("fields/{}", segments[3..].join("/"))
                } else {
                    segments[2..].join("/")
                };
                (scope, item, field_str)
            }
            other => return Err(UriError::InvalidScope(other.to_string())),
        };

        let field = field_str.parse().unwrap();

        Ok(Self {
            raw: trimmed.to_string(),
            scope,
            item,
            field,
            modifiers,
        })
    }
}
