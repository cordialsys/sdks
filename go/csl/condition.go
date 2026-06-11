package csl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// condToken represents a token in a condition expression.
type condToken struct {
	kind  condTokenKind
	value string
}

type condTokenKind int

const (
	condTokenString condTokenKind = iota
	condTokenInteger
	condTokenFloat
	condTokenBool
	condTokenVariable   // $var.path
	condTokenOperator   // ==, !=, <, <=, >, >=, =
	condTokenAnd        // AND, &&
	condTokenOr         // OR, ||
	condTokenNot        // NOT, ~, -
	condTokenLParen     // (
	condTokenRParen     // )
	condTokenFunction   // elapsed(...)
	condTokenIdentifier // bare word
)

// condLexer tokenizes a condition string.
type condLexer struct {
	input string
	pos   int
}

func (l *condLexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *condLexer) advance() {
	l.pos++
}

func (l *condLexer) skipWhitespace() {
	for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t') {
		l.pos++
	}
}

func (l *condLexer) tokenize() ([]condToken, error) {
	var tokens []condToken
	for {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}
		ch := l.peek()

		switch {
		case ch == '(':
			tokens = append(tokens, condToken{kind: condTokenLParen, value: "("})
			l.advance()
		case ch == ')':
			tokens = append(tokens, condToken{kind: condTokenRParen, value: ")"})
			l.advance()
		case ch == '~':
			tokens = append(tokens, condToken{kind: condTokenNot, value: "~"})
			l.advance()
		case ch == '!' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '=':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: "!="})
			l.pos += 2
		case ch == '=' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '=':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: "=="})
			l.pos += 2
		case ch == '=':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: "=="})
			l.advance()
		case ch == '<' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '=':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: "<="})
			l.pos += 2
		case ch == '<':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: "<"})
			l.advance()
		case ch == '>' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '=':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: ">="})
			l.pos += 2
		case ch == '>':
			tokens = append(tokens, condToken{kind: condTokenOperator, value: ">"})
			l.advance()
		case ch == '&' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '&':
			tokens = append(tokens, condToken{kind: condTokenAnd, value: "&&"})
			l.pos += 2
		case ch == '|' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '|':
			tokens = append(tokens, condToken{kind: condTokenOr, value: "||"})
			l.pos += 2
		case ch == '"':
			s, err := l.readQuotedString()
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, condToken{kind: condTokenString, value: s})
		case ch == '$':
			v := l.readVariable()
			tokens = append(tokens, condToken{kind: condTokenVariable, value: v})
		case unicode.IsDigit(rune(ch)):
			num := l.readNumber()
			if strings.Contains(num, ".") {
				tokens = append(tokens, condToken{kind: condTokenFloat, value: num})
			} else {
				tokens = append(tokens, condToken{kind: condTokenInteger, value: num})
			}
		case ch == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(rune(l.input[l.pos+1])):
			// Could be a negative number or a NOT operator.
			// If the previous token is an operand, treat as NOT; otherwise as negative number.
			if len(tokens) > 0 && isOperandToken(tokens[len(tokens)-1]) {
				tokens = append(tokens, condToken{kind: condTokenNot, value: "-"})
				l.advance()
			} else {
				num := l.readNumber()
				if strings.Contains(num, ".") {
					tokens = append(tokens, condToken{kind: condTokenFloat, value: num})
				} else {
					tokens = append(tokens, condToken{kind: condTokenInteger, value: num})
				}
			}
		case ch == '-':
			tokens = append(tokens, condToken{kind: condTokenNot, value: "-"})
			l.advance()
		case unicode.IsLetter(rune(ch)) || ch == '_':
			word := l.readWord()
			switch strings.ToUpper(word) {
			case "AND":
				tokens = append(tokens, condToken{kind: condTokenAnd, value: "AND"})
			case "OR":
				tokens = append(tokens, condToken{kind: condTokenOr, value: "OR"})
			case "NOT":
				tokens = append(tokens, condToken{kind: condTokenNot, value: "NOT"})
			case "TRUE":
				tokens = append(tokens, condToken{kind: condTokenBool, value: "true"})
			case "FALSE":
				tokens = append(tokens, condToken{kind: condTokenBool, value: "false"})
			default:
				// Check if followed by '(' -> function call
				l.skipWhitespace()
				if l.pos < len(l.input) && l.input[l.pos] == '(' {
					// Read the function call including parentheses
					fnCall := word + l.readParenBlock()
					tokens = append(tokens, condToken{kind: condTokenFunction, value: fnCall})
				} else {
					tokens = append(tokens, condToken{kind: condTokenIdentifier, value: word})
				}
			}
		default:
			return nil, fmt.Errorf("unexpected character in condition: %c", ch)
		}
	}
	return tokens, nil
}

