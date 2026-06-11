use crate::signing::{Identity, SigningAlgorithm, SigningError};
use serde::Deserialize;
use std::fs;
use std::path::{Path, PathBuf};
use std::str::FromStr;

#[derive(Debug, thiserror::Error)]
pub enum KeyringError {
    #[error("home directory not found")]
    MissingHome,
    #[error("io: {0}")]
    Io(#[from] std::io::Error),
    #[error("toml: {0}")]
    Toml(#[from] toml::de::Error),
    #[error("signing: {0}")]
    Signing(#[from] SigningError),
    #[error("{0}")]
    Invalid(String),
}

#[derive(Debug, Clone)]
pub struct Keyring {
    dir: PathBuf,
}

impl Keyring {
    pub fn new(treasury_id: impl AsRef<str>) -> Result<Self, KeyringError> {
        let home = home::home_dir().ok_or(KeyringError::MissingHome)?;
        Ok(Self {
            dir: home
                .join(".local")
                .join("share")
                .join("treasury")
                .join(treasury_id.as_ref())
                .join("keyring"),
        })
    }

    pub fn from_dir(dir: impl Into<PathBuf>) -> Self {
        Self { dir: dir.into() }
    }

    pub fn dir(&self) -> &Path {
        &self.dir
    }

    pub fn generate_key(
        &self,
        name: &str,
        algorithm: SigningAlgorithm,
    ) -> Result<Identity, KeyringError> {
        fs::create_dir_all(&self.dir)?;
        let identity = match algorithm {
            SigningAlgorithm::EcdsaK256Sha256 => Identity::generate_k256(name),
            SigningAlgorithm::EcdsaP256Sha256 => Identity::generate_p256(name),
            SigningAlgorithm::Ed25519 => Identity::generate_ed25519(name),
        };
        self.save_key(name, &identity)?;
        Ok(identity)
    }

    pub fn save_key(&self, name: &str, identity: &Identity) -> Result<(), KeyringError> {
        fs::create_dir_all(&self.dir)?;
        let content = format!(
            "algorithm = {:?}\nsecret_key = {:?}\npublic_key = {:?}\n",
            identity.algorithm.to_string(),
            identity.private_key_hex(),
            identity.public_key_hex
        );
        fs::write(self.key_path(name), content)?;
        Ok(())
    }

    pub fn get_key(&self, name: &str) -> Result<Identity, KeyringError> {
        let content = fs::read_to_string(self.key_path(name))?;
        let stored: StoredKey = toml::from_str(&content)?;
        let algorithm =
            SigningAlgorithm::from_str(&stored.algorithm).map_err(KeyringError::Invalid)?;
        let identity = match algorithm {
            SigningAlgorithm::EcdsaK256Sha256 => Identity::load_k256(name, &stored.secret_key)?,
            SigningAlgorithm::EcdsaP256Sha256 => Identity::load_p256(name, &stored.secret_key)?,
            SigningAlgorithm::Ed25519 => Identity::load_ed25519(name, &stored.secret_key)?,
        };
        if let Some(public_key) = stored.public_key
            && public_key != identity.public_key_hex
        {
            return Err(KeyringError::Invalid(format!(
                "stored public key {public_key} does not match derived public key {}",
                identity.public_key_hex
            )));
        }
        Ok(identity)
    }

    pub fn list_keys(&self) -> Result<Vec<String>, KeyringError> {
        let mut names = Vec::new();
        if !self.dir.exists() {
            return Ok(names);
        }
        for entry in fs::read_dir(&self.dir)? {
            let entry = entry?;
            if entry.file_type()?.is_file()
                && let Some(name) = entry.path().file_stem().and_then(|v| v.to_str())
            {
                names.push(name.to_string());
            }
        }
        names.sort();
        Ok(names)
    }

    pub fn delete_key(&self, name: &str) -> Result<(), KeyringError> {
        fs::remove_file(self.key_path(name))?;
        Ok(())
    }

    fn key_path(&self, name: &str) -> PathBuf {
        self.dir.join(format!("{name}.toml"))
    }
}

#[derive(Debug, Deserialize)]
struct StoredKey {
    algorithm: String,
    secret_key: String,
    public_key: Option<String>,
}
