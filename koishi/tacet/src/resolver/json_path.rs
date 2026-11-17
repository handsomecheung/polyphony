use crate::error::ResolverError;
use serde_json::Value;

/// Evaluates a dot-delimited path on a JSON string and extracts the target value.
pub fn extract_json_path(json_str: &str, path: &str) -> Result<String, ResolverError> {
    let parsed: Value = serde_json::from_str(json_str).map_err(|e| {
        ResolverError::JsonPathError(format!("Failed to parse field value as JSON: {e}"))
    })?;

    let mut current = &parsed;
    for segment in path.split('.') {
        if segment.is_empty() {
            continue;
        }

        match current {
            Value::Object(map) => {
                current = map.get(segment).ok_or_else(|| {
                    ResolverError::JsonPathError(format!(
                        "Key '{segment}' not found in JSON object"
                    ))
                })?;
            }
            Value::Array(list) => {
                let idx: usize = segment.parse().map_err(|_| {
                    ResolverError::JsonPathError(format!("Expected array index, got '{segment}'"))
                })?;
                current = list.get(idx).ok_or_else(|| {
                    ResolverError::JsonPathError(format!("Array index {idx} out of bounds"))
                })?;
            }
            _ => {
                return Err(ResolverError::JsonPathError(format!(
                    "Cannot traverse segment '{segment}' on non-container JSON value"
                )))
            }
        }
    }

    match current {
        Value::String(s) => Ok(s.clone()),
        Value::Null => Ok("".to_string()),
        other => Ok(other.to_string()),
    }
}