func isOperandToken(t condToken) bool {
	switch t.kind {
	case condTokenString, condTokenInteger, condTokenFloat, condTokenBool,
		condTokenVariable, condTokenFunction, condTokenIdentifier, condTokenRParen:
		return true
	}
	return false
}

func (l *condLexer) readQuotedString() (string, error) {
	l.advance() // skip opening "
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			next := l.input[l.pos+1]
			switch next {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(next)
			}
			l.pos += 2
			continue
		}
		if ch == '"' {
			l.advance()
			return sb.String(), nil
		}
		sb.WriteByte(ch)
		l.advance()
	}
	return "", fmt.Errorf("unterminated string in condition")
}

func (l *condLexer) readVariable() string {
	start := l.pos
	l.advance() // skip $
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' || ch == '-' || ch == '.' {
			l.advance()
		} else {
			break
		}
	}
	return l.input[start:l.pos]
}

func (l *condLexer) readNumber() string {
	start := l.pos
	if l.pos < len(l.input) && l.input[l.pos] == '-' {
		l.advance()
	}
	for l.pos < len(l.input) && (unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '.') {
		l.advance()
	}
	return l.input[start:l.pos]
}

func (l *condLexer) readWord() string {
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' || ch == '-' {
			l.advance()
		} else {
			break
		}
	}
	return l.input[start:l.pos]
}

func (l *condLexer) readParenBlock() string {
	if l.pos >= len(l.input) || l.input[l.pos] != '(' {
		return ""
	}
	start := l.pos
	depth := 0
	inQuote := false
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '"' && !inQuote {
			inQuote = true
			l.advance()
			continue
		}
		if inQuote {
			if ch == '\\' && l.pos+1 < len(l.input) {
				l.pos += 2
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			l.advance()
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				l.advance()
				return l.input[start:l.pos]
			}
		}
		l.advance()
	}
	return l.input[start:l.pos]
}

// condParser is a recursive descent parser for condition expressions.
// Grammar:
//
//	expr     -> orExpr
//	orExpr   -> andExpr (OR andExpr)*
//	andExpr  -> notExpr (AND notExpr)*
//	notExpr  -> NOT notExpr | cmpExpr
//	cmpExpr  -> primary (op primary)?
//	primary  -> '(' expr ')' | atom
//	atom     -> string | integer | float | bool | variable | function | identifier
type condParser struct {
	tokens []condToken
	pos    int
	// resolveValue is called to resolve variable references and function calls.
	resolveValue func(s string) (string, error)
}

func (p *condParser) peek() *condToken {
	if p.pos >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.pos]
}

func (p *condParser) advance() condToken {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

// condValue represents a resolved value in condition evaluation.
type condValue struct {
	str     string
	integer *int64
	float64 *float64
	boolean *bool
}

func condStr(s string) condValue {
	return condValue{str: s}
}

func condInt(i int64) condValue {
	return condValue{str: strconv.FormatInt(i, 10), integer: &i}
}

func condFloat(f float64) condValue {
	return condValue{str: strconv.FormatFloat(f, 'f', -1, 64), float64: &f}
}

func condBool(b bool) condValue {
	bv := b
	if b {
		return condValue{str: "true", boolean: &bv}
	}
	return condValue{str: "false", boolean: &bv}
}

func (v condValue) asBool() bool {
	if v.boolean != nil {
		return *v.boolean
	}
	// Truthy: non-empty, non-"false", non-"0"
	return v.str != "" && v.str != "false" && v.str != "0"
}

func (v condValue) asFloat() (float64, bool) {
	if v.float64 != nil {
		return *v.float64, true
	}
	if v.integer != nil {
		return float64(*v.integer), true
	}
	f, err := strconv.ParseFloat(v.str, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func (v condValue) asInt() (int64, bool) {
	if v.integer != nil {
		return *v.integer, true
	}
	i, err := strconv.ParseInt(v.str, 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}

// EvaluateCondition evaluates a condition string and returns true/false.
// resolveValue is called to resolve $variable references and function calls like elapsed($var).
func EvaluateCondition(condition string, resolveValue func(string) (string, error)) (bool, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return true, nil
	}

	lexer := &condLexer{input: condition}
	tokens, err := lexer.tokenize()
	if err != nil {
		return false, fmt.Errorf("condition lexer: %w", err)
	}
	if len(tokens) == 0 {
		return true, nil
	}

	parser := &condParser{
		tokens:       tokens,
		resolveValue: resolveValue,
	}

	result, err := parser.parseExpr()
	if err != nil {
		return false, err
	}

	return result.asBool(), nil
}

func (p *condParser) parseExpr() (condValue, error) {
	return p.parseOr()
}

func (p *condParser) parseOr() (condValue, error) {
	left, err := p.parseAnd()
	if err != nil {
		return condValue{}, err
	}
	for {
		t := p.peek()
		if t == nil || t.kind != condTokenOr {
			break
		}
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return condValue{}, err
		}
		result := left.asBool() || right.asBool()
		left = condBool(result)
	}
	return left, nil
}

func (p *condParser) parseAnd() (condValue, error) {
	left, err := p.parseNot()
	if err != nil {
		return condValue{}, err
	}
	for {
		t := p.peek()
		if t == nil || t.kind != condTokenAnd {
			break
		}
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return condValue{}, err
		}
		result := left.asBool() && right.asBool()
		left = condBool(result)
	}
	return left, nil
}

func (p *condParser) parseNot() (condValue, error) {
	t := p.peek()
	if t != nil && t.kind == condTokenNot {
		p.advance()
		val, err := p.parseNot()
		if err != nil {
			return condValue{}, err
		}
		return condBool(!val.asBool()), nil
	}
	return p.parseCmp()
}

func (p *condParser) parseCmp() (condValue, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return condValue{}, err
	}
	t := p.peek()
	if t == nil || t.kind != condTokenOperator {
		return left, nil
	}
	op := p.advance()
	right, err := p.parsePrimary()
	if err != nil {
		return condValue{}, err
	}
	result, err := compareValues(left, op.value, right)
	if err != nil {
		return condValue{}, err
	}
	return condBool(result), nil
}

