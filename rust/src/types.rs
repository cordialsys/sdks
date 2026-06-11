use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::fmt;
use std::str::FromStr;

/// OpenAPI types generated from `../openapi/treasury.yaml`.
pub mod openapi;

pub type Object = Map<String, Value>;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AccountVariant {
    Internal,
    Shared,
    External,
    Contract,
    Validator,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AddressVariant {
    Internal,
    Shared,
    External,
    Contract,
    Validator,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum RuleVariant {
    Allow,
    Require,
    Deny,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum KeyVariant {
    Engine,
    Shared,
    User,
    Internal,
    System,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum CredentialVariant {
    K256,
    P256,
    Invite,
    Ed255,
    WebAuthn,
    WebAuthnUv,
    Session,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum UserVariant {
    Human,
    Machine,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum AssetVariant {
    Native,
    Token,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum ChainVariant {
    Native,
    Custom,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "kebab-case")]
pub enum StakingVariant {
    Stake,
    Unstake,
    Withdraw,
}

macro_rules! impl_variant_from_str {
    ($ty:ty => {$($wire:literal => $variant:ident),+ $(,)?}) => {
        impl FromStr for $ty {
            type Err = String;

            fn from_str(s: &str) -> Result<Self, Self::Err> {
                match s {
                    $($wire => Ok(Self::$variant),)+
                    _ => Err(format!("invalid {}: {}", stringify!($ty), s)),
                }
            }
        }
    };
}

impl_variant_from_str!(AccountVariant => {
    "internal" => Internal,
    "shared" => Shared,
    "external" => External,
    "contract" => Contract,
    "validator" => Validator,
});
impl_variant_from_str!(AddressVariant => {
    "internal" => Internal,
    "shared" => Shared,
    "external" => External,
    "contract" => Contract,
    "validator" => Validator,
});
impl_variant_from_str!(RuleVariant => {
    "allow" => Allow,
    "require" => Require,
    "deny" => Deny,
});
impl_variant_from_str!(KeyVariant => {
    "engine" => Engine,
    "shared" => Shared,
    "user" => User,
    "internal" => Internal,
    "system" => System,
});
impl_variant_from_str!(CredentialVariant => {
    "k256" => K256,
    "p256" => P256,
    "invite" => Invite,
    "ed255" => Ed255,
    "web-authn" => WebAuthn,
    "web-authn-uv" => WebAuthnUv,
    "session" => Session,
});
impl_variant_from_str!(UserVariant => {
    "human" => Human,
    "machine" => Machine,
});
impl_variant_from_str!(AssetVariant => {
    "native" => Native,
    "token" => Token,
});
impl_variant_from_str!(ChainVariant => {
    "native" => Native,
    "custom" => Custom,
});
impl_variant_from_str!(StakingVariant => {
    "stake" => Stake,
    "unstake" => Unstake,
    "withdraw" => Withdraw,
});

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct Metadata {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub display_name: Option<String>,
    #[serde(default, skip_serializing_if = "Object::is_empty")]
    pub labels: Object,
    #[serde(default, skip_serializing_if = "Object::is_empty")]
    pub annotations: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateAccountRequest {
    pub variant: AccountVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateAddressRequest {
    pub variant: AddressVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateRuleRequest {
    pub variant: RuleVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateKeyRequest {
    pub variant: KeyVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateCredentialRequest {
    pub variant: CredentialVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateUserRequest {
    pub variant: UserVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateAssetRequest {
    pub variant: AssetVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateChainRequest {
    pub variant: ChainVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateStakingRequest {
    pub variant: StakingVariant,
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateResourceRequest {
    #[serde(flatten)]
    pub metadata: Metadata,
    #[serde(flatten)]
    pub data: Object,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ErrorResponse {
    pub code: Value,
    pub status: String,
    pub message: String,
    #[serde(default)]
    pub details: Vec<Value>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ResourceType {
    AccessRule,
    Account,
    Address,
    Asset,
    Call,
    CallRule,
    Chain,
    Credential,
    Feature,
    Host,
    Key,
    Operation,
    Role,
    Signatory,
    Signature,
    Signer,
    SoftwareUpdate,
    Staking,
    StakingRule,
    Symbol,
    Tag,
    Transaction,
    Transfer,
    TransferRule,
    Treasury,
    Type,
    User,
}

impl ResourceType {
    pub fn singular(self) -> &'static str {
        match self {
            Self::AccessRule => "access-rule",
            Self::Account => "account",
            Self::Address => "address",
            Self::Asset => "asset",
            Self::Call => "call",
            Self::CallRule => "call-rule",
            Self::Chain => "chain",
            Self::Credential => "credential",
            Self::Feature => "feature",
            Self::Host => "host",
            Self::Key => "key",
            Self::Operation => "operation",
            Self::Role => "role",
            Self::Signatory => "signatory",
            Self::Signature => "signature",
            Self::Signer => "signer",
            Self::SoftwareUpdate => "software-update",
            Self::Staking => "staking",
            Self::StakingRule => "staking-rule",
            Self::Symbol => "symbol",
            Self::Tag => "tag",
            Self::Transaction => "transaction",
            Self::Transfer => "transfer",
            Self::TransferRule => "transfer-rule",
            Self::Treasury => "treasury",
            Self::Type => "type",
            Self::User => "user",
        }
    }

    pub fn path(self) -> &'static str {
        match self {
            Self::AccessRule => "access-rules",
            Self::Account => "accounts",
            Self::Address => "addresses",
            Self::Asset => "assets",
            Self::Call => "calls",
            Self::CallRule => "call-rules",
            Self::Chain => "chains",
            Self::Credential => "credentials",
            Self::Feature => "features",
            Self::Host => "hosts",
            Self::Key => "keys",
            Self::Operation => "operations",
            Self::Role => "roles",
            Self::Signatory => "signatories",
            Self::Signature => "signatures",
            Self::Signer => "signers",
            Self::SoftwareUpdate => "software-updates",
            Self::Staking => "stakings",
            Self::StakingRule => "staking-rules",
            Self::Symbol => "symbols",
            Self::Tag => "tags",
            Self::Transaction => "transactions",
            Self::Transfer => "transfers",
            Self::TransferRule => "transfer-rules",
            Self::Treasury => "treasuries",
            Self::Type => "types",
            Self::User => "users",
        }
    }

    pub fn parent_path(self) -> Option<&'static str> {
        match self {
            Self::Address | Self::Asset | Self::Call | Self::Symbol => Some("chains"),
            Self::Credential => Some("users"),
            _ => None,
        }
    }
}

impl fmt::Display for ResourceType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.singular())
    }
}

impl FromStr for ResourceType {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "access-rule" | "access rule" | "access-rules" | "access rules" => Ok(Self::AccessRule),
            "account" | "accounts" => Ok(Self::Account),
            "address" | "addresses" => Ok(Self::Address),
            "asset" | "assets" => Ok(Self::Asset),
            "call" | "calls" => Ok(Self::Call),
            "call-rule" | "call rule" | "call-rules" | "call rules" => Ok(Self::CallRule),
            "chain" | "chains" => Ok(Self::Chain),
            "credential" | "credentials" => Ok(Self::Credential),
            "feature" | "features" => Ok(Self::Feature),
            "host" | "hosts" => Ok(Self::Host),
            "key" | "keys" => Ok(Self::Key),
            "operation" | "operations" => Ok(Self::Operation),
            "role" | "roles" => Ok(Self::Role),
            "signatory" | "signatories" => Ok(Self::Signatory),
            "signature" | "signatures" => Ok(Self::Signature),
            "signer" | "signers" => Ok(Self::Signer),
            "software-update" | "software update" | "software-updates" | "software updates" => {
                Ok(Self::SoftwareUpdate)
            }
            "staking" | "stakings" => Ok(Self::Staking),
            "staking-rule" | "staking rule" | "staking-rules" | "staking rules" => {
                Ok(Self::StakingRule)
            }
            "symbol" | "symbols" => Ok(Self::Symbol),
            "tag" | "tags" => Ok(Self::Tag),
            "transaction" | "transactions" => Ok(Self::Transaction),
            "transfer" | "transfers" => Ok(Self::Transfer),
            "transfer-rule" | "transfer rule" | "transfer-rules" | "transfer rules" => {
                Ok(Self::TransferRule)
            }
            "treasury" | "treasuries" => Ok(Self::Treasury),
            "type" | "types" => Ok(Self::Type),
            "user" | "users" => Ok(Self::User),
            _ => Err(format!("unknown resource type: {s}")),
        }
    }
}
