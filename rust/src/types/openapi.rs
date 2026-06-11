// Generated from ../openapi/treasury.yaml by rust/typegen. Do not edit by hand.
use serde::{Deserialize, Serialize};

/// - `create`: Create resource, client may attempt to select resource ID
/// - `get`: Specific resource
/// - `list`: All resources of a resource type, filtered by parent ID if set
/// - `update`: Modify existing resource (`version` is used engine-side to prevent accidental reversion of concurrent modification attempts)
/// - `delete`: Delete a resource
///
/// Custom actions are defined by resource type, currently: `CustomUserAction` (`custom/heartbeat`), `CustomFeatureAction` (`custom/activate` and `custom/disable`), and , `CustomTransferRuleAction` (`custom/activate` and `custom/disable`).
///
/// `custom/recheck` added in `25.6.3`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum Action {
    #[serde(rename = "create")]
    Create,
    #[serde(rename = "get")]
    Get,
    #[serde(rename = "list")]
    List,
    #[serde(rename = "update")]
    Update,
    #[serde(rename = "delete")]
    Delete,
    #[serde(rename = "custom/activate")]
    CustomActivate,
    #[serde(rename = "custom/disable")]
    CustomDisable,
    #[serde(rename = "custom/abort")]
    CustomAbort,
    #[serde(rename = "custom/retry")]
    CustomRetry,
    #[serde(rename = "custom/price")]
    CustomPrice,
    #[serde(rename = "custom/heartbeat")]
    CustomHeartbeat,
    #[serde(rename = "custom/cancel")]
    CustomCancel,
    #[serde(rename = "custom/recheck")]
    CustomRecheck,
    #[serde(rename = "custom/share")]
    CustomShare,
    #[serde(rename = "custom/load")]
    CustomLoad,
    #[serde(rename = "custom/restore")]
    CustomRestore,
    #[serde(rename = "custom/set")]
    CustomSet,
    #[serde(rename = "custom/fail")]
    CustomFail,
    #[serde(rename = "custom/fee-payer")]
    CustomFeePayer,
    #[serde(rename = "custom/confirm")]
    CustomConfirm,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum AddressState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "registering")]
    Registering,
}

/// `internal` means the engine has the key, `external` means it does not.
/// The `shared` variant is deprecated: it meant that the `key` was `shared` and hence allowed use via the (dangerous) raw signing API. There is now an `allow_dangerous_raw_signing` flag instead.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum AddressVariant {
    #[serde(rename = "internal")]
    Internal,
    #[serde(rename = "shared")]
    Shared,
    #[serde(rename = "external")]
    External,
    #[serde(rename = "contract")]
    Contract,
    #[serde(rename = "validator")]
    Validator,
}

/// Signing algorithm. ed255 and taproot are Schnorr-like, k256* and p256 are ECDSA.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum Algorithm {
    #[serde(rename = "ed255")]
    Ed255,
    #[serde(rename = "k256-keccak")]
    K256Keccak,
    #[serde(rename = "k256-sha2")]
    K256Sha2,
    #[serde(rename = "p256")]
    P256,
    #[serde(rename = "taproot")]
    Taproot,
    #[serde(rename = "bls12-381-g2-blake2")]
    Bls12381G2Blake2,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum AssetVariant {
    #[serde(rename = "native")]
    Native,
    #[serde(rename = "token")]
    Token,
}

/// Active / deleted state for resources that don't have a more specific state machine.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum BasicState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "deleted")]
    Deleted,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum CallMethod {
    #[serde(rename = "eth_sendTransaction")]
    EthSendTransaction,
    #[serde(rename = "eth_signTransaction")]
    EthSignTransaction,
    #[serde(rename = "personal_sign")]
    PersonalSign,
    #[serde(rename = "eth_signTypedData_v4")]
    EthSignTypedDataV4,
    #[serde(rename = "solana:signIn")]
    SolanaSignIn,
    #[serde(rename = "solana:signMessage")]
    SolanaSignMessage,
    #[serde(rename = "solana:signTransaction")]
    SolanaSignTransaction,
    #[serde(rename = "solana:signAndSendTransaction")]
    SolanaSignAndSendTransaction,
    #[serde(rename = "canton:accept")]
    CantonAccept,
}

/// In the `active` state, the subordinate resource is being processed, lookup its state for more details.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum CallState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "succeeded")]
    Succeeded,
    #[serde(rename = "failed")]
    Failed,
}

