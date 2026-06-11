package csl

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Parse parses a complete CSL program (multi-line source text) and returns
// a Program AST or the first error encountered.
func Parse(source string) (*Program, error) {
	p := &parser{lines: strings.Split(source, "\n")}
	var cmds []Command
	for p.lineIdx < len(p.lines) {
		cmd, err := p.parseLine(p.lines[p.lineIdx])
		if err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
		p.lineIdx++
	}
	return &Program{Commands: cmds}, nil
}

// ParseLine parses a single CSL line and returns the corresponding Command.
func ParseLine(line string) (Command, error) {
	p := &parser{lines: []string{line}, lineIdx: 0}
	return p.parseLine(line)
}

// ---------------------------------------------------------------------------
// Resource-type lookup tables
// ---------------------------------------------------------------------------

// singularResourceTypes maps lowercase singular forms to ResourceType values.
var singularResourceTypes = map[string]ResourceType{
	"account":         ResourceAccount,
	"address":         ResourceAddress,
	"asset":           ResourceAsset,
	"chain":           ResourceChain,
	"credential":      ResourceCredential,
	"key":             ResourceKey,
	"role":            ResourceRole,
	"user":            ResourceUser,
	"transfer":        ResourceTransfer,
	"transaction":     ResourceTransaction,
	"operation":       ResourceOperation,
	"treasury":        ResourceTreasury,
	"signer":          ResourceSigner,
	"signatory":       ResourceSignatory,
	"symbol":          ResourceSymbol,
	"tag":             ResourceTag,
	"feature":         ResourceFeature,
	"staking":         ResourceStaking,
	"software-update": ResourceSoftwareUpdate,
	"call":            ResourceCall,
	"type":            ResourceType_,
	"host":            ResourceHost,
	"access-rule":     ResourceAccessRule,
	"transfer-rule":   ResourceTransferRule,
	"call-rule":       ResourceCallRule,
	"staking-rule":    ResourceStakingRule,
	"client-key":      ResourceClientKey,
	"signature":       ResourceSignature,
	// Two-word space-separated forms used in CSL scripts
	"access rule":     ResourceAccessRule,
	"transfer rule":   ResourceTransferRule,
	"call rule":       ResourceCallRule,
	"staking rule":    ResourceStakingRule,
	"client key":      ResourceClientKey,
	"software update": ResourceSoftwareUpdate,
}

// pluralResourceTypes maps lowercase plural forms (including multi-word) to ResourceType values.
var pluralResourceTypes = map[string]ResourceType{
	"accounts":         ResourceAccount,
	"addresses":        ResourceAddress,
	"assets":           ResourceAsset,
	"chains":           ResourceChain,
	"credentials":      ResourceCredential,
	"keys":             ResourceKey,
	"roles":            ResourceRole,
	"users":            ResourceUser,
	"transfers":        ResourceTransfer,
	"transactions":     ResourceTransaction,
	"operations":       ResourceOperation,
	"treasuries":       ResourceTreasury,
	"signers":          ResourceSigner,
	"signatories":      ResourceSignatory,
	"symbols":          ResourceSymbol,
	"tags":             ResourceTag,
	"features":         ResourceFeature,
	"stakings":         ResourceStaking,
	"software-updates": ResourceSoftwareUpdate,
	"calls":            ResourceCall,
	"types":            ResourceType_,
	"hosts":            ResourceHost,
	"access rules":     ResourceAccessRule,
	"transfer rules":   ResourceTransferRule,
	"call rules":       ResourceCallRule,
	"staking rules":    ResourceStakingRule,
	"client keys":      ResourceClientKey,
	"signatures":       ResourceSignature,
	"access policy":    ResourceAccessPolicy,
	"transfer policy":  ResourceTransferPolicy,
	"call policy":      ResourceCallPolicy,
	"staking policy":   ResourceStakingPolicy,
}

// variantSets maps resource types to their allowed variant strings.
var variantSets = map[ResourceType][]string{
	ResourceAccount:      {"internal", "shared", "external", "contract", "validator"},
	ResourceAddress:      {"internal", "shared", "external", "contract", "validator"},
	ResourceUser:         {"human", "machine"},
	ResourceCredential:   {"k256", "p256", "invite", "ed255", "web-authn", "web-authn-uv"},
	ResourceKey:          {"engine", "shared", "user", "internal"},
	ResourceAccessRule:   {"allow", "require", "deny"},
	ResourceTransferRule: {"allow", "require", "deny"},
	ResourceCallRule:     {"allow", "require", "deny"},
	ResourceStakingRule:  {"allow", "require", "deny"},
	ResourceClientKey:    {"k256", "p256", "invite", "ed255", "ed25519", "session"},
	ResourceAsset:        {"native", "token"},
	ResourceChain:        {"native", "custom"},
	ResourceStaking:      {"stake", "unstake", "withdraw"},
	ResourceSignatory:    {"mock-yubi-hsm2", "yubi-hsm2"},
}

// ---------------------------------------------------------------------------
// Internal parser state
// ---------------------------------------------------------------------------

type parser struct {
	lines   []string
	lineIdx int // current line number (0-based)
}

func (p *parser) errorf(format string, args ...interface{}) error {
	return &ParseError{
		Line:    p.lineIdx + 1,
		Column:  0,
		Message: fmt.Sprintf(format, args...),
	}
}

// ---------------------------------------------------------------------------
// Top-level line parser
// ---------------------------------------------------------------------------

