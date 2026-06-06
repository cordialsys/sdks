/**
 * Re-exports of OpenAPI-generated resource types.
 * Use these instead of reaching into the openapi.d.ts directly.
 */
import type { components } from "./openapi.js";

// Core resource types
export type AccessRule = components["schemas"]["AccessRule"];
export type AccessRuleData = components["schemas"]["AccessRuleData"];
export type AccessRuleName = components["schemas"]["AccessRuleName"];
export type AccessRulePage = components["schemas"]["AccessRulePage"];

export type Account = components["schemas"]["Account"];
export type AccountData = components["schemas"]["AccountData"];
export type AccountName = components["schemas"]["AccountName"];
export type AccountPage = components["schemas"]["AccountPage"];

export type Address = components["schemas"]["Address"];
export type AddressData = components["schemas"]["AddressData"];
export type AddressName = components["schemas"]["AddressName"];
export type AddressPage = components["schemas"]["AddressPage"];
export type AddressVariant = components["schemas"]["AddressVariant"];

export type Asset = components["schemas"]["Asset"];
export type AssetData = components["schemas"]["AssetData"];
export type AssetName = components["schemas"]["AssetName"];
export type AssetPage = components["schemas"]["AssetPage"];

export type Call = components["schemas"]["Call"];
export type CallData = components["schemas"]["CallData"];
export type CallName = components["schemas"]["CallName"];
export type CallPage = components["schemas"]["CallPage"];

export type CallRule = components["schemas"]["CallRule"];
export type CallRuleData = components["schemas"]["CallRuleData"];
export type CallRuleName = components["schemas"]["CallRuleName"];
export type CallRulePage = components["schemas"]["CallRulePage"];

export type Chain = components["schemas"]["Chain"];
export type ChainData = components["schemas"]["ChainData"];
export type ChainName = components["schemas"]["ChainName"];
export type ChainPage = components["schemas"]["ChainPage"];

export type Credential = components["schemas"]["Credential"];
export type CredentialData = components["schemas"]["CredentialData"];
export type CredentialName = components["schemas"]["CredentialName"];
export type CredentialPage = components["schemas"]["CredentialPage"];

export type Feature = components["schemas"]["Feature"];
export type FeatureData = components["schemas"]["FeatureData"];
export type FeatureName = components["schemas"]["FeatureName"];
export type FeaturePage = components["schemas"]["FeaturePage"];

export type Host = components["schemas"]["Host"];
export type HostData = components["schemas"]["HostData"];
export type HostName = components["schemas"]["HostName"];
export type HostPage = components["schemas"]["HostPage"];

export type Key = components["schemas"]["Key"];
export type KeyData = components["schemas"]["KeyData"];
export type KeyName = components["schemas"]["KeyName"];
export type KeyPage = components["schemas"]["KeyPage"];

export type Operation = components["schemas"]["Operation"];
export type OperationData = components["schemas"]["OperationData"];
export type OperationName = components["schemas"]["OperationName"];
export type OperationPage = components["schemas"]["OperationPage"];
export type OperationState = components["schemas"]["OperationState"];

export type Role = components["schemas"]["Role"];
export type RoleData = components["schemas"]["RoleData"];
export type RoleName = components["schemas"]["RoleName"];
export type RolePage = components["schemas"]["RolePage"];

export type Signatory = components["schemas"]["Signatory"];
export type SignatoryData = components["schemas"]["SignatoryData"];
export type SignatoryName = components["schemas"]["SignatoryName"];
export type SignatoryPage = components["schemas"]["SignatoryPage"];

export type Signature = components["schemas"]["Signature"];
export type SignatureData = components["schemas"]["SignatureData"];
export type SignatureName = components["schemas"]["SignatureName"];
export type SignaturePage = components["schemas"]["SignaturePage"];

export type Signer = components["schemas"]["Signer"];
export type SignerData = components["schemas"]["SignerData"];
export type SignerName = components["schemas"]["SignerName"];
export type SignerPage = components["schemas"]["SignerPage"];

