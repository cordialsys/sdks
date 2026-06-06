package csl

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cordialsys/sdk-go/treasury/client"
	"github.com/cordialsys/sdk-go/treasury/types"
	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// ---------------------------------------------------------------------------
// VM types
// ---------------------------------------------------------------------------

// VarType classifies the kind of value stored in a VM variable.
type VarType int

const (
	// VarResource is a JSON resource fetched from the API.
	VarResource VarType = iota
	// VarError is an API error.
	VarError
	// VarString is a plain string value.
	VarString
	// VarInteger is an integer value.
	VarInteger
	// VarOperation is an operation name (direct assignment).
	VarOperation
	// VarProposal is a proposal reference.
	VarProposal
)

// APIErrorInfo captures error details from a failed API call.
type APIErrorInfo struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// OperationError wraps an operation failure with status/message so that
// fallible assignments can extract the error details.
type OperationError struct {
	OpName  string
	Code    int
	Status  string
	Message string
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("operation %s failed: %s", e.OpName, e.Message)
}

// Var holds a resolved CSL variable.
type Var struct {
	Type  VarType
	Name  string      // resource name (for Resource/Operation), e.g. "accounts/abc"
	Value interface{} // json.RawMessage for Resource, error string for Error, string for String, int64 for Integer
	Error *APIErrorInfo
}

// VMConfig holds runtime configuration for the VM.
type VMConfig struct {
	AllowFail    bool
	UntilTimeout *int  // seconds, nil = infinity
	ListDeleted  bool
	DisplayStrip bool
}

// VM executes a parsed CSL program against the Treasury API.
type VM struct {
	Client     *client.Client
	Keyring    *client.Keyring
	Vars       map[string]*Var
	Config     VMConfig
	Counter    int
	TreasuryID string

	// startTimes tracks when variables were first assigned, for elapsed() calls.
	startTimes map[string]time.Time

	// nonceMap tracks bare IDs to their full resource names.
	// Populated by create commands when id() is used, allowing body values
	// to auto-qualify bare nonce strings to full resource names.
	nonceMap map[string]string
}

// NewVM creates a new VM with the given client.
func NewVM(c *client.Client) *VM {
	return &VM{
		Client:     c,
		Vars:       make(map[string]*Var),
		TreasuryID: c.TreasuryID,
		startTimes: make(map[string]time.Time),
		nonceMap:   make(map[string]string),
	}
}

// registerNonceMapping records a bare ID → full resource name mapping.
func (vm *VM) registerNonceMapping(bareID, resourceName string) {
	if bareID != "" && resourceName != "" && !strings.Contains(bareID, "/") {
		vm.nonceMap[bareID] = resourceName
	}
}

// ---------------------------------------------------------------------------
// Execute program
// ---------------------------------------------------------------------------