func (p *parser) parseLine(raw string) (Command, error) {
	line := strings.TrimSpace(raw)

	// Empty / comment / shebang -> Nop
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
		return Nop{}, nil
	}

	// Tokenise the line into words (respecting quotes, braces, arrays).
	tokens := tokeniseLine(line)
	if len(tokens) == 0 {
		return Nop{}, nil
	}

	keyword := tokens[0]

	switch keyword {
	case "exit":
		return Exit{}, nil
	case "exit_if_set":
		if len(tokens) < 2 {
			return nil, p.errorf("exit_if_set requires an environment variable name")
		}
		return ExitIfSet{EnvVar: tokens[1]}, nil
	case "set":
		return p.parseSetSetting(tokens[1:])
	case "unset":
		return UnsetSetting{Key: tokens[1:]}, nil
	case "assert":
		cond := strings.TrimSpace(strings.TrimPrefix(line, "assert"))
		return Assert{Condition: cond}, nil
	case "until":
		cond := strings.TrimSpace(strings.TrimPrefix(line, "until"))
		return Until{Condition: cond}, nil
	case "create":
		return p.parseCreate(line, tokens[1:])
	case "propose":
		return p.parsePropose(line, tokens[1:])
	case "update":
		return p.parseUpdate(line, tokens[1:])
	case "delete":
		return p.parseDelete(tokens[1:])
	case "get":
		return p.parseGet(tokens[1:], line)
	case "list":
		return p.parseList(tokens[1:], line)
	case "approve":
		return p.parseApprove(tokens[1:])
	case "submit":
		return p.parseSubmit(tokens[1:])
	case "cancel":
		return p.parseCancel(tokens[1:])
	case "for":
		return p.parseForLoop(line)
	case "blueprint":
		return p.parseBlueprint(tokens[1:])
	case "download":
		return p.parseDownload(tokens[1:])
	case "upload":
		return p.parseUpload(tokens[1:])
	case "convert":
		return p.parseConvert(line, tokens[1:])
	case "replace":
		return p.parseReplace(line, tokens[1:])
	case "custom":
		return p.parseCustom(line, tokens[1:])
	// Custom actions parsed directly by keyword
	case "abort", "retry", "recheck", "activate", "disable", "heartbeat",
		"fee-payer", "load", "price":
		return p.parseCustomAction(keyword, tokens[1:])
	// Shorthand resource access: address XYZ for SOL, asset SOL for SOL, key NAME
	case "address", "asset", "key", "user", "account", "role",
		"chain", "symbol", "tag", "credential", "transfer",
		"staking", "transaction", "operation", "signer", "signatory",
		"feature", "treasury", "type", "host", "call", "signature",
		"access-rule", "transfer-rule", "call-rule", "staking-rule",
		"client-key", "client", "software-update":
		return p.parseResourceShorthand(keyword, tokens[1:], line)
	// Plural resource type keywords as standalone list commands
	case "treasuries", "accounts", "users", "roles", "keys", "addresses",
		"transfers", "operations", "transactions", "features", "chains",
		"assets", "symbols", "tags", "credentials", "signers", "signatures",
		"stakings":
		if rt, ok := pluralResourceTypes[keyword]; ok {
			if len(tokens) > 1 {
				return p.parseList(tokens[1:], line)
			}
			return ListCmd{ResourceType: rt}, nil
		}
	}

	// Variable assignment: $var = ... or $var, $err = ...
	// But a bare $var (no = or :=) is a display command
	if strings.HasPrefix(keyword, "$") {
		if strings.Contains(line, "=") || strings.Contains(line, ":=") {
			return p.parseAssignment(line)
		}
		val, err := p.parseValueFromString(line)
		if err != nil {
			return nil, p.errorf("invalid variable reference: %s", line)
		}
		return ValueCmd{Value: val}, nil
	}

	// Check if this is an assignment (identifier = value or identifier := value)
	if strings.Contains(line, ":=") || strings.Contains(line, "=") {
		// Fallible assignment: "_, err = ..." or "x, err = ..."
		eqIdx := strings.Index(line, "=")
		if eqIdx > 0 {
			lhs := strings.TrimSpace(line[:eqIdx])
			lhs = strings.TrimSuffix(lhs, ":") // handle `:=`
			if strings.Contains(lhs, ",") {
				return p.parseAssignment(line)
			}
		}
		// Make sure it's not a comparison inside a value expression
		// by checking that the first token looks like an identifier.
		// Also handle the case where = is attached to the identifier (e.g., "keyETH= value")
		if isIdentifier(keyword) {
			return p.parseAssignment(line)
		}
		if strings.Contains(keyword, "=") {
			beforeEq := strings.SplitN(keyword, "=", 2)[0]
			beforeEq = strings.TrimSuffix(beforeEq, ":")
			if isIdentifier(beforeEq) {
				return p.parseAssignment(line)
			}
		}
	}

	// Fallback: try to interpret as a standalone value expression.
	val, err := p.parseValueFromString(line)
	if err != nil {
		return nil, p.errorf("unrecognised command: %s", keyword)
	}
	return ValueCmd{Value: val}, nil
}

// ---------------------------------------------------------------------------
// Setting commands
// ---------------------------------------------------------------------------

func (p *parser) parseSetSetting(tokens []string) (Command, error) {
	if len(tokens) < 2 {
		return nil, p.errorf("set requires at least a key and a value")
	}
	// Remove "=" separator if present (e.g. "sign.with = root-key" -> "sign.with root-key")
	var filtered []string
	for _, t := range tokens {
		if t != "=" {
			filtered = append(filtered, t)
		}
	}
	tokens = filtered
	if len(tokens) < 2 {
		return nil, p.errorf("set requires at least a key and a value")
	}
	// Last token is the value (or last 2-3 are values).
	// Heuristic: keys don't start with quotes or $.
	// We treat everything before the first "value-like" token as the key.
	keyParts, valueParts := splitSettingKeyValue(tokens)
	if len(keyParts) == 0 || len(valueParts) == 0 {
		return nil, p.errorf("set: could not determine key and value")
	}
	cmd := SetSetting{Key: keyParts, Value: valueParts[0]}
	if len(valueParts) > 1 {
		v2 := valueParts[1]
		cmd.Value2 = &v2
	}
	if len(valueParts) > 2 {
		v3 := valueParts[2]
		cmd.Value3 = &v3
	}
	return cmd, nil
}

// splitSettingKeyValue splits tokens into key-parts and value-parts.
// The value starts at the first token that looks like a value (quoted string,
// variable, number, boolean, etc.).
func splitSettingKeyValue(tokens []string) ([]string, []string) {
	for i, t := range tokens {
		if i == 0 {
			continue // first token is always part of the key
		}
		if isValueToken(t) {
			return tokens[:i], tokens[i:]
		}
	}
	// Fallback: everything except last is key
	if len(tokens) >= 2 {
		return tokens[:len(tokens)-1], tokens[len(tokens)-1:]
	}
	return tokens, nil
}

// isIdentifier returns true if the string looks like a CSL identifier (variable name).
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	s = strings.TrimPrefix(s, "$")
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
				return false
			}
		}
	}
	return true
}