export type SoftwareUpdate = components["schemas"]["SoftwareUpdate"];
export type SoftwareUpdateData = components["schemas"]["SoftwareUpdateData"];
export type SoftwareUpdateName = components["schemas"]["SoftwareUpdateName"];
export type SoftwareUpdatePage = components["schemas"]["SoftwareUpdatePage"];

export type Staking = components["schemas"]["Staking"];
export type StakingData = components["schemas"]["StakingData"];
export type StakingName = components["schemas"]["StakingName"];
export type StakingPage = components["schemas"]["StakingPage"];

export type StakingRule = components["schemas"]["StakingRule"];
export type StakingRuleData = components["schemas"]["StakingRuleData"];
export type StakingRuleName = components["schemas"]["StakingRuleName"];
export type StakingRulePage = components["schemas"]["StakingRulePage"];

export type Symbol = components["schemas"]["Symbol"];
export type SymbolData = components["schemas"]["SymbolData"];
export type SymbolName = components["schemas"]["SymbolName"];
export type SymbolPage = components["schemas"]["SymbolPage"];

export type Tag = components["schemas"]["Tag"];
export type TagData = components["schemas"]["TagData"];
export type TagName = components["schemas"]["TagName"];
export type TagPage = components["schemas"]["TagPage"];

export type Transaction = components["schemas"]["Transaction"];
export type TransactionData = components["schemas"]["TransactionData"];
export type TransactionName = components["schemas"]["TransactionName"];
export type TransactionPage = components["schemas"]["TransactionPage"];

export type Transfer = components["schemas"]["Transfer"];
export type TransferData = components["schemas"]["TransferData"];
export type TransferName = components["schemas"]["TransferName"];
export type TransferPage = components["schemas"]["TransferPage"];

export type TransferRule = components["schemas"]["TransferRule"];
export type TransferRuleData = components["schemas"]["TransferRuleData"];
export type TransferRuleName = components["schemas"]["TransferRuleName"];
export type TransferRulePage = components["schemas"]["TransferRulePage"];

export type Treasury = components["schemas"]["Treasury"];
export type TreasuryData = components["schemas"]["TreasuryData"];
export type TreasuryName = components["schemas"]["TreasuryName"];
export type TreasuryPage = components["schemas"]["TreasuryPage"];

export type Type = components["schemas"]["Type"];
export type TypeData = components["schemas"]["TypeData"];
export type TypeName = components["schemas"]["TypeName"];
export type TypePage = components["schemas"]["TypePage"];

export type User = components["schemas"]["User"];
export type UserData = components["schemas"]["UserData"];
export type UserName = components["schemas"]["UserName"];
export type UserPage = components["schemas"]["UserPage"];

// Common types
export type Algorithm = components["schemas"]["Algorithm"];
export type Authentication = components["schemas"]["Authentication"];
export type BasicState = components["schemas"]["BasicState"];
export type Error = components["schemas"]["Error"];
export type ErrorCode = components["schemas"]["ErrorCode"];
export type ExplicitFeePayerPolicy = components["schemas"]["ExplicitFeePayerPolicy"];
export type FeePayerPolicy = components["schemas"]["FeePayerPolicy"];
export type Id = components["schemas"]["Id"];
export type Labels = components["schemas"]["Labels"];
export type Metadata = components["schemas"]["Metadata"];
export type Notes = components["schemas"]["Notes"];
export type Pagination = components["schemas"]["Pagination"];
export type Proposal = components["schemas"]["Proposal"];
export type Reference = components["schemas"]["Reference"];
export type Request = components["schemas"]["Request"];
export type ResourceVersion = components["schemas"]["ResourceVersion"];
export type Response = components["schemas"]["Response"];
export type Tags = components["schemas"]["Tags"];
export type Timestamp = components["schemas"]["Timestamp"];

// Re-export the components type for advanced usage
export type { components };
