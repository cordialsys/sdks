/**
 * CSL Condition Evaluator.
 *
 * Evaluates boolean conditions with AND/OR/NOT and comparisons.
 * Grammar:
 *   condition = factor (("AND"|"&&") factor)*
 *   factor    = term (("OR"|"||") term)*
 *   term      = "NOT" term | "!" term | simple
 *   simple    = "(" condition ")" | restriction
 *   restriction = comparable [comparator comparable]
 *   comparable = member | function_call
 *   member     = value ["." field]*
 *   comparator = "==" | "=" | "!=" | "<" | "<=" | ">" | ">="
 */

export type EvalValue =
  | { type: "string"; value: string }
  | { type: "integer"; value: number }
  | { type: "float"; value: number }
  | { type: "boolean"; value: boolean }
  | { type: "array"; value: EvalValue[] }
  | { type: "null" };

export type ConditionNode =
  | { type: "and"; left: ConditionNode; right: ConditionNode }
  | { type: "or"; left: ConditionNode; right: ConditionNode }
  | { type: "not"; operand: ConditionNode }
  | {
      type: "comparison";
      left: ComparableNode;
      op: Comparator;
      right: ComparableNode;
    }
  | { type: "truthy"; value: ComparableNode };

export type ComparableNode =
  | { type: "member"; variable: string; fields: string[] }
  | { type: "literal"; value: EvalValue }
  | { type: "function"; name: string; args: ComparableNode[] };

export type Comparator = "==" | "!=" | "<" | "<=" | ">" | ">=";

class ConditionParser {
  private tokens: string[];
  private pos: number;

  constructor(input: string) {
    this.tokens = tokenize(input);
    this.pos = 0;
  }

  peek(): string | undefined {
    return this.tokens[this.pos];
  }

  advance(): string {
    return this.tokens[this.pos++]!;
  }

  match(token: string): boolean {
    if (this.peek() === token) {
      this.advance();
      return true;
    }
    return false;
  }

  parseCondition(): ConditionNode {
    let left = this.parseFactor();
    while (this.peek() === "AND" || this.peek() === "&&") {
      this.advance();
      const right = this.parseFactor();
      left = { type: "and", left, right };
    }
    return left;
  }

  parseFactor(): ConditionNode {
    let left = this.parseTerm();
    while (this.peek() === "OR" || this.peek() === "||") {
      this.advance();
      const right = this.parseTerm();
      left = { type: "or", left, right };
    }
    return left;
  }

  parseTerm(): ConditionNode {
    if (this.peek() === "NOT" || this.peek() === "!") {
      this.advance();
      const operand = this.parseTerm();
      return { type: "not", operand };
    }
    return this.parseSimple();
  }

  parseSimple(): ConditionNode {
    if (this.match("(")) {
      const cond = this.parseCondition();
      this.match(")");
      return cond;
    }
    return this.parseRestriction();
  }

  parseRestriction(): ConditionNode {
    const left = this.parseComparable();
    const op = this.tryParseComparator();
    if (op) {
      const right = this.parseComparable();
      return { type: "comparison", left, op, right };
    }
    return { type: "truthy", value: left };
  }

  tryParseComparator(): Comparator | null {
    const t = this.peek();
    if (t === "==" || t === "=" || t === "!=" || t === "<" || t === "<=" || t === ">" || t === ">=") {
      this.advance();
      if (t === "=") return "==";
      return t as Comparator;
    }
    return null;
  }

