use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use ed25519_dalek::{Signature as Ed25519Signature, SigningKey as Ed25519SigningKey, VerifyingKey};
use k256::ecdsa::signature::Signer as KSigner;
use k256::ecdsa::{Signature as K256Signature, SigningKey as K256SigningKey};
use p256::ecdsa::{Signature as P256Signature, SigningKey as P256SigningKey};
use rand_core::OsRng;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fmt;
use std::str::FromStr;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum SigningAlgorithm {
    EcdsaK256Sha256,
    EcdsaP256Sha256,
    Ed25519,
}

impl SigningAlgorithm {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::EcdsaK256Sha256 => "ecdsa-k256-sha256",
            Self::EcdsaP256Sha256 => "ecdsa-p256-sha256",
            Self::Ed25519 => "ed25519",
        }
    }
}

impl fmt::Display for SigningAlgorithm {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

impl FromStr for SigningAlgorithm {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "ecdsa-k256-sha256" => Ok(Self::EcdsaK256Sha256),
            "ecdsa-p256-sha256" => Ok(Self::EcdsaP256Sha256),
            "ed25519" => Ok(Self::Ed25519),
            _ => Err(format!("unsupported signing algorithm: {s}")),
        }
    }
}

#[derive(Debug, thiserror::Error)]
pub enum SigningError {
    #[error("hex decode: {0}")]
    Hex(#[from] hex::FromHexError),
    #[error("invalid key: {0}")]
    InvalidKey(String),
    #[error("signing failed: {0}")]
    Sign(String),
}

pub trait Signer: Send + Sync {
    fn public_key(&self) -> &str;
    fn signing_algorithm(&self) -> SigningAlgorithm;
    fn sign_http_message(&self, signature_base: &[u8]) -> Result<Vec<u8>, SigningError>;
}

#[derive(Clone)]
enum PrivateKey {
    K256(K256SigningKey),
    P256(P256SigningKey),
    Ed25519(Ed25519SigningKey),
}

#[derive(Clone)]
pub struct Identity {
    pub user_name: String,
    pub public_key_hex: String,
    pub algorithm: SigningAlgorithm,
    private_key: PrivateKey,
}

impl Identity {
    pub fn generate_k256(user_name: impl Into<String>) -> Self {
        let key = K256SigningKey::random(&mut OsRng);
        let public_key_hex = hex::encode(key.verifying_key().to_encoded_point(true).as_bytes());
        Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::EcdsaK256Sha256,
            private_key: PrivateKey::K256(key),
        }
    }

    pub fn generate_p256(user_name: impl Into<String>) -> Self {
        let key = P256SigningKey::random(&mut OsRng);
        let public_key_hex = hex::encode(key.verifying_key().to_encoded_point(true).as_bytes());
        Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::EcdsaP256Sha256,
            private_key: PrivateKey::P256(key),
        }
    }

    pub fn generate_ed25519(user_name: impl Into<String>) -> Self {
        let key = Ed25519SigningKey::generate(&mut OsRng);
        let public_key_hex = hex::encode(key.verifying_key().to_bytes());
        Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::Ed25519,
            private_key: PrivateKey::Ed25519(key),
        }
    }

    pub fn load_k256(user_name: impl Into<String>, secret_hex: &str) -> Result<Self, SigningError> {
        let bytes = hex::decode(secret_hex)?;
        let key = K256SigningKey::from_slice(&bytes)
            .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
        let public_key_hex = hex::encode(key.verifying_key().to_encoded_point(true).as_bytes());
        Ok(Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::EcdsaK256Sha256,
            private_key: PrivateKey::K256(key),
        })
    }

    pub fn load_p256(user_name: impl Into<String>, secret_hex: &str) -> Result<Self, SigningError> {
        let bytes = hex::decode(secret_hex)?;
        let key = P256SigningKey::from_slice(&bytes)
            .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
        let public_key_hex = hex::encode(key.verifying_key().to_encoded_point(true).as_bytes());
        Ok(Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::EcdsaP256Sha256,
            private_key: PrivateKey::P256(key),
        })
    }

    pub fn load_ed25519(
        user_name: impl Into<String>,
        seed_hex: &str,
    ) -> Result<Self, SigningError> {
        let bytes = hex::decode(seed_hex)?;
        let seed: [u8; 32] = bytes
            .try_into()
            .map_err(|_| SigningError::InvalidKey("ed25519 seed must be 32 bytes".to_string()))?;
        let key = Ed25519SigningKey::from_bytes(&seed);
        let public_key_hex = hex::encode(key.verifying_key().to_bytes());
        Ok(Self {
            user_name: user_name.into(),
            public_key_hex,
            algorithm: SigningAlgorithm::Ed25519,
            private_key: PrivateKey::Ed25519(key),
        })
    }

    pub fn private_key_hex(&self) -> String {
        match &self.private_key {
            PrivateKey::K256(key) => hex::encode(key.to_bytes()),
            PrivateKey::P256(key) => hex::encode(key.to_bytes()),
            PrivateKey::Ed25519(key) => hex::encode(key.to_bytes()),
        }
    }

    pub fn verify(&self, message: &[u8], signature: &[u8]) -> Result<bool, SigningError> {
        match &self.private_key {
            PrivateKey::K256(key) => {
                use k256::ecdsa::signature::Verifier;
                let sig = K256Signature::from_slice(signature)
                    .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
                Ok(key.verifying_key().verify(message, &sig).is_ok())
            }
            PrivateKey::P256(key) => {
                use p256::ecdsa::signature::Verifier;
                let sig = P256Signature::from_slice(signature)
                    .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
                Ok(key.verifying_key().verify(message, &sig).is_ok())
            }
            PrivateKey::Ed25519(_) => {
                let public = hex::decode(&self.public_key_hex)?;
                let public: [u8; 32] = public.try_into().map_err(|_| {
                    SigningError::InvalidKey("ed25519 public key must be 32 bytes".to_string())
                })?;
                let verifying_key = VerifyingKey::from_bytes(&public)
                    .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
                let sig = Ed25519Signature::from_slice(signature)
                    .map_err(|err| SigningError::InvalidKey(err.to_string()))?;
                Ok(verifying_key.verify_strict(message, &sig).is_ok())
            }
        }
    }
}

