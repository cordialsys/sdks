import type * as R from "../types/resources.js";

export interface TreasuryResourceMap {
  "access-rule": {
    name: R.AccessRuleName;
    resource: R.AccessRule;
    page: R.AccessRulePage;
  };
  account: {
    name: R.AccountName;
    resource: R.Account;
    page: R.AccountPage;
  };
  address: {
    name: R.AddressName;
    resource: R.Address;
    page: R.AddressPage;
  };
  asset: {
    name: R.AssetName;
    resource: R.Asset;
    page: R.AssetPage;
  };
  call: {
    name: R.CallName;
    resource: R.Call;
    page: R.CallPage;
  };
  "call-rule": {
    name: R.CallRuleName;
    resource: R.CallRule;
    page: R.CallRulePage;
  };
  chain: {
    name: R.ChainName;
    resource: R.Chain;
    page: R.ChainPage;
  };
  credential: {
    name: R.CredentialName;
    resource: R.Credential;
    page: R.CredentialPage;
  };
  feature: {
    name: R.FeatureName;
    resource: R.Feature;
    page: R.FeaturePage;
  };
  host: {
    name: R.HostName;
    resource: R.Host;
    page: R.HostPage;
  };
  key: {
    name: R.KeyName;
    resource: R.Key;
    page: R.KeyPage;
  };
  operation: {
    name: R.OperationName;
    resource: R.Operation;
    page: R.OperationPage;
  };
  role: {
    name: R.RoleName;
    resource: R.Role;
    page: R.RolePage;
  };
  signatory: {
    name: R.SignatoryName;
    resource: R.Signatory;
    page: R.SignatoryPage;
  };
  signature: {
    name: R.SignatureName;
    resource: R.Signature;
    page: R.SignaturePage;
  };
  signer: {
    name: R.SignerName;
    resource: R.Signer;
    page: R.SignerPage;
  };
  "software-update": {
    name: R.SoftwareUpdateName;
    resource: R.SoftwareUpdate;
    page: R.SoftwareUpdatePage;
  };
  staking: {
    name: R.StakingName;
    resource: R.Staking;
    page: R.StakingPage;
  };
  "staking-rule": {
    name: R.StakingRuleName;
    resource: R.StakingRule;
    page: R.StakingRulePage;
  };
  symbol: {
    name: R.SymbolName;
    resource: R.Symbol;
    page: R.SymbolPage;
  };
  tag: {
    name: R.TagName;
    resource: R.Tag;
    page: R.TagPage;
  };
  transaction: {
    name: R.TransactionName;
    resource: R.Transaction;
    page: R.TransactionPage;
  };
  transfer: {
    name: R.TransferName;
    resource: R.Transfer;
    page: R.TransferPage;
  };
  "transfer-rule": {
    name: R.TransferRuleName;
    resource: R.TransferRule;
    page: R.TransferRulePage;
  };
  treasury: {
    name: R.TreasuryName;
    resource: R.Treasury;
    page: R.TreasuryPage;
  };
  type: {
    name: R.TypeName;
    resource: R.Type;
    page: R.TypePage;
  };
  user: {
    name: R.UserName;
    resource: R.User;
    page: R.UserPage;
  };
}

export type TreasuryResourceType = keyof TreasuryResourceMap;
export type TreasuryResourceName<T extends TreasuryResourceType> = TreasuryResourceMap[T]["name"];
export type TreasuryResource<T extends TreasuryResourceType> = TreasuryResourceMap[T]["resource"];
export type TreasuryResourcePage<T extends TreasuryResourceType> = TreasuryResourceMap[T]["page"];
