// Package csl implements a parser for the Cordial Scripting Language (.csl files).
// It parses CSL source text line by line and produces an abstract syntax tree (AST).
package csl

import "fmt"

// ---------------------------------------------------------------------------
// Resource types
// ---------------------------------------------------------------------------

// ResourceType enumerates every resource kind recognised by CSL.
type ResourceType int

const (
	ResourceAccount ResourceType = iota
	ResourceAddress
	ResourceAsset
	ResourceChain
	ResourceCredential
	ResourceKey
	ResourceRole
	ResourceUser
	ResourceTransfer
	ResourceTransaction
	ResourceOperation
	ResourceTreasury
	ResourceSigner
	ResourceSignatory
	ResourceSymbol
	ResourceTag
	ResourceFeature
	ResourceStaking
	ResourceSoftwareUpdate
	ResourceCall
	ResourceType_
	ResourceHost
	ResourceAccessRule
	ResourceTransferRule
	ResourceCallRule
	ResourceStakingRule
	ResourceClientKey
	ResourceSignature
	ResourceAccessPolicy
	ResourceTransferPolicy
	ResourceCallPolicy
	ResourceStakingPolicy
)

// String returns the canonical singular display name of the resource type.
func (r ResourceType) String() string {
	switch r {
	case ResourceAccount:
		return "account"
	case ResourceAddress:
		return "address"
	case ResourceAsset:
		return "asset"
	case ResourceChain:
		return "chain"
	case ResourceCredential:
		return "credential"
	case ResourceKey:
		return "key"
	case ResourceRole:
		return "role"
	case ResourceUser:
		return "user"
	case ResourceTransfer:
		return "transfer"
	case ResourceTransaction:
		return "transaction"
	case ResourceOperation:
		return "operation"
	case ResourceTreasury:
		return "treasury"
	case ResourceSigner:
		return "signer"
	case ResourceSignatory:
		return "signatory"
	case ResourceSymbol:
		return "symbol"
	case ResourceTag:
		return "tag"
	case ResourceFeature:
		return "feature"
	case ResourceStaking:
		return "staking"
	case ResourceSoftwareUpdate:
		return "software-update"
	case ResourceCall:
		return "call"
	case ResourceType_:
		return "type"
	case ResourceHost:
		return "host"
	case ResourceAccessRule:
		return "access-rule"
	case ResourceTransferRule:
		return "transfer-rule"
	case ResourceCallRule:
		return "call-rule"
	case ResourceStakingRule:
		return "staking-rule"
	case ResourceClientKey:
		return "client-key"
	case ResourceSignature:
		return "signature"
	case ResourceAccessPolicy:
		return "access policy"
	case ResourceTransferPolicy:
		return "transfer policy"
	case ResourceCallPolicy:
		return "call policy"
	case ResourceStakingPolicy:
		return "staking policy"
	default:
		return "unknown"
	}
}

// APIPath returns the URL path segment used for the resource type.
func (r ResourceType) APIPath() string {
	switch r {
	case ResourceAccount:
		return "accounts"
	case ResourceAddress:
		return "addresses"
	case ResourceAsset:
		return "assets"
	case ResourceChain:
		return "chains"
	case ResourceCredential:
		return "credentials"
	case ResourceKey:
		return "keys"
	case ResourceRole:
		return "roles"
	case ResourceUser:
		return "users"
	case ResourceTransfer:
		return "transfers"
	case ResourceTransaction:
		return "transactions"
	case ResourceOperation:
		return "operations"
	case ResourceTreasury:
		return "treasuries"
	case ResourceSigner:
		return "signers"
	case ResourceSignatory:
		return "signatories"
	case ResourceSymbol:
		return "symbols"
	case ResourceTag:
		return "tags"
	case ResourceFeature:
		return "features"
	case ResourceStaking:
		return "stakings"
	case ResourceSoftwareUpdate:
		return "software-updates"
	case ResourceCall:
		return "calls"
	case ResourceType_:
		return "types"
	case ResourceHost:
		return "hosts"
	case ResourceAccessRule:
		return "access-rules"
	case ResourceTransferRule:
		return "transfer-rules"
	case ResourceCallRule:
		return "call-rules"
	case ResourceStakingRule:
		return "staking-rules"
	case ResourceClientKey:
		return "client-keys"
	case ResourceSignature:
		return "signatures"
	case ResourceAccessPolicy:
		return "access-policy"
	case ResourceTransferPolicy:
		return "transfer-policy"
	case ResourceCallPolicy:
		return "call-policy"
	case ResourceStakingPolicy:
		return "staking-policy"
	default:
		return "unknown"
	}
}

// ---------------------------------------------------------------------------
// Command – interface implemented by every AST node that represents one line.
// ---------------------------------------------------------------------------

// Command is the interface satisfied by every CSL command AST node.
type Command interface {
	commandType() string
}

// ---------------------------------------------------------------------------
// Concrete command types
// ---------------------------------------------------------------------------

// Nop represents an empty line, comment, or shebang.
type Nop struct{}

func (Nop) commandType() string { return "Nop" }