impl Signer for Identity {
    fn public_key(&self) -> &str {
        &self.public_key_hex
    }

    fn signing_algorithm(&self) -> SigningAlgorithm {
        self.algorithm
    }

    fn sign_http_message(&self, signature_base: &[u8]) -> Result<Vec<u8>, SigningError> {
        match &self.private_key {
            PrivateKey::K256(key) => {
                let sig: K256Signature = key.sign(signature_base);
                Ok(sig.to_bytes().to_vec())
            }
            PrivateKey::P256(key) => {
                let sig: P256Signature = key.sign(signature_base);
                Ok(sig.to_bytes().to_vec())
            }
            PrivateKey::Ed25519(key) => {
                use ed25519_dalek::Signer as _;
                Ok(key.sign(signature_base).to_bytes().to_vec())
            }
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SignatureHeaders {
    pub content_digest: String,
    pub signature_input: String,
    pub signature: String,
}

pub fn sign_request(
    signer: &dyn Signer,
    method: &str,
    path_and_query: &str,
    body: &[u8],
    treasury_id: &str,
    host_id: Option<&str>,
    tag: Option<&str>,
) -> Result<SignatureHeaders, SigningError> {
    let digest = Sha256::digest(body);
    let content_digest = format!("sha-256=:{}:", BASE64.encode(digest));

    let mut covered = vec![
        "\"@method\"",
        "\"@path\"",
        "\"content-digest\"",
        "\"treasury\"",
    ];
    if host_id.is_some() {
        covered.push("\"treasury-host\"");
    }

    let mut base = String::new();
    base.push_str(&format!("\"@method\": {}\n", method.to_uppercase()));
    base.push_str(&format!("\"@path\": {path_and_query}\n"));
    base.push_str(&format!("\"content-digest\": {content_digest}\n"));
    base.push_str(&format!("\"treasury\": {treasury_id}\n"));
    if let Some(host_id) = host_id {
        base.push_str(&format!("\"treasury-host\": {host_id}\n"));
    }

    let params = format!(
        "({});keyid=\"{}\";alg=\"{}\"{}",
        covered.join(" "),
        signer.public_key(),
        signer.signing_algorithm(),
        tag.map(|tag| format!(";tag=\"{tag}\"")).unwrap_or_default()
    );
    base.push_str(&format!("\"@signature-params\": {params}"));

    let signature = signer.sign_http_message(base.as_bytes())?;
    Ok(SignatureHeaders {
        content_digest,
        signature_input: format!("sig1={params}"),
        signature: format!("sig1=:{}:", BASE64.encode(signature)),
    })
}