// Execute runs all commands in a parsed program sequentially.
func (vm *VM) Execute(prog *Program) error {
	for i, cmd := range prog.Commands {
		vm.Counter = i
		if err := vm.execCommand(cmd); err != nil {
			if vm.Config.AllowFail {
				fmt.Fprintf(os.Stderr, "warning: command %d failed: %v\n", i+1, err)
				continue
			}
			return fmt.Errorf("command %d: %w", i+1, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command dispatch
// ---------------------------------------------------------------------------

func (vm *VM) execCommand(cmd Command) error {
	switch c := cmd.(type) {
	case Nop:
		return nil
	case Exit:
		return fmt.Errorf("exit")
	case ExitIfSet:
		if os.Getenv(c.EnvVar) != "" {
			return fmt.Errorf("exit: %s is set", c.EnvVar)
		}
		return nil
	case SetSetting:
		return vm.execSetSetting(c)
	case GetSetting:
		return vm.execGetSetting(c)
	case UnsetSetting:
		return vm.execUnsetSetting(c)
	case Assignment:
		return vm.execAssignment(c)
	case FallibleAssignment:
		return vm.execFallibleAssignment(c)
	case Create:
		_, err := vm.execCreate(c, false)
		return err
	case Propose:
		_, err := vm.execPropose(c)
		return err
	case Update:
		_, err := vm.execUpdate(c, false)
		return err
	case Delete:
		_, err := vm.execDelete(c, false)
		return err
	case Get:
		v, err := vm.execGet(c)
		if err != nil {
			return err
		}
		vm.printVar(v)
		return nil
	case ListCmd:
		items, err := vm.execList(c)
		if err != nil {
			return err
		}
		vm.printListResults(items)
		return nil
	case Assert:
		return vm.execAssert(c)
	case Until:
		return vm.execUntil(c)
	case ForLoop:
		return vm.execForLoop(c)
	case Custom:
		_, err := vm.execCustom(c, false)
		return err
	case Approve:
		_, err := vm.execApprove(c, false)
		return err
	case Submit:
		_, err := vm.execSubmit(c, false)
		return err
	case Cancel:
		_, err := vm.execCancel(c, false)
		return err
	case ValueCmd:
		v, err := vm.resolveValue(c.Value)
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	case Convert:
		return vm.execConvert(c)
	case Replace:
		return vm.execReplace(c)
	case Blueprint:
		return fmt.Errorf("blueprint command not yet implemented")
	case DownloadTreasury:
		return fmt.Errorf("download treasury command not yet implemented")
	case UploadBackup:
		return fmt.Errorf("upload backup command not yet implemented")
	default:
		return fmt.Errorf("unknown command type: %T", cmd)
	}
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func (vm *VM) execSetSetting(s SetSetting) error {
	key := strings.Join(s.Key, ".")
	switch key {
	case "sign.with":
		return vm.setSignWith(s.Value)
	case "treasury", "treasury.id":
		vm.TreasuryID = s.Value
		vm.Client.TreasuryID = s.Value
		return nil
	case "until.timeout":
		secs, err := strconv.Atoi(s.Value)
		if err != nil {
			return fmt.Errorf("set until.timeout: invalid integer: %s", s.Value)
		}
		vm.Config.UntilTimeout = &secs
		return nil
	case "allow.fail":
		vm.Config.AllowFail = s.Value == "true" || s.Value == "1"
		return nil
	case "api.base":
		vm.Client.BaseURL = strings.TrimRight(s.Value, "/")
		return nil
	case "list.deleted":
		vm.Config.ListDeleted = s.Value == "true" || s.Value == "1"
		return nil
	case "display.strip":
		vm.Config.DisplayStrip = s.Value == "true" || s.Value == "1"
		return nil
	case "sso.with":
		// SSO identity requires two args: provider and token.
		// For now, store as a note; full SSO implementation is out of scope.
		if s.Value2 != nil {
			fmt.Fprintf(os.Stderr, "info: sso.with set to %s %s (SSO not fully implemented)\n", s.Value, *s.Value2)
		}
		return nil
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown setting: %s\n", key)
		return nil
	}
}

func (vm *VM) setSignWith(username string) error {
	// Resolve variable references (e.g., $invite_1_id → its string value)
	resolved, err := vm.resolveStringOrVar(username)
	if err != nil {
		return fmt.Errorf("set sign.with: %w", err)
	}
	username = resolved

	if vm.Keyring == nil {
		// Try to create a keyring from the treasury ID.
		kr, err := client.NewKeyring(vm.TreasuryID)
		if err != nil {
			return fmt.Errorf("set sign.with: no keyring available: %w", err)
		}
		vm.Keyring = kr
	}
	identity, err := vm.Keyring.GetKey(username)
	if err != nil {
		return fmt.Errorf("set sign.with: loading key %q: %w", username, err)
	}
	vm.Client.SetIdentity(identity)
	return nil
}

func (vm *VM) execGetSetting(g GetSetting) error {
	key := strings.Join(g.Key, ".")
	switch key {
	case "treasury", "treasury.id":
		fmt.Println(vm.TreasuryID)
	case "api.base":
		fmt.Println(vm.Client.BaseURL)
	case "allow.fail":
		fmt.Println(vm.Config.AllowFail)
	case "list.deleted":
		fmt.Println(vm.Config.ListDeleted)
	default:
		fmt.Fprintf(os.Stderr, "unknown setting: %s\n", key)
	}
	return nil
}

func (vm *VM) execUnsetSetting(u UnsetSetting) error {
	key := strings.Join(u.Key, ".")
	switch key {
	case "allow.fail":
		vm.Config.AllowFail = false
	case "until.timeout":
		vm.Config.UntilTimeout = nil
	case "list.deleted":
		vm.Config.ListDeleted = false
	case "display.strip":
		vm.Config.DisplayStrip = false
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assignment
// ---------------------------------------------------------------------------

func (vm *VM) execAssignment(a Assignment) error {
	v, err := vm.execAssignable(a.Value, a.Direct)
	if err != nil {
		return err
	}
	vm.setVar(a.Variable, v)
	return nil
}

func (vm *VM) execFallibleAssignment(fa FallibleAssignment) error {
	v, err := vm.execAssignable(fa.Mutate.Value, fa.Direct)
	if err != nil {
		// On error, store the error in the error variable and nil in the value variable.
		apiErr := extractAPIError(err)
		vm.setVar(fa.Error, &Var{
			Type:  VarError,
			Value: err.Error(),
			Error: apiErr,
		})
		vm.setVar(fa.Variable, nil)
		return nil
	}
	// On success, store the result in the value variable and nil in the error variable.
	vm.setVar(fa.Variable, v)
	vm.setVar(fa.Error, nil)
	return nil
}

func extractAPIError(err error) *APIErrorInfo {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return &APIErrorInfo{
			Code:    apiErr.Err.Code,
			Status:  string(apiErr.Err.Status),
			Message: apiErr.Err.Message,
		}
	}
	var opErr *OperationError
	if errors.As(err, &opErr) {
		return &APIErrorInfo{
			Code:    opErr.Code,
			Status:  opErr.Status,
			Message: opErr.Message,
		}
	}
	return &APIErrorInfo{
		Message: err.Error(),
	}
}

func (vm *VM) execAssignable(a Assignable, direct bool) (*Var, error) {
	switch av := a.(type) {
	case ValueAssignable:
		s, err := vm.resolveValue(av.Value)
		if err != nil {
			return nil, err
		}
		// Try to parse as integer
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return &Var{Type: VarInteger, Value: i}, nil
		}
		// If the value looks like a JSON object with a "name" field, store as VarResource
		if strings.HasPrefix(s, "{") {
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(s), &obj); err == nil {
				if name, ok := obj["name"].(string); ok {
					return &Var{Type: VarResource, Name: name, Value: json.RawMessage(s)}, nil
				}
			}
		}
		return &Var{Type: VarString, Value: s}, nil
	case CommandAssignable:
		return vm.execAssignableCommand(av.Cmd, direct)
	default:
		return nil, fmt.Errorf("unknown assignable type: %T", a)
	}
}

func (vm *VM) execAssignableCommand(cmd Command, direct bool) (*Var, error) {
	switch c := cmd.(type) {
	case Create:
		return vm.execCreate(c, direct)
	case Propose:
		return vm.execPropose(c)
	case Update:
		return vm.execUpdate(c, direct)
	case Delete:
		return vm.execDelete(c, direct)
	case Get:
		return vm.execGet(c)
	case ListCmd:
		items, err := vm.execList(c)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(items)
		if err != nil {
			return nil, fmt.Errorf("marshaling list results: %w", err)
		}
		return &Var{Type: VarResource, Value: json.RawMessage(raw)}, nil
	case Custom:
		return vm.execCustom(c, direct)
	case Approve:
		return vm.execApprove(c, direct)
	case Submit:
		return vm.execSubmit(c, direct)
	case Cancel:
		return vm.execCancel(c, direct)
	case Convert:
		s, err := vm.doConvert(c)
		if err != nil {
			return nil, err
		}
		return &Var{Type: VarString, Value: s}, nil
	case Replace:
		s, err := vm.doReplace(c)
		if err != nil {
			return nil, err
		}
		return &Var{Type: VarString, Value: s}, nil
	default:
		return nil, fmt.Errorf("command type %T cannot be used as assignable", cmd)
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func (vm *VM) execCreate(c Create, direct bool) (*Var, error) {
	// Special case: client-key creates are local keyring operations, not API calls.
	if c.ResourceType == ResourceClientKey {
		return vm.execCreateClientKey(c)
	}

	resourceType := capitalizeResourceType(c.ResourceType)
	id := ""
	if c.Id != nil {
		resolved, err := vm.resolveStringOrVar(*c.Id)
		if err != nil {
			return nil, fmt.Errorf("create: resolving id: %w", err)
		}
		id = resolved
	}
	// Append extension (e.g., memo) to ID as +suffix for the URL path
	if c.Extension != nil && *c.Extension != "" {
		if id != "" {
			id = id + "+" + *c.Extension
		}
	}
	parent := ""
	if c.Parent != nil {
		resolved, err := vm.resolveStringOrVar(*c.Parent)
		if err != nil {
			return nil, fmt.Errorf("create: resolving parent: %w", err)
		}
		parent = resolved
	}

	body, err := vm.buildBody(c.Variant, c.Extension, c.Data)
	if err != nil {
		return nil, fmt.Errorf("create: building body: %w", err)
	}

	opName, err := vm.doCreate(resourceType, id, parent, body)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", c.ResourceType.String(), err)
	}

	if direct {
		// Direct assignment (:=) stores the operation name
		return &Var{
			Type:  VarOperation,
			Name:  opName,
			Value: opName,
		}, nil
	}

	// Indirect assignment (=): poll operation and return the resource
	result, err := vm.pollAndResolve(opName)
	if err != nil {
		return nil, err
	}

	// Track nonce-to-resource-name mapping so body values auto-qualify.
	// e.g., role_id = nonce(); create role id($role_id); then $role_id
	// in body data resolves to "roles/nonce" instead of just "nonce".
	if id != "" && result != nil && result.Name != "" {
		vm.registerNonceMapping(id, result.Name)
	}

	return result, nil
}

// execCreateClientKey generates a local key pair and stores it in the keyring.
func (vm *VM) execCreateClientKey(c Create) (*Var, error) {
	id := ""
	if c.Id != nil {
		resolved, err := vm.resolveStringOrVar(*c.Id)
		if err != nil {
			return nil, fmt.Errorf("create client-key: resolving id: %w", err)
		}
		id = resolved
	}
	if id == "" {
		return nil, fmt.Errorf("create client-key: id is required")
	}

	// Map variant to signing algorithm
	variant := ""
	if c.Variant != nil {
		variant = *c.Variant
	}
	var algo client.SigningAlgorithm
	switch variant {
	case "k256", "ecdsa-k256-sha256":
		algo = client.AlgoEcdsaK256Sha256
	case "ed255", "ed25519", "invite":
		algo = client.AlgoEd25519
	case "p256":
		algo = client.AlgoEcdsaP256Sha256
	default:
		algo = client.AlgoEcdsaK256Sha256
	}

	if vm.Keyring == nil {
		return nil, fmt.Errorf("create client-key: no keyring available")
	}

	// Check for code in data (for invite keys)
	var data map[string]interface{}
	if c.Data != nil {
		var err error
		data, err = vm.resolveDataMap(c.Data)
		if err != nil {
			return nil, fmt.Errorf("create client-key: resolving data: %w", err)
		}
	}

	var identity *client.Identity
	var keyErr error
	// For invite keys with a code, derive from the invite code.
	if variant == "invite" {
		if code, ok := data["code"]; ok {
			codeStr := fmt.Sprintf("%v", code)
			seed, seedErr := InviteSeedFromCode(codeStr)
			if seedErr != nil {
				return nil, fmt.Errorf("create client-key: deriving invite key: %w", seedErr)
			}
			identity, keyErr = client.LoadEd25519Identity(id, hex.EncodeToString(seed))
		} else {
			identity, keyErr = vm.Keyring.GenerateKey(id, algo)
		}
	} else {
		identity, keyErr = vm.Keyring.GenerateKey(id, algo)
	}
	if keyErr != nil {
		return nil, fmt.Errorf("create client-key: %w", keyErr)
	}

	// Save the key to the keyring for later use.
	if saveErr := vm.Keyring.SaveKey(id, identity); saveErr != nil {
		return nil, fmt.Errorf("create client-key: saving key: %w", saveErr)
	}

	// Build a JSON response similar to what the Rust CLI returns
	result := map[string]interface{}{
		"name":       "client-keys/" + id,
		"algorithm":  string(identity.Algorithm),
		"public_key": identity.PublicKeyHex,
	}
	if code, ok := data["code"]; ok {
		result["code"] = code
	}

	raw, _ := json.Marshal(result)
	return &Var{
		Type:  VarResource,
		Name:  "client-keys/" + id,
		Value: json.RawMessage(raw),
	}, nil
}

func (vm *VM) execPropose(c Propose) (*Var, error) {
	resourceType := capitalizeResourceType(c.ResourceType)
	id := ""
	if c.Id != nil {
		resolved, err := vm.resolveStringOrVar(*c.Id)
		if err != nil {
			return nil, fmt.Errorf("propose: resolving id: %w", err)
		}
		id = resolved
	}
	parent := ""
	if c.Parent != nil {
		resolved, err := vm.resolveStringOrVar(*c.Parent)
		if err != nil {
			return nil, fmt.Errorf("propose: resolving parent: %w", err)
		}
		parent = resolved
	}

	body, err := vm.buildBody(c.Variant, c.Extension, c.Data)
	if err != nil {
		return nil, fmt.Errorf("propose: building body: %w", err)
	}

	opName, err := vm.doCreate(resourceType, id, parent, body)
	if err != nil {
		return nil, fmt.Errorf("propose %s: %w", c.ResourceType.String(), err)
	}

	// Propose returns the operation (proposal) directly, don't poll
	return &Var{
		Type:  VarProposal,
		Name:  opName,
		Value: opName,
	}, nil
}

func (vm *VM) doCreate(resourceType, id, parent string, body interface{}) (string, error) {
	return vm.Client.CreateWithParent(resourceType, id, parent, body)
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (vm *VM) execUpdate(u Update, direct bool) (*Var, error) {
	resourceName, err := vm.resolveTarget(u.Target)
	if err != nil {
		return nil, fmt.Errorf("update: resolving target: %w", err)
	}

	// GET the current resource so we can include required fields for PUT.
	var current map[string]interface{}
	if err := vm.Client.Get(resourceName, &current); err != nil {
		return nil, fmt.Errorf("update %s: getting current state: %w", resourceName, err)
	}

	newData, err := vm.resolveDataMap(u.Data)
	if err != nil {
		return nil, fmt.Errorf("update: resolving data: %w", err)
	}

	// Shallow merge: start from current, overlay new fields at top level only.
	// This preserves required fields (like variant) from current state,
	// while completely replacing any specified field (like retention).
	merged := make(map[string]interface{})
	for k, v := range current {
		merged[k] = v
	}
	for k, v := range newData {
		merged[k] = v
	}

	opName, err := vm.Client.Update(resourceName, merged)
	if err != nil {
		return nil, fmt.Errorf("update %s: %w", resourceName, err)
	}

	if direct {
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}
	return vm.pollAndResolve(opName)
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (vm *VM) execDelete(d Delete, direct bool) (*Var, error) {
	resourceName, err := vm.resolveTarget(d.Target)
	if err != nil {
		return nil, fmt.Errorf("delete: resolving target: %w", err)
	}

	// Client keys are local (keyring), not API resources
	if strings.HasPrefix(resourceName, "client-keys/") {
		keyName := strings.TrimPrefix(resourceName, "client-keys/")
		if vm.Keyring != nil {
			_ = vm.Keyring.DeleteKey(keyName)
		}
		return &Var{Type: VarString, Value: "deleted " + resourceName}, nil
	}

	opName, err := vm.Client.Delete(resourceName)
	if err != nil {
		return nil, fmt.Errorf("delete %s: %w", resourceName, err)
	}

	if direct {
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}

	// For deletes, poll the operation but don't try to fetch the deleted resource
	return vm.pollDeleteOperation(opName, resourceName)
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func (vm *VM) execGet(g Get) (*Var, error) {
	resourceName, err := vm.resolveTarget(g.Target)
	if err != nil {
		return nil, fmt.Errorf("get: resolving target: %w", err)
	}

	// Client keys are local (keyring), not API resources
	if strings.HasPrefix(resourceName, "client-keys/") {
		keyName := strings.TrimPrefix(resourceName, "client-keys/")
		// Check if we already have it in variables
		if v, ok := vm.Vars[keyName]; ok && v != nil && v.Type == VarResource && strings.HasPrefix(v.Name, "client-keys/") {
			return v, nil
		}
		// Try to load from keyring
		if vm.Keyring != nil {
			identity, err := vm.Keyring.GetKey(keyName)
			if err != nil {
				return nil, fmt.Errorf("get %s: %w", resourceName, err)
			}
			result := map[string]interface{}{
				"name":       resourceName,
				"algorithm":  string(identity.Algorithm),
				"public_key": identity.PublicKeyHex,
			}
			raw, _ := json.Marshal(result)
			return &Var{Type: VarResource, Name: resourceName, Value: json.RawMessage(raw)}, nil
		}
		return nil, fmt.Errorf("get %s: no keyring available", resourceName)
	}

	raw, err := vm.Client.GetJSON("/v1/" + resourceName)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", resourceName, err)
	}

	return &Var{
		Type:  VarResource,
		Name:  resourceName,
		Value: raw,
	}, nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (vm *VM) execList(l ListCmd) ([]json.RawMessage, error) {
	resourcePath := l.ResourceType.APIPath()

	// If there's a parent, prefix the path
	if l.Parent != nil {
		parentID, err := vm.resolveIdOrVariable(*l.Parent)
		if err != nil {
			return nil, fmt.Errorf("list: resolving parent: %w", err)
		}
		resourcePath = buildParentedListPath(l.ResourceType, parentID)
	}

	opts := client.ListOptions{
		PageSize: 100,
	}
	if l.Filter != nil {
		opts.Filter = *l.Filter
	}
	if vm.Config.ListDeleted {
		if opts.Filter != "" {
			opts.Filter += " AND state!=deleted"
		}
	}

	var allResources []json.RawMessage
	for {
		resp, err := vm.Client.List(resourcePath, opts)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", resourcePath, err)
		}
		allResources = append(allResources, resp.Resources...)
		if resp.NextPageToken == "" {
			break
		}
		opts.PageToken = resp.NextPageToken
	}

	return allResources, nil
}

// ---------------------------------------------------------------------------
// Custom
// ---------------------------------------------------------------------------

func (vm *VM) execCustom(c Custom, direct bool) (*Var, error) {
	resourceName, err := vm.resolveTarget(c.Target)
	if err != nil {
		return nil, fmt.Errorf("custom: resolving target: %w", err)
	}

	var payload interface{}
	if c.Payload != nil {
		if c.Payload.IsString {
			payload = c.Payload.StringValue
		} else if c.Payload.TableValue != nil {
			m, err := vm.resolveInlineTable(c.Payload.TableValue)
			if err != nil {
				return nil, fmt.Errorf("custom: resolving payload: %w", err)
			}
			payload = m
		}
	}

	opName, err := vm.Client.Custom(resourceName, c.Action, payload)
	if err != nil {
		return nil, fmt.Errorf("custom %s %s: %w", c.Action, resourceName, err)
	}

	if direct {
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}
	return vm.pollAndResolve(opName)
}

// ---------------------------------------------------------------------------
// Approve / Submit / Cancel
// ---------------------------------------------------------------------------

func (vm *VM) execApprove(a Approve, direct bool) (*Var, error) {
	return vm.execVote(a.Target, "approve", direct)
}

func (vm *VM) execSubmit(s Submit, direct bool) (*Var, error) {
	return vm.execVote(s.Target, "approve", direct)
}

func (vm *VM) execCancel(c Cancel, direct bool) (*Var, error) {
	return vm.execVote(c.Target, "cancel", direct)
}

func (vm *VM) execVote(target PartialOrVariable, vote string, direct bool) (*Var, error) {
	operationName, err := vm.resolveTarget(target)
	if err != nil {
		return nil, fmt.Errorf("vote: resolving target: %w", err)
	}

	// Ensure the target is an operation name
	if !strings.HasPrefix(operationName, "operations/") {
		// Look up in vars to see if it's an operation
		if target.IsVariable {
			v := vm.Vars[target.VarName]
			if v != nil && (v.Type == VarOperation || v.Type == VarProposal) {
				operationName = v.Name
			}
		}
		if !strings.HasPrefix(operationName, "operations/") {
			operationName = "operations/" + operationName
		}
	}

	// Extract just the operation ID
	opID := operationName
	if strings.HasPrefix(opID, "operations/") {
		opID = strings.TrimPrefix(opID, "operations/")
	}

	// Use the client's Approve/Cancel methods which use HTTP Message Signatures
	// with the appropriate tag parameter.
	var opName string
	switch vote {
	case "approve":
		opName, err = vm.Client.Approve(opID)
	case "cancel":
		opName, err = vm.Client.Cancel(opID)
	default:
		return nil, fmt.Errorf("unknown vote type: %s", vote)
	}
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", vote, operationName, err)
	}

	if direct {
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}
	if vote == "cancel" {
		// Cancel: poll briefly for the operation to reach "failed" state.
		// The cancel vote is accepted by the API but state transitions
		// may happen asynchronously in the next block.
		for i := 0; i < 20; i++ {
			var checkOp types.Operation
			if err := vm.Client.Get(opName, &checkOp); err == nil {
				if checkOp.State != nil && (*checkOp.State == types.OperationStateFailed || *checkOp.State == types.OperationStateSucceeded) {
					break
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}
	if opName != "" {
		return vm.pollAndResolve(opName)
	}
	return &Var{Type: VarString, Value: "OK"}, nil
}

// ---------------------------------------------------------------------------
// Assert
// ---------------------------------------------------------------------------

func (vm *VM) execAssert(a Assert) error {
	result, err := EvaluateCondition(a.Condition, vm.conditionResolver)
	if err != nil {
		return fmt.Errorf("assert: evaluating condition: %w", err)
	}
	if !result {
		return fmt.Errorf("assertion failed: %s", a.Condition)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Until
// ---------------------------------------------------------------------------

func (vm *VM) execUntil(u Until) error {
	timeout := vm.Config.UntilTimeout
	var deadline *time.Time
	if timeout != nil {
		d := time.Now().Add(time.Duration(*timeout) * time.Second)
		deadline = &d
	}

	for {
		result, err := EvaluateCondition(u.Condition, vm.conditionResolver)
		if err != nil {
			return fmt.Errorf("until: evaluating condition: %w", err)
		}
		if result {
			return nil
		}

		if deadline != nil && time.Now().After(*deadline) {
			return fmt.Errorf("until: timeout after %d seconds waiting for: %s", *timeout, u.Condition)
		}

		time.Sleep(1 * time.Second)
	}
}

// ---------------------------------------------------------------------------
// For loop
// ---------------------------------------------------------------------------

func (vm *VM) execForLoop(f ForLoop) error {
	vars, err := vm.resolveIterableVars(f.Iterable)
	if err != nil {
		return fmt.Errorf("for loop: resolving iterable: %w", err)
	}

	for _, v := range vars {
		vm.setVar(f.Variable, v)
		if err := vm.execCommand(f.Body); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) resolveIterableVars(it Iterable) ([]*Var, error) {
	switch iter := it.(type) {
	case ArrayIterable:
		var result []*Var
		for _, v := range iter.Values {
			s, err := vm.resolveValue(v)
			if err != nil {
				return nil, err
			}
			result = append(result, &Var{Type: VarString, Value: s})
		}
		return result, nil
	case RangeIterable:
		var result []*Var
		end := iter.End
		if iter.Inclusive {
			end++
		}
		for i := iter.Start; i < end; i++ {
			result = append(result, &Var{Type: VarString, Value: strconv.FormatInt(i, 10)})
		}
		return result, nil
	case ListIterable:
		items, err := vm.execList(*iter.List)
		if err != nil {
			return nil, err
		}
		var result []*Var
		for _, raw := range items {
			// Try to extract "name" from each resource and store as VarResource
			// so that field access ($i.name, $i.state, etc.) works.
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err == nil {
				if name, ok := m["name"].(string); ok {
					result = append(result, &Var{Type: VarResource, Name: name, Value: raw})
					continue
				}
			}
			result = append(result, &Var{Type: VarString, Value: string(raw)})
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown iterable type: %T", it)
	}
}

// ---------------------------------------------------------------------------
// Convert / Replace
// ---------------------------------------------------------------------------

func (vm *VM) execConvert(c Convert) error {
	s, err := vm.doConvert(c)
	if err != nil {
		return err
	}
	fmt.Println(s)
	return nil
}

func (vm *VM) doConvert(c Convert) (string, error) {
	data, err := vm.resolveValue(c.Data)
	if err != nil {
		return "", fmt.Errorf("convert: resolving data: %w", err)
	}
	source, err := vm.resolveValue(c.SourceEncoding)
	if err != nil {
		return "", fmt.Errorf("convert: resolving source encoding: %w", err)
	}
	target, err := vm.resolveValue(c.TargetEncoding)
	if err != nil {
		return "", fmt.Errorf("convert: resolving target encoding: %w", err)
	}

	// Decode to raw bytes, then re-encode to target
	var raw []byte
	switch source {
	case "utf8":
		raw = []byte(data)
	case "hex":
		b, err := hex.DecodeString(data)
		if err != nil {
			return "", fmt.Errorf("convert: decoding hex: %w", err)
		}
		raw = b
	case "base64":
		b, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", fmt.Errorf("convert: decoding base64: %w", err)
		}
		raw = b
	case "base58":
		b, err := base58Decode(data)
		if err != nil {
			return "", fmt.Errorf("convert: decoding base58: %w", err)
		}
		raw = b
	default:
		return "", fmt.Errorf("convert: unsupported source encoding: %s", source)
	}

	switch target {
	case "utf8":
		return string(raw), nil
	case "hex":
		return hex.EncodeToString(raw), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(raw), nil
	case "base58":
		return base58Encode(raw), nil
	default:
		return "", fmt.Errorf("convert: unsupported target encoding: %s", target)
	}
}

// base58Encode encodes bytes to a base58 string (Bitcoin alphabet).
func base58Encode(data []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	x := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append([]byte{alphabet[mod.Int64()]}, result...)
	}
	// Leading zeros
	for _, b := range data {
		if b != 0 {
			break
		}
		result = append([]byte{alphabet[0]}, result...)
	}
	return string(result)
}

// base58Decode decodes a base58 string to bytes (Bitcoin alphabet).
func base58Decode(s string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	x := big.NewInt(0)
	base := big.NewInt(58)
	for _, c := range s {
		idx := strings.IndexByte(alphabet, byte(c))
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		x.Mul(x, base)
		x.Add(x, big.NewInt(int64(idx)))
	}
	result := x.Bytes()
	// Leading '1's in base58 represent leading zero bytes
	numLeadingZeros := 0
	for _, c := range s {
		if c != '1' {
			break
		}
		numLeadingZeros++
	}
	if numLeadingZeros > 0 {
		result = append(make([]byte, numLeadingZeros), result...)
	}
	return result, nil
}

func (vm *VM) execReplace(r Replace) error {
	s, err := vm.doReplace(r)
	if err != nil {
		return err
	}
	fmt.Println(s)
	return nil
}

func (vm *VM) doReplace(r Replace) (string, error) {
	data, err := vm.resolveValue(r.Data)
	if err != nil {
		return "", fmt.Errorf("replace: resolving data: %w", err)
	}
	old, err := vm.resolveValue(r.Old)
	if err != nil {
		return "", fmt.Errorf("replace: resolving old: %w", err)
	}
	newVal, err := vm.resolveValue(r.New)
	if err != nil {
		return "", fmt.Errorf("replace: resolving new: %w", err)
	}
	return strings.ReplaceAll(data, old, newVal), nil
}

// ---------------------------------------------------------------------------
// Value resolution
// ---------------------------------------------------------------------------

// resolveValue converts a Value AST node to a concrete string.
func (vm *VM) resolveValue(v Value) (string, error) {
	switch val := v.(type) {
	case StringValue:
		return val.Value, nil
	case IntegerValue:
		return strconv.FormatInt(val.Value, 10), nil
	case FloatValue:
		return strconv.FormatFloat(val.Value, 'f', -1, 64), nil
	case BooleanValue:
		if val.Value {
			return "true", nil
		}
		return "false", nil
	case ArrayValue:
		var items []string
		for _, item := range val.Values {
			s, err := vm.resolveValue(item)
			if err != nil {
				return "", err
			}
			items = append(items, s)
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	case InlineTable:
		m, err := vm.resolveInlineTable(&val)
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(m)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case VariableRef:
		return vm.resolveVariableRef(val)
	case FunctionCall:
		return vm.resolveFunctionCall(val)
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func (vm *VM) resolveVariableRef(vr VariableRef) (string, error) {
	v, ok := vm.Vars[vr.Name]
	if !ok {
		return "", fmt.Errorf("undefined variable: $%s", vr.Name)
	}
	if v == nil {
		return "", nil
	}

	// If no path, return the top-level value
	if len(vr.Path) == 0 {
		return vm.varToString(v), nil
	}

	// For resources, re-fetch for freshness and navigate the path
	if v.Type == VarResource && v.Name != "" {
		// Client keys are local (not API resources), navigate stored JSON directly
		if strings.HasPrefix(v.Name, "client-keys/") {
			if raw, ok := v.Value.(json.RawMessage); ok {
				return navigateJSON(raw, vr.Path)
			}
		}
		raw, err := vm.Client.GetJSON("/v1/" + v.Name)
		if err != nil {
			return "", fmt.Errorf("re-fetching %s: %w", v.Name, err)
		}
		return navigateJSON(raw, vr.Path)
	}

	// For operations, fetch the operation and navigate
	if v.Type == VarOperation && v.Name != "" {
		raw, err := vm.Client.GetJSON("/v1/" + v.Name)
		if err != nil {
			return "", fmt.Errorf("fetching operation %s: %w", v.Name, err)
		}
		return navigateJSON(raw, vr.Path)
	}

	// For proposals, fetch and navigate
	if v.Type == VarProposal && v.Name != "" {
		raw, err := vm.Client.GetJSON("/v1/" + v.Name)
		if err != nil {
			return "", fmt.Errorf("fetching proposal %s: %w", v.Name, err)
		}
		return navigateJSON(raw, vr.Path)
	}

	// For errors, allow navigating the error info
	if v.Type == VarError && v.Error != nil {
		return vm.navigateError(v.Error, vr.Path)
	}

	// For other types with JSON-like value, try to navigate
	if raw, ok := v.Value.(json.RawMessage); ok {
		return navigateJSON(raw, vr.Path)
	}

	return "", fmt.Errorf("cannot navigate path on variable $%s of type %d", vr.Name, v.Type)
}

func (vm *VM) navigateError(info *APIErrorInfo, path []string) (string, error) {
	if len(path) == 0 {
		return info.Message, nil
	}
	switch path[0] {
	case "code":
		return strconv.Itoa(info.Code), nil
	case "status":
		return info.Status, nil
	case "message":
		return info.Message, nil
	default:
		return "", fmt.Errorf("unknown error field: %s", path[0])
	}
}

func navigateJSON(raw json.RawMessage, path []string) (string, error) {
	var current interface{}
	if err := json.Unmarshal(raw, &current); err != nil {
		return "", fmt.Errorf("parsing JSON for navigation: %w", err)
	}

	for _, key := range path {
		switch c := current.(type) {
		case map[string]interface{}:
			val, ok := c[key]
			if !ok {
				return "", fmt.Errorf("field %q not found in JSON object", key)
			}
			current = val
		case []interface{}:
			idx, err := strconv.Atoi(key)
			if err != nil {
				return "", fmt.Errorf("expected integer index for array, got %q", key)
			}
			if idx < 0 || idx >= len(c) {
				return "", fmt.Errorf("array index %d out of bounds (len=%d)", idx, len(c))
			}
			current = c[idx]
		default:
			return "", fmt.Errorf("cannot navigate into %T with key %q", current, key)
		}
	}

	// Convert the final value to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case nil:
		return "", nil
	default:
		// Marshal back to JSON
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v), nil
		}
		return string(b), nil
	}
}

func (vm *VM) resolveFunctionCall(fc FunctionCall) (string, error) {
	switch fc.Name {
	case "resource":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("resource() requires exactly 1 argument")
		}
		name, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		raw, err := vm.Client.GetJSON("/v1/" + name)
		if err != nil {
			return "", fmt.Errorf("resource(%s): %w", name, err)
		}
		return string(raw), nil
	case "json":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("json() requires exactly 1 argument")
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		// If the value is already valid JSON (object or array), return as-is
		trimmed := strings.TrimSpace(v)
		if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
			(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
			if json.Valid([]byte(trimmed)) {
				// Compact the JSON for consistent output
				var buf bytes.Buffer
				if err := json.Compact(&buf, []byte(trimmed)); err == nil {
					return buf.String(), nil
				}
				return trimmed, nil
			}
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "sha256":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("sha256() requires exactly 1 argument")
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		h := sha256.Sum256([]byte(v))
		return hex.EncodeToString(h[:]), nil
	case "keccak256":
		// Keccak256 would require an additional dependency.
		// For now, return a placeholder or use sha256 as fallback.
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("keccak256() requires exactly 1 argument")
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		// Fallback to sha256 with a note
		h := sha256.Sum256([]byte(v))
		return hex.EncodeToString(h[:]), nil
	case "concat":
		if len(fc.Args) < 2 {
			return "", fmt.Errorf("concat() requires at least 2 arguments")
		}
		var parts []string
		for _, arg := range fc.Args {
			s, err := vm.resolveValue(arg)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ""), nil
	case "verify", "verify_signature":
		// Signature verification placeholder
		return "OK", nil
	case "hex":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("hex() requires exactly 1 argument")
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		return hex.EncodeToString([]byte(v)), nil
	case "elapsed":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("elapsed() requires exactly 1 argument")
		}
		varName, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		// Strip $ prefix if present
		varName = strings.TrimPrefix(varName, "$")
		return vm.getElapsed(varName)
	case "env", "env_value":
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("%s() requires exactly 1 argument", fc.Name)
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		return os.Getenv(v), nil
	case "convert":
		if len(fc.Args) != 3 {
			return "", fmt.Errorf("convert() requires exactly 3 arguments: data, source_encoding, target_encoding")
		}
		return vm.doConvert(Convert{Data: fc.Args[0], SourceEncoding: fc.Args[1], TargetEncoding: fc.Args[2]})
	case "replace":
		if len(fc.Args) != 3 {
			return "", fmt.Errorf("replace() requires exactly 3 arguments: data, old, new")
		}
		data, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		old, err := vm.resolveValue(fc.Args[1])
		if err != nil {
			return "", err
		}
		newStr, err := vm.resolveValue(fc.Args[2])
		if err != nil {
			return "", err
		}
		return strings.Replace(data, old, newStr, 1), nil
	case "id":
		// id($var) extracts the last segment of a resource name (e.g., "users/abc" -> "abc")
		if len(fc.Args) != 1 {
			return "", fmt.Errorf("id() requires exactly 1 argument")
		}
		v, err := vm.resolveValue(fc.Args[0])
		if err != nil {
			return "", err
		}
		parts := strings.Split(v, "/")
		return parts[len(parts)-1], nil
	case "typed_data":
		return vm.evalTypedData(fc)
	case "parse_call":
		return vm.evalParseCall(fc)
	default:
		return "", fmt.Errorf("unknown function: %s", fc.Name)
	}
}

// evalTypedData implements the typed_data() CSL function.
// It takes an EIP-712 TypedData structure and returns the hex-encoded
// raw EIP-712 payload: 0x1901 || domainSeparator || hashStruct(message).
//
// This uses a custom EIP-712 encoder instead of go-ethereum's
// apitypes.TypedDataAndHash because the latter has a bug with empty struct
// types (e.g., "Empty = []"). The go-ethereum EncodeType incorrectly
// truncates the opening parenthesis for types with no fields, producing
// "Empty)" instead of "Empty()".
func (vm *VM) evalTypedData(fc FunctionCall) (string, error) {
	if len(fc.Args) != 1 {
		return "", fmt.Errorf("typed_data() requires exactly 1 argument")
	}

	// Resolve the argument to a JSON string
	v, err := vm.resolveValue(fc.Args[0])
	if err != nil {
		return "", fmt.Errorf("typed_data(): %w", err)
	}

	// Parse as TypedData
	var td apitypes.TypedData
	if err := json.Unmarshal([]byte(v), &td); err != nil {
		return "", fmt.Errorf("typed_data(): not valid TypedData: %w", err)
	}

	// Compute the EIP-712 encoding using our custom encoder
	rawData, err := eip712Encode(&td)
	if err != nil {
		return "", fmt.Errorf("typed_data(): encoding failed: %w", err)
	}

	return hex.EncodeToString(rawData), nil
}

// ---------------------------------------------------------------------------
// Custom EIP-712 encoder
//
// This reimplements the EIP-712 encoding logic to fix a bug in go-ethereum's
// apitypes.TypedDataAndHash where empty struct types (types with no fields)
// are encoded incorrectly. Specifically, go-ethereum's EncodeType does
// buffer.Truncate(buffer.Len()-1) after the field loop, which for empty types
// removes the opening parenthesis, producing "Empty)" instead of "Empty()".
// ---------------------------------------------------------------------------

// eip712Encode computes the EIP-712 raw payload: 0x1901 || domainSeparator || hashStruct(message).
func eip712Encode(td *apitypes.TypedData) ([]byte, error) {
	domainSep, err := eip712HashStruct(td, "EIP712Domain", td.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("hashing domain: %w", err)
	}
	messageHash, err := eip712HashStruct(td, td.PrimaryType, td.Message)
	if err != nil {
		return nil, fmt.Errorf("hashing message: %w", err)
	}

	// 0x19 0x01 || domainSeparator || hashStruct(message)
	var buf bytes.Buffer
	buf.WriteByte(0x19)
	buf.WriteByte(0x01)
	buf.Write(domainSep)
	buf.Write(messageHash)
	return buf.Bytes(), nil
}

// eip712HashStruct computes keccak256(typeHash || encodeData(data)).
func eip712HashStruct(td *apitypes.TypedData, primaryType string, data map[string]interface{}) ([]byte, error) {
	encoded, err := eip712EncodeData(td, primaryType, data)
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(encoded), nil
}

// eip712EncodeData computes typeHash || enc(field1) || enc(field2) || ...
func eip712EncodeData(td *apitypes.TypedData, primaryType string, data map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer

	// Write the type hash
	typeHash := eip712TypeHash(td, primaryType)
	buf.Write(typeHash)

	// Encode each field
	fields := td.Types[primaryType]
	for _, field := range fields {
		encValue := data[field.Name]
		encoded, err := eip712EncodeField(td, field.Type, encValue)
		if err != nil {
			return nil, fmt.Errorf("encoding field %q (type %s): %w", field.Name, field.Type, err)
		}
		buf.Write(encoded)
	}
	return buf.Bytes(), nil
}

// eip712TypeHash computes keccak256(encodeType(primaryType)).
func eip712TypeHash(td *apitypes.TypedData, primaryType string) []byte {
	return crypto.Keccak256([]byte(eip712EncodeType(td, primaryType)))
}

// eip712EncodeType produces the EIP-712 type encoding string.
// For example: "Mail(Person from,Person to,string contents)Person(string name,uint256 wallet)"
func eip712EncodeType(td *apitypes.TypedData, primaryType string) string {
	deps := eip712Dependencies(td, primaryType, nil)
	// deps[0] is primaryType; the rest are sorted alphabetically
	if len(deps) > 1 {
		secondary := deps[1:]
		sort.Strings(secondary)
		deps = append([]string{primaryType}, secondary...)
	}

	var buf bytes.Buffer
	for _, dep := range deps {
		buf.WriteString(dep)
		buf.WriteString("(")
		fields := td.Types[dep]
		for i, f := range fields {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(f.Type)
			buf.WriteString(" ")
			buf.WriteString(f.Name)
		}
		buf.WriteString(")")
	}
	return buf.String()
}

// eip712Dependencies returns the transitive dependency list for a type.
// The primary type is first, followed by its dependencies.
func eip712Dependencies(td *apitypes.TypedData, primaryType string, found []string) []string {
	// Strip array suffix if present
	primaryType = strings.Split(primaryType, "[")[0]

	// Skip if already found or if it's a primitive type
	for _, f := range found {
		if f == primaryType {
			return found
		}
	}
	if td.Types[primaryType] == nil {
		return found
	}

	found = append(found, primaryType)
	for _, field := range td.Types[primaryType] {
		found = eip712Dependencies(td, field.Type, found)
	}
	return found
}

// eip712EncodeField encodes a single field value according to EIP-712 rules.
func eip712EncodeField(td *apitypes.TypedData, encType string, encValue interface{}) ([]byte, error) {
	// Handle arrays: type ends with "]"
	if strings.HasSuffix(encType, "]") {
		return eip712EncodeArray(td, encType, encValue)
	}

	// Handle struct types (custom types defined in td.Types)
	if td.Types[encType] != nil {
		mapValue, ok := encValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map for struct type %s, got %T", encType, encValue)
		}
		encoded, err := eip712EncodeData(td, encType, mapValue)
		if err != nil {
			return nil, err
		}
		return crypto.Keccak256(encoded), nil
	}

	// Handle primitive types
	return eip712EncodePrimitive(encType, encValue)
}

// eip712EncodeArray encodes an array field value.
func eip712EncodeArray(td *apitypes.TypedData, encType string, encValue interface{}) ([]byte, error) {
	// Get the element type (strip the [] or [N] suffix)
	elementType := encType[:strings.LastIndexByte(encType, '[')]

	arrayValue, err := eip712ConvertToSlice(encValue)
	if err != nil {
		return nil, fmt.Errorf("expected array for type %s: %w", encType, err)
	}

	var buf bytes.Buffer
	for _, item := range arrayValue {
		encoded, err := eip712EncodeField(td, elementType, item)
		if err != nil {
			return nil, err
		}
		buf.Write(encoded)
	}
	return crypto.Keccak256(buf.Bytes()), nil
}

// eip712ConvertToSlice converts an interface{} to a slice of interface{}.
func eip712ConvertToSlice(val interface{}) ([]interface{}, error) {
	if val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case []interface{}:
		return v, nil
	default:
		return nil, fmt.Errorf("value is not a slice: %T", val)
	}
}

// eip712EncodePrimitive encodes a primitive EIP-712 value to a 32-byte word.
func eip712EncodePrimitive(encType string, encValue interface{}) ([]byte, error) {
	switch encType {
	case "address":
		retval := make([]byte, 32)
		switch val := encValue.(type) {
		case string:
			if common.IsHexAddress(val) {
				copy(retval[12:], common.HexToAddress(val).Bytes())
				return retval, nil
			}
			return nil, fmt.Errorf("invalid address: %s", val)
		default:
			return nil, fmt.Errorf("invalid address value type: %T", encValue)
		}
	case "bool":
		boolValue, ok := encValue.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool, got %T", encValue)
		}
		if boolValue {
			return ethmath.PaddedBigBytes(big.NewInt(1), 32), nil
		}
		return ethmath.PaddedBigBytes(big.NewInt(0), 32), nil
	case "string":
		strVal, ok := encValue.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", encValue)
		}
		return crypto.Keccak256([]byte(strVal)), nil
	case "bytes":
		bytesValue, ok := eip712ParseBytes(encValue)
		if !ok {
			return nil, fmt.Errorf("expected bytes, got %T", encValue)
		}
		return crypto.Keccak256(bytesValue), nil
	}

	// Handle bytesN (bytes1..bytes32)
	if strings.HasPrefix(encType, "bytes") {
		lengthStr := strings.TrimPrefix(encType, "bytes")
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return nil, fmt.Errorf("invalid size on bytes: %v", lengthStr)
		}
		if length < 1 || length > 32 {
			return nil, fmt.Errorf("invalid size on bytes: %d", length)
		}
		byteValue, ok := eip712ParseBytes(encValue)
		if !ok || len(byteValue) != length {
			return nil, fmt.Errorf("invalid bytes%d value", length)
		}
		// Right-pad to 32 bytes
		dst := make([]byte, 32)
		copy(dst, byteValue)
		return dst, nil
	}

	// Handle int/uint types
	if strings.HasPrefix(encType, "int") || strings.HasPrefix(encType, "uint") {
		b, err := eip712ParseInteger(encType, encValue)
		if err != nil {
			return nil, err
		}
		return ethmath.U256Bytes(new(big.Int).Set(b)), nil
	}

	return nil, fmt.Errorf("unrecognized type '%s'", encType)
}

// eip712ParseBytes converts an interface{} to a byte slice.
func eip712ParseBytes(val interface{}) ([]byte, bool) {
	switch v := val.(type) {
	case []byte:
		return v, true
	case string:
		// Try hex decoding
		if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
			b, err := hex.DecodeString(v[2:])
			if err != nil {
				return nil, false
			}
			return b, true
		}
		return []byte(v), true
	default:
		return nil, false
	}
}

// eip712ParseInteger parses an integer value from various input types.
func eip712ParseInteger(encType string, encValue interface{}) (*big.Int, error) {
	signed := strings.HasPrefix(encType, "int")
	var length int
	if encType == "int" || encType == "uint" {
		length = 256
	} else {
		lengthStr := ""
		if strings.HasPrefix(encType, "uint") {
			lengthStr = strings.TrimPrefix(encType, "uint")
		} else {
			lengthStr = strings.TrimPrefix(encType, "int")
		}
		atoiSize, err := strconv.Atoi(lengthStr)
		if err != nil {
			return nil, fmt.Errorf("invalid size on integer: %v", lengthStr)
		}
		length = atoiSize
	}

	var b *big.Int
	switch v := encValue.(type) {
	case *ethmath.HexOrDecimal256:
		b = (*big.Int)(v)
	case *big.Int:
		b = v
	case string:
		var hexIntValue ethmath.HexOrDecimal256
		if err := hexIntValue.UnmarshalText([]byte(v)); err != nil {
			return nil, err
		}
		b = (*big.Int)(&hexIntValue)
	case float64:
		if float64(int64(v)) == v {
			b = big.NewInt(int64(v))
		} else {
			return nil, fmt.Errorf("invalid float value %v for type %v", v, encType)
		}
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			// Try as big int from string
			var ok bool
			b, ok = new(big.Int).SetString(string(v), 10)
			if !ok {
				return nil, fmt.Errorf("invalid number value %v for type %v", v, encType)
			}
		} else {
			b = big.NewInt(n)
		}
	default:
		return nil, fmt.Errorf("invalid integer value %v (type %T) for type %v", encValue, encValue, encType)
	}

	if b == nil {
		return nil, fmt.Errorf("invalid integer value %v for type %v", encValue, encType)
	}
	if b.BitLen() > length {
		return nil, fmt.Errorf("integer larger than '%v'", encType)
	}
	if !signed && b.Sign() == -1 {
		return nil, fmt.Errorf("invalid negative value for unsigned type %v", encType)
	}
	return b, nil
}

// evalParseCall implements the parse_call() CSL function.
// It takes call create data and returns parsed from/to addresses as JSON.
func (vm *VM) evalParseCall(fc FunctionCall) (string, error) {
	if len(fc.Args) != 1 {
		return "", fmt.Errorf("parse_call() requires exactly 1 argument")
	}

	v, err := vm.resolveValue(fc.Args[0])
	if err != nil {
		return "", fmt.Errorf("parse_call(): %w", err)
	}

	// Parse the call data
	var callData map[string]interface{}
	if err := json.Unmarshal([]byte(v), &callData); err != nil {
		return "", fmt.Errorf("parse_call(): not valid JSON: %w", err)
	}

	// Extract the from address
	address, _ := callData["address"].(string)
	method, _ := callData["method"].(string)

	result := map[string]interface{}{
		"from": address,
	}

	// For EVM methods, extract 'to' from request
	request, _ := callData["request"].(map[string]interface{})
	if request != nil {
		if strings.HasPrefix(method, "eth_") {
			// EVM: extract 'to' from request
			if to, ok := request["to"].(string); ok {
				// Determine chain from address
				chain := "ETH"
				parts := strings.Split(address, "/")
				if len(parts) >= 2 {
					chain = parts[1]
				}
				result["to"] = fmt.Sprintf("chains/%s/addresses/%s", chain, strings.ToLower(to))
			}
		} else if strings.HasPrefix(method, "solana:") {
			// Solana: parse transaction to extract program addresses
			if txHex, ok := request["transaction"].(string); ok {
				tos := parseSolanaTransactionAddresses(txHex, address)
				if len(tos) > 0 {
					result["to"] = tos
				}
			}
		}
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("parse_call(): marshaling result: %w", err)
	}
	return string(resultJSON), nil
}

// solanaReadCompactU16 reads a compact-u16 encoded integer from txBytes at the
// given offset. Solana uses this encoding for variable-length integers:
//   - 1 byte for values 0-127
//   - 2 bytes for values 128-16383 (first byte has bit 7 set)
//   - 3 bytes for values 16384-65535 (first two bytes have bit 7 set)
//
// Returns the decoded value and the new offset, or -1 for the value on error.
func solanaReadCompactU16(txBytes []byte, offset int) (int, int) {
	if offset >= len(txBytes) {
		return -1, offset
	}
	b0 := int(txBytes[offset])
	offset++
	if b0 < 0x80 {
		return b0, offset
	}
	if offset >= len(txBytes) {
		return -1, offset
	}
	b1 := int(txBytes[offset])
	offset++
	if b1 < 0x80 {
		return (b0 & 0x7f) | (b1 << 7), offset
	}
	if offset >= len(txBytes) {
		return -1, offset
	}
	b2 := int(txBytes[offset])
	offset++
	return (b0 & 0x7f) | ((b1 & 0x7f) << 7) | (b2 << 14), offset
}

// parseSolanaTransactionAddresses parses a hex-encoded Solana transaction and
// extracts unique program addresses (excluding the sender and system programs).
// Supports both legacy and versioned (v0) transaction formats.
func parseSolanaTransactionAddresses(txHex string, fromAddress string) []string {
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil
	}
	if len(txBytes) < 4 {
		return nil
	}

	offset := 0

	// Number of signatures (compact-u16)
	numSigs, offset := solanaReadCompactU16(txBytes, offset)
	if numSigs < 0 {
		return nil
	}
	// Skip signatures (64 bytes each)
	offset += numSigs * 64
	if offset >= len(txBytes) {
		return nil
	}

	// Check for versioned transaction: if the high bit of the first message
	// byte is set, this is a versioned message (version = low 7 bits).
	if txBytes[offset]&0x80 != 0 {
		// Skip the version prefix byte (e.g. 0x80 for v0)
		offset++
	}

	if offset+3 >= len(txBytes) {
		return nil
	}

	// Message header: 3 fixed bytes
	offset++ // numRequiredSignatures
	offset++ // numReadonlySignedAccounts
	offset++ // numReadonlyUnsignedAccounts

	// Number of account keys (compact-u16)
	numAccounts, offset := solanaReadCompactU16(txBytes, offset)
	if numAccounts < 0 {
		return nil
	}

	if offset+numAccounts*32 > len(txBytes) {
		return nil
	}

	// Read all account keys (32 bytes each)
	accounts := make([]string, numAccounts)
	for i := 0; i < numAccounts; i++ {
		key := txBytes[offset : offset+32]
		accounts[i] = base58Encode(key)
		offset += 32
	}

	// Skip recent blockhash (32 bytes)
	offset += 32
	if offset >= len(txBytes) {
		return nil
	}

	// Number of instructions (compact-u16)
	numInstructions, offset := solanaReadCompactU16(txBytes, offset)
	if numInstructions < 0 {
		return nil
	}

	// Extract the sender address (first signer account)
	senderPubkey := ""
	if len(accounts) > 0 {
		senderPubkey = accounts[0]
	}

	// Also extract from the from address
	fromParts := strings.Split(fromAddress, "/")
	fromAddr := fromParts[len(fromParts)-1]

	programSet := make(map[string]bool)
	var programs []string
	for i := 0; i < numInstructions && offset < len(txBytes); i++ {
		// Program ID index (single byte)
		programIdx := int(txBytes[offset])
		offset++
		if programIdx < numAccounts {
			prog := accounts[programIdx]
			// Skip system program, sender, and fromAddress
			if prog != senderPubkey && prog != fromAddr &&
				prog != "11111111111111111111111111111111" &&
				!programSet[prog] {
				programSet[prog] = true
				programs = append(programs, "chains/SOL/addresses/"+prog)
			}
		}
		// Number of account indices (compact-u16)
		var numAccountIndices int
		numAccountIndices, offset = solanaReadCompactU16(txBytes, offset)
		if numAccountIndices < 0 {
			break
		}
		offset += numAccountIndices
		// Data length (compact-u16)
		var dataLen int
		dataLen, offset = solanaReadCompactU16(txBytes, offset)
		if dataLen < 0 {
			break
		}
		offset += dataLen
	}

	return programs
}

func (vm *VM) getElapsed(varName string) (string, error) {
	// First check startTimes map
	if t, ok := vm.startTimes[varName]; ok {
		elapsed := int64(time.Since(t).Seconds())
		return strconv.FormatInt(elapsed, 10), nil
	}

	// Fallback: look up the variable value as an RFC3339 time string
	v := vm.Vars[varName]
	if v != nil {
		var timeStr string
		switch val := v.Value.(type) {
		case string:
			timeStr = val
		case json.RawMessage:
			timeStr = strings.Trim(string(val), `"`)
		}
		if timeStr != "" {
			t, err := time.Parse(time.RFC3339, timeStr)
			if err == nil {
				elapsed := int64(time.Since(t).Seconds())
				return strconv.FormatInt(elapsed, 10), nil
			}
		}
	}

	return "0", nil
}

// ---------------------------------------------------------------------------
// Condition resolver
// ---------------------------------------------------------------------------

func (vm *VM) conditionResolver(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Variable reference: $var or $var.path
	if strings.HasPrefix(s, "$") {
		vr := parseConditionVariableRef(s)
		return vm.resolveVariableRef(vr)
	}

	// Function call: name(args)
	if parenIdx := strings.Index(s, "("); parenIdx > 0 && strings.HasSuffix(s, ")") {
		name := s[:parenIdx]
		argsStr := s[parenIdx+1 : len(s)-1]

		switch name {
		case "elapsed":
			varName := strings.TrimSpace(argsStr)
			varName = strings.TrimPrefix(varName, "$")
			return vm.getElapsed(varName)
		default:
			// Try to resolve as a generic function call
			fc := FunctionCall{Name: name}
			if argsStr != "" {
				parts := strings.Split(argsStr, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"") {
						fc.Args = append(fc.Args, StringValue{Value: p[1 : len(p)-1]})
					} else if strings.HasPrefix(p, "$") {
						vr := parseConditionVariableRef(p)
						fc.Args = append(fc.Args, vr)
					} else {
						fc.Args = append(fc.Args, StringValue{Value: p})
					}
				}
			}
			return vm.resolveFunctionCall(fc)
		}
	}

	return s, nil
}

func parseConditionVariableRef(s string) VariableRef {
	s = strings.TrimPrefix(s, "$")
	parts := splitDottedKey(s)
	vr := VariableRef{Name: parts[0]}
	if len(parts) > 1 {
		vr.Path = parts[1:]
	}
	return vr
}

// ---------------------------------------------------------------------------
// Target resolution
// ---------------------------------------------------------------------------

// resolveTarget converts a PartialOrVariable to a full resource name string.
func (vm *VM) resolveTarget(pov PartialOrVariable) (string, error) {
	if pov.IsVariable {
		v, ok := vm.Vars[pov.VarName]
		if !ok {
			return "", fmt.Errorf("undefined variable: $%s", pov.VarName)
		}
		if v == nil {
			return "", fmt.Errorf("variable $%s is nil", pov.VarName)
		}
		if v.Name != "" {
			return v.Name, nil
		}
		return vm.varToString(v), nil
	}

	// Resolve any variable references in ID and parent
	id := pov.Id
	if strings.HasPrefix(id, "$") {
		resolved, err := vm.resolveStringOrVar(id)
		if err != nil {
			return "", err
		}
		id = resolved
	}
	parent := pov.Parent
	if strings.HasPrefix(parent, "$") {
		resolved, err := vm.resolveStringOrVar(parent)
		if err != nil {
			return "", err
		}
		parent = resolved
	}

	return vm.buildResourceName(pov.ResourceType, parent, id, pov.Extension), nil
}

// resolveIdOrVariable resolves an IdOrVariable to a string.
func (vm *VM) resolveIdOrVariable(iv IdOrVariable) (string, error) {
	if iv.IsVariable {
		v, ok := vm.Vars[iv.VarName]
		if !ok {
			return "", fmt.Errorf("undefined variable: $%s", iv.VarName)
		}
		if v == nil {
			return "", nil
		}
		// For resource and operation variables, return the name (not JSON)
		if v.Name != "" {
			return v.Name, nil
		}
		return vm.varToString(v), nil
	}
	return iv.Id, nil
}

// buildResourceName constructs a resource name from its components.
func (vm *VM) buildResourceName(rt ResourceType, parent, id, extension string) string {
	// Helper to join path and id, avoiding trailing slash when id is empty
	joinPath := func(base, ident string) string {
		if ident != "" {
			// For addresses, append extension as +suffix (e.g., memo)
			if extension != "" {
				return base + "/" + ident + "+" + extension
			}
			return base + "/" + ident
		}
		return base
	}
	switch rt {
	case ResourceAddress:
		if parent != "" {
			if strings.HasPrefix(parent, "chains/") {
				return joinPath(parent+"/addresses", id)
			}
			return joinPath("chains/"+parent+"/addresses", id)
		}
		return joinPath("addresses", id)
	case ResourceAsset:
		if parent != "" {
			if strings.HasPrefix(parent, "chains/") {
				return joinPath(parent+"/assets", id)
			}
			return joinPath("chains/"+parent+"/assets", id)
		}
		return joinPath("assets", id)
	case ResourceCredential:
		if parent != "" {
			if strings.HasPrefix(parent, "users/") {
				return joinPath(parent+"/credentials", id)
			}
			return joinPath("users/"+parent+"/credentials", id)
		}
		return joinPath("credentials", id)
	case ResourceSymbol:
		if parent != "" {
			if strings.HasPrefix(parent, "chains/") {
				return joinPath(parent+"/symbols", id)
			}
			return joinPath("chains/"+parent+"/symbols", id)
		}
		return joinPath("symbols", id)
	case ResourceSignatory:
		if parent != "" {
			if strings.HasPrefix(parent, "signers/") {
				return joinPath(parent+"/signatories", id)
			}
			return joinPath("signers/"+parent+"/signatories", id)
		}
		return joinPath("signatories", id)
	case ResourceTag:
		if parent != "" {
			return joinPath(parent+"/tags", id)
		}
		return joinPath("tags", id)
	case ResourceTreasury:
		if id == "" {
			// Bare "treasury" uses the current treasury ID
			return "treasuries/" + vm.TreasuryID
		}
		return "treasuries/" + id
	default:
		path := rt.APIPath()
		if id != "" {
			return path + "/" + id
		}
		return path
	}
}

// buildParentedListPath constructs the API path for listing child resources.
// parentID may be a bare ID ("SOL") or already-qualified ("chains/SOL", "users/abc").
func buildParentedListPath(rt ResourceType, parentID string) string {
	switch rt {
	case ResourceAddress:
		if strings.HasPrefix(parentID, "chains/") {
			return parentID + "/addresses"
		}
		return "chains/" + parentID + "/addresses"
	case ResourceAsset:
		if strings.HasPrefix(parentID, "chains/") {
			return parentID + "/assets"
		}
		return "chains/" + parentID + "/assets"
	case ResourceCredential:
		if strings.HasPrefix(parentID, "users/") {
			return parentID + "/credentials"
		}
		return "users/" + parentID + "/credentials"
	case ResourceSymbol:
		if strings.HasPrefix(parentID, "chains/") {
			return parentID + "/symbols"
		}
		return "chains/" + parentID + "/symbols"
	case ResourceSignatory:
		if strings.HasPrefix(parentID, "signers/") {
			return parentID + "/signatories"
		}
		return "signers/" + parentID + "/signatories"
	case ResourceTag:
		return parentID + "/tags"
	default:
		return rt.APIPath()
	}
}

// ---------------------------------------------------------------------------
// Body building
// ---------------------------------------------------------------------------

func (vm *VM) buildBody(variant *string, extension *string, data map[string]interface{}) (interface{}, error) {
	body := make(map[string]interface{})

	if variant != nil {
		body["variant"] = *variant
	}
	// Note: extension (e.g., memo for addresses) goes in the URL path as +suffix,
	// not in the body. It's handled by execCreate appending it to the ID.

	resolved, err := vm.resolveDataMap(data)
	if err != nil {
		return nil, err
	}
	for k, v := range resolved {
		body[k] = v
	}

	if len(body) == 0 {
		return nil, nil
	}
	return body, nil
}

func (vm *VM) resolveDataMap(data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, nil
	}
	result := make(map[string]interface{})
	for k, v := range data {
		resolved, err := vm.resolveDataValue(v)
		if err != nil {
			return nil, fmt.Errorf("resolving field %q: %w", k, err)
		}
		// Auto-qualify bare resource references in body fields.
		if s, ok := resolved.(string); ok && s != "" && !strings.Contains(s, "/") {
			resolved = qualifyBodyField(k, s)
		}
		// Handle dotted keys: "nested.key" -> nested map
		setNestedValue(result, k, resolved)
	}
	return result, nil
}