// Exit represents the "exit" command.
type Exit struct{}

func (Exit) commandType() string { return "Exit" }

// ExitIfSet represents "exit_if_set <envvar>".
type ExitIfSet struct {
	EnvVar string
}

func (ExitIfSet) commandType() string { return "ExitIfSet" }

// SetSetting represents "set <key...> <value> [value2] [value3]".
type SetSetting struct {
	Key    []string
	Value  string
	Value2 *string
	Value3 *string
}

func (SetSetting) commandType() string { return "SetSetting" }

// GetSetting represents "get setting <key...>".
type GetSetting struct {
	Key []string
}

func (GetSetting) commandType() string { return "GetSetting" }

// UnsetSetting represents "unset <key...>".
type UnsetSetting struct {
	Key []string
}

func (UnsetSetting) commandType() string { return "UnsetSetting" }

// Assignable is the right-hand side of a variable assignment.
// It can be a Value, a Command (whose result is assigned), etc.
type Assignable interface {
	assignableType() string
}

// ValueAssignable wraps a Value for use in assignments.
type ValueAssignable struct {
	Value Value
}

func (ValueAssignable) assignableType() string { return "Value" }

// CommandAssignable wraps a Command whose result is assigned to a variable.
type CommandAssignable struct {
	Cmd Command
}

func (CommandAssignable) assignableType() string { return "Command" }

// Mutate describes the right-hand side of a fallible assignment.
type Mutate struct {
	Value Assignable
}

// Assignment represents "$var = <value>" or "$var := <value>".
// Direct=true means `:=` (direct/literal assignment).
type Assignment struct {
	Variable string
	Value    Assignable
	Direct   bool
}

func (Assignment) commandType() string { return "Assignment" }

// FallibleAssignment represents "$var, $err = <value>" or "$var, $err := <value>".
type FallibleAssignment struct {
	Variable string
	Error    string
	Mutate   Mutate
	Direct   bool
}

func (FallibleAssignment) commandType() string { return "FallibleAssignment" }

// Create represents "create [variant] <resource> [id] [for <parent>] [with <ext>] [{data}]".
type Create struct {
	Variant      *string
	ResourceType ResourceType
	Parent       *string
	Id           *string
	Extension    *string
	Data         map[string]interface{}
}

func (Create) commandType() string { return "Create" }

// Propose has the same shape as Create but represents the "propose" verb.
type Propose struct {
	Variant      *string
	ResourceType ResourceType
	Parent       *string
	Id           *string
	Extension    *string
	Data         map[string]interface{}
}

func (Propose) commandType() string { return "Propose" }

// Update represents "update <target> {data}".
type Update struct {
	Target PartialOrVariable
	Data   map[string]interface{}
}

func (Update) commandType() string { return "Update" }

// Delete represents "delete <target>".
type Delete struct {
	Target   PartialOrVariable
	Proposed bool
}

func (Delete) commandType() string { return "Delete" }

// Get represents "get <target>".
type Get struct {
	Target   PartialOrVariable
	Proposed bool
}

func (Get) commandType() string { return "Get" }

// ListCmd represents "list <resource> [for <parent>] [| <filter>]".
type ListCmd struct {
	ResourceType ResourceType
	Parent       *IdOrVariable
	Filter       *string
	Proposed     bool
}

func (ListCmd) commandType() string { return "List" }

// Assert represents "assert <condition>".
type Assert struct {
	Condition string
}

func (Assert) commandType() string { return "Assert" }

// Until represents "until <condition>".
type Until struct {
	Condition string
}

func (Until) commandType() string { return "Until" }

// ForLoop represents "for <var> in <iterable> { <command> }".
type ForLoop struct {
	Variable string
	Iterable Iterable
	Body     Command
}

func (ForLoop) commandType() string { return "ForLoop" }

// Custom represents "custom <action> <target> [payload]".
type Custom struct {
	Action  string
	Target  PartialOrVariable
	Payload *CustomPayload
}

func (Custom) commandType() string { return "Custom" }

// Approve represents "approve <target>".
type Approve struct {
	Target PartialOrVariable
}

func (Approve) commandType() string { return "Approve" }

// Submit represents "submit <target>".
type Submit struct {
	Target PartialOrVariable
}

func (Submit) commandType() string { return "Submit" }

// Cancel represents "cancel <target>".
type Cancel struct {
	Target PartialOrVariable
}

func (Cancel) commandType() string { return "Cancel" }

// Blueprint represents "blueprint <type> <root-invite>".
type Blueprint struct {
	Type       string
	RootInvite string
}

func (Blueprint) commandType() string { return "Blueprint" }

// DownloadTreasury represents "download treasury <id> [<directory>]".
type DownloadTreasury struct {
	TreasuryId string
	Directory  *string
}

func (DownloadTreasury) commandType() string { return "DownloadTreasury" }

// UploadBackup represents "upload backup <file>".
type UploadBackup struct {
	File string
}

func (UploadBackup) commandType() string { return "UploadBackup" }