func isValueToken(t string) bool {
	if strings.HasPrefix(t, "\"") || strings.HasPrefix(t, "$") {
		return true
	}
	if t == "true" || t == "false" {
		return true
	}
	if _, err := strconv.ParseInt(t, 10, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(t, 64); err == nil {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Create / Propose
// ---------------------------------------------------------------------------

func (p *parser) parseCreate(line string, tokens []string) (Command, error) {
	c, err := p.parseCreateLike(line, tokens)
	if err != nil {
		return nil, err
	}
	return Create{
		Variant:      c.variant,
		ResourceType: c.resourceType,
		Parent:       c.parent,
		Id:           c.id,
		Extension:    c.extension,
		Data:         c.data,
	}, nil
}

func (p *parser) parsePropose(line string, tokens []string) (Command, error) {
	c, err := p.parseCreateLike(line, tokens)
	if err != nil {
		return nil, err
	}
	return Propose{
		Variant:      c.variant,
		ResourceType: c.resourceType,
		Parent:       c.parent,
		Id:           c.id,
		Extension:    c.extension,
		Data:         c.data,
	}, nil
}

type createParts struct {
	variant      *string
	resourceType ResourceType
	parent       *string
	id           *string
	extension    *string
	data         map[string]interface{}
}

func (p *parser) parseCreateLike(line string, tokens []string) (*createParts, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("create/propose requires a resource type")
	}

	idx := 0
	cp := &createParts{}

	// Try to match a resource type directly from the first token(s).
	resType, consumed, err := p.matchResourceType(tokens[idx:])
	if err != nil {
		// First token is not a resource type. It might be a variant.
		// Try: tokens[0] is a variant, tokens[1:] is the resource type.
		if len(tokens) >= 2 {
			rt2, consumed2, err2 := p.matchResourceType(tokens[1:])
			if err2 != nil {
				return nil, p.errorf("unrecognised resource type: %s %s", tokens[0], tokens[1])
			}
			if isVariantFor(rt2, tokens[0]) {
				v := tokens[0]
				cp.variant = &v
				cp.resourceType = rt2
				idx = 1 + consumed2
			} else {
				return nil, p.errorf("'%s' is not a valid variant for %s", tokens[0], rt2)
			}
		} else {
			return nil, err
		}
	} else {
		// First token matched a resource type. But check if the first token
		// could also be a variant for the next token's resource type.
		if consumed == 1 && len(tokens[idx:]) >= 2 {
			rt2, consumed2, err2 := p.matchResourceType(tokens[idx+1:])
			if err2 == nil && consumed2 >= 1 {
				if isVariantFor(rt2, tokens[idx]) {
					v := tokens[idx]
					cp.variant = &v
					cp.resourceType = rt2
					idx += 1 + consumed2
				} else {
					cp.resourceType = resType
					idx += consumed
				}
			} else {
				cp.resourceType = resType
				idx += consumed
			}
		} else {
			cp.resourceType = resType
			idx += consumed
		}
	}

	// Parse remaining tokens: [id] [for <parent>] [with <extension>] [{data}]
	for idx < len(tokens) {
		tok := tokens[idx]
		switch {
		case tok == "for":
			idx++
			if idx >= len(tokens) {
				return nil, p.errorf("expected parent after 'for'")
			}
			parent := resolveIdToken(tokens[idx], p)
			cp.parent = &parent
			idx++
		case tok == "with":
			idx++
			if idx >= len(tokens) {
				return nil, p.errorf("expected extension after 'with'")
			}
			ext := stripQuotes(tokens[idx])
			cp.extension = &ext
			idx++
		case strings.HasPrefix(tok, "{"):
			// Inline table data – parse from the raw line.
			braceStart := strings.Index(line, "{")
			if braceStart < 0 {
				return nil, p.errorf("expected inline table data")
			}
			tbl, err := p.parseInlineTableFromString(line[braceStart:])
			if err != nil {
				return nil, err
			}
			cp.data = tableToMap(tbl)
			idx = len(tokens) // consumed rest
		default:
			// Must be the id.
			if cp.id == nil {
				id := resolveIdToken(tok, p)
				cp.id = &id
				idx++
			} else {
				idx++ // skip unknown token
			}
		}
	}

	return cp, nil
}

func resolveIdToken(tok string, p *parser) string {
	// id($var) or id("literal") – extract inner value.
	// Preserve $ prefix so resolveStringOrVar can detect explicit variable
	// references, while bare tokens like SOL remain literal IDs.
	if strings.HasPrefix(tok, "id(") && strings.HasSuffix(tok, ")") {
		inner := tok[3 : len(tok)-1]
		return stripQuotes(inner)
	}
	return stripQuotes(tok)
}

func stripVariable(s string) string {
	if strings.HasPrefix(s, "$") {
		return s[1:]
	}
	return s
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// isVariantFor checks if v is a recognised variant for resource type rt.
func isVariantFor(rt ResourceType, v string) bool {
	variants, ok := variantSets[rt]
	if !ok {
		return false
	}
	for _, allowed := range variants {
		if v == allowed {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (p *parser) parseUpdate(line string, tokens []string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("update requires a target")
	}

	target, remaining, err := p.parsePartialOrVariable(tokens)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	// Find inline table in remaining tokens or raw line.
	braceIdx := -1
	for i, t := range remaining {
		if strings.HasPrefix(t, "{") {
			braceIdx = i
			break
		}
	}
	if braceIdx >= 0 {
		braceStart := strings.Index(line, "{")
		if braceStart >= 0 {
			tbl, err := p.parseInlineTableFromString(line[braceStart:])
			if err != nil {
				return nil, err
			}
			data = tableToMap(tbl)
		}
	}

	return Update{Target: target, Data: data}, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func (p *parser) parseDelete(tokens []string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("delete requires a target")
	}
	proposed := false
	idx := 0
	if tokens[0] == "proposed" {
		proposed = true
		idx++
	}
	target, _, err := p.parsePartialOrVariable(tokens[idx:])
	if err != nil {
		return nil, err
	}
	return Delete{Target: target, Proposed: proposed}, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func (p *parser) parseGet(tokens []string, line string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("get requires a target")
	}

	// "get setting <key...>"
	if tokens[0] == "setting" {
		return GetSetting{Key: tokens[1:]}, nil
	}

	proposed := false
	idx := 0
	if tokens[0] == "proposed" {
		proposed = true
		idx++
	}
	target, _, err := p.parsePartialOrVariable(tokens[idx:])
	if err != nil {
		return nil, err
	}
	return Get{Target: target, Proposed: proposed}, nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func (p *parser) parseList(tokens []string, line string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("list requires a resource type")
	}

	proposed := false
	idx := 0
	if tokens[0] == "proposed" {
		proposed = true
		idx++
	}

	rt, consumed, err := p.matchPluralResourceType(tokens[idx:])
	if err != nil {
		// Try singular as well.
		rt, consumed, err = p.matchResourceType(tokens[idx:])
		if err != nil {
			return nil, p.errorf("list: unrecognised resource type: %s", tokens[idx])
		}
	}
	idx += consumed

	cmd := ListCmd{ResourceType: rt, Proposed: proposed}

	// Optional: for <parent>
	if idx < len(tokens) && tokens[idx] == "for" {
		idx++
		if idx >= len(tokens) {
			return nil, p.errorf("expected parent after 'for'")
		}
		pv := parseIdOrVariable(tokens[idx])
		cmd.Parent = &pv
		idx++
	}

	// Optional: | <filter>
	pipeIdx := strings.Index(line, "|")
	if pipeIdx >= 0 {
		filter := strings.TrimSpace(line[pipeIdx+1:])
		cmd.Filter = &filter
	}

	return cmd, nil
}

func parseIdOrVariable(tok string) IdOrVariable {
	if strings.HasPrefix(tok, "$") {
		return IdOrVariable{IsVariable: true, VarName: tok[1:]}
	}
	return IdOrVariable{Id: stripQuotes(tok)}
}

// ---------------------------------------------------------------------------
// Approve / Submit / Cancel
// ---------------------------------------------------------------------------

func (p *parser) parseApprove(tokens []string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("approve requires a target")
	}
	target, _, err := p.parsePartialOrVariable(tokens)
	if err != nil {
		return nil, err
	}
	return Approve{Target: target}, nil
}

func (p *parser) parseSubmit(tokens []string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("submit requires a target")
	}
	target, _, err := p.parsePartialOrVariable(tokens)
	if err != nil {
		return nil, err
	}
	return Submit{Target: target}, nil
}

func (p *parser) parseCancel(tokens []string) (Command, error) {
	if len(tokens) == 0 {
		return nil, p.errorf("cancel requires a target")
	}
	target, _, err := p.parsePartialOrVariable(tokens)
	if err != nil {
		return nil, err
	}
	return Cancel{Target: target}, nil
}

// ---------------------------------------------------------------------------
// For loop
// ---------------------------------------------------------------------------

func (p *parser) parseForLoop(line string) (Command, error) {
	// for <var> in <iterable> { <command> }
	rest := strings.TrimSpace(strings.TrimPrefix(line, "for"))

	// Extract variable name
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return nil, p.errorf("for loop: expected 'for <var> in <iterable> { <command> }'")
	}
	varName := parts[0]
	if parts[1] != "in" {
		return nil, p.errorf("for loop: expected 'in' keyword, got '%s'", parts[1])
	}

	// Everything after "in" up to the body brace
	afterIn := strings.TrimSpace(rest[strings.Index(rest, " in ")+4:])

	// Find the body: the last { ... } block
	bodyBraceStart := findBodyBraceStart(afterIn)
	if bodyBraceStart < 0 {
		return nil, p.errorf("for loop: could not find body block '{ ... }'")
	}

	iterableStr := strings.TrimSpace(afterIn[:bodyBraceStart])
	bodyStr := strings.TrimSpace(afterIn[bodyBraceStart:])

	// Strip outer braces from body
	if strings.HasPrefix(bodyStr, "{") && strings.HasSuffix(bodyStr, "}") {
		bodyStr = strings.TrimSpace(bodyStr[1 : len(bodyStr)-1])
	}

	iterable, err := p.parseIterable(iterableStr)
	if err != nil {
		return nil, err
	}

	body, err := p.parseLine(bodyStr)
	if err != nil {
		return nil, err
	}

	return ForLoop{Variable: varName, Iterable: iterable, Body: body}, nil
}

// findBodyBraceStart finds the index of the opening '{' that starts the body
// (as opposed to an inline-table or array brace inside the iterable).
func findBodyBraceStart(s string) int {
	// The body brace is the last top-level '{' that has a matching '}'.
	depth := 0
	lastOpen := -1
	inQuote := false
	for i, ch := range s {
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch ch {
		case '{':
			if depth == 0 {
				lastOpen = i
			}
			depth++
		case '}':
			depth--
		case '[':
			depth++
		case ']':
			depth--
		}
	}
	return lastOpen
}

func (p *parser) parseIterable(s string) (Iterable, error) {
	s = strings.TrimSpace(s)

	// Array literal: [ ... ]
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		vals, err := p.parseValueList(inner)
		if err != nil {
			return nil, err
		}
		return ArrayIterable{Values: vals}, nil
	}

	// Range: start..end or start..=end
	if strings.Contains(s, "..") {
		inclusive := strings.Contains(s, "..=")
		var parts []string
		if inclusive {
			parts = strings.SplitN(s, "..=", 2)
		} else {
			parts = strings.SplitN(s, "..", 2)
		}
		start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			return nil, p.errorf("range: invalid start: %s", parts[0])
		}
		end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, p.errorf("range: invalid end: %s", parts[1])
		}
		return RangeIterable{Start: start, End: end, Inclusive: inclusive}, nil
	}

	// List command (starts with "list")
	if strings.HasPrefix(s, "list ") {
		cmd, err := p.parseLine(s)
		if err != nil {
			return nil, err
		}
		if lc, ok := cmd.(*ListCmd); ok {
			return ListIterable{List: lc}, nil
		}
		if lc, ok := cmd.(ListCmd); ok {
			return ListIterable{List: &lc}, nil
		}
		return nil, p.errorf("expected list command in for-loop iterable")
	}

	// Bare plural resource type (possibly with filter): "signers" or "accounts | filter"
	// Split on pipe for filters
	filterParts := strings.SplitN(s, "|", 2)
	resourcePart := strings.TrimSpace(filterParts[0])
	var filterStr *string
	if len(filterParts) == 2 {
		f := strings.TrimSpace(filterParts[1])
		filterStr = &f
	}

	tokens := strings.Fields(resourcePart)
	// Check for "list_type for parent" pattern
	if len(tokens) >= 3 && tokens[len(tokens)-2] == "for" {
		parentTokens := tokens[:len(tokens)-2]
		parentID := tokens[len(tokens)-1]
		if rt, consumed, err := p.matchPluralResourceType(parentTokens); err == nil && consumed == len(parentTokens) {
			pov := IdOrVariable{Id: parentID}
			if strings.HasPrefix(parentID, "$") {
				pov = IdOrVariable{VarName: parentID[1:], IsVariable: true}
			}
			return ListIterable{List: &ListCmd{ResourceType: rt, Parent: &pov, Filter: filterStr}}, nil
		}
	}
	if rt, consumed, err := p.matchPluralResourceType(tokens); err == nil && consumed == len(tokens) {
		return ListIterable{List: &ListCmd{ResourceType: rt, Filter: filterStr}}, nil
	}

	return nil, p.errorf("unrecognised iterable: %s", s)
}