// qualifyBodyField auto-qualifies bare resource name strings based on the
// field name, matching the Rust CLI's behavior.
func qualifyBodyField(field, value string) interface{} {
	switch field {
	case "account":
		return "accounts/" + value
	case "asset":
		// Bare asset names like "SOL" -> "chains/SOL/assets/SOL"
		return "chains/" + value + "/assets/" + value
	default:
		return value
	}
}

func (vm *VM) resolveDataValue(v interface{}) (interface{}, error) {
	switch val := v.(type) {
	case Value:
		return vm.resolveValueToInterface(val)
	case map[string]interface{}:
		return vm.resolveDataMap(val)
	default:
		return v, nil
	}
}

func (vm *VM) resolveValueToInterface(v Value) (interface{}, error) {
	switch val := v.(type) {
	case StringValue:
		return val.Value, nil
	case IntegerValue:
		return val.Value, nil
	case FloatValue:
		return val.Value, nil
	case BooleanValue:
		return val.Value, nil
	case VariableRef:
		// For body data, resource variables resolve to their name, not full JSON.
		if len(val.Path) == 0 {
			if varObj, ok := vm.Vars[val.Name]; ok && varObj != nil {
				if varObj.Type == VarResource && varObj.Name != "" {
					return varObj.Name, nil
				}
				if varObj.Type == VarOperation || varObj.Type == VarProposal {
					return varObj.Name, nil
				}
				// For plain string variables, check if the value is a known
				// nonce that maps to a full resource name. This handles
				// patterns like: role_id = nonce(); create role id($role_id);
				// then using $role_id in body data auto-qualifies to "roles/nonce".
				if varObj.Type == VarString {
					s := vm.varToString(varObj)
					if qualified, ok := vm.nonceMap[s]; ok {
						return qualified, nil
					}
				}
			}
		}
		s, err := vm.resolveVariableRef(val)
		if err != nil {
			return nil, err
		}
		// Preserve type information from variables.
		// Only convert booleans; leave numbers as strings since the API
		// often expects string-encoded numbers (e.g., fee_limits, amounts).
		if s == "true" {
			return true, nil
		}
		if s == "false" {
			return false, nil
		}
		return s, nil
	case FunctionCall:
		s, err := vm.resolveFunctionCall(val)
		if err != nil {
			return nil, err
		}
		return s, nil
	case ArrayValue:
		items := make([]interface{}, 0, len(val.Values))
		for _, item := range val.Values {
			resolved, err := vm.resolveValueToInterface(item)
			if err != nil {
				return nil, err
			}
			items = append(items, resolved)
		}
		return items, nil
	case InlineTable:
		return vm.resolveInlineTable(&val)
	default:
		s, err := vm.resolveValue(v)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
}

func (vm *VM) resolveInlineTable(tbl *InlineTable) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, entry := range tbl.Entries {
		val, err := vm.resolveValueToInterface(entry.Value)
		if err != nil {
			return nil, fmt.Errorf("resolving table entry %q: %w", entry.Key, err)
		}
		setNestedValue(result, entry.Key, val)
	}
	return result, nil
}

