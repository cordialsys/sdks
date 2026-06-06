package client

import (
	"fmt"

	"github.com/cordialsys/sdk-go/treasury/types"
)

// CreateAccountRequest contains the typed payload for creating an account.
type CreateAccountRequest struct {
	types.Metadata
	types.AccountData

	// Variant is the account key custody model. Account variants currently use
	// the generated AddressVariant enum in the OpenAPI schema.
	Variant types.AddressVariant `json:"variant"`
}

// CreateAccessRuleRequest contains the typed payload for creating an access rule.
type CreateAccessRuleRequest struct {
	types.Metadata
	types.AccessRuleData
	Variant types.RuleVariant `json:"variant"`
}

// CreateAddressRequest contains the typed payload for creating an address.
type CreateAddressRequest struct {
	types.Metadata
	types.AddressData
	Variant types.AddressVariant `json:"variant"`
}

// CreateAssetRequest contains the typed payload for creating an asset.
type CreateAssetRequest struct {
	types.Metadata
	types.AssetData
	Variant types.AssetVariant `json:"variant"`
}

// CreateChainRequest contains the typed payload for creating a chain.
type CreateChainRequest struct {
	types.Metadata
	types.ChainData
	Variant types.ChainVariant `json:"variant"`
}

// CreateCallRequest contains the typed payload for creating a call.
type CreateCallRequest struct {
	types.Metadata
	types.CallData
}

// CreateCallRuleRequest contains the typed payload for creating a call rule.
type CreateCallRuleRequest struct {
	types.Metadata
	types.CallRuleData
	Variant types.RuleVariant `json:"variant"`
}

// CreateCredentialRequest contains the typed payload for creating a credential.
type CreateCredentialRequest struct {
	types.Metadata
	types.CredentialData
	Variant types.CredentialVariant `json:"variant"`
}

// CreateFeatureRequest contains the typed payload for creating a feature flag.
type CreateFeatureRequest struct {
	types.Metadata
	types.FeatureData
}

// CreateKeyRequest contains the typed payload for creating a key.
type CreateKeyRequest struct {
	types.Metadata
	types.KeyData
	Variant types.KeyVariant `json:"variant"`
}

// CreateRoleRequest contains the typed payload for creating a role.
type CreateRoleRequest struct {
	types.Metadata
	types.RoleData
}

// CreateSignatureRequest contains the typed payload for creating a signature.
type CreateSignatureRequest struct {
	types.Metadata
	types.SignatureData
}

// CreateSoftwareUpdateRequest contains the typed payload for creating a software update.
type CreateSoftwareUpdateRequest struct {
	types.Metadata
	types.SoftwareUpdateData
}

// CreateStakingRequest contains the typed payload for creating a staking operation.
type CreateStakingRequest struct {
	types.Metadata
	types.StakingCreateData
	Variant types.StakingVariant `json:"variant"`
}

// CreateStakingRuleRequest contains the typed payload for creating a staking rule.
type CreateStakingRuleRequest struct {
	types.Metadata
	types.StakingRuleData
	Variant types.RuleVariant `json:"variant"`
}

// CreateSymbolRequest contains the typed payload for creating a symbol.
type CreateSymbolRequest struct {
	types.Metadata
	types.SymbolData
}

// CreateTagRequest contains the typed payload for creating a tag.
type CreateTagRequest struct {
	types.Metadata
	types.TagData
}

// CreateTransferRequest contains the typed payload for creating a transfer.
type CreateTransferRequest struct {
	types.Metadata
	types.TransferCreateData
}

// CreateTransferRuleRequest contains the typed payload for creating a transfer rule.
type CreateTransferRuleRequest struct {
	types.Metadata
	types.TransferRuleData
	Variant types.RuleVariant `json:"variant"`
}

// CreateUserRequest contains the typed payload for creating a user.
type CreateUserRequest struct {
	types.Metadata
	types.UserData
	Variant types.UserVariant `json:"variant"`
}

// CreateAccount creates an account and returns the resulting operation name.
func (c *Client) CreateAccount(id string, req CreateAccountRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid account variant: %s", req.Variant)
	}
	return c.CreateWithParent("Account", id, "", req)
}