// ---------------------------------------------------------------------------
// Blueprint / Download / Upload
// ---------------------------------------------------------------------------

func (p *parser) parseBlueprint(tokens []string) (Command, error) {
	if len(tokens) < 2 {
		return nil, p.errorf("blueprint requires type and root-invite")
	}
	return Blueprint{Type: tokens[0], RootInvite: tokens[1]}, nil
}

func (p *parser) parseDownload(tokens []string) (Command, error) {
	host := false
	if len(tokens) > 0 && tokens[0] == "host" {
		host = true
		tokens = tokens[1:]
	}

	if len(tokens) < 2 || tokens[0] != "treasury" {
		return nil, p.errorf("expected 'download [host] treasury <id> [directory]'")
	}
	cmd := DownloadTreasury{TreasuryId: resolveIdToken(tokens[1], p), Host: host}
	if len(tokens) >= 3 {
		d := stripQuotes(tokens[2])
		cmd.Directory = &d
	}
	return cmd, nil
}

func (p *parser) parseUpload(tokens []string) (Command, error) {
	host := false
	if len(tokens) > 0 && tokens[0] == "host" {
		host = true
		tokens = tokens[1:]
	}
	if len(tokens) < 2 || tokens[0] != "backup" {
		return nil, p.errorf("expected 'upload [host] backup <file>'")
	}
	return UploadBackup{File: stripQuotes(tokens[1]), Host: host}, nil
}