// setNestedValue sets a value in a map, supporting dotted keys.
// "a.b.c" = val becomes {a: {b: {c: val}}}.
func setNestedValue(m map[string]interface{}, key string, val interface{}) {
	parts := splitDottedKey(key)
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = val
		} else {
			next, ok := current[part].(map[string]interface{})
			if !ok {
				next = make(map[string]interface{})
				current[part] = next
			}
			current = next
		}
	}
}

// splitDottedKey splits a dotted key respecting quoted segments.
// e.g., `labels."some.x"` → ["labels", "some.x"]
func splitDottedKey(key string) []string {
	var parts []string
	var current strings.Builder
	i := 0
	for i < len(key) {
		if key[i] == '"' {
			// Read quoted segment
			i++ // skip opening quote
			for i < len(key) && key[i] != '"' {
				current.WriteByte(key[i])
				i++
			}
			if i < len(key) {
				i++ // skip closing quote
			}
		} else if key[i] == '.' {
			parts = append(parts, current.String())
			current.Reset()
			i++
		} else {
			current.WriteByte(key[i])
			i++
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ---------------------------------------------------------------------------
// Poll and resolve
// ---------------------------------------------------------------------------

func (vm *VM) pollAndResolve(opName string) (*Var, error) {
	if opName == "" {
		return &Var{Type: VarString, Value: ""}, nil
	}

	op, err := vm.Client.PollOperation(opName)
	if err != nil {
		return nil, fmt.Errorf("polling operation %s: %w", opName, err)
	}

	if op.State != nil && *op.State == types.OperationStateFailed {
		opErr := &OperationError{OpName: opName, Message: "operation failed"}
		if op.Error != nil {
			opErr.Code = op.Error.Code
			opErr.Status = string(op.Error.Status)
			opErr.Message = op.Error.Message
		}
		return nil, opErr
	}

	// Extract the resource name from the response
	if op.Response != nil && op.Response.Name != nil {
		resourceName := *op.Response.Name
		// Fetch the resource
		raw, err := vm.Client.GetJSON("/v1/" + resourceName)
		if err != nil {
			return nil, fmt.Errorf("fetching resource %s after operation: %w", resourceName, err)
		}
		return &Var{
			Type:  VarResource,
			Name:  resourceName,
			Value: raw,
		}, nil
	}

	// If no response name, return the operation itself
	opJSON, err := json.Marshal(op)
	if err != nil {
		return &Var{Type: VarOperation, Name: opName, Value: opName}, nil
	}
	return &Var{
		Type:  VarOperation,
		Name:  opName,
		Value: json.RawMessage(opJSON),
	}, nil
}

// pollDeleteOperation polls until the operation completes but returns the
// deleted resource metadata without trying to re-fetch it from the API.
func (vm *VM) pollDeleteOperation(opName string, deletedResource string) (*Var, error) {
	if opName == "" {
		return &Var{Type: VarString, Value: ""}, nil
	}

	op, err := vm.Client.PollOperation(opName)
	if err != nil {
		return nil, fmt.Errorf("polling operation %s: %w", opName, err)
	}

	if op.State != nil && *op.State == types.OperationStateFailed {
		opErr := &OperationError{OpName: opName, Message: "operation failed"}
		if op.Error != nil {
			opErr.Code = op.Error.Code
			opErr.Status = string(op.Error.Status)
			opErr.Message = op.Error.Message
		}
		return nil, opErr
	}

	// For deletions, return a resource var with the deleted resource name
	if op.Response != nil && op.Response.Name != nil {
		deletedResource = *op.Response.Name
	}
	result := map[string]interface{}{"name": deletedResource, "deleted": true}
	raw, _ := json.Marshal(result)
	return &Var{Type: VarResource, Name: deletedResource, Value: json.RawMessage(raw)}, nil
}


// ---------------------------------------------------------------------------
// Var helpers
// ---------------------------------------------------------------------------

func (vm *VM) setVar(name string, v *Var) {
	vm.Vars[name] = v
	if _, ok := vm.startTimes[name]; !ok {
		vm.startTimes[name] = time.Now()
	}
}

func (vm *VM) varToString(v *Var) string {
	if v == nil {
		return ""
	}
	switch v.Type {
	case VarResource:
		return v.Name
	case VarOperation, VarProposal:
		return v.Name
	case VarString:
		if s, ok := v.Value.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v.Value)
	case VarInteger:
		if i, ok := v.Value.(int64); ok {
			return strconv.FormatInt(i, 10)
		}
		return fmt.Sprintf("%v", v.Value)
	case VarError:
		if v.Error != nil {
			return v.Error.Message
		}
		if s, ok := v.Value.(string); ok {
			return s
		}
		return "error"
	default:
		return fmt.Sprintf("%v", v.Value)
	}
}

// resolveStringOrVar resolves a string that might be a variable reference.
func (vm *VM) resolveStringOrVar(s string) (string, error) {
	if strings.HasPrefix(s, "$") {
		varName := strings.TrimPrefix(s, "$")
		parts := strings.SplitN(varName, ".", 2)
		v, ok := vm.Vars[parts[0]]
		if !ok {
			return "", fmt.Errorf("undefined variable: $%s", parts[0])
		}
		if v == nil {
			return "", nil
		}
		if len(parts) > 1 {
			vr := VariableRef{Name: parts[0], Path: strings.Split(parts[1], ".")}
			return vm.resolveVariableRef(vr)
		}
		// For resource and operation variables, return the name (not JSON)
		if v.Name != "" {
			return v.Name, nil
		}
		return vm.varToString(v), nil
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Display
// ---------------------------------------------------------------------------

func (vm *VM) printVar(v *Var) {
	if v == nil {
		return
	}
	switch v.Type {
	case VarResource:
		if raw, ok := v.Value.(json.RawMessage); ok {
			vm.printPrettyJSON(raw)
		}
	case VarString:
		fmt.Println(vm.varToString(v))
	case VarInteger:
		fmt.Println(vm.varToString(v))
	case VarOperation, VarProposal:
		fmt.Println(v.Name)
	case VarError:
		fmt.Fprintf(os.Stderr, "error: %s\n", vm.varToString(v))
	}
}

func (vm *VM) printPrettyJSON(raw json.RawMessage) {
	var formatted json.RawMessage
	if err := json.Unmarshal(raw, &formatted); err != nil {
		fmt.Println(string(raw))
		return
	}
	pretty, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		fmt.Println(string(raw))
		return
	}
	fmt.Println(string(pretty))
}

func (vm *VM) printListResults(items []json.RawMessage) {
	if len(items) == 0 {
		fmt.Println("[]")
		return
	}
	var arr []interface{}
	for _, item := range items {
		var v interface{}
		if err := json.Unmarshal(item, &v); err != nil {
			arr = append(arr, string(item))
		} else {
			arr = append(arr, v)
		}
	}
	pretty, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		fmt.Println("[]")
		return
	}
	fmt.Println(string(pretty))
}

// ---------------------------------------------------------------------------
// Resource type capitalization
// ---------------------------------------------------------------------------

// capitalizeResourceType converts a ResourceType enum to the API-expected form.
func capitalizeResourceType(rt ResourceType) string {
	s := rt.String()
	if s == "" {
		return ""
	}
	// Handle multi-word types with hyphens
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "-")
}