// CreateAccessRule creates an access rule and returns the resulting operation name.
func (c *Client) CreateAccessRule(id string, req CreateAccessRuleRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid access rule variant: %s", req.Variant)
	}
	return c.CreateWithParent("AccessRule", id, "", req)
}

// CreateAddress creates an address under a chain and returns the resulting operation name.
func (c *Client) CreateAddress(chainID, id string, req CreateAddressRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid address variant: %s", req.Variant)
	}
	return c.CreateWithParent("Address", id, chainID, req)
}

// CreateAsset creates an asset under a chain and returns the resulting operation name.
func (c *Client) CreateAsset(chainID, id string, req CreateAssetRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid asset variant: %s", req.Variant)
	}
	return c.CreateWithParent("Asset", id, chainID, req)
}

// CreateChain creates a chain and returns the resulting operation name.
func (c *Client) CreateChain(id string, req CreateChainRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid chain variant: %s", req.Variant)
	}
	return c.CreateWithParent("Chain", id, "", req)
}

// CreateCall creates a call under a chain and returns the resulting operation name.
func (c *Client) CreateCall(chainID, id string, req CreateCallRequest) (string, error) {
	return c.CreateWithParent("Call", id, chainID, req)
}

// CreateCallRule creates a call rule and returns the resulting operation name.
func (c *Client) CreateCallRule(id string, req CreateCallRuleRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid call rule variant: %s", req.Variant)
	}
	return c.CreateWithParent("CallRule", id, "", req)
}

// CreateCredential creates a credential under a user and returns the resulting operation name.
func (c *Client) CreateCredential(userID, id string, req CreateCredentialRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid credential variant: %s", req.Variant)
	}
	return c.CreateWithParent("Credential", id, userID, req)
}

// CreateFeature creates a feature and returns the resulting operation name.
func (c *Client) CreateFeature(id string, req CreateFeatureRequest) (string, error) {
	return c.CreateWithParent("Feature", id, "", req)
}

// CreateKey creates a key and returns the resulting operation name.
func (c *Client) CreateKey(id string, req CreateKeyRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid key variant: %s", req.Variant)
	}
	return c.CreateWithParent("Key", id, "", req)
}

// CreateRole creates a role and returns the resulting operation name.
func (c *Client) CreateRole(id string, req CreateRoleRequest) (string, error) {
	return c.CreateWithParent("Role", id, "", req)
}

// CreateSignature creates a signature and returns the resulting operation name.
func (c *Client) CreateSignature(id string, req CreateSignatureRequest) (string, error) {
	return c.CreateWithParent("Signature", id, "", req)
}

// CreateSoftwareUpdate creates a software update and returns the resulting operation name.
func (c *Client) CreateSoftwareUpdate(id string, req CreateSoftwareUpdateRequest) (string, error) {
	return c.CreateWithParent("SoftwareUpdate", id, "", req)
}

// CreateStaking creates a staking operation and returns the resulting operation name.
func (c *Client) CreateStaking(id string, req CreateStakingRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid staking variant: %s", req.Variant)
	}
	return c.CreateWithParent("Staking", id, "", req)
}

// CreateStakingRule creates a staking rule and returns the resulting operation name.
func (c *Client) CreateStakingRule(id string, req CreateStakingRuleRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid staking rule variant: %s", req.Variant)
	}
	return c.CreateWithParent("StakingRule", id, "", req)
}

// CreateSymbol creates a symbol under a chain and returns the resulting operation name.
func (c *Client) CreateSymbol(chainID, id string, req CreateSymbolRequest) (string, error) {
	return c.CreateWithParent("Symbol", id, chainID, req)
}

// CreateTag creates a tag and returns the resulting operation name.
func (c *Client) CreateTag(id string, req CreateTagRequest) (string, error) {
	return c.CreateWithParent("Tag", id, "", req)
}

// CreateTransfer creates a transfer and returns the resulting operation name.
func (c *Client) CreateTransfer(id string, req CreateTransferRequest) (string, error) {
	return c.CreateWithParent("Transfer", id, "", req)
}

// CreateTransferRule creates a transfer rule and returns the resulting operation name.
func (c *Client) CreateTransferRule(id string, req CreateTransferRuleRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid transfer rule variant: %s", req.Variant)
	}
	return c.CreateWithParent("TransferRule", id, "", req)
}