// ---------------------------------------------------------------------------
// Convert / Replace
// ---------------------------------------------------------------------------

func (p *parser) parseConvert(line string, tokens []string) (Command, error) {
	// convert <data> from <source> to <target>
	fromIdx := -1
	toIdx := -1
	for i, t := range tokens {
		if t == "from" {
			fromIdx = i
		}
		if t == "to" && fromIdx >= 0 {
			toIdx = i
		}
	}
	if fromIdx < 0 || toIdx < 0 {
		return nil, p.errorf("convert: expected 'convert <data> from <source> to <target>'")
	}
	dataStr := strings.Join(tokens[:fromIdx], " ")
	sourceStr := strings.Join(tokens[fromIdx+1:toIdx], " ")
	targetStr := strings.Join(tokens[toIdx+1:], " ")

	data, err := p.parseValueFromString(dataStr)
	if err != nil {
		return nil, err
	}
	source, err := p.parseValueFromString(sourceStr)
	if err != nil {
		return nil, err
	}
	target, err := p.parseValueFromString(targetStr)
	if err != nil {
		return nil, err
	}
	return Convert{Data: data, SourceEncoding: source, TargetEncoding: target}, nil
}

func (p *parser) parseReplace(line string, tokens []string) (Command, error) {
	// replace <data> <old> <new>
	if len(tokens) < 3 {
		return nil, p.errorf("replace requires data, old, and new arguments")
	}
	data, err := p.parseValueFromString(tokens[0])
	if err != nil {
		return nil, err
	}
	old, err := p.parseValueFromString(tokens[1])
	if err != nil {
		return nil, err
	}
	newVal, err := p.parseValueFromString(tokens[2])
	if err != nil {
		return nil, err
	}
	return Replace{Data: data, Old: old, New: newVal}, nil
}

// ---------------------------------------------------------------------------
// Custom
// ---------------------------------------------------------------------------

func (p *parser) parseCustom(line string, tokens []string) (Command, error) {
	if len(tokens) < 2 {
		return nil, p.errorf("custom requires an action and target")
	}
	action := tokens[0]
	target, remaining, err := p.parsePartialOrVariable(tokens[1:])
	if err != nil {
		return nil, err
	}

	cmd := Custom{Action: action, Target: target}

	// Check for payload in remaining tokens
	if len(remaining) > 0 {
		// Check if there's an inline table
		braceIdx := strings.Index(line, "{")
		if braceIdx >= 0 {
			tbl, err := p.parseInlineTableFromString(line[braceIdx:])
			if err != nil {
				return nil, err
			}
			cmd.Payload = &CustomPayload{TableValue: tbl}
		} else {
			s := strings.Join(remaining, " ")
			cmd.Payload = &CustomPayload{IsString: true, StringValue: stripQuotes(s)}
		}
	}

	return cmd, nil
}

// parseCustomAction handles keyword-based custom actions like "abort $tf", "heartbeat user id(...)", etc.
func (p *parser) parseCustomAction(action string, tokens []string) (Command, error) {
	if len(tokens) < 1 {
		return nil, p.errorf("%s requires a target", action)
	}

	target, remaining, err := p.parsePartialOrVariable(tokens)
	if err != nil {
		return nil, err
	}

	cmd := Custom{Action: action, Target: target}

	if len(remaining) > 0 {
		// Check if remaining tokens form an inline table
		rest := strings.Join(remaining, " ")
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "{") {
			table, err := p.parseInlineTableFromString(rest)
			if err != nil {
				return nil, err
			}
			cmd.Payload = &CustomPayload{TableValue: table}
		} else {
			payload := stripQuotes(rest)
			cmd.Payload = &CustomPayload{IsString: true, StringValue: payload}
		}
	}

	return cmd, nil
}

// parseResourceShorthand handles bare resource references like "address XYZ for SOL",
// "asset SOL for SOL", "key NAME", etc. These resolve to Get commands.
func (p *parser) parseResourceShorthand(resourceType string, tokens []string, line string) (Command, error) {
	// If it contains = or :=, it's an assignment
	if strings.Contains(line, ":=") || strings.Contains(line, "=") {
		return p.parseAssignment(line)
	}

	// Map singular to our resource type enum (try two-word forms first)
	rt, ok := singularResourceTypes[resourceType]
	if !ok && len(tokens) > 0 {
		twoWord := resourceType + " " + tokens[0]
		if rt2, ok2 := singularResourceTypes[twoWord]; ok2 {
			rt = rt2
			ok = true
			tokens = tokens[1:] // consume the second word
		}
	}
	if !ok {
		return nil, p.errorf("unknown resource type: %s", resourceType)
	}

	if len(tokens) < 1 {
		// Special case: "treasury" alone means "get the current treasury"
		if rt == ResourceTreasury {
			return Get{Target: PartialOrVariable{ResourceType: rt, Id: ""}}, nil
		}
		return nil, p.errorf("%s requires an identifier", resourceType)
	}

	id := tokens[0]
	if strings.HasPrefix(id, "$") {
		// Variable reference - this is a get with variable
		return Get{Target: PartialOrVariable{IsVariable: true, VarName: strings.TrimPrefix(id, "$")}}, nil
	}

	var parent string
	var extension string

	for i := 1; i < len(tokens); i++ {
		if tokens[i] == "for" && i+1 < len(tokens) {
			parent = tokens[i+1]
			i++
		} else if tokens[i] == "with" && i+1 < len(tokens) {
			extension = tokens[i+1]
			i++
		}
	}

	return Get{Target: PartialOrVariable{
		ResourceType: rt,
		Id:           id,
		Parent:       parent,
		Extension:    extension,
	}}, nil
}

// ---------------------------------------------------------------------------
// Assignment
// ---------------------------------------------------------------------------