  parseComparable(): ComparableNode {
    const t = this.peek();

    // Function call: name(args)
    if (t && /^[a-zA-Z_]/.test(t) && this.tokens[this.pos + 1] === "(") {
      const name = this.advance();
      this.advance(); // (
      const args: ComparableNode[] = [];
      while (this.peek() !== ")" && this.peek() !== undefined) {
        if (args.length > 0) this.match(",");
        args.push(this.parseComparable());
      }
      this.match(")");
      return { type: "function", name, args };
    }

    // Variable: $var or $var.field
    if (t && t.startsWith("$")) {
      this.advance();
      const parts = t.slice(1).split(".");
      return { type: "member", variable: parts[0]!, fields: parts.slice(1) };
    }

    // Boolean literal
    if (t === "true" || t === "false") {
      this.advance();
      return { type: "literal", value: { type: "boolean", value: t === "true" } };
    }

    // Integer literal
    if (t && /^-?\d+$/.test(t)) {
      this.advance();
      return { type: "literal", value: { type: "integer", value: parseInt(t, 10) } };
    }

    // Float literal
    if (t && /^-?\d+\.\d+$/.test(t)) {
      this.advance();
      return { type: "literal", value: { type: "float", value: parseFloat(t) } };
    }

    // Quoted string
    if (t && t.startsWith('"')) {
      this.advance();
      return {
        type: "literal",
        value: { type: "string", value: t.slice(1, -1) },
      };
    }

    // Bare identifier (treated as string)
    if (t && /^[a-zA-Z_-]/.test(t)) {
      this.advance();
      const parts = t.split(".");
      if (parts.length > 1) {
        return { type: "member", variable: parts[0]!, fields: parts.slice(1) };
      }
      return { type: "literal", value: { type: "string", value: t } };
    }

    // Array literal
    if (t === "[") {
      this.advance();
      const elems: EvalValue[] = [];
      while (this.peek() !== "]" && this.peek() !== undefined) {
        if (elems.length > 0) this.match(",");
        const comp = this.parseComparable();
        if (comp.type === "literal") {
          elems.push(comp.value);
        }
      }
      this.match("]");
      return { type: "literal", value: { type: "array", value: elems } };
    }

    throw new Error(`Unexpected token in condition: ${t}`);
  }
}

function tokenize(input: string): string[] {
  const tokens: string[] = [];
  let i = 0;

  while (i < input.length) {
    // Skip whitespace
    if (input[i] === " " || input[i] === "\t") {
      i++;
      continue;
    }

    // Two-char operators
    const two = input.slice(i, i + 2);
    if (two === "==" || two === "!=" || two === "<=" || two === ">=" || two === "&&" || two === "||") {
      tokens.push(two);
      i += 2;
      continue;
    }

    // Single-char operators
    if ("=<>()!,[]".includes(input[i]!)) {
      tokens.push(input[i]!);
      i++;
      continue;
    }

    // Quoted string
    if (input[i] === '"') {
      let s = '"';
      i++;
      while (i < input.length && input[i] !== '"') {
        if (input[i] === "\\") {
          i++; // skip backslash
          // Add the escaped character directly (unescape)
          if (i < input.length) {
            s += input[i++];
          }
        } else {
          s += input[i++];
        }
      }
      s += '"';
      i++; // skip closing quote
      tokens.push(s);
      continue;
    }

    // Variable with dotted path: $var.field1.field2
    if (input[i] === "$") {
      let tok = "$";
      i++;
      while (i < input.length && /[a-zA-Z0-9_.\-]/.test(input[i]!)) {
        tok += input[i++];
      }
      tokens.push(tok);
      continue;
    }

    // Words and numbers
    if (/[a-zA-Z0-9_\-.]/.test(input[i]!)) {
      let tok = "";
      while (i < input.length && /[a-zA-Z0-9_.\-]/.test(input[i]!)) {
        tok += input[i++];
      }
      tokens.push(tok);
      continue;
    }

    i++;
  }

  return tokens;
}

/**
 * Parse a condition string into an AST.
 */
export function parseCondition(input: string): ConditionNode {
  const parser = new ConditionParser(input);
  return parser.parseCondition();
}

/**
 * Extract variable names referenced in a condition.
 */
export function extractVariables(node: ConditionNode): string[] {
  const vars = new Set<string>();

  function walk(n: ConditionNode): void {
    switch (n.type) {
      case "and":
      case "or":
        walk(n.left);
        walk(n.right);
        break;
      case "not":
        walk(n.operand);
        break;
      case "comparison":
        walkComparable(n.left);
        walkComparable(n.right);
        break;
      case "truthy":
        walkComparable(n.value);
        break;
    }
  }

  function walkComparable(n: ComparableNode): void {
    if (n.type === "member") vars.add(n.variable);
    if (n.type === "function") n.args.forEach(walkComparable);
  }

  walk(node);
  return Array.from(vars);
}

/**
 * Evaluate a condition against resolved values.
 */