func (p *condParser) parsePrimary() (condValue, error) {
	t := p.peek()
	if t == nil {
		return condValue{}, fmt.Errorf("unexpected end of condition")
	}
	if t.kind == condTokenLParen {
		p.advance()
		val, err := p.parseExpr()
		if err != nil {
			return condValue{}, err
		}
		rp := p.peek()
		if rp == nil || rp.kind != condTokenRParen {
			return condValue{}, fmt.Errorf("expected ')' in condition")
		}
		p.advance()
		return val, nil
	}
	return p.parseAtom()
}

func (p *condParser) parseAtom() (condValue, error) {
	t := p.peek()
	if t == nil {
		return condValue{}, fmt.Errorf("unexpected end of condition")
	}
	switch t.kind {
	case condTokenString:
		p.advance()
		return condStr(t.value), nil
	case condTokenInteger:
		p.advance()
		i, err := strconv.ParseInt(t.value, 10, 64)
		if err != nil {
			return condValue{}, fmt.Errorf("invalid integer in condition: %s", t.value)
		}
		return condInt(i), nil
	case condTokenFloat:
		p.advance()
		f, err := strconv.ParseFloat(t.value, 64)
		if err != nil {
			return condValue{}, fmt.Errorf("invalid float in condition: %s", t.value)
		}
		return condFloat(f), nil
	case condTokenBool:
		p.advance()
		return condBool(t.value == "true"), nil
	case condTokenVariable:
		p.advance()
		resolved, err := p.resolveValue(t.value)
		if err != nil {
			return condValue{}, err
		}
		// Try to parse as number
		if i, err := strconv.ParseInt(resolved, 10, 64); err == nil {
			return condInt(i), nil
		}
		if f, err := strconv.ParseFloat(resolved, 64); err == nil {
			return condFloat(f), nil
		}
		if resolved == "true" {
			return condBool(true), nil
		}
		if resolved == "false" {
			return condBool(false), nil
		}
		return condStr(resolved), nil
	case condTokenFunction:
		p.advance()
		resolved, err := p.resolveValue(t.value)
		if err != nil {
			return condValue{}, err
		}
		if i, err := strconv.ParseInt(resolved, 10, 64); err == nil {
			return condInt(i), nil
		}
		if f, err := strconv.ParseFloat(resolved, 64); err == nil {
			return condFloat(f), nil
		}
		return condStr(resolved), nil
	case condTokenIdentifier:
		// Bare identifiers treated as string values
		p.advance()
		s := t.value
		if s == "true" {
			return condBool(true), nil
		}
		if s == "false" {
			return condBool(false), nil
		}
		return condStr(s), nil
	default:
		return condValue{}, fmt.Errorf("unexpected token in condition: %s", t.value)
	}
}

func compareValues(left condValue, op string, right condValue) (bool, error) {
	// Try numeric comparison first
	lf, lok := left.asFloat()
	rf, rok := right.asFloat()
	if lok && rok {
		switch op {
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		case "<":
			return lf < rf, nil
		case "<=":
			return lf <= rf, nil
		case ">":
			return lf > rf, nil
		case ">=":
			return lf >= rf, nil
		}
	}

	// Fall back to string comparison
	ls := left.str
	rs := right.str
	switch op {
	case "==":
		return ls == rs, nil
	case "!=":
		return ls != rs, nil
	case "<":
		return ls < rs, nil
	case "<=":
		return ls <= rs, nil
	case ">":
		return ls > rs, nil
	case ">=":
		return ls >= rs, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %s", op)
	}
}