func (p *parser) parseAssignment(line string) (Command, error) {
	// Patterns:
	// $var = <rhs>
	// $var := <rhs>
	// $var, $err = <rhs>
	// $var, $err := <rhs>

	// Find the assignment operator
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return nil, p.errorf("expected assignment operator '=' or ':='")
	}

	direct := false
	lhs := line[:eqIdx]
	rhs := line[eqIdx+1:]

	if strings.HasSuffix(strings.TrimSpace(lhs), ":") {
		direct = true
		lhs = strings.TrimSuffix(strings.TrimSpace(lhs), ":")
		lhs = lhs + " " // keep it parseable
	}

	lhs = strings.TrimSpace(lhs)
	rhs = strings.TrimSpace(rhs)

	// Check for fallible assignment ($var, $err)
	if strings.Contains(lhs, ",") {
		parts := strings.SplitN(lhs, ",", 2)
		varName := strings.TrimSpace(parts[0])
		errName := strings.TrimSpace(parts[1])
		varName = strings.TrimPrefix(varName, "$")
		errName = strings.TrimPrefix(errName, "$")

		assignable, err := p.parseAssignable(rhs)
		if err != nil {
			return nil, err
		}
		return FallibleAssignment{
			Variable: varName,
			Error:    errName,
			Mutate:   Mutate{Value: assignable},
			Direct:   direct,
		}, nil
	}

	varName := strings.TrimPrefix(lhs, "$")
	assignable, err := p.parseAssignable(rhs)
	if err != nil {
		return nil, err
	}
	return Assignment{Variable: varName, Value: assignable, Direct: direct}, nil
}

func (p *parser) parseAssignable(rhs string) (Assignable, error) {
	rhs = strings.TrimSpace(rhs)

	// Check if rhs is a command (create, get, list, custom, convert, replace, etc.)
	tokens := tokeniseLine(rhs)
	if len(tokens) > 0 {
		switch tokens[0] {
		case "create", "propose", "get", "list", "update", "delete",
			"custom", "approve", "submit", "cancel", "convert", "replace",
			"download", "upload", "blueprint",
			// Custom action keywords (parsed as Custom commands):
			"abort", "retry", "recheck", "activate", "disable", "heartbeat",
			"fee-payer", "load", "price":
			cmd, err := p.parseLine(rhs)
			if err != nil {
				return nil, err
			}
			return CommandAssignable{Cmd: cmd}, nil
		}
		// Check if it's a bare resource type (e.g. "asset SOL for SOL" → get command)
		isRT := false
		if _, ok := singularResourceTypes[tokens[0]]; ok {
			isRT = true
		}
		// Also check two-word resource types (e.g. "client key root-p256")
		if !isRT && len(tokens) >= 2 {
			twoWord := tokens[0] + " " + tokens[1]
			if _, ok := singularResourceTypes[twoWord]; ok {
				isRT = true
			}
		}
		if isRT {
			cmd, err := p.parseGet(tokens, rhs)
			if err == nil {
				return CommandAssignable{Cmd: cmd}, nil
			}
		}
	}

	// Otherwise it's a value expression.
	val, err := p.parseValueFromString(rhs)
	if err != nil {
		return nil, err
	}
	return ValueAssignable{Value: val}, nil
}

// ---------------------------------------------------------------------------
// PartialOrVariable parsing
// ---------------------------------------------------------------------------

// parsePartialOrVariable consumes tokens to build a PartialOrVariable and returns
// the remaining unconsumed tokens.
func (p *parser) parsePartialOrVariable(tokens []string) (PartialOrVariable, []string, error) {
	if len(tokens) == 0 {
		return PartialOrVariable{}, nil, p.errorf("expected target")
	}

	// Variable reference
	if strings.HasPrefix(tokens[0], "$") {
		vr := parseVariableRefString(tokens[0])
		return PartialOrVariable{IsVariable: true, VarName: vr.Name}, tokens[1:], nil
	}

	// Resource type + optional id + optional for/with
	rt, consumed, err := p.matchResourceType(tokens)
	if err != nil {
		// Also try plural resource types (e.g. "assets" in "price assets")
		rt2, consumed2, err2 := p.matchPluralResourceType(tokens)
		if err2 != nil {
			// If not a known resource type, treat as a bare variable name (without $ prefix).
			// CSL allows referencing variables without $ in target position.
			return PartialOrVariable{IsVariable: true, VarName: tokens[0]}, tokens[1:], nil
		}
		rt = rt2
		consumed = consumed2
	}
	pov := PartialOrVariable{ResourceType: rt}
	idx := consumed

	for idx < len(tokens) {
		tok := tokens[idx]
		switch {
		case tok == "for":
			idx++
			if idx >= len(tokens) {
				return pov, tokens[idx:], nil
			}
			pov.Parent = resolveIdToken(tokens[idx], p)
			idx++
		case tok == "with":
			idx++
			if idx >= len(tokens) {
				return pov, tokens[idx:], nil
			}
			pov.Extension = stripQuotes(tokens[idx])
			idx++
		case strings.HasPrefix(tok, "{"):
			// Inline table starts – stop consuming for the target.
			return pov, tokens[idx:], nil
		case tok == "|":
			return pov, tokens[idx:], nil
		default:
			if pov.Id == "" {
				pov.Id = resolveIdToken(tok, p)
				idx++
			} else {
				return pov, tokens[idx:], nil
			}
		}
	}

	return pov, nil, nil
}

// ---------------------------------------------------------------------------
// Resource type matching
// ---------------------------------------------------------------------------

// matchResourceType tries to match a singular resource type from the token
// stream. It returns the ResourceType, the number of tokens consumed, or an error.
func (p *parser) matchResourceType(tokens []string) (ResourceType, int, error) {
	if len(tokens) == 0 {
		return 0, 0, p.errorf("expected resource type")
	}

	// Try two-word space-separated forms first (e.g. "access rule", "client key")
	if len(tokens) >= 2 {
		twoWord := tokens[0] + " " + tokens[1]
		if rt, ok := singularResourceTypes[twoWord]; ok {
			return rt, 2, nil
		}
	}

	// Single-word or hyphenated forms (e.g. "access-rule", "account")
	if rt, ok := singularResourceTypes[tokens[0]]; ok {
		return rt, 1, nil
	}

	return 0, 0, p.errorf("unrecognised resource type: %s", tokens[0])
}