export function evaluateCondition(
  node: ConditionNode,
  values: Map<string, EvalValue>,
  functions?: Map<string, (args: EvalValue[]) => EvalValue>,
): boolean {
  function evalNode(n: ConditionNode): boolean {
    switch (n.type) {
      case "and":
        return evalNode(n.left) && evalNode(n.right);
      case "or":
        return evalNode(n.left) || evalNode(n.right);
      case "not":
        return !evalNode(n.operand);
      case "comparison":
        return compare(resolveComparable(n.left), n.op, resolveComparable(n.right));
      case "truthy": {
        const val = resolveComparable(n.value);
        return isTruthy(val);
      }
    }
  }

  function resolveComparable(n: ComparableNode): EvalValue {
    switch (n.type) {
      case "literal":
        return n.value;
      case "member": {
        let val = values.get(n.variable);
        if (!val) return { type: "null" };
        // Navigate fields
        for (const field of n.fields) {
          if (val && val.type === "string") {
            // Try JSON parse
            try {
              const obj = JSON.parse(val.value);
              val = toEvalValue(obj[field]);
            } catch {
              return { type: "null" };
            }
          } else {
            return { type: "null" };
          }
        }
        return val || { type: "null" };
      }
      case "function": {
        const fn = functions?.get(n.name);
        if (fn) {
          const args = n.args.map(resolveComparable);
          return fn(args);
        }
        // Built-in: elapsed
        if (n.name === "elapsed" && n.args.length === 1) {
          const arg = resolveComparable(n.args[0]!);
          if (arg.type === "string") {
            const then = new Date(arg.value).getTime();
            return { type: "integer", value: Math.floor((Date.now() - then) / 1000) };
          }
        }
        return { type: "null" };
      }
    }
  }

  return evalNode(node);
}

function toEvalValue(v: unknown): EvalValue {
  if (v === null || v === undefined) return { type: "null" };
  if (typeof v === "string") return { type: "string", value: v };
  if (typeof v === "number") return Number.isInteger(v) ? { type: "integer", value: v } : { type: "float", value: v };
  if (typeof v === "boolean") return { type: "boolean", value: v };
  if (Array.isArray(v)) return { type: "array", value: v.map(toEvalValue) };
  return { type: "string", value: JSON.stringify(v) };
}

function isTruthy(v: EvalValue): boolean {
  switch (v.type) {
    case "null":
      return false;
    case "boolean":
      return v.value;
    case "string":
      return v.value !== "" && v.value !== "false";
    case "integer":
    case "float":
      return v.value !== 0;
    case "array":
      return v.value.length > 0;
  }
}

function compare(left: EvalValue, op: Comparator, right: EvalValue): boolean {
  // Handle null
  if (left.type === "null" || right.type === "null") {
    if (op === "==") return left.type === "null" && right.type === "null";
    if (op === "!=") return left.type !== "null" || right.type !== "null";
    return false;
  }

  // Boolean comparison
  if (left.type === "boolean" && right.type === "boolean") {
    if (op === "==") return left.value === right.value;
    if (op === "!=") return left.value !== right.value;
    throw new Error("Booleans can only be compared with == and !=");
  }

  // Numeric comparison
  if (
    (left.type === "integer" || left.type === "float") &&
    (right.type === "integer" || right.type === "float")
  ) {
    const l = left.value;
    const r = right.value;
    switch (op) {
      case "==": return l === r;
      case "!=": return l !== r;
      case "<": return l < r;
      case "<=": return l <= r;
      case ">": return l > r;
      case ">=": return l >= r;
    }
  }

  // String comparison
  if (left.type === "string" && right.type === "string") {
    const l = left.value;
    const r = right.value;
    switch (op) {
      case "==": return l === r;
      case "!=": return l !== r;
      case "<": return l < r;
      case "<=": return l <= r;
      case ">": return l > r;
      case ">=": return l >= r;
    }
  }

  // Cross-type: try coercing strings to integers
  if (left.type === "string" && (right.type === "integer" || right.type === "float")) {
    const l = parseFloat(left.value);
    if (!isNaN(l)) return compare({ type: "float", value: l }, op, right);
  }
  if ((left.type === "integer" || left.type === "float") && right.type === "string") {
    const r = parseFloat(right.value);
    if (!isNaN(r)) return compare(left, op, { type: "float", value: r });
  }

  // Array comparison (deep equality)
  if (left.type === "array" && right.type === "array") {
    if (op === "==") {
      if (left.value.length !== right.value.length) return false;
      for (let i = 0; i < left.value.length; i++) {
        if (!compare(left.value[i]!, "==", right.value[i]!)) return false;
      }
      return true;
    }
    if (op === "!=") return !compare(left, "==", right);
    throw new Error("Arrays can only be compared with == and !=");
  }

  throw new Error(
    `Cannot compare ${left.type} (${JSON.stringify(left)}) with ${right.type} (${JSON.stringify(right)})`,
  );
}
