/**
 * CSL Parser - Recursive descent parser for Treasury CSL scripts.
 *
 * Ports the Rust Winnow-based parser to hand-rolled TypeScript.
 */
import { parse as parseToml } from "smol-toml";
import type * as AST from "../lang/index.js";

// Variable store used during parsing for variable resolution
export interface VarStore {
  get(name: string): VarValue | undefined;
  has(name: string): boolean;
}

export type VarValue =
  | { type: "error"; value: Error }
  | { type: "resource"; value: { name: string; partial: AST.Partial } }
  | { type: "proposal"; value: { name: string; partial: AST.Partial } }
  | { type: "string"; value: string }
  | { type: "integer"; value: number };

// Generate a 16-character Base58 nonce (matching Rust implementation)
const BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
function generateNonce(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => BASE58_ALPHABET[b % BASE58_ALPHABET.length])
    .join("");
}

// Resource type singular/plural mappings
const SINGULAR_TO_PLURAL: Record<string, string> = {
  "access-rule": "access-rules",
  account: "accounts",
  address: "addresses",
  airdrop: "airdrops",
  asset: "assets",
  backup: "backups",
  call: "calls",
  "call-rule": "call-rules",
  chain: "chains",
  credential: "credentials",
  feature: "features",
  host: "hosts",
  key: "keys",
  operation: "operations",
  role: "roles",
  signatory: "signatories",
  signature: "signatures",
  signer: "signers",
  "software-update": "software-updates",
  staking: "stakings",
  "staking-rule": "staking-rules",
  symbol: "symbols",
  tag: "tags",
  transaction: "transactions",
  transfer: "transfers",
  "transfer-rule": "transfer-rules",
  treasury: "treasuries",
  type: "types",
  user: "users",
  "client-key": "client-keys",
};

const PLURAL_TO_SINGULAR: Record<string, string> = {};
for (const [s, p] of Object.entries(SINGULAR_TO_PLURAL)) {
  PLURAL_TO_SINGULAR[p] = s;
}

// Also allow space-separated aliases
const SINGULAR_ALIASES: Record<string, string> = {
  "access rule": "access-rule",
  "call rule": "call-rule",
  "client key": "client-key",
  "software update": "software-update",
  "staking rule": "staking-rule",
  "transfer rule": "transfer-rule",
};

const PLURAL_ALIASES: Record<string, string> = {
  "access rules": "access-rules",
  "access policy": "access-rules",
  "client keys": "client-keys",
  keyring: "client-keys",
  "software updates": "software-updates",
  "transfer rules": "transfer-rules",
  "transfer policy": "transfer-rules",
};

// Variant definitions per resource type
const RESOURCE_VARIANTS: Record<string, string[]> = {
  "access-rule": ["allow", "require", "deny"],
  account: ["internal", "external", "contract", "shared", "validator"],
  address: ["internal", "external", "contract", "shared", "validator"],
  asset: ["native", "token"],
  "call-rule": ["allow", "require", "deny"],
  chain: ["native", "custom"],
  credential: ["invite", "web-authn", "web-authn-uv", "session", "ed255", "k256", "p256"],
  "client-key": ["invite", "ecdsa-k256-sha256", "ecdsa-p256-sha256", "ed25519", "k256", "p256", "ed255"],
  key: ["user", "shared", "internal", "engine", "system"],
  signatory: ["yubi-hsm-2", "mock-yubi-hsm2"],
  staking: ["stake", "unstake", "withdraw"],
  "staking-rule": ["allow", "require", "deny"],
  "transfer-rule": ["allow", "require", "deny"],
  user: ["human", "machine"],
};

const CUSTOM_ACTIONS = [
  "unprice", "price", "activate", "abort", "disable",
  "fee-payer", "heartbeat", "retry", "recheck", "load",
  "share", "confirm", "set", "fail", "restore",
];

const DELAYED_FUNCTIONS: Record<string, AST.DelayedFunction> = {
  keccak256: "keccak256",
  sha256: "sha256",
  resource: "resource",
  typed_data: "typed-data",
  parse_call: "parse-call",
  json: "json",
  verify: "verify-signature",
  verify_signature: "verify-signature",
};

class ParseError extends Error {
  constructor(
    message: string,
    public input: string,
    public position: number,
  ) {
    super(message);
    this.name = "ParseError";
  }
}

// Optional invite-public resolver (set externally to avoid circular deps)
let invitePublicResolver: ((code: string) => string | null) | undefined;

export function setInvitePublicResolver(fn: (code: string) => string | null): void {
  invitePublicResolver = fn;
}

class Parser {
  private input: string;
  private pos: number;
  private vars: VarStore;

  constructor(input: string, vars: VarStore) {
    this.input = input;
    this.pos = 0;
    this.vars = vars;
  }

  private remaining(): string {
    return this.input.slice(this.pos);
  }

  private skipWs(): void {
    while (this.pos < this.input.length && (this.input[this.pos] === " " || this.input[this.pos] === "\t")) {
      this.pos++;
    }
  }

  private skipWs1(): boolean {
    const start = this.pos;
    this.skipWs();
    return this.pos > start;
  }

  private match(str: string): boolean {
    if (this.input.startsWith(str, this.pos)) {
      this.pos += str.length;
      return true;
    }
    return false;
  }

  private peek(str: string): boolean {
    return this.input.startsWith(str, this.pos);
  }

  private peekChar(): string | undefined {
    return this.input[this.pos];
  }

  private isWordBoundary(): boolean {
    const c = this.input[this.pos];
    return (
      c === undefined ||
      c === " " ||
      c === "\t" ||
      c === "{" ||
      c === "}" ||
      c === "(" ||
      c === ")" ||
      c === "," ||
      c === "=" ||
      c === "|" ||
      c === ";" ||
      c === "\n" ||
      c === "\r"
    );
  }