// matchPluralResourceType tries to match a plural (or multi-word plural)
// resource type from the token stream.
func (p *parser) matchPluralResourceType(tokens []string) (ResourceType, int, error) {
	if len(tokens) == 0 {
		return 0, 0, p.errorf("expected plural resource type")
	}

	// Try two-word plural forms first (e.g. "access rules", "client keys")
	if len(tokens) >= 2 {
		twoWord := tokens[0] + " " + tokens[1]
		if rt, ok := pluralResourceTypes[twoWord]; ok {
			return rt, 2, nil
		}
	}

	// Single word plural
	if rt, ok := pluralResourceTypes[tokens[0]]; ok {
		return rt, 1, nil
	}

	return 0, 0, p.errorf("unrecognised plural resource type: %s", tokens[0])
}

// ---------------------------------------------------------------------------
// Value parsing
// ---------------------------------------------------------------------------

// parseValueFromString parses a CSL value expression from a string.
func (p *parser) parseValueFromString(s string) (Value, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return StringValue{Value: ""}, nil
	}

	// Triple-quoted string
	if strings.HasPrefix(s, `"""`) {
		end := strings.Index(s[3:], `"""`)
		if end < 0 {
			return nil, p.errorf("unterminated triple-quoted string")
		}
		return StringValue{Value: s[3 : 3+end]}, nil
	}

	// Quoted string
	if s[0] == '"' {
		val, err := parseQuotedString(s)
		if err != nil {
			return nil, p.errorf("%s", err.Error())
		}
		return StringValue{Value: val}, nil
	}

	// Boolean
	if s == "true" {
		return BooleanValue{Value: true}, nil
	}
	if s == "false" {
		return BooleanValue{Value: false}, nil
	}

	// Integer (before float to avoid 123 matching as float)
	if intVal, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntegerValue{Value: intVal}, nil
	}

	// Float
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return FloatValue{Value: floatVal}, nil
	}

	// Array literal
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == "" {
			return ArrayValue{Values: nil}, nil
		}
		vals, err := p.parseValueList(inner)
		if err != nil {
			return nil, err
		}
		return ArrayValue{Values: vals}, nil
	}

	// Inline table
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		tbl, err := p.parseInlineTableFromString(s)
		if err != nil {
			return nil, err
		}
		return *tbl, nil
	}

	// Variable reference: $var or $var.field.field2
	if strings.HasPrefix(s, "$") {
		return parseVariableRefString(s), nil
	}

	// Function call: name(args...)
	if parenIdx := strings.Index(s, "("); parenIdx > 0 && strings.HasSuffix(s, ")") {
		name := s[:parenIdx]
		if isValidIdentifier(name) || strings.Contains(name, "-") || strings.Contains(name, "_") {
			argsStr := s[parenIdx+1 : len(s)-1]
			return p.parseFunctionCall(name, argsStr)
		}
	}

	// Bare string (unquoted identifier-like value)
	return StringValue{Value: s}, nil
}

// parseQuotedString parses a double-quoted string, handling escapes.
func parseQuotedString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' {
		return "", fmt.Errorf("expected quoted string")
	}
	var result strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				result.WriteByte('"')
			case '\\':
				result.WriteByte('\\')
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			default:
				result.WriteByte('\\')
				result.WriteByte(s[i+1])
			}
			i += 2
			continue
		}
		if s[i] == '"' {
			return result.String(), nil
		}
		result.WriteByte(s[i])
		i++
	}
	return "", fmt.Errorf("unterminated quoted string")
}

// parseVariableRefString parses "$var.field1.field2" into a VariableRef.
// Supports quoted segments: $var.labels."dotted.key"
func parseVariableRefString(s string) VariableRef {
	s = strings.TrimPrefix(s, "$")
	parts := splitDottedKey(s)
	vr := VariableRef{Name: parts[0]}
	if len(parts) > 1 {
		vr.Path = parts[1:]
	}
	return vr
}

// parseFunctionCall parses a function call, evaluating parse-time functions
// immediately and storing delayed functions as FunctionCall AST nodes.
func (p *parser) parseFunctionCall(name string, argsStr string) (Value, error) {
	args, err := p.parseArgList(argsStr)
	if err != nil {
		return nil, err
	}

	// Parse-time functions: evaluate immediately.
	switch name {
	case "nonce":
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, p.errorf("nonce: %s", err)
		}
		return StringValue{Value: hex.EncodeToString(b)}, nil
	case "now":
		return StringValue{Value: time.Now().UTC().Format(time.RFC3339)}, nil
	case "hex":
		if len(args) != 1 {
			return nil, p.errorf("hex() requires exactly 1 argument")
		}
		if sv, ok := args[0].(StringValue); ok {
			return StringValue{Value: hex.EncodeToString([]byte(sv.Value))}, nil
		}
		// If not a literal string, fall through to delayed.
		return FunctionCall{Name: name, Args: args}, nil
	case "id":
		if len(args) != 1 {
			return nil, p.errorf("id() requires exactly 1 argument")
		}
		return args[0], nil
	case "concat":
		if len(args) != 2 {
			return nil, p.errorf("concat() requires exactly 2 arguments")
		}
		// If both are literal strings, concat immediately.
		a, aOk := args[0].(StringValue)
		b, bOk := args[1].(StringValue)
		if aOk && bOk {
			return StringValue{Value: a.Value + b.Value}, nil
		}
		return FunctionCall{Name: name, Args: args}, nil
	case "env", "env_value":
		if len(args) != 1 {
			return nil, p.errorf("%s() requires exactly 1 argument", name)
		}
		if sv, ok := args[0].(StringValue); ok {
			return StringValue{Value: os.Getenv(sv.Value)}, nil
		}
		return FunctionCall{Name: name, Args: args}, nil
	case "invite-public":
		if len(args) != 1 {
			return nil, p.errorf("invite-public() requires exactly 1 argument")
		}
		if sv, ok := args[0].(StringValue); ok {
			pubHex, err := invitePublicKey(sv.Value)
			if err != nil {
				return nil, p.errorf("invite-public: %s", err)
			}
			return StringValue{Value: pubHex}, nil
		}
		return FunctionCall{Name: name, Args: args}, nil
	}

	// All other functions are stored as-is (delayed).
	return FunctionCall{Name: name, Args: args}, nil
}