/// Call operations. The `signature` calls are not dangerous (not raw-signing).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum CallVariant {
    #[serde(rename = "signature")]
    Signature,
    #[serde(rename = "transaction")]
    Transaction,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ChainVariant {
    #[serde(rename = "native")]
    Native,
    #[serde(rename = "custom")]
    Custom,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum CredentialVariant {
    #[serde(rename = "session")]
    Session,
    #[serde(rename = "k256")]
    K256,
    #[serde(rename = "web-authn")]
    WebAuthn,
    #[serde(rename = "web-authn-uv")]
    WebAuthnUv,
    #[serde(rename = "invite")]
    Invite,
    #[serde(rename = "ed255")]
    Ed255,
    #[serde(rename = "p256")]
    P256,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum DisableableState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "disabled")]
    Disabled,
    #[serde(rename = "deleted")]
    Deleted,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ErrorStatus {
    #[serde(rename = "Cancelled")]
    Cancelled,
    #[serde(rename = "Unknown")]
    Unknown,
    #[serde(rename = "Invalid Argument")]
    InvalidArgument,
    #[serde(rename = "Deadline Exceeded")]
    DeadlineExceeded,
    #[serde(rename = "Not Found")]
    NotFound,
    #[serde(rename = "Already Exists")]
    AlreadyExists,
    #[serde(rename = "Permission Denied")]
    PermissionDenied,
    #[serde(rename = "Resource Exhausted")]
    ResourceExhausted,
    #[serde(rename = "Failed Precondition")]
    FailedPrecondition,
    #[serde(rename = "Aborted")]
    Aborted,
    #[serde(rename = "Out Of Range")]
    OutOfRange,
    #[serde(rename = "Unimplemented")]
    Unimplemented,
    #[serde(rename = "Internal")]
    Internal,
    #[serde(rename = "Unavailable")]
    Unavailable,
    #[serde(rename = "Data Loss")]
    DataLoss,
    #[serde(rename = "Unauthenticated")]
    Unauthenticated,
    #[serde(rename = "Canceled")]
    Canceled,
}

/// Added in `v25.13.1`.
///
/// Compared to the FeePayerPolicy, this adds the explicit variant "unset".
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ExplicitFeePayerPolicy {
    #[serde(rename = "allow")]
    Allow,
    #[serde(rename = "deny")]
    Deny,
    #[serde(rename = "unset")]
    Unset,
}

/// Added in `v25.13.1`.
///
/// Permit or deny address to be used as a fee sponsor for transactions.  Default is to deny.
///
/// May be set on account level, which will be inherited by all addresses in the account.
///
/// Any update resulting in an inconsistent setting between an address in an account will be rejected.
///
/// While resources may be created with an initial fee-payer policy, it may only be updated using the custom fee-payer action.  It will otherwise be immutable.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum FeePayerPolicy {
    #[serde(rename = "allow")]
    Allow,
    #[serde(rename = "deny")]
    Deny,
}

/// `raw` (default) format as is the packed (only) encoding for ed255, and 64 bytes (x, y) coordinates for ECDSA.
/// `compressed` is the SEC1 ECDSA encoding with leading 0x02/0x03 for the sign of y, then x coordinate
/// `uncompressed` is SEC1 ECDSA encoding with leading 0x04, then default format.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum KeyFormat {
    #[serde(rename = "raw")]
    Raw,
    #[serde(rename = "compressed")]
    Compressed,
    #[serde(rename = "uncompressed")]
    Uncompressed,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum KeyState {
    #[serde(rename = "generating")]
    Generating,
    #[serde(rename = "generated")]
    Generated,
    #[serde(rename = "importing")]
    Importing,
    #[serde(rename = "imported")]
    Imported,
    #[serde(rename = "paused")]
    Paused,
    #[serde(rename = "rotating")]
    Rotating,
    #[serde(rename = "failed")]
    Failed,
    #[serde(rename = "signatory-generating")]
    SignatoryGenerating,
}

/// Keys may be either engine-controlled (for instance, keys underlying `Address` resources), or user-controlled (with the direct key generation API).
///
/// - **internal** Can only be used by higher level APIs (like creating Transfers or Stakings).
/// - **user** Can only be use for raw signing (creating Signatures).
/// - **shared** Can be used with any API.
///
/// `engine` is deprecated in `25.1.1` and aliased to `internal`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum KeyVariant {
    #[serde(rename = "engine")]
    Engine,
    #[serde(rename = "shared")]
    Shared,
    #[serde(rename = "user")]
    User,
    #[serde(rename = "internal")]
    Internal,
}