// ValueCmd wraps a standalone value expression as a command.
type ValueCmd struct {
	Value Value
}

func (ValueCmd) commandType() string { return "Value" }

// Convert represents "convert <data> from <source-encoding> to <target-encoding>".
type Convert struct {
	Data           Value
	SourceEncoding Value
	TargetEncoding Value
}

func (Convert) commandType() string { return "Convert" }

// Replace represents "replace <data> <old> <new>".
type Replace struct {
	Data Value
	Old  Value
	New  Value
}

func (Replace) commandType() string { return "Replace" }

// ---------------------------------------------------------------------------
// Value – interface for any expression that produces a value.
// ---------------------------------------------------------------------------

// Value is the interface satisfied by every CSL value expression.
type Value interface {
	valueType() string
}

// StringValue is a literal string.
type StringValue struct {
	Value string
}

func (StringValue) valueType() string { return "String" }

// IntegerValue is a literal integer.
type IntegerValue struct {
	Value int64
}

func (IntegerValue) valueType() string { return "Integer" }

// FloatValue is a literal floating-point number.
type FloatValue struct {
	Value float64
}

func (FloatValue) valueType() string { return "Float" }

// BooleanValue is a literal boolean.
type BooleanValue struct {
	Value bool
}

func (BooleanValue) valueType() string { return "Boolean" }

// ArrayValue is a literal array of values.
type ArrayValue struct {
	Values []Value
}

func (ArrayValue) valueType() string { return "Array" }

// InlineTable is a TOML-style `{ key = value, ... }` literal.
type InlineTable struct {
	Entries []TableEntry
}

func (InlineTable) valueType() string { return "InlineTable" }

// TableEntry is a single key/value pair inside an InlineTable.
// Key may be a dotted path like "nested.key" or a quoted key like "chains/SOL".
type TableEntry struct {
	Key   string
	Value Value
}

// VariableRef is a reference to a variable, optionally with a dotted path.
// For "$var.field1.field2", Name="var" and Path=["field1","field2"].
type VariableRef struct {
	Name string
	Path []string
}

func (VariableRef) valueType() string { return "VariableRef" }

// FunctionCall is an invocation of a named function with arguments.
type FunctionCall struct {
	Name string
	Args []Value
}

func (FunctionCall) valueType() string { return "FunctionCall" }

// ---------------------------------------------------------------------------
// PartialOrVariable – either a resource partial or a $variable reference.
// ---------------------------------------------------------------------------

// PartialOrVariable identifies a resource either by a $variable or by
// explicit resource-type / parent / id / extension components.
type PartialOrVariable struct {
	IsVariable   bool
	VarName      string       // populated when IsVariable is true
	ResourceType ResourceType // populated when IsVariable is false
	Parent       string
	Id           string
	Extension    string // e.g. "with memo1"
}

// String returns a human-readable representation.
func (p PartialOrVariable) String() string {
	if p.IsVariable {
		return "$" + p.VarName
	}
	s := p.ResourceType.String()
	if p.Parent != "" {
		s += " for " + p.Parent
	}
	if p.Id != "" {
		s += " " + p.Id
	}
	if p.Extension != "" {
		s += " with " + p.Extension
	}
	return s
}

// ---------------------------------------------------------------------------
// IdOrVariable
// ---------------------------------------------------------------------------

// IdOrVariable is either a literal id string or a $variable reference.
type IdOrVariable struct {
	IsVariable bool
	VarName    string
	Id         string
}

// String returns the textual representation.
func (iv IdOrVariable) String() string {
	if iv.IsVariable {
		return "$" + iv.VarName
	}
	return iv.Id
}

// ---------------------------------------------------------------------------
// Iterable – used in for loops
// ---------------------------------------------------------------------------

// Iterable is the interface for things that can appear after "in" in a for loop.
type Iterable interface {
	iterableType() string
}

// ArrayIterable is an array literal used as a for-loop iterable.
type ArrayIterable struct {
	Values []Value
}

func (ArrayIterable) iterableType() string { return "Array" }

// RangeIterable is an integer range (start..end or start..=end).
type RangeIterable struct {
	Start     int64
	End       int64
	Inclusive bool
}

func (RangeIterable) iterableType() string { return "Range" }

// ListIterable wraps a list command used as a for-loop iterable.
type ListIterable struct {
	List *ListCmd
}

func (ListIterable) iterableType() string { return "List" }

// ---------------------------------------------------------------------------
// CustomPayload
// ---------------------------------------------------------------------------

// CustomPayload is the optional payload of a Custom command, either a string
// or an inline table.
type CustomPayload struct {
	IsString    bool
	StringValue string
	TableValue  *InlineTable
}

// ---------------------------------------------------------------------------
// Program – the top-level AST node
// ---------------------------------------------------------------------------

// Program is the top-level result of parsing a CSL file.
type Program struct {
	Commands []Command
}

// ---------------------------------------------------------------------------
// ParseError
// ---------------------------------------------------------------------------

// ParseError represents a parsing error with line information.
type ParseError struct {
	Line    int
	Column  int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Line, e.Column, e.Message)
}