  private matchWord(word: string): boolean {
    const saved = this.pos;
    if (this.input.startsWith(word, this.pos)) {
      this.pos += word.length;
      if (this.isWordBoundary()) {
        return true;
      }
    }
    this.pos = saved;
    return false;
  }

  private tryParse<T>(fn: () => T | null): T | null {
    const saved = this.pos;
    try {
      const result = fn();
      if (result === null) {
        this.pos = saved;
      }
      return result;
    } catch {
      this.pos = saved;
      return null;
    }
  }

  private error(msg: string): never {
    throw new ParseError(msg, this.input, this.pos);
  }

  // Parse an identifier (alphanumeric, dashes, underscores)
  parseId(): string | null {
    this.skipWs();
    const start = this.pos;
    while (this.pos < this.input.length) {
      const c = this.input[this.pos];
      if (/[a-zA-Z0-9_-]/.test(c)) {
        this.pos++;
      } else {
        break;
      }
    }
    if (this.pos === start) return null;
    return this.input.slice(start, this.pos);
  }

  // Parse a variable reference: $identifier or $identifier.field.field2
  parseVariableName(): string | null {
    const saved = this.pos;
    this.skipWs();
    if (!this.match("$")) {
      this.pos = saved;
      return null;
    }
    const id = this.parseId();
    if (!id) {
      this.pos = saved;
      return null;
    }
    return id;
  }

  // Parse a variable value: $var or $var.path.to.field
  parseVariableValue(): AST.Variable | null {
    const saved = this.pos;
    this.skipWs();
    if (!this.match("$")) {
      this.pos = saved;
      return null;
    }
    const start = this.pos;
    while (this.pos < this.input.length && /[a-zA-Z0-9_-]/.test(this.input[this.pos])) {
      this.pos++;
    }
    if (this.pos === start) {
      this.pos = saved;
      return null;
    }
    const identifier = this.input.slice(start, this.pos);

    const path: string[] = [];
    while (this.match(".")) {
      // Try quoted string first (for keys like "SOL.address")
      const quoted = this.tryParse(() => this.parseQuotedString());
      if (quoted !== null) {
        path.push(quoted);
        continue;
      }
      const field = this.parseId();
      if (!field) {
        // Put back the dot
        this.pos--;
        break;
      }
      path.push(field);
    }

    return { identifier, path };
  }

  // Parse a resource type by its singular name
  parseSingular(): string | null {
    this.skipWs();
    const remaining = this.remaining();

    // Try space-separated aliases first
    for (const [alias, singular] of Object.entries(SINGULAR_ALIASES)) {
      if (remaining.startsWith(alias) && this.isWordBoundaryAt(this.pos + alias.length)) {
        this.pos += alias.length;
        return singular;
      }
    }

    // Try direct singular names (longest match first)
    const singulars = Object.keys(SINGULAR_TO_PLURAL).sort((a, b) => b.length - a.length);
    for (const s of singulars) {
      if (remaining.startsWith(s) && this.isWordBoundaryAt(this.pos + s.length)) {
        this.pos += s.length;
        return s;
      }
    }

    return null;
  }

  private isWordBoundaryAt(pos: number): boolean {
    const c = this.input[pos];
    return (
      c === undefined ||
      c === " " ||
      c === "\t" ||
      c === "{" ||
      c === "}" ||
      c === "(" ||
      c === ")" ||
      c === "," ||
      c === "=" ||
      c === "|" ||
      c === ";" ||
      c === "\n" ||
      c === "\r"
    );
  }

  // Parse a resource type by its plural name
  parsePlural(): string | null {
    this.skipWs();
    const remaining = this.remaining();

    // Try aliases first
    for (const [alias, plural] of Object.entries(PLURAL_ALIASES)) {
      if (remaining.startsWith(alias) && this.isWordBoundaryAt(this.pos + alias.length)) {
        this.pos += alias.length;
        return PLURAL_TO_SINGULAR[plural] || alias;
      }
    }

    // Try plural names (longest match first)
    const plurals = Object.values(SINGULAR_TO_PLURAL).sort((a, b) => b.length - a.length);
    for (const p of plurals) {
      if (remaining.startsWith(p) && this.isWordBoundaryAt(this.pos + p.length)) {
        this.pos += p.length;
        return PLURAL_TO_SINGULAR[p]!;
      }
    }

    return null;
  }

  // Parse IDs: supports patterns like:
  //   id                      -> { id }
  //   id for parent           -> { id, parentId }
  //   for parent              -> { parentId }
  //   id1 id2                 -> { parentId: id1, id: id2 }
  parseIds(): { id?: string; parentId?: string } {
    this.skipWs();

    // Try "for parent" (no id, just parent)
    const forOnly = this.tryParse(() => {
      if (!this.matchWord("for")) return null;
      if (!this.skipWs1()) return null;
      const parent = this.parseIdOrQuotedString();
      if (!parent) return null;
      return { parentId: parent };
    });
    if (forOnly) {
      // Check for "with extension"
      const ext = this.tryParseWith();
      if (ext && forOnly.parentId) {
        // No ID, just parent - extension not supported without ID
      }
      return forOnly;
    }

    const first = this.tryParse(() => this.parseIdOrQuotedString());
    if (!first) return {};

    this.skipWs();

    // Try "id for parent [with extension]"
    const forParent = this.tryParse(() => {
      if (!this.matchWord("for")) return null;
      if (!this.skipWs1()) return null;
      const parent = this.parseIdOrQuotedString();
      if (!parent) return null;
      return { id: first, parentId: parent };
    });
    if (forParent) {
      // Check for "with extension" (e.g., address memo)
      const ext = this.tryParseWith();
      if (ext) {
        return { id: `${forParent.id}+${ext}`, parentId: forParent.parentId };
      }
      return forParent;
    }

    // Try second ID (old style: parent id)
    const second = this.tryParse(() => this.parseIdOrQuotedString());
    if (!second) return { id: first };

    return { parentId: first, id: second };
  }