// CreateUser creates a user and returns the resulting operation name.
func (c *Client) CreateUser(id string, req CreateUserRequest) (string, error) {
	if !req.Variant.Valid() {
		return "", fmt.Errorf("invalid user variant: %s", req.Variant)
	}
	return c.CreateWithParent("User", id, "", req)
}

// UpdateAccount updates an account and returns the resulting operation name.
func (c *Client) UpdateAccount(resourceName string, account types.Account) (string, error) {
	return c.Update(resourceName, account)
}

// UpdateAccessRule updates an access rule and returns the resulting operation name.
func (c *Client) UpdateAccessRule(resourceName string, rule types.AccessRule) (string, error) {
	return c.Update(resourceName, rule)
}

// UpdateAddress updates an address and returns the resulting operation name.
func (c *Client) UpdateAddress(resourceName string, address types.Address) (string, error) {
	return c.Update(resourceName, address)
}

// UpdateAsset updates an asset and returns the resulting operation name.
func (c *Client) UpdateAsset(resourceName string, asset types.Asset) (string, error) {
	return c.Update(resourceName, asset)
}

// UpdateChain updates a chain and returns the resulting operation name.
func (c *Client) UpdateChain(resourceName string, chain types.Chain) (string, error) {
	return c.Update(resourceName, chain)
}

// UpdateCall updates a call and returns the resulting operation name.
func (c *Client) UpdateCall(resourceName string, call types.Call) (string, error) {
	return c.Update(resourceName, call)
}

// UpdateCallRule updates a call rule and returns the resulting operation name.
func (c *Client) UpdateCallRule(resourceName string, rule types.CallRule) (string, error) {
	return c.Update(resourceName, rule)
}

// UpdateCredential updates a credential and returns the resulting operation name.
func (c *Client) UpdateCredential(resourceName string, credential types.Credential) (string, error) {
	return c.Update(resourceName, credential)
}

// UpdateFeature updates a feature and returns the resulting operation name.
func (c *Client) UpdateFeature(resourceName string, feature types.Feature) (string, error) {
	return c.Update(resourceName, feature)
}

// UpdateKey updates a key and returns the resulting operation name.
func (c *Client) UpdateKey(resourceName string, key types.Key) (string, error) {
	return c.Update(resourceName, key)
}

// UpdateRole updates a role and returns the resulting operation name.
func (c *Client) UpdateRole(resourceName string, role types.Role) (string, error) {
	return c.Update(resourceName, role)
}

// UpdateSignature updates a signature and returns the resulting operation name.
func (c *Client) UpdateSignature(resourceName string, signature types.Signature) (string, error) {
	return c.Update(resourceName, signature)
}

// UpdateSoftwareUpdate updates a software update and returns the resulting operation name.
func (c *Client) UpdateSoftwareUpdate(resourceName string, update types.SoftwareUpdate) (string, error) {
	return c.Update(resourceName, update)
}

// UpdateStaking updates a staking operation and returns the resulting operation name.
func (c *Client) UpdateStaking(resourceName string, staking types.Staking) (string, error) {
	return c.Update(resourceName, staking)
}

// UpdateStakingRule updates a staking rule and returns the resulting operation name.
func (c *Client) UpdateStakingRule(resourceName string, rule types.StakingRule) (string, error) {
	return c.Update(resourceName, rule)
}

// UpdateSymbol updates a symbol and returns the resulting operation name.
func (c *Client) UpdateSymbol(resourceName string, symbol types.Symbol) (string, error) {
	return c.Update(resourceName, symbol)
}

// UpdateTag updates a tag and returns the resulting operation name.
func (c *Client) UpdateTag(resourceName string, tag types.Tag) (string, error) {
	return c.Update(resourceName, tag)
}

// UpdateTransfer updates a transfer and returns the resulting operation name.
func (c *Client) UpdateTransfer(resourceName string, transfer types.Transfer) (string, error) {
	return c.Update(resourceName, transfer)
}

// UpdateTransferRule updates a transfer rule and returns the resulting operation name.
func (c *Client) UpdateTransferRule(resourceName string, rule types.TransferRule) (string, error) {
	return c.Update(resourceName, rule)
}

// UpdateUser updates a user and returns the resulting operation name.
func (c *Client) UpdateUser(resourceName string, user types.User) (string, error) {
	return c.Update(resourceName, user)
}
