use crate::error::StorageError;
use directories::ProjectDirs;
use std::fs;
use std::path::{Path, PathBuf};

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

#[derive(Debug, Clone)]
pub struct StoragePaths {
    pub config_dir: PathBuf,
    pub cache_dir: PathBuf,
}

impl StoragePaths {
    /// Creates default storage paths following XDG standards:
    /// - config_dir: `~/.config/tacet/`
    /// - cache_dir: `~/.cache/tacet/`
    pub fn default_paths() -> Result<Self, StorageError> {
        if let Some(proj_dirs) = ProjectDirs::from("", "", "tacet") {
            Ok(Self {
                config_dir: proj_dirs.config_dir().to_path_buf(),
                cache_dir: proj_dirs.cache_dir().to_path_buf(),
            })
        } else {
            let home = std::env::var("HOME").map_err(|_| StorageError::DirectoryNotFound)?;
            let home_path = PathBuf::from(home);
            Ok(Self {
                config_dir: home_path.join(".config").join("tacet"),
                cache_dir: home_path.join(".cache").join("tacet"),
            })
        }
    }

    /// Creates custom storage paths (e.g. for testing with temporary directories).
    pub fn custom(config_dir: PathBuf, cache_dir: PathBuf) -> Self {
        Self {
            config_dir,
            cache_dir,
        }
    }

    pub fn session_file(&self) -> PathBuf {
        self.config_dir.join("session.json")
    }

    pub fn vault_cache_file(&self) -> PathBuf {
        self.cache_dir.join("vault.cache")
    }

    pub fn device_id_file(&self) -> PathBuf {
        self.config_dir.join("device_id")
    }

    /// Retrieves or generates a persistent UUID for device tracking with Bitwarden/Vaultwarden.
    pub fn get_or_create_device_id(&self) -> Result<String, StorageError> {
        self.ensure_dirs()?;
        let file = self.device_id_file();
        if file.exists() {
            if let Ok(content) = fs::read_to_string(&file) {
                let id = content.trim();
                if !id.is_empty() {
                    return Ok(id.to_string());
                }
            }
        }

        let new_id = uuid::Uuid::new_v4().to_string();
        Self::write_secure_file(&file, new_id.as_bytes())?;
        Ok(new_id)
    }

    /// Ensures config and cache directories exist with mode 0700 on Unix.
    pub fn ensure_dirs(&self) -> Result<(), StorageError> {
        fs::create_dir_all(&self.config_dir)?;
        fs::create_dir_all(&self.cache_dir)?;

        #[cfg(unix)]
        {
            fs::set_permissions(&self.config_dir, fs::Permissions::from_mode(0o700))?;
            fs::set_permissions(&self.cache_dir, fs::Permissions::from_mode(0o700))?;
        }

        Ok(())
    }

    /// Writes data to a file securely with mode 0600 on Unix.
    pub fn write_secure_file<P: AsRef<Path>>(path: P, content: &[u8]) -> Result<(), StorageError> {
        let p = path.as_ref();
        if let Some(parent) = p.parent() {
            fs::create_dir_all(parent)?;
            #[cfg(unix)]
            let _ = fs::set_permissions(parent, fs::Permissions::from_mode(0o700));
        }

        fs::write(p, content)?;

        #[cfg(unix)]
        fs::set_permissions(p, fs::Permissions::from_mode(0o600))?;

        Ok(())
    }
}