// parseArgList splits a comma-separated argument list and parses each as a Value.
func (p *parser) parseArgList(s string) ([]Value, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := splitCommaRespectingNesting(s)
	var vals []Value
	for _, part := range parts {
		v, err := p.parseValueFromString(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// parseValueList parses a comma-separated list of values (used for arrays).
func (p *parser) parseValueList(s string) ([]Value, error) {
	return p.parseArgList(s)
}

// InviteSeedFromCode derives a 32-byte ed25519 seed from an invite code.
// The invite code (hex string) is left-padded with zeros to 64 hex characters,
// then decoded to 32 bytes. This matches the Rust Treasury CLI behavior.
func InviteSeedFromCode(code string) ([]byte, error) {
	// Left-pad with zeros to 64 hex characters.
	padded := fmt.Sprintf("%064s", code)
	padded = strings.ReplaceAll(padded, " ", "0")
	seed, err := hex.DecodeString(padded)
	if err != nil {
		// If the code is not valid hex, hex-encode it first, then pad.
		padded = fmt.Sprintf("%064s", hex.EncodeToString([]byte(code)))
		padded = strings.ReplaceAll(padded, " ", "0")
		seed, err = hex.DecodeString(padded)
		if err != nil {
			return nil, fmt.Errorf("invalid invite code: %w", err)
		}
	}
	if len(seed) != 32 {
		return nil, fmt.Errorf("invite code too long: decoded to %d bytes, need 32", len(seed))
	}
	return seed, nil
}

// invitePublicKey computes the hex-encoded ed25519 public key from an invite code.
func invitePublicKey(code string) (string, error) {
	seed, err := InviteSeedFromCode(code)
	if err != nil {
		return "", err
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return hex.EncodeToString(pubKey), nil
}

// ---------------------------------------------------------------------------
// Inline table parsing
// ---------------------------------------------------------------------------

// parseInlineTableFromString parses a TOML-style inline table: { k = v, ... }.
func (p *parser) parseInlineTableFromString(s string) (*InlineTable, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, p.errorf("expected inline table: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return &InlineTable{}, nil
	}

	entries, err := p.parseTableEntries(inner)
	if err != nil {
		return nil, err
	}
	return &InlineTable{Entries: entries}, nil
}

func (p *parser) parseTableEntries(s string) ([]TableEntry, error) {
	parts := splitCommaRespectingNesting(s)
	var entries []TableEntry
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		entry, err := p.parseTableEntry(part)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (p *parser) parseTableEntry(s string) (TableEntry, error) {
	// key = value
	eqIdx := strings.Index(s, "=")
	if eqIdx < 0 {
		return TableEntry{}, p.errorf("expected '=' in table entry: %s", s)
	}
	key := strings.TrimSpace(s[:eqIdx])
	valStr := strings.TrimSpace(s[eqIdx+1:])

	// Key might be quoted
	key = stripQuotes(key)

	val, err := p.parseValueFromString(valStr)
	if err != nil {
		return TableEntry{}, err
	}
	return TableEntry{Key: key, Value: val}, nil
}

// tableToMap converts an InlineTable to a map[string]interface{} for the
// Create/Update data fields.
func tableToMap(tbl *InlineTable) map[string]interface{} {
	if tbl == nil {
		return nil
	}
	m := make(map[string]interface{})
	for _, e := range tbl.Entries {
		m[e.Key] = e.Value
	}
	return m
}

// ---------------------------------------------------------------------------
// Tokeniser
// ---------------------------------------------------------------------------

// tokeniseLine splits a line into tokens, respecting quoted strings and brace nesting.
func tokeniseLine(line string) []string {
	var tokens []string
	var current strings.Builder
	i := 0
	n := len(line)

	flush := func() {
		s := current.String()
		if s != "" {
			tokens = append(tokens, s)
			current.Reset()
		}
	}

	for i < n {
		ch := line[i]

		// Triple-quoted string
		if ch == '"' && i+2 < n && line[i+1] == '"' && line[i+2] == '"' {
			flush()
			end := strings.Index(line[i+3:], `"""`)
			if end < 0 {
				current.WriteString(line[i:])
				i = n
			} else {
				endPos := i + 3 + end + 3
				tokens = append(tokens, line[i:endPos])
				i = endPos
			}
			continue
		}

		// Quoted string
		if ch == '"' {
			flush()
			j := i + 1
			for j < n {
				if line[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if line[j] == '"' {
					j++
					break
				}
				j++
			}
			tokens = append(tokens, line[i:j])
			i = j
			continue
		}

		// Brace-delimited block { ... }
		if ch == '{' {
			flush()
			depth := 0
			j := i
			inQuote := false
			for j < n {
				if line[j] == '"' && !inQuote {
					inQuote = true
					j++
					continue
				}
				if inQuote {
					if line[j] == '\\' && j+1 < n {
						j += 2
						continue
					}
					if line[j] == '"' {
						inQuote = false
					}
					j++
					continue
				}
				if line[j] == '{' {
					depth++
				} else if line[j] == '}' {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				j++
			}
			tokens = append(tokens, line[i:j])
			i = j
			continue
		}

		// Array literal [ ... ]
		if ch == '[' {
			flush()
			depth := 0
			j := i
			inQuote := false
			for j < n {
				if line[j] == '"' && !inQuote {
					inQuote = true
					j++
					continue
				}
				if inQuote {
					if line[j] == '\\' && j+1 < n {
						j += 2
						continue
					}
					if line[j] == '"' {
						inQuote = false
					}
					j++
					continue
				}
				if line[j] == '[' {
					depth++
				} else if line[j] == ']' {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				j++
			}
			tokens = append(tokens, line[i:j])
			i = j
			continue
		}

		// Function call with parens: word(...)
		if ch == '(' && current.Len() > 0 {
			// Include the parens as part of the current token
			depth := 0
			j := i
			inQuote := false
			for j < n {
				if line[j] == '"' && !inQuote {
					inQuote = true
					j++
					continue
				}
				if inQuote {
					if line[j] == '\\' && j+1 < n {
						j += 2
						continue
					}
					if line[j] == '"' {
						inQuote = false
					}
					j++
					continue
				}
				if line[j] == '(' {
					depth++
				} else if line[j] == ')' {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				j++
			}
			current.WriteString(line[i:j])
			i = j
			continue
		}

		// Whitespace separator
		if ch == ' ' || ch == '\t' {
			flush()
			i++
			continue
		}

		current.WriteByte(ch)
		i++
	}
	flush()
	return tokens
}

// splitCommaRespectingNesting splits on commas that are not nested inside
// brackets, braces, or quotes.
func splitCommaRespectingNesting(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && !inQuote {
			inQuote = true
			current.WriteByte(ch)
			continue
		}
		if inQuote {
			if ch == '\\' && i+1 < len(s) {
				current.WriteByte(ch)
				i++
				current.WriteByte(s[i])
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			current.WriteByte(ch)
			continue
		}
		switch ch {
		case '{', '[', '(':
			depth++
			current.WriteByte(ch)
		case '}', ']', ')':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// isValidIdentifier returns true if s is a valid identifier (letters, digits,
// underscores, hyphens).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, ch := range s {
		if i == 0 && unicode.IsDigit(ch) {
			return false
		}
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' {
			return false
		}
	}
	return true
}