  private tryParseWith(): string | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("with")) return null;
      if (!this.skipWs1()) return null;
      return this.parseIdOrQuotedString();
    });
  }

  private parseIdOrQuotedString(): string | null {
    this.skipWs();
    if (this.peekChar() === '"') return this.parseQuotedString();
    // Try id() function syntax: id(name)
    const idFn = this.tryParse(() => {
      if (!this.matchWord("id")) return null;
      if (!this.match("(")) return null;
      this.skipWs();
      let inner: string | null;
      if (this.peekChar() === '"') {
        inner = this.parseQuotedString();
      } else if (this.peekChar() === '$') {
        const varName = this.parseVariableName();
        if (!varName) return null;
        // Return the variable reference as-is; VM will resolve
        inner = `$${varName}`;
      } else {
        inner = this.parseId();
      }
      if (!inner) return null;
      this.skipWs();
      if (!this.match(")")) return null;
      return inner;
    });
    if (idFn !== null) return idFn;
    return this.parseId();
  }

  // Parse a resource partial: resource-type id1 [id2]
  parsePartial(): AST.Partial | null {
    return this.tryParse(() => {
      const resource = this.parseSingular();
      if (!resource) return null;

      if (!this.skipWs1()) {
        // No IDs, just the type
        if (resource === "treasury") {
          return { resource };
        }
        return null;
      }

      const ids = this.parseIds();
      if (!ids.id && resource !== "treasury") return null;

      return { resource, ...ids };
    });
  }

  // Parse partial or variable
  parsePartialOrVariable(): AST.PartialOrVariable | null {
    // Try partial first
    const partial = this.tryParse(() => this.parsePartial());
    if (partial) return { type: "name", value: partial };

    // Try variable
    const varName = this.tryParse(() => this.parseVariableName());
    if (varName) return { type: "variable", value: varName };

    // Try bare ID
    const id = this.tryParse(() => this.parseId());
    if (id) return { type: "variable", value: id };

    return null;
  }

  // Parse collection (plural) or partial or variable
  parseCollectionOrPartialOrVariable(): AST.PartialOrVariable | null {
    const partial = this.tryParse(() => this.parsePartial());
    if (partial) return { type: "name", value: partial };

    const varName = this.tryParse(() => this.parseVariableName());
    if (varName) return { type: "variable", value: varName };

    const plural = this.tryParse(() => this.parsePlural());
    if (plural) return { type: "name", value: { resource: plural } };

    const id = this.tryParse(() => this.parseId());
    if (id) return { type: "variable", value: id };

    return null;
  }

  // Parse a string value (quoted or with variable interpolation)
  parseQuotedString(): string | null {
    this.skipWs();
    if (!this.match('"')) return null;

    // Check for triple-quote
    if (this.match('""')) {
      // Triple-quoted string
      const start = this.pos;
      const end = this.input.indexOf('"""', this.pos);
      if (end === -1) this.error("Unterminated triple-quoted string");
      this.pos = end + 3;
      return this.input.slice(start, end);
    }

    let result = "";
    while (this.pos < this.input.length) {
      const c = this.input[this.pos];
      if (c === "\\") {
        this.pos++;
        const next = this.input[this.pos];
        if (next === '"') result += '"';
        else if (next === "\\") result += "\\";
        else if (next === "n") result += "\n";
        else if (next === "t") result += "\t";
        else result += next;
        this.pos++;
      } else if (c === '"') {
        this.pos++;
        return result;
      } else {
        result += c;
        this.pos++;
      }
    }
    this.error("Unterminated string");
  }

  // Parse a string value (which may be a quoted string, variable, function call, or bare string)
  parseString(): string | null {
    this.skipWs();

    // Try functions that resolve at parse time
    const fnResult = this.tryParse(() => this.parseStringFunction());
    if (fnResult !== null) return fnResult;

    // Try variable resolution
    const varVal = this.tryParse(() => this.parseVariableValue());
    if (varVal) {
      return this.resolveVariableToString(varVal);
    }

    // Try quoted string
    const quoted = this.tryParse(() => this.parseQuotedString());
    if (quoted !== null) return quoted;

    // Try bare simple key (unquoted string)
    return this.parseSimpleKey();
  }

  private resolveVariableToString(v: AST.Variable): string {
    const val = this.vars.get(v.identifier);
    if (!val) return `$${v.identifier}${v.path.length ? "." + v.path.join(".") : ""}`;

    if (val.type === "string") return val.value;
    if (val.type === "integer") return val.value.toString();
    if (val.type === "resource" || val.type === "proposal") {
      if (v.path.length === 0) return val.value.name;
      return `$${v.identifier}.${v.path.join(".")}`;
    }
    return `$${v.identifier}`;
  }

  // Parse parse-time string functions like concat(), nonce(), now(), hex(), etc.
  private parseStringFunction(): string | null {
    this.skipWs();

    // concat(s1, s2)
    if (this.matchWord("concat")) {
      if (!this.match("(")) return null;
      this.skipWs();
      const s1 = this.parseString();
      this.skipWs();
      this.match(",");
      this.skipWs();
      const s2 = this.parseString();
      this.skipWs();
      this.match(")");
      return (s1 || "") + (s2 || "");
    }

    // nonce()
    if (this.matchWord("nonce")) {
      this.match("(");
      this.match(")");
      return generateNonce();
    }

    // now()
    if (this.matchWord("now")) {
      this.match("(");
      this.match(")");
      return new Date().toISOString();
    }

    // hex(string)
    if (this.matchWord("hex")) {
      if (!this.match("(")) return null;
      this.skipWs();
      const s = this.parseString();
      this.skipWs();
      this.match(")");
      if (s) {
        const bytes = new TextEncoder().encode(s);
        return Array.from(bytes)
          .map((b) => b.toString(16).padStart(2, "0"))
          .join("");
      }
      return "";
    }

    // invite-public(hex_code)
    if (this.matchWord("invite-public")) {
      if (!this.match("(")) return null;
      this.skipWs();
      const code = this.parseString();
      this.skipWs();
      this.match(")");
      if (code) {
        try {
          const key = invitePublicResolver?.(code);
          if (key) return key;
        } catch {
          // Fallback: return placeholder
        }
      }
      return `invite-public(${code})`;
    }

    // elapsed(timestamp)
    if (this.matchWord("elapsed")) {
      if (!this.match("(")) return null;
      this.skipWs();
      const ts = this.parseString();
      this.skipWs();
      this.match(")");
      if (ts) {
        const then = new Date(ts).getTime();
        const now = Date.now();
        return Math.floor((now - then) / 1000).toString();
      }
      return "0";
    }

    // env(VAR) or env_value(VAR)
    if (this.matchWord("env") || this.matchWord("env_value")) {
      if (!this.match("(")) return null;
      this.skipWs();
      const varName = this.parseString();
      this.skipWs();
      this.match(")");
      if (varName) {
        return process.env[varName] || "";
      }
      return "";
    }

    return null;
  }

  // Parse a simple (unquoted) key
  private parseSimpleKey(): string | null {
    this.skipWs();
    const start = this.pos;
    while (this.pos < this.input.length) {
      const c = this.input[this.pos];
      if (/[a-zA-Z0-9_.+\/-]/.test(c)) {
        this.pos++;
      } else {
        break;
      }
    }
    if (this.pos === start) return null;
    return this.input.slice(start, this.pos);
  }

  // Parse a setting key like "until.timeout" -> ["until", "timeout"]
  parseKey(): string[] | null {
    this.skipWs();
    const parts: string[] = [];
    const first = this.parseId();
    if (!first) return null;
    parts.push(first);
    while (this.match(".")) {
      const part = this.parseId();
      if (!part) {
        this.pos--;
        break;
      }
      parts.push(part);
    }
    return parts;
  }

  // Parse an inline table: { key = value, ... }
  parseInlineTable(): AST.InlineTable | null {
    this.skipWs();
    if (!this.match("{")) return null;
    this.skipWs();

    if (this.match("}")) return {};

    // Collect the TOML-like content between braces
    let depth = 1;
    const start = this.pos;
    while (this.pos < this.input.length && depth > 0) {
      const c = this.input[this.pos];
      if (c === "{") depth++;
      else if (c === "}") depth--;
      if (depth > 0) this.pos++;
    }

    if (depth !== 0) this.error("Unterminated inline table");

    const content = this.input.slice(start, this.pos);
    this.pos++; // skip closing }

    // Resolve variables in the content
    const resolved = this.resolveVarsInString(content);

    // Parse as TOML inline table
    try {
      // Wrap in a table assignment so smol-toml can parse it
      const tomlStr = `__root = { ${resolved} }`;
      const parsed = parseToml(tomlStr);
      return parsed.__root as AST.InlineTable;
    } catch {
      // Try direct parse
      try {
        const tomlStr = resolved
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean)
          .join("\n");
        return parseToml(tomlStr) as unknown as AST.InlineTable;
      } catch (e2) {
        throw new ParseError(`Failed to parse inline table: ${e2}`, content, 0);
      }
    }
  }

  // Resolve $variable and function references in a string for TOML parsing
  private resolveVarsInString(s: string): string {
    // First resolve functions: nonce(), now(), hex(...), concat(...), invite-public(...)
    let result = s.replace(/\bnonce\(\)/g, () => `"${generateNonce()}"`);
    result = result.replace(/\bnow\(\)/g, () => `"${new Date().toISOString()}"`);
    result = result.replace(/\bhex\(([^)]+)\)/g, (_match, inner) => {
      const resolved = inner.replace(/^"(.*)"$/, "$1");
      const bytes = new TextEncoder().encode(resolved);
      return `"${Array.from(bytes).map((b: number) => b.toString(16).padStart(2, "0")).join("")}"`;
    });
    result = result.replace(/\bconcat\(([^,]+),\s*([^)]+)\)/g, (_match, s1, s2) => {
      const v1 = s1.replace(/^"(.*)"$/, "$1").trim();
      const v2 = s2.replace(/^"(.*)"$/, "$1").trim();
      return `"${v1}${v2}"`;
    });
    // Resolve invite-public(hex_code) function
    result = result.replace(/\binvite-public\(([^)]+)\)/g, (_match, inner) => {
      const code = inner.replace(/^"(.*)"$/, "$1").trim();
      if (invitePublicResolver) {
        const pubkey = invitePublicResolver(code);
        if (pubkey) return `"${pubkey}"`;
      }
      return `"invite-public(${code})"`;
    });

    // Then resolve variables
    result = result.replace(/\$([a-zA-Z0-9_-]+)(?:\.([a-zA-Z0-9_.]+))?/g, (match, id, path) => {
      const val = this.vars.get(id);
      if (!val) return `"${match}"`; // Quote unresolved vars for TOML safety
      if (val.type === "string") {
        // Escape special characters for TOML string embedding
        const escaped = val.value
          .replace(/\\/g, "\\\\")
          .replace(/"/g, '\\"')
          .replace(/\n/g, "\\n")
          .replace(/\t/g, "\\t");
        return `"${escaped}"`;
      }
      if (val.type === "integer") return val.value.toString();
      if (val.type === "resource" || val.type === "proposal") {
        if (!path) return `"${val.value.name}"`;
        // .name resolves to the resource name
        if (path === "name") return `"${val.value.name}"`;
        // Quote for TOML, resolve at runtime in resolveInlineTable
        return `"${match}"`;
      }
      if (val.type === "error") return `"${match}"`;
      return `"${match}"`;
    });
    return result;
  }

  // Parse a value (string, integer, boolean, array, inline table, variable, function)
  parseValue(): AST.Value | null {
    this.skipWs();

    // Try delayed function
    const fn = this.tryParse(() => this.parseDelayedFunction());
    if (fn) return { type: "function", value: fn };

    // Try inline table
    if (this.peekChar() === "{") {
      const table = this.parseInlineTable();
      if (table) return { type: "inline-table", value: table };
    }

    // Try array
    if (this.peekChar() === "[") {
      const arr = this.parseArray();
      if (arr) return { type: "array", value: arr };
    }

    // Try variable
    if (this.peekChar() === "$") {
      const v = this.parseVariableValue();
      if (v) return { type: "variable", value: v };
    }

    // Try boolean
    if (this.matchWord("true")) return { type: "boolean", value: true };
    if (this.matchWord("false")) return { type: "boolean", value: false };

    // Try integer (negative or positive)
    const intVal = this.tryParse(() => this.parseInteger());
    if (intVal !== null) return { type: "integer", value: intVal };

    // Try string function
    const strFn = this.tryParse(() => this.parseStringFunction());
    if (strFn !== null) return { type: "string", value: strFn };

    // Try quoted string
    if (this.peekChar() === '"') {
      const s = this.parseQuotedString();
      if (s !== null) return { type: "string", value: s };
    }

    // Try bare string (simple key-like)
    const simple = this.tryParse(() => this.parseSimpleKey());
    if (simple) return { type: "string", value: simple };

    return null;
  }

  private parseInteger(): number | null {
    this.skipWs();
    const start = this.pos;
    if (this.input[this.pos] === "-") this.pos++;
    const digitStart = this.pos;
    while (this.pos < this.input.length && /[0-9]/.test(this.input[this.pos])) {
      this.pos++;
    }
    if (this.pos === digitStart) {
      this.pos = start;
      return null;
    }
    // Make sure the next char isn't a letter (would be an ID, not int)
    if (this.pos < this.input.length && /[a-zA-Z_-]/.test(this.input[this.pos])) {
      this.pos = start;
      return null;
    }
    return parseInt(this.input.slice(start, this.pos), 10);
  }

  // Parse an array: [val1, val2, ...] or range: 1..5, 1..=5
  parseArray(): AST.Value[] | null {
    this.skipWs();
    if (!this.match("[")) return null;
    this.skipWs();

    if (this.match("]")) return [];

    const values: AST.Value[] = [];
    const first = this.parseValue();
    if (!first) this.error("Expected value in array");
    values.push(first);

    // Check for range syntax
    if (first.type === "integer" && this.match("..")) {
      const inclusive = this.match("=");
      const end = this.parseValue();
      if (!end || end.type !== "integer") this.error("Expected integer for range end");
      this.skipWs();
      this.match("]");
      // Expand range
      const result: AST.Value[] = [];
      const limit = inclusive ? end.value + 1 : end.value;
      for (let i = first.value; i < limit; i++) {
        result.push({ type: "integer", value: i });
      }
      return result;
    }

    while (this.pos < this.input.length) {
      this.skipWs();
      if (this.match("]")) break;
      if (!this.match(",")) this.error("Expected ',' in array");
      this.skipWs();
      if (this.peek("]")) {
        this.match("]");
        break;
      }
      const val = this.parseValue();
      if (!val) this.error("Expected value in array");
      values.push(val);
    }

    return values;
  }

  // Parse a delayed function: keccak256(...), json(...), etc.
  parseDelayedFunction(): AST.FunctionCall | null {
    this.skipWs();

    for (const [name, fn] of Object.entries(DELAYED_FUNCTIONS)) {
      if (this.matchWord(name) || (this.input.startsWith(name + "(", this.pos) && ((this.pos += name.length), true))) {
        this.skipWs();
        if (!this.match("(")) {
          this.pos -= name.length;
          return null;
        }
        this.skipWs();

        const params: AST.Value[] = [];
        const first = this.parseValue();
        if (first) {
          params.push(first);
          while (this.pos < this.input.length) {
            this.skipWs();
            if (!this.match(",")) break;
            this.skipWs();
            const val = this.parseValue();
            if (!val) break;
            params.push(val);
          }
        }
        this.skipWs();
        this.match(")");

        return { name: fn, parameters: params };
      }
    }

    return null;
  }

  // Parse variant-resource combo like "internal account" or just "account"
  parseVariantResource(): { variant?: string; resource: string } | null {
    this.skipWs();

    // Try variant + resource combos
    for (const [resource, variants] of Object.entries(RESOURCE_VARIANTS)) {
      for (const variant of variants) {
        const result = this.tryParse(() => {
          if (!this.matchWord(variant)) return null;
          if (!this.skipWs1()) return null;
          // Try all singular forms (including aliases)
          const singular = resource;
          if (this.matchWord(singular)) return { variant, resource: singular };
          // Try space-separated form
          const spaceForm = singular.replace(/-/g, " ");
          if (spaceForm !== singular && this.matchWord(spaceForm)) return { variant, resource: singular };
          return null;
        });
        if (result) return result;
      }
    }

    // Try plain resource
    const resource = this.parseSingular();
    if (resource) return { resource };

    return null;
  }

  // Parse a create command
  parseCreate(): AST.Create | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("create")) return null;
      if (!this.skipWs1()) return null;

      const vr = this.parseVariantResource();
      if (!vr) return null;

      this.skipWs();
      const ids = this.peekChar() !== "{" ? (this.tryParse(() => this.parseIds()) || {}) : {};

      this.skipWs();
      const data = this.tryParse(() => this.parseInlineTable()) || {};

      return {
        variant: vr.variant,
        partial: { resource: vr.resource, ...ids },
        data,
      };
    });
  }

  // Parse a propose command
  parsePropose(): AST.Create | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("propose")) return null;
      if (!this.skipWs1()) return null;

      const vr = this.parseVariantResource();
      if (!vr) return null;

      this.skipWs();
      const ids = this.peekChar() !== "{" ? (this.tryParse(() => this.parseIds()) || {}) : {};

      this.skipWs();
      const data = this.tryParse(() => this.parseInlineTable()) || {};

      return {
        variant: vr.variant,
        partial: { resource: vr.resource, ...ids },
        data,
      };
    });
  }

  parseUpdate(): AST.Update | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("update")) return null;
      if (!this.skipWs1()) return null;

      const what = this.parsePartialOrVariable();
      if (!what) return null;

      this.skipWs();
      const data = this.tryParse(() => this.parseInlineTable()) || {};

      return { what, data };
    });
  }

  parseDelete(): AST.Delete | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("delete")) return null;
      if (!this.skipWs1()) return null;

      let proposed = false;
      if (this.matchWord("proposed")) {
        proposed = true;
        this.skipWs1();
      }

      const pov = this.parsePartialOrVariable();
      if (!pov) return null;

      return { pov, proposed };
    });
  }

  parseApprove(): AST.PartialOrVariable | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("approve")) return null;
      if (!this.skipWs1()) return null;
      return this.parsePartialOrVariable();
    });
  }

  parseCancel(): AST.PartialOrVariable | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("cancel")) return null;
      if (!this.skipWs1()) return null;
      return this.parsePartialOrVariable();
    });
  }

  parseSubmit(): AST.PartialOrVariable | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("submit")) return null;
      if (!this.skipWs1()) return null;
      if (this.matchWord("proposed")) this.skipWs1();
      return this.parsePartialOrVariable();
    });
  }

  parseCustom(): AST.Custom | null {
    return this.tryParse(() => {
      this.skipWs();
      let action: string | null = null;
      for (const a of CUSTOM_ACTIONS) {
        if (this.matchWord(a)) {
          action = a;
          break;
        }
      }
      if (!action) return null;
      if (!this.skipWs1()) return null;

      const what = this.parseCollectionOrPartialOrVariable();
      if (!what) return null;

      this.skipWs();
      let payload: AST.CustomPayload | undefined;
      if (this.remaining().trim()) {
        const table = this.tryParse(() => this.parseInlineTable());
        if (table) {
          payload = { type: "object", value: table };
        } else {
          const str = this.tryParse(() => this.parseString());
          if (str) {
            payload = { type: "string", value: str };
          }
        }
      }

      return { action, what, payload };
    });
  }

  parseSet(): AST.Setting | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("set")) return null;
      if (!this.skipWs1()) return null;

      const key = this.parseKey();
      if (!key) return null;

      this.skipWs();
      if (!this.match("=")) return null;
      this.skipWs();

      const value = this.parseString();
      if (value === null) this.error("Expected value for setting");

      let value2: string | undefined;
      let value3: string | undefined;

      if (this.tryParse(() => { this.skipWs(); return this.match(",") ? true : null; })) {
        this.skipWs();
        value2 = this.parseString() || undefined;
        if (this.tryParse(() => { this.skipWs(); return this.match(",") ? true : null; })) {
          this.skipWs();
          value3 = this.parseString() || undefined;
        }
      }

      return { key, value, value2, value3 };
    });
  }

  parseUnset(): AST.SettingKey | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("unset")) return null;
      if (!this.skipWs1()) return null;
      const key = this.parseKey();
      if (!key) return null;
      return { key };
    });
  }

  parseSetting(): AST.SettingKey | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("setting")) return null;
      if (!this.skipWs1()) return null;
      const key = this.parseKey();
      if (!key) return null;
      return { key };
    });
  }

  parseAssert(): AST.Assert | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("assert")) return null;
      if (!this.skipWs1()) return null;
      const condition = this.remaining().trim();
      this.pos = this.input.length;
      return { condition };
    });
  }

  parseUntil(): AST.Until | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("until")) return null;
      if (!this.skipWs1()) return null;
      const condition = this.remaining().trim();
      this.pos = this.input.length;
      return { condition };
    });
  }

  parseConvert(): AST.Convert | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("convert")) return null;
      if (!this.match("(")) return null;
      this.skipWs();
      const data = this.parseValue();
      if (!data) return null;
      this.skipWs();
      this.match(",");
      this.skipWs();
      const sourceEncoding = this.parseValue();
      if (!sourceEncoding) return null;
      this.skipWs();
      this.match(",");
      this.skipWs();
      const targetEncoding = this.parseValue();
      if (!targetEncoding) return null;
      this.skipWs();
      this.match(")");
      return { data, sourceEncoding, targetEncoding };
    });
  }

  parseReplace(): AST.Replace | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("replace")) return null;
      if (!this.match("(")) return null;
      this.skipWs();
      const data = this.parseValue();
      if (!data) return null;
      this.skipWs();
      this.match(",");
      this.skipWs();
      const old = this.parseValue();
      if (!old) return null;
      this.skipWs();
      this.match(",");
      this.skipWs();
      const newVal = this.parseValue();
      if (!newVal) return null;
      this.skipWs();
      this.match(")");
      return { data, old, new: newVal };
    });
  }

  parseGet(): AST.Get | null {
    return this.tryParse(() => {
      this.skipWs();
      // Optional "get" keyword
      this.tryParse(() => {
        if (!this.matchWord("get")) return null;
        this.skipWs1();
        return true;
      });

      let proposed = false;
      if (this.matchWord("proposed")) {
        proposed = true;
        this.skipWs1();
      }

      const pov = this.parsePartialOrVariable();
      if (!pov) return null;

      // If variable followed by '.' or '(', this is a field access or function call, not a get
      const next = this.peekChar();
      if (pov.type === "variable" && (next === "." || next === "(")) return null;

      return { pov, proposed };
    });
  }

  parseList(): AST.List | null {
    return this.tryParse(() => {
      this.skipWs();
      // Optional "list" or "get" keyword
      this.tryParse(() => {
        if (this.matchWord("list") || this.matchWord("get")) {
          this.skipWs1();
          return true;
        }
        return null;
      });

      let proposed = false;
      if (this.matchWord("proposed")) {
        proposed = true;
        this.skipWs1();
      }

      const resource = this.parsePlural();
      if (!resource) return null;

      // Optional parent: "for $var" or parent ID
      let parent: AST.IdOrVariable | undefined;
      const parentResult = this.tryParse(() => {
        if (!this.skipWs1()) return null;
        if (this.matchWord("for")) {
          this.skipWs1();
          const varName = this.parseVariableName();
          if (varName) return { type: "variable" as const, value: varName };
          const id = this.parseId();
          if (id) return { type: "id" as const, value: id };
        }
        // Try bare parent ID
        const id = this.parseId();
        if (id) return { type: "id" as const, value: id };
        return null;
      });
      if (parentResult) parent = parentResult;

      // Optional filter: | filter_string
      let filter: string | undefined;
      this.skipWs();
      if (this.match("|")) {
        this.skipWs();
        // Take until { (for loops) or end of line
        const remaining = this.remaining();
        const braceIdx = remaining.indexOf("{");
        if (braceIdx >= 0) {
          filter = remaining.slice(0, braceIdx).trim();
          this.pos += braceIdx;
        } else {
          filter = remaining.trim();
          this.pos = this.input.length;
        }
      }

      return { resource, parent, filter, proposed };
    });
  }

  parseForLoop(): AST.ForLoop | null {
    return this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("for")) return null;
      if (!this.skipWs1()) return null;

      const variable = this.parseId();
      if (!variable) return null;

      this.skipWs();
      if (!this.matchWord("in")) return null;
      this.skipWs1();

      // Parse iterable: bare range, array, or list
      let iterable: AST.ForLoop["iterable"];

      // Try bare range: 1..3 or 1..=3
      const bareRange = this.tryParse(() => {
        const start = this.parseInteger();
        if (start === null) return null;
        if (!this.match("..")) return null;
        const inclusive = this.match("=");
        const end = this.parseInteger();
        if (end === null) return null;
        const result: AST.Value[] = [];
        const limit = inclusive ? end + 1 : end;
        for (let i = start; i < limit; i++) {
          result.push({ type: "integer", value: i });
        }
        return result;
      });

      if (bareRange) {
        iterable = { type: "array", value: bareRange };
      } else {
        const array = this.tryParse(() => this.parseArray());
        if (array) {
          iterable = { type: "array", value: array };
        } else {
          const list = this.parseList();
          if (!list) return null;
          iterable = { type: "list", value: list };
        }
      }

      this.skipWs();
      if (!this.match("{")) return null;
      this.skipWs();

      // Find the matching closing brace, accounting for nesting
      const start = this.pos;
      let depth = 1;
      while (this.pos < this.input.length && depth > 0) {
        if (this.input[this.pos] === "{") depth++;
        else if (this.input[this.pos] === "}") depth--;
        if (depth > 0) this.pos++;
      }
      const body = this.input.slice(start, this.pos);
      this.pos++; // skip }

      const bodyTrimmed = body.trim();
      const command = parseCommand(bodyTrimmed, this.vars);

      return { variable, iterable, command, bodyText: bodyTrimmed };
    });
  }

  parseAssignment(): AST.Assignment | null {
    return this.tryParse(() => {
      this.skipWs();
      const variable = this.parseId();
      if (!variable) return null;

      this.skipWs();
      let direct = false;
      if (this.match(":=")) {
        direct = true;
      } else if (this.match("=")) {
        direct = false;
      } else {
        return null;
      }
      this.skipWs();

      // Try various assignable forms
      const assignable = this.parseAssignable();
      if (!assignable) return null;

      return { variable, value: assignable, direct };
    });
  }

  private parseAssignable(): AST.Assignable | null {
    // Order matters! Try more specific parsers first.
    let result: AST.Assignable | null;

    result = this.tryAssignable(() => this.parseCreate(), (v) => ({ type: "create", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parsePropose(), (v) => ({ type: "propose", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseUpdate(), (v) => ({ type: "update", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseDelete(), (v) => ({ type: "delete", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseApprove(), (v) => ({ type: "approve", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseSubmit(), (v) => ({ type: "submit", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseCancel(), (v) => ({ type: "cancel", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parsePartial(), (v) => ({ type: "partial", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseCustom(), (v) => ({ type: "custom", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseConvert(), (v) => ({ type: "convert", value: v }));
    if (result) return result;

    result = this.tryAssignable(() => this.parseReplace(), (v) => ({ type: "replace", value: v }));
    if (result) return result;

    result = this.tryAssignable(
      () => this.parseDelayedFunction(),
      (v) => ({ type: "function", value: v }),
    );
    if (result) return result;

    result = this.tryAssignable(
      () => this.parseSetting(),
      (v) => ({ type: "setting", value: v }),
    );
    if (result) return result;

    // Try get
    result = this.tryAssignable(
      () => this.parseGet(),
      (v) => ({ type: "get", value: v }),
    );
    if (result) return result;

    // Try value
    const val = this.parseValue();
    if (val) return { type: "string", value: val };

    return null;
  }

  private tryAssignable<T>(parse: () => T | null, map: (v: T) => AST.Assignable): AST.Assignable | null {
    const result = this.tryParse(parse);
    if (result !== null) return map(result);
    return null;
  }

  parseFallibleAssignment(): AST.FallibleAssignment | null {
    return this.tryParse(() => {
      this.skipWs();
      const variable = this.parseId();
      if (!variable) return null;

      if (!this.match(",")) return null;
      this.skipWs();

      const error = this.parseId();
      if (!error) return null;

      this.skipWs();
      let direct = false;
      if (this.match(":=")) {
        direct = true;
      } else if (this.match("=")) {
        direct = false;
      } else {
        return null;
      }
      this.skipWs();

      const mutate = this.parseMutate();
      if (!mutate) return null;

      return { variable, error, mutate, direct };
    });
  }

  private parseMutate(): AST.Mutate | null {
    let r: AST.Mutate | null;

    r = this.tryMutate(() => this.parseCreate(), (v) => ({ type: "create", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parsePropose(), (v) => ({ type: "propose", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseUpdate(), (v) => ({ type: "update", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseDelete(), (v) => ({ type: "delete", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseApprove(), (v) => ({ type: "approve", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseCancel(), (v) => ({ type: "cancel", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseSubmit(), (v) => ({ type: "submit", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseCustom(), (v) => ({ type: "custom", value: v }));
    if (r) return r;
    r = this.tryMutate(() => this.parseGet(), (v) => ({ type: "get", value: v }));
    if (r) return r;

    return null;
  }

  private tryMutate<T>(parse: () => T | null, map: (v: T) => AST.Mutate): AST.Mutate | null {
    const result = this.tryParse(parse);
    if (result !== null) return map(result);
    return null;
  }

  // Main command parser
  parseCommand(): AST.Command {
    this.skipWs();
    if (this.pos >= this.input.length || this.remaining().trim() === "") {
      return { type: "nop" };
    }

    // Comment
    if (this.peek("#") || this.peek("//")) {
      return { type: "nop" };
    }

    // Exit
    let result: AST.Command | null;

    result = this.tryParse(() => {
      this.skipWs();
      if (this.matchWord("exit") || this.matchWord("quit")) return { type: "exit" as const };
      return null;
    });
    if (result) return result;

    // exit_if_set
    result = this.tryParse(() => {
      this.skipWs();
      if (!this.matchWord("exit_if_set")) return null;
      this.skipWs1();
      const envVar = this.parseString();
      if (envVar && process.env[envVar] !== undefined) return { type: "exit" as const };
      return { type: "nop" as const };
    });
    if (result) return result;

    // Fallible assignment: var,err = ...
    const fallible = this.tryParse(() => this.parseFallibleAssignment());
    if (fallible) return { type: "fallible-assignment", value: fallible };

    // Assignment: var = ...
    const assignment = this.tryParse(() => this.parseAssignment());
    if (assignment) return { type: "assignment", value: assignment };

    // Assert
    const assert = this.tryParse(() => this.parseAssert());
    if (assert) return { type: "assert", value: assert };

    // Convert
    const convert = this.tryParse(() => this.parseConvert());
    if (convert) return { type: "convert", value: convert };

    // Replace
    const replace = this.tryParse(() => this.parseReplace());
    if (replace) return { type: "replace", value: replace };

    // For loop
    const forLoop = this.tryParse(() => this.parseForLoop());
    if (forLoop) return { type: "for-loop", value: forLoop };

    // Until
    const until = this.tryParse(() => this.parseUntil());
    if (until) return { type: "until", value: until };

    // Create
    const create = this.tryParse(() => this.parseCreate());
    if (create) return { type: "create", value: create };

    // Propose
    const propose = this.tryParse(() => this.parsePropose());
    if (propose) return { type: "propose", value: propose };

    // Update
    const update = this.tryParse(() => this.parseUpdate());
    if (update) return { type: "update", value: update };

    // Delete
    const del = this.tryParse(() => this.parseDelete());
    if (del) return { type: "delete", value: del };

    // Set setting
    const set = this.tryParse(() => this.parseSet());
    if (set) return { type: "set-setting", value: set };

    // Get setting
    const setting = this.tryParse(() => this.parseSetting());
    if (setting) return { type: "get-setting", value: setting };

    // Unset
    const unset = this.tryParse(() => this.parseUnset());
    if (unset) return { type: "unset", value: unset };

    // Approve
    const approve = this.tryParse(() => this.parseApprove());
    if (approve) return { type: "approve", value: approve };

    // Cancel
    const cancel = this.tryParse(() => this.parseCancel());
    if (cancel) return { type: "cancel", value: cancel };

    // Submit
    const submit = this.tryParse(() => this.parseSubmit());
    if (submit) return { type: "submit", value: submit };

    // Custom action
    const custom = this.tryParse(() => this.parseCustom());
    if (custom) return { type: "custom", value: custom };

    // Read (get/list) - before value expression so get/list keywords are handled
    const list = this.tryParse(() => this.parseList());
    if (list) return { type: "read", value: { type: "list", value: list } };

    const get = this.tryParse(() => this.parseGet());
    if (get) return { type: "read", value: { type: "get", value: get } };

    // Value expression (implicit get is handled in the VM)
    const value = this.tryParse(() => this.parseValue());
    if (value) return { type: "value", value };

    this.error(`Unrecognized command: ${this.remaining().slice(0, 40)}`);
  }
}

// Empty var store for when no variables are available
class EmptyVarStore implements VarStore {
  get(): undefined {
    return undefined;
  }
  has(): boolean {
    return false;
  }
}

/**
 * Parse a single CSL command.
 */
export function parseCommand(input: string, vars?: VarStore): AST.Command {
  const parser = new Parser(input.trim(), vars || new EmptyVarStore());
  return parser.parseCommand();
}

/**
 * Split a CSL script into individual command strings.
 * Commands are separated by `;` or newlines. Handles `{ }` blocks for for-loops.
 */
export function splitCommands(script: string): string[] {
  const commands: string[] = [];
  let current = "";
  let depth = 0;
  let inString = false;
  let escape = false;

  for (let i = 0; i < script.length; i++) {
    const c = script[i];

    if (escape) {
      current += c;
      escape = false;
      continue;
    }

    if (c === "\\") {
      current += c;
      escape = true;
      continue;
    }

    if (c === '"' && !escape) {
      inString = !inString;
      current += c;
      continue;
    }

    if (inString) {
      current += c;
      continue;
    }

    if (c === "{") {
      depth++;
      current += c;
      continue;
    }

    if (c === "}") {
      depth--;
      current += c;
      continue;
    }

    if (depth === 0 && (c === ";" || c === "\n")) {
      const trimmed = current.trim();
      if (trimmed && !trimmed.startsWith("#") && !trimmed.startsWith("//")) {
        commands.push(trimmed);
      }
      current = "";
      continue;
    }

    current += c;
  }

  const trimmed = current.trim();
  if (trimmed && !trimmed.startsWith("#") && !trimmed.startsWith("//")) {
    commands.push(trimmed);
  }

  return commands;
}

export { Parser, ParseError, EmptyVarStore };