/// A valid request can either be immediately allowed or denied (and will end up in the `succeeded` or `failed` state)_, or require additional approvals (starts in the `authorizing` state).
///
/// An initiator has the operation ID, and can poll the operation until it is in one of the two terminal state.
///
/// An approver or challenger can poll all operations, filtering by `authorizing`, and send their votes.
///
/// ### Cases
/// **authorizing**: User sent a valid `Request` for an operation, but further approvals are required to authorize the operation.
///
/// **creating-resource**: The operation is authorized, but the engine is waiting for an off-chain worker to name the resource. Only current case is `Generate (internal) Address`, where the address ID is derived from the public key of the keypair that is created by the signer for this address.
///
/// **succeeded**: Terminal state - the operation completed successfully. The `resource_name` field in the `Operation` can be used to fetch the new resource.
///
/// **failed**: Terminal state - the operation failed (either challenged, timed out, or creating the resource after authorization failed).
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum OperationState {
    #[serde(rename = "authorizing")]
    Authorizing,
    #[serde(rename = "creating-resource")]
    CreatingResource,
    #[serde(rename = "succeeded")]
    Succeeded,
    #[serde(rename = "failed")]
    Failed,
}

/// Priority Preset defers to Treasury to determine the gas fees to land transactions on chain.
///
/// **low**: Will be lower than market rate to save on fees.
///
/// **market**: Pay current market rate.
///
/// **aggressive**: Pay a premium over market rate to ensure transaction always lands quickly.
///
/// **very-aggressive**: Pay an even larger premium, meant for very volatile periods.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum PriorityPreset {
    #[serde(rename = "low")]
    Low,
    #[serde(rename = "market")]
    Market,
    #[serde(rename = "aggressive")]
    Aggressive,
    #[serde(rename = "very-aggressive")]
    VeryAggressive,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum RequestVariant {
    #[serde(rename = "initiate")]
    Initiate,
    #[serde(rename = "approve")]
    Approve,
    #[serde(rename = "cancel")]
    Cancel,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ResourceType {
    #[serde(rename = "AccessRule")]
    AccessRule,
    #[serde(rename = "Account")]
    Account,
    #[serde(rename = "Address")]
    Address,
    #[serde(rename = "Asset")]
    Asset,
    #[serde(rename = "Chain")]
    Chain,
    #[serde(rename = "Credential")]
    Credential,
    #[serde(rename = "Feature")]
    Feature,
    #[serde(rename = "Key")]
    Key,
    #[serde(rename = "Operation")]
    Operation,
    #[serde(rename = "Role")]
    Role,
    #[serde(rename = "Signature")]
    Signature,
    #[serde(rename = "Transfer")]
    Transfer,
    #[serde(rename = "TransferRule")]
    TransferRule,
    #[serde(rename = "User")]
    User,
    #[serde(rename = "Treasury")]
    Treasury,
    #[serde(rename = "Transaction")]
    Transaction,
    #[serde(rename = "SoftwareUpdate")]
    SoftwareUpdate,
    #[serde(rename = "Signer")]
    Signer,
    #[serde(rename = "Symbol")]
    Symbol,
    #[serde(rename = "KeyResponse")]
    KeyResponse,
    #[serde(rename = "SignatureResponse")]
    SignatureResponse,
    #[serde(rename = "Staking")]
    Staking,
    #[serde(rename = "Signatory")]
    Signatory,
    #[serde(rename = "StakingRule")]
    StakingRule,
    #[serde(rename = "Tag")]
    Tag,
    #[serde(rename = "Host")]
    Host,
    #[serde(rename = "Call")]
    Call,
    #[serde(rename = "CallRule")]
    CallRule,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum RuleDecision {
    #[serde(rename = "allow")]
    Allow,
    #[serde(rename = "deny")]
    Deny,
    #[serde(rename = "pending")]
    Pending,
    #[serde(rename = "cancel")]
    Cancel,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum RuleVariant {
    #[serde(rename = "allow")]
    Allow,
    #[serde(rename = "require")]
    Require,
    #[serde(rename = "deny")]
    Deny,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ShareVariant {
    #[serde(rename = "generated")]
    Generated,
    #[serde(rename = "rotated")]
    Rotated,
    #[serde(rename = "failed")]
    Failed,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum SignatoryState {
    #[serde(rename = "initializing")]
    Initializing,
    #[serde(rename = "active")]
    Active,
}

/// Raw (default) format is (r, s) each as 32 byte big-endian integers.
/// With recovery adds a byte for a total length of 65 bytes for ECDSA signatures.
/// DER uses the DER encoding of the (r, s) pair, for a typical length of 70 bytes.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum SignatureFormat {
    #[serde(rename = "raw")]
    Raw,
    #[serde(rename = "recovery")]
    Recovery,
    #[serde(rename = "der")]
    Der,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum SignatureState {
    #[serde(rename = "signing")]
    Signing,
    #[serde(rename = "signed")]
    Signed,
    #[serde(rename = "failed")]
    Failed,
    #[serde(rename = "signatory-signing")]
    SignatorySigning,
    #[serde(rename = "authorizing")]
    Authorizing,
}

/// Added in `25.1.1`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum SignerState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "inactive")]
    Inactive,
}

/// **Active**: The software update is ongoing and no more changes are allowed to be made to the Treasury instances until the update completes.
///
/// **Scheduled**: The software update is scheduled to occur in the future.
///
/// **Succeeded**:  The update has completed successfully by local supervisor process.
///
/// **Failed**: The update has failed and was aborted by local supervisor process.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum SoftwareUpdateState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "scheduled")]
    Scheduled,
    #[serde(rename = "succeeded")]
    Succeeded,
    #[serde(rename = "failed")]
    Failed,
}

/// Staking operations.  Not all chains use `withdraw`, as it's done automatically in `unstake` transactions.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum StakingVariant {
    #[serde(rename = "stake")]
    Stake,
    #[serde(rename = "unstake")]
    Unstake,
    #[serde(rename = "withdraw")]
    Withdraw,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum TransactionErrorStatus {
    #[serde(rename = "InvalidArgument")]
    InvalidArgument,
    #[serde(rename = "FailedPrecondition")]
    FailedPrecondition,
    #[serde(rename = "Unavailable")]
    Unavailable,
    #[serde(rename = "TimedOut")]
    TimedOut,
    #[serde(rename = "InsufficientBalance")]
    InsufficientBalance,
    #[serde(rename = "InsufficientBalanceForGas")]
    InsufficientBalanceForGas,
    #[serde(rename = "SigningFailed")]
    SigningFailed,
    #[serde(rename = "BroadcastFailed")]
    BroadcastFailed,
    #[serde(rename = "Aborted")]
    Aborted,
    #[serde(rename = "Reverted")]
    Reverted,
    #[serde(rename = "Unknown")]
    Unknown,
    #[serde(rename = "FailedPreparation")]
    FailedPreparation,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum TransferPolicyDetailPolicy {
    #[serde(rename = "transfer")]
    Transfer,
    #[serde(rename = "staking")]
    Staking,
    #[serde(rename = "call")]
    Call,
}

/// - **preparing**: Gathering inputs needed to construct the transaction for the transfer.
/// - **queued**: The transaction is waiting on another transaction to complete before continuing.
/// - **signing**: Waiting for the signer to sign the binary transaction
/// - **submitting**: Waiting for confirmation that the signed transactions was submitted successfully
/// - **finalizing**: Waiting for confirmation that the signed transactions was is sufficiently final (requirement depends on the upstream chain, for instance Cosmos is instantly final, whereas Bitcoin usually uses 6 confirmations for finality)
/// - **succeeded**: Transaction has enough confirmations to be considered final for the given chain
/// - **failed**: This transfer failed, and nothing can be done to make further progress
/// - **reverted**:  The transfer landed on chain but reverted due to an error.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum TransferState {
    #[serde(rename = "preparing")]
    Preparing,
    #[serde(rename = "signing")]
    Signing,
    #[serde(rename = "submitting")]
    Submitting,
    #[serde(rename = "finalizing")]
    Finalizing,
    #[serde(rename = "succeeded")]
    Succeeded,
    #[serde(rename = "failed")]
    Failed,
    #[serde(rename = "queued")]
    Queued,
    #[serde(rename = "reverted")]
    Reverted,
}

/// `internal` if `to` is an internal or shared address, `external` if it is an external address
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum TransferVariant {
    #[serde(rename = "internal")]
    Internal,
    #[serde(rename = "external")]
    External,
}

/// Added after `25.12.1`.
///
/// **active**: User has a credential registered.
///
/// **inactive**: User does not have any credential or invite code.
///
/// **invited**: User has been invited.  User will be deleted if their invitation expires or is deleted.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum UserState {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "inactive")]
    Inactive,
    #[serde(rename = "invited")]
    Invited,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum UserVariant {
    #[serde(rename = "human")]
    Human,
    #[serde(rename = "machine")]
    Machine,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum Vote {
    #[serde(rename = "approve")]
    Approve,
    #[serde(rename = "cancel")]
    Cancel,
}
