/**
 * CSL Virtual Machine - Executes parsed CSL commands against a Treasury API.
 */
import {
  type TreasuryClient,
  type SigningKey,
  type NormalAlgorithm,
  Keyring,
  SigningKey as SK,
  VerifyingKey,
  TreasuryApiError,
  singularToPlural,
  pluralToSingular,
  getResourceTypeBySingular,
  parseNormalAlgorithm,
  keccak256 as keccak256Fn,
  sha256 as sha256Fn,
} from "@cordialsys/treasury-sdk";
import { hex, base58 } from "@scure/base";
import { secp256k1 } from "@noble/curves/secp256k1";
import { ed25519 } from "@noble/curves/ed25519";
import { bls12_381 } from "@noble/curves/bls12-381";
import { blake2b } from "@noble/hashes/blake2b";
import type * as AST from "../lang/index.js";
import { parseCommand, splitCommands, setInvitePublicResolver } from "../parser/index.js";
import type { VarStore, VarValue } from "../parser/index.js";

// Set up the invite-public resolver
setInvitePublicResolver((code: string) => {
  const key = SK.fromInvite(code);
  if (key) return key.toPublicBytesHex();
  return null;
});
import {
  parseCondition,
  extractVariables,
  evaluateCondition,
  type EvalValue,
} from "./condition.js";

export { parseCondition, evaluateCondition, extractVariables };
export type { EvalValue };

export class CslError extends Error {
  code: string;
  detail?: string;

  constructor(code: string, message: string, detail?: string) {
    super(message);
    this.name = "CslError";
    this.code = code;
    this.detail = detail;
  }
}

/** Map gRPC numeric codes to human-readable status strings. */
const GRPC_STATUS: Record<number, string> = {
  0: "OK", 1: "Cancelled", 2: "Unknown", 3: "Invalid Argument",
  4: "Deadline Exceeded", 5: "Not Found", 6: "Already Exists",
  7: "Permission Denied", 8: "Resource Exhausted", 9: "Failed Precondition",
  10: "Aborted", 11: "Out of Range", 12: "Unimplemented",
  13: "Internal", 14: "Unavailable", 15: "Data Loss", 16: "Unauthenticated",
};

/** Extract status string from operation error (handles both numeric gRPC codes and string statuses). */
function opErrorStatus(error: Record<string, unknown>): string {
  if (typeof error.status === "string" && error.status !== "") return error.status;
  if (typeof error.code === "number") return GRPC_STATUS[error.code] || "Unknown";
  if (typeof error.code === "string") return error.code;
  return "Unknown";
}

export interface VmConfig {
  allowFail?: boolean;
  untilTimeout?: number; // seconds
  displayStrip?: boolean;
  listDeleted?: boolean;
}

interface ResourceRef {
  name: string;
  partial: AST.Partial;
}

type VmVarValue =
  | { type: "error"; value: CslError }
  | { type: "resource"; value: ResourceRef }
  | { type: "proposal"; value: ResourceRef }
  | { type: "string"; value: string }
  | { type: "integer"; value: number };

class VmVarStore implements VarStore {
  private map = new Map<string, VmVarValue>();

  get(name: string): VarValue | undefined {
    const v = this.map.get(name);
    if (!v) return undefined;
    switch (v.type) {
      case "string":
        return { type: "string", value: v.value };
      case "integer":
        return { type: "integer", value: v.value };
      case "resource":
        return { type: "resource", value: { name: v.value.name, partial: v.value.partial } };
      case "proposal":
        return { type: "proposal", value: { name: v.value.name, partial: v.value.partial } };
      case "error":
        return { type: "error", value: v.value };
    }
  }

  has(name: string): boolean {
    return this.map.has(name);
  }

  getRaw(name: string): VmVarValue | undefined {
    return this.map.get(name);
  }

  set(name: string, value: VmVarValue): void {
    if (name === "_") return; // discard
    this.map.set(name, value);
  }

  remove(name: string): void {
    this.map.delete(name);
  }

  getResource(name: string): ResourceRef | undefined {
    const v = this.map.get(name);
    if (v?.type === "resource") return v.value;
    return undefined;
  }

  getProposal(name: string): ResourceRef | undefined {
    const v = this.map.get(name);
    if (v?.type === "proposal") return v.value;
    return undefined;
  }
}

export class CslVm {
  private client: TreasuryClient;
  private vars = new VmVarStore();
  private config: VmConfig = {};
  private keyring: Keyring;
  private clientKeys = new Map<string, SigningKey>();
  private output: string[] = [];
  private startTime = Date.now();

  constructor(client: TreasuryClient, keyring?: Keyring) {
    this.client = client;
    this.keyring = keyring || new Keyring(client.treasuryId);
  }

  getOutput(): string[] {
    return this.output;
  }

  private log(msg: string): void {
    this.output.push(msg);
  }

  private async resolveValue(value: AST.Value): Promise<unknown> {
    switch (value.type) {
      case "string":
        return value.value;
      case "integer":
        return value.value;
      case "float":
        return value.value;
      case "boolean":
        return value.value;
      case "array": {
        const resolved = [];
        for (const item of value.value) {
          resolved.push(await this.resolveValue(item));
        }
        return resolved;
      }
      case "inline-table":
        return await this.resolveInlineTable(value.value);
      case "variable":
        return await this.resolveVariable(value.value);
      case "function":
        return await this.evaluateFunction(value.value);
    }
  }

  private async resolveInlineTable(
    table: AST.InlineTable,
  ): Promise<Record<string, unknown>> {
    const result: Record<string, unknown> = {};
    for (const [key, val] of Object.entries(table)) {
      if (typeof val === "object" && val !== null && !Array.isArray(val)) {
        result[key] = await this.resolveInlineTable(
          val as AST.InlineTable,
        );
      } else if (typeof val === "string" && val.startsWith("$")) {
        // Deferred variable resolution for $var or $var.field references
        result[key] = await this.resolveDeferredVar(val);
      } else if (Array.isArray(val)) {
        const resolved = [];
        for (const item of val) {
          if (typeof item === "string" && item.startsWith("$")) {
            resolved.push(await this.resolveDeferredVar(item));
          } else {
            resolved.push(item);
          }
        }
        result[key] = resolved;
      } else {
        result[key] = val;
      }
    }
    return result;
  }

  private async resolveDeferredVar(ref: string): Promise<unknown> {
    const match = ref.match(/^\$([a-zA-Z0-9_-]+)(?:\.(.+))?$/);
    if (!match) return ref;
    const [, id, pathStr] = match;
    const path = pathStr ? pathStr.split(".") : [];
    const varVal = this.vars.getRaw(id);
    if (!varVal) return ref;
    if (varVal.type === "string") return varVal.value;
    if (varVal.type === "integer") return varVal.value;
    if (varVal.type === "resource" || varVal.type === "proposal") {
      if (path.length === 0) return varVal.value.name;
      // Handle client keys locally
      if (varVal.value.partial.resource === "client-key") {
        const keyName = varVal.value.partial.id || varVal.value.name.replace("client-keys/", "");
        const ck = this.clientKeys.get(keyName);
        if (ck) {
          const keyObj: Record<string, unknown> = {
            name: varVal.value.name,
            public_key: ck.toPublicBytesHex(),
            algorithm: ck.algorithm,
          };
          let current: unknown = keyObj;
          for (const field of path) {
            if (current && typeof current === "object") {
              current = (current as Record<string, unknown>)[field];
            } else {
              return undefined;
            }
          }
          return current;
        }
      }
      // Fetch resource and navigate path
      const resource = await this.client.get(varVal.value.name);
      let current: unknown = resource;
      for (const field of path) {
        if (current && typeof current === "object") {
          current = (current as Record<string, unknown>)[field];
        } else {
          return undefined;
        }
      }
      return current;
    }
    return ref;
  }

  private async resolveVariable(v: AST.Variable): Promise<unknown> {
    const varVal = this.vars.getRaw(v.identifier);
    if (!varVal) {
      throw new CslError("NOT_FOUND", `Variable $${v.identifier} not bound`);
    }

    if (varVal.type === "error") {
      if (v.path.length > 0) {
        // Navigate error fields: status → code, message → message
        const errorObj: Record<string, string> = {
          status: varVal.value.code,
          message: varVal.value.message,
          code: varVal.value.code,
        };
        let current: unknown = errorObj;
        for (const field of v.path) {
          if (current && typeof current === "object") {
            current = (current as Record<string, unknown>)[field];
          } else {
            return undefined;
          }
        }
        return current;
      }
      throw new CslError("FAILED_PRECONDITION", `Variable $${v.identifier} contains an error: ${varVal.value.message}`);
    }

    // Handle client key references locally
    if ((varVal.type === "resource") && varVal.value.partial.resource === "client-key") {
      const keyName = varVal.value.partial.id || varVal.value.name.replace("client-keys/", "");
      const ck = this.clientKeys.get(keyName);
      if (ck) {
        if (v.path.length > 0) {
          const keyObj: Record<string, string> = {
            name: varVal.value.name,
            public_key: ck.toPublicBytesHex(),
            algorithm: ck.algorithm,
          };
          let current: unknown = keyObj;
          for (const field of v.path) {
            if (current && typeof current === "object") {
              current = (current as Record<string, unknown>)[field];
            } else {
              return undefined;
            }
          }
          return current;
        }
        return varVal.value.name;
      }
    }

    // If it's a resource reference with no field path, refetch from API
    if ((varVal.type === "resource" || varVal.type === "proposal") && v.path.length === 0) {
      try {
        const resource = await this.client.get(varVal.value.name);
        return resource;
      } catch {
        return varVal.value.name;
      }
    }

    // Navigate field path
    if (v.path.length > 0) {
      let current: unknown;
      if (varVal.type === "resource" || varVal.type === "proposal") {
        current = await this.client.get(varVal.value.name);
      } else if (varVal.type === "string") {
        try {
          current = JSON.parse(varVal.value);
        } catch {
          current = varVal.value;
        }
      } else {
        current = varVal.value;
      }

      for (const field of v.path) {
        if (current && typeof current === "object") {
          current = (current as Record<string, unknown>)[field];
        } else {
          return undefined;
        }
      }
      return current;
    }

    switch (varVal.type) {
      case "string":
        return varVal.value;
      case "integer":
        return varVal.value;
      case "resource":
      case "proposal":
        return varVal.value.name;
    }
  }

  private async evaluateFunction(fn: AST.FunctionCall): Promise<unknown> {
    const params = [];
    for (const p of fn.parameters) {
      params.push(await this.resolveValue(p));
    }

    switch (fn.name) {
      case "json":
        return JSON.stringify(params[0]);
      case "keccak256": {
        const input = hex.decode(String(params[0]));
        return hex.encode(keccak256Fn(input));
      }
      case "sha256": {
        const input = hex.decode(String(params[0]));
        return hex.encode(sha256Fn(input));
      }
      case "verify-signature": {
        const alg = String(params[0]);
        const sigHex = String(params[1]);
        const msgHex = String(params[2]);
        const pubkeyHex = String(params[3]);
        return this.verifySignature(alg, sigHex, msgHex, pubkeyHex);
      }
      case "resource": {
        const name = String(params[0]);
        return this.client.get(name);
      }
      case "typed-data": {
        // EIP-712 typed data encoding - simplified implementation
        // The Rust version uses the full TypedData struct with encode().
        // For now, we JSON-stringify the input and hash it, since full EIP-712
        // implementation is complex and rarely tested.
        const input = params[0];
        if (typeof input === "object" && input !== null) {
          return this.encodeTypedData(input as Record<string, unknown>);
        }
        throw new CslError("INVALID_ARGUMENT", "typed_data expects an object parameter");
      }
      case "parse-call": {
        const input = params[0];
        if (typeof input !== "object" || input === null) {
          throw new CslError("INVALID_ARGUMENT", "parse_call expects an object parameter");
        }
        return this.parseCallData(input as Record<string, unknown>);
      }
      default:
        throw new CslError("UNIMPLEMENTED", `Function ${fn.name} not implemented`);
    }
  }

  /**
   * Verify a cryptographic signature. Supports multiple algorithms:
   * - ed255: Ed25519
   * - solana:signMessage: Solana off-chain message signing (wraps ed255)
   * - k256-sha2: secp256k1 with SHA-256 (standard)
   * - k256-keccak: secp256k1 with Keccak256 (Ethereum)
   * - personal_sign: Ethereum personal_sign (wraps k256-keccak with prefix)
   */
  private verifySignature(alg: string, sigHex: string, msgHex: string, pubkeyHex: string): string {
    const sigBytes = hex.decode(sigHex);
    const msgBytes = hex.decode(msgHex);
    const pubkeyBytes = hex.decode(pubkeyHex);

    switch (alg) {
      case "ed255":
      case "ed25519": {
        const vk = VerifyingKey.fromPublicBytes("ed25519", pubkeyBytes);
        if (!vk) throw new CslError("INVALID_ARGUMENT", "not an ed255 public key");
        if (!vk.verify(msgBytes, sigBytes)) {
          throw new CslError("INVALID_ARGUMENT", "ed255 signature verification failed");
        }
        return "OK";
      }
      case "solana:signMessage": {
        // Solana off-chain: prepend 0xff + "solana offchain" + msg
        const prefixed = new Uint8Array(1 + 15 + msgBytes.length);
        prefixed[0] = 0xff;
        prefixed.set(new TextEncoder().encode("solana offchain"), 1);
        prefixed.set(msgBytes, 16);
        return this.verifySignature("ed255", sigHex, hex.encode(prefixed), pubkeyHex);
      }
      case "k256-sha2": {
        const vk = VerifyingKey.fromPublicBytes("ecdsa-k256-sha256", pubkeyBytes);
        if (!vk) throw new CslError("INVALID_ARGUMENT", "not a k256 public key");
        // k256-sha2: noble-curves k256 sign/verify already hashes with SHA-256
        // so VerifyingKey.verify (which does sha256 internally) works directly
        const trimmedSig = sigBytes.length === 65 ? sigBytes.slice(0, 64) : sigBytes;
        if (!vk.verify(msgBytes, trimmedSig)) {
          throw new CslError("INVALID_ARGUMENT", "k256-sha2 signature verification failed");
        }
        return "OK";
      }
      case "k256-keccak": {
        // k256-keccak: hash with keccak256, then verify with raw ECDSA
        // We need to use secp256k1 directly since our VerifyingKey always uses SHA-256
        const trimmedSig = sigBytes.length === 65 ? sigBytes.slice(0, 64) : sigBytes;
        try {
          const k256Sig = secp256k1.Signature.fromCompact(trimmedSig);
          const digest = keccak256Fn(msgBytes);
          const valid = secp256k1.verify(k256Sig as unknown as Uint8Array, digest, pubkeyBytes);
          if (!valid) throw new Error("verification failed");
          return "OK";
        } catch {
          throw new CslError("INVALID_ARGUMENT", "k256-keccak signature verification failed");
        }
      }
      case "personal_sign": {
        // Ethereum personal_sign: prepend \x19 + "Ethereum Signed Message:\n{len}" + msg
        const prefix = new TextEncoder().encode(`Ethereum Signed Message:\n${msgBytes.length}`);
        const prefixed = new Uint8Array(1 + prefix.length + msgBytes.length);
        prefixed[0] = 0x19;
        prefixed.set(prefix, 1);
        prefixed.set(msgBytes, 1 + prefix.length);
        return this.verifySignature("k256-keccak", sigHex, hex.encode(prefixed), pubkeyHex);
      }
      case "bls12-381-g2-blake2": {
        // Custom BLS verification: G2 keys, G1 signatures, Blake2b message hashing
        // Matches Rust: blake2b(msg, 64 bytes) -> scalar -> G1*scalar -> pairing check
        try {
          // Deserialize public key as G2 point (96 compressed or 192 uncompressed)
          const pkPoint = bls12_381.G2.Point.fromHex(pubkeyBytes);
          // Blake2b hash message to 64-byte scalar
          const scalarBytes = blake2b(msgBytes, { dkLen: 64 });
          // Convert to scalar in Fr field (little-endian, matching Rust's Scalar::from_bytes_wide)
          const reversed = Uint8Array.from(scalarBytes).reverse();
          const scalar = bls12_381.fields.Fr.create(
            BigInt("0x" + hex.encode(reversed))
          );
          // Multiply G1 generator by scalar to get message point
          const msgPoint = bls12_381.G1.Point.BASE.multiply(scalar);
          // Deserialize signature as G1 point (48 compressed or 96 uncompressed)
          const sigPoint = bls12_381.G1.Point.fromHex(sigBytes);
          // Verify: e(sig, G2_gen) == e(msg_point, pk)
          const g2Gen = bls12_381.G2.Point.BASE;
          const lhs = bls12_381.pairing(sigPoint, g2Gen);
          const rhs = bls12_381.pairing(msgPoint, pkPoint);
          const valid = bls12_381.fields.Fp12.eql(lhs, rhs);
          if (!valid) throw new Error("verification failed");
          return "OK";
        } catch (e) {
          if (e instanceof CslError) throw e;
          throw new CslError("INVALID_ARGUMENT", `bls12-381-g2-blake2 signature verification failed: ${e}`);
        }
      }
      default:
        throw new CslError("UNIMPLEMENTED", `Signature verification not supported for algorithm: ${alg}`);
    }
  }

  /**
   * Parse call creation data to extract from/to addresses.
   * Matches Rust's call::CreateData::parse().
   */
  private parseCallData(data: Record<string, unknown>): Record<string, unknown> {
    const address = String(data.address || "");
    const method = String(data.method || "");
    const request = (data.request || {}) as Record<string, unknown>;

    // Extract chain from address: "chains/SOL/addresses/abc" -> "chains/SOL"
    const chainMatch = address.match(/^(chains\/[^/]+)\//);
    const chain = chainMatch ? chainMatch[1] : "";

    // Message-signing methods: return only from, no to
    if (["personal_sign", "eth_signTypedData_v4", "solana:signMessage"].includes(method)) {
      return { from: address };
    }

    // EVM transaction methods
    if (method === "eth_sendTransaction" || method === "eth_signTransaction") {
      const to = String(request.to || "");
      if (to) {
        const normalizedTo = to.toLowerCase();
        return { from: address, to: `${chain}/addresses/${normalizedTo}` };
      }
      return { from: address };
    }

    // SVM transaction methods - parse Solana serialized transaction
    if (method === "solana:signTransaction" || method === "solana:signAndSendTransaction") {
      const txHex = String(request.transaction || "");
      if (!txHex) return { from: address };

      try {
        const txBytes = hex.decode(txHex);
        const programIds = this.parseSolanaTransactionProgramIds(txBytes);
        if (programIds.length > 0) {
          const toAddrs = programIds.map(pid => `${chain}/addresses/${pid}`);
          // Set1 serialization: single value = string, multiple = array
          const to = toAddrs.length === 1 ? toAddrs[0] : toAddrs;
          return { from: address, to };
        }
      } catch {
        // If parsing fails, just return from
      }
      return { from: address };
    }

    return { from: address };
  }

  /**
   * Parse a serialized Solana transaction to extract unique program IDs.
   * Handles both legacy and v0 versioned transactions.
   */
  private parseSolanaTransactionProgramIds(txBytes: Uint8Array): string[] {
    let offset = 0;

    const readCompactU16 = (): number => {
      let val = 0;
      let shift = 0;
      for (let i = 0; i < 3; i++) {
        const b = txBytes[offset++];
        val |= (b & 0x7f) << shift;
        shift += 7;
        if ((b & 0x80) === 0) break;
      }
      return val;
    };

    // Read number of signatures
    const numSignatures = readCompactU16();
    // Skip signatures (each 64 bytes)
    offset += numSignatures * 64;

    // Check for versioned transaction (prefix byte)
    let isVersioned = false;
    if (offset < txBytes.length && (txBytes[offset] & 0x80) !== 0) {
      // Versioned transaction - version byte
      const version = txBytes[offset] & 0x7f;
      if (version === 0) {
        isVersioned = true;
        offset++;
      }
    }

    // Read message header
    const numRequiredSignatures = txBytes[offset++];
    const _numReadonlySignedAccounts = txBytes[offset++];
    const _numReadonlyUnsignedAccounts = txBytes[offset++];

    // Read static account keys
    const numAccountKeys = readCompactU16();
    const accountKeys: string[] = [];
    for (let i = 0; i < numAccountKeys; i++) {
      const key = txBytes.slice(offset, offset + 32);
      accountKeys.push(base58.encode(key));
      offset += 32;
    }

    // Skip recent blockhash (32 bytes)
    offset += 32;

    // Read instructions
    const numInstructions = readCompactU16();
    const programIds = new Set<string>();
    for (let i = 0; i < numInstructions; i++) {
      const programIdIndex = txBytes[offset++];
      if (programIdIndex < accountKeys.length) {
        programIds.add(accountKeys[programIdIndex]);
      }
      // Skip accounts
      const numAccounts = readCompactU16();
      offset += numAccounts;
      // Skip data
      const dataLen = readCompactU16();
      offset += dataLen;
    }

    // For versioned transactions, skip address lookup tables (we only need static keys)
    return Array.from(programIds);
  }

  /**
   * Encode EIP-712 typed data. Simplified implementation.
   */
  private encodeTypedData(data: Record<string, unknown>): string {
    const types = data.types as Record<string, Array<{ name: string; type: string }>>;
    const primaryType = data.primaryType as string;
    const domain = (data.domain ?? {}) as Record<string, unknown>;
    const message = (data.message ?? {}) as Record<string, unknown>;

    const keccakStr = (s: string): Uint8Array =>
      keccak256Fn(new TextEncoder().encode(s));
    const keccakBytes = (b: Uint8Array): Uint8Array => keccak256Fn(b);
    const concatBytes = (...arrays: Uint8Array[]): Uint8Array => {
      const total = arrays.reduce((sum, a) => sum + a.length, 0);
      const result = new Uint8Array(total);
      let off = 0;
      for (const arr of arrays) { result.set(arr, off); off += arr.length; }
      return result;
    };
    const parseHexStr = (s: string): Uint8Array => {
      const cleaned = s.startsWith("0x") ? s.slice(2) : s;
      return hex.decode(cleaned);
    };
    const leftPad32 = (bytes: Uint8Array): Uint8Array => {
      const r = new Uint8Array(32);
      r.set(bytes, 32 - bytes.length);
      return r;
    };
    const rightPad32 = (bytes: Uint8Array): Uint8Array => {
      const r = new Uint8Array(32);
      r.set(bytes, 0);
      return r;
    };
    const encodeUint256 = (value: string | number | bigint): Uint8Array => {
      let n = typeof value === "bigint" ? value : BigInt(String(value));
      const r = new Uint8Array(32);
      for (let i = 31; i >= 0 && n > 0n; i--) { r[i] = Number(n & 0xFFn); n >>= 8n; }
      return r;
    };

    // Build encodeType string for a type with sorted dependencies
    const encodeType = (typeName: string): string => {
      const visited = new Set<string>();
      const collectDeps = (name: string) => {
        if (visited.has(name)) return;
        const fields = types[name];
        if (!fields) return;
        visited.add(name);
        for (const field of fields) {
          collectDeps(field.type.replace(/\[.*\]$/, ""));
        }
      };
      collectDeps(typeName);
      const buildStr = (name: string): string => {
        const fields = types[name];
        if (!fields) return "";
        return `${name}(${fields.map(f => `${f.type} ${f.name}`).join(",")})`;
      };
      const deps = Array.from(visited).filter(n => n !== typeName).sort().map(buildStr).filter(Boolean);
      return buildStr(typeName) + deps.join("");
    };

    const computeTypeHash = (typeName: string): Uint8Array => keccakStr(encodeType(typeName));

    // Encode a single value as a 32-byte EIP-712 data word
    const eip712DataWord = (typeName: string, value: unknown): Uint8Array => {
      if (typeName.endsWith("]")) {
        const baseType = typeName.substring(0, typeName.lastIndexOf("["));
        const arr = value as unknown[];
        return keccakBytes(concatBytes(...arr.map(item => eip712DataWord(baseType, item))));
      }
      const rootType = typeName.replace(/\[.*\]$/, "");
      if (rootType in types && rootType !== "EIP712Domain") {
        const obj = value as Record<string, unknown>;
        const fields = types[rootType];
        return keccakBytes(concatBytes(computeTypeHash(rootType), ...fields.map(f => eip712DataWord(f.type, obj[f.name]))));
      }
      if (typeName === "string") return keccakStr(String(value ?? ""));
      if (typeName === "bytes") return keccakBytes(parseHexStr(String(value)));
      if (typeName === "address") return leftPad32(parseHexStr(String(value)));
      if (typeName === "bool") { const r = new Uint8Array(32); r[31] = value ? 1 : 0; return r; }
      if (typeName.startsWith("uint") || typeName.startsWith("int")) return encodeUint256(value as string | number | bigint);
      if (typeName.startsWith("bytes")) return rightPad32(parseHexStr(String(value)));
      return encodeUint256(value as string | number | bigint);
    };

    // Compute domain separator using only fields present in the domain object
    const domainFieldDefs = [
      { solType: "string", name: "name" },
      { solType: "string", name: "version" },
      { solType: "uint256", name: "chainId" },
      { solType: "address", name: "verifyingContract" },
      { solType: "bytes32", name: "salt" },
    ];
    const presentDomainFields = domainFieldDefs.filter(f => domain[f.name] !== undefined && domain[f.name] !== null);
    const domainEncodeType = "EIP712Domain(" + presentDomainFields.map(f => `${f.solType} ${f.name}`).join(",") + ")";
    const domainSeparator = keccakBytes(concatBytes(
      keccakStr(domainEncodeType),
      ...presentDomainFields.map(f => eip712DataWord(f.solType, domain[f.name])),
    ));

    const prefix = new Uint8Array([0x19, 0x01]);
    if (primaryType === "EIP712Domain") {
      return hex.encode(concatBytes(prefix, domainSeparator));
    }

    const fields = types[primaryType] || [];
    const hashStruct = keccakBytes(concatBytes(
      computeTypeHash(primaryType),
      ...fields.map(f => eip712DataWord(f.type, message[f.name])),
    ));

    return hex.encode(concatBytes(prefix, domainSeparator, hashStruct));
  }

  private resolvePartialOrVariable(
    pov: AST.PartialOrVariable,
  ): { resourceName: string; partial: AST.Partial; proposed: boolean } {
    if (pov.type === "name") {
      const p = pov.value;
      // Resolve variable references in IDs
      const resolvedId = this.resolveStringRef(p.id);
      const resolvedParentId = this.resolveStringRef(p.parentId);
      // Client keys are local-only, don't go through singularToPlural
      if (p.resource === "client-key") {
        const name = `client-keys/${resolvedId || ""}`;
        return { resourceName: name, partial: { ...p, id: resolvedId, parentId: resolvedParentId }, proposed: false };
      }
      // singularToPlural imported at top
      const plural = singularToPlural(p.resource);
      let name: string;
      if (resolvedParentId) {
        const parentMeta = getResourceTypeBySingular(p.resource);
        name = `${parentMeta.nested}/${resolvedParentId}/${plural}/${resolvedId}`;
      } else if (resolvedId) {
        name = `${plural}/${resolvedId}`;
      } else if (p.resource === "treasury") {
        // Bare "treasury" expands to "treasuries/{current treasury ID}"
        name = `${plural}/${this.client.treasuryId}`;
      } else {
        name = plural;
      }
      return { resourceName: name, partial: { ...p, id: resolvedId, parentId: resolvedParentId }, proposed: false };
    }

    // Variable reference
    const varName = pov.value;
    const resource = this.vars.getResource(varName);
    if (resource) return { resourceName: resource.name, partial: resource.partial, proposed: false };

    const proposal = this.vars.getProposal(varName);
    if (proposal) return { resourceName: proposal.name, partial: proposal.partial, proposed: true };

    throw new CslError("NOT_FOUND", `Variable $${varName} not bound to a resource`);
  }

  /**
   * Execute a single command.
   */
  async execute(command: AST.Command): Promise<void> {
    try {
      await this.executeInner(command);
    } catch (e) {
      if (this.config.allowFail) {
        if (e instanceof CslError) {
          this.log(`[allow.fail] ${e.code}: ${e.message}`);
        } else if (e instanceof Error) {
          this.log(`[allow.fail] ERROR: ${e.message}`);
        }
        return;
      }
      throw e;
    }
  }

  private async executeInner(command: AST.Command): Promise<void> {
    switch (command.type) {
      case "nop":
        return;

      case "exit":
        throw new CslError("EXIT", "exit");

      case "value": {
        // If it's a variable that references a resource, do a get
        if (command.value.type === "variable") {
          const v = command.value.value;
          const resource = this.vars.getResource(v.identifier);
          if (resource && resource.partial.resource !== "client-key") {
            if (v.path.length === 0) {
              const result = await this.client.get(resource.name);
              this.log(JSON.stringify(result, null, 2));
              return;
            }
            // Fall through to resolveValue for path navigation
          }
        }
        const val = await this.resolveValue(command.value);
        if (val !== undefined && val !== null) {
          this.log(typeof val === "string" ? val : JSON.stringify(val, null, 2));
        }
        return;
      }

      case "read":
        await this.executeRead(command.value);
        return;

      case "create":
        await this.executeCreate(command.value, false);
        return;

      case "propose":
        await this.executeCreate(command.value, false);
        return;

      case "update":
        await this.executeUpdate(command.value, false);
        return;

      case "delete":
        await this.executeDelete(command.value, false);
        return;

      case "custom":
        await this.executeCustom(command.value, false);
        return;

      case "approve": {
        const { resourceName } = this.resolvePartialOrVariable(command.value);
        await this.client.approve(resourceName);
        return;
      }

      case "cancel": {
        const { resourceName } = this.resolvePartialOrVariable(command.value);
        await this.client.cancel(resourceName);
        return;
      }

      case "submit": {
        const { resourceName } = this.resolvePartialOrVariable(command.value);
        // Submit a proposal - custom action
        await this.client.custom(resourceName, "submit");
        return;
      }

      case "assert":
        await this.executeAssert(command.value);
        return;

      case "for-loop":
        await this.executeForLoop(command.value);
        return;

      case "until":
        await this.executeUntil(command.value);
        return;

      case "assignment":
        await this.executeAssignment(command.value);
        return;

      case "fallible-assignment":
        await this.executeFallibleAssignment(command.value);
        return;

      case "set-setting":
        await this.executeSetting(command.value);
        return;

      case "get-setting":
        await this.executeGetSetting(command.value);
        return;

      case "unset":
        await this.executeUnset(command.value);
        return;

      case "convert": {
        const cv = command.value as AST.Convert;
        const data = String(await this.resolveValue(cv.data));
        const from = String(await this.resolveValue(cv.sourceEncoding));
        const to = String(await this.resolveValue(cv.targetEncoding));
        const result = this.convertEncoding(data, from, to);
        this.log(result);
        return;
      }
      case "replace": {
        const rv = command.value as AST.Replace;
        const data = String(await this.resolveValue(rv.data));
        const old = String(await this.resolveValue(rv.old));
        const newVal = String(await this.resolveValue(rv.new));
        this.log(this.hexReplace(data, old, newVal));
        return;
      }
    }
  }

  private async executeRead(read: AST.Read): Promise<void> {
    if (read.type === "get") {
      // For variable references, check if it's a non-resource variable first
      if (read.value.pov.type === "variable") {
        const varVal = this.vars.getRaw(read.value.pov.value);
        if (!varVal) {
          // Variable not bound - just return silently (Rust VM does the same)
          return;
        }
        if (varVal.type === "error") {
          // Display error details
          this.log(JSON.stringify({ status: varVal.value.code, message: varVal.value.message }, null, 2));
          return;
        }
        if (varVal.type === "string") {
          this.log(varVal.value);
          return;
        }
        if (varVal.type === "integer") {
          this.log(varVal.value.toString());
          return;
        }
      }
      const { resourceName, partial } = this.resolvePartialOrVariable(read.value.pov);
      // Client keys are local-only, don't send to API
      if (partial.resource === "client-key") {
        const keyName = partial.id || resourceName.replace("client-keys/", "");
        const ck = this.clientKeys.get(keyName);
        if (ck) {
          this.log(JSON.stringify({ name: resourceName, public_key: ck.toPublicBytesHex(), algorithm: ck.algorithm }, null, 2));
        }
        return;
      }
      const result = await this.client.get(resourceName);
      this.log(JSON.stringify(result, null, 2));
    } else {
      const list = read.value;
      // singularToPlural imported at top
      let parentId: string | undefined;
      if (list.parent) {
        if (list.parent.type === "variable") {
          const resource = this.vars.getResource(list.parent.value);
          if (resource) {
            // Extract the ID from the resource name
            const parts = resource.name.split("/");
            parentId = parts[parts.length - 1];
          }
        } else {
          parentId = list.parent.value;
        }
      }

      const result = await this.client.list(list.resource, {
        parentId,
        filter: list.filter,
        deleted: this.config.listDeleted,
      });
      this.log(JSON.stringify(result, null, 2));
    }
  }

  /**
   * Resolve a string that may contain a $variable reference to its actual value.
   */
  private resolveStringRef(s: string | undefined): string | undefined {
    if (!s) return s;
    if (s.startsWith("$")) {
      const varName = s.slice(1);
      const varVal = this.vars.getRaw(varName);
      if (varVal) {
        switch (varVal.type) {
          case "string": return varVal.value;
          case "integer": return varVal.value.toString();
          case "resource":
          case "proposal": {
            // For resource refs, extract the ID from the name
            const parts = varVal.value.name.split("/");
            return parts[parts.length - 1] || varVal.value.name;
          }
        }
      }
    }
    return s;
  }

  /**
   * Replace hex bytes: decode all three args as hex, find first occurrence of old in data,
   * replace with new, return hex-encoded result. Matches Rust VM behavior.
   */
  private hexReplace(dataHex: string, oldHex: string, newHex: string): string {
    if (!oldHex) throw new CslError("INVALID_ARGUMENT", "replace: old cannot be empty");
    const dataBytes = hex.decode(dataHex);
    const oldBytes = hex.decode(oldHex);
    const newBytes = hex.decode(newHex);

    // Find first occurrence
    for (let i = 0; i <= dataBytes.length - oldBytes.length; i++) {
      let match = true;
      for (let j = 0; j < oldBytes.length; j++) {
        if (dataBytes[i + j] !== oldBytes[j]) { match = false; break; }
      }
      if (match) {
        const result = new Uint8Array(dataBytes.length - oldBytes.length + newBytes.length);
        result.set(dataBytes.subarray(0, i), 0);
        result.set(newBytes, i);
        result.set(dataBytes.subarray(i + oldBytes.length), i + newBytes.length);
        return hex.encode(result);
      }
    }
    throw new CslError("INVALID_ARGUMENT", "replace: old not found in data");
  }

  /**
   * For rule types with WhoFilter (access-rule, transfer-rule, staking-rule, call-rule),
   * the Rust VM's typed deserialization automatically prefixes bare IDs in `initiate`
   * and `approve` fields with "users/" (via Name1::from_str → Name1::Display).
   * We replicate that here since we send raw JSON.
   */
  private convertEncoding(data: string, from: string, to: string): string {
    // Decode to bytes
    let bytes: Uint8Array;
    switch (from) {
      case "hex":
        bytes = hex.decode(data.startsWith("0x") ? data.slice(2) : data);
        break;
      case "base64":
        bytes = Uint8Array.from(atob(data), c => c.charCodeAt(0));
        break;
      case "base58":
        bytes = base58.decode(data);
        break;
      case "utf8":
      case "utf-8":
        bytes = new TextEncoder().encode(data);
        break;
      default:
        throw new CslError("INVALID_ARGUMENT", `Unknown source encoding: ${from}`);
    }
    // Encode to target
    switch (to) {
      case "hex":
        return hex.encode(bytes);
      case "base64":
        return btoa(String.fromCharCode(...bytes));
      case "base58":
        return base58.encode(bytes);
      case "utf8":
      case "utf-8":
        return new TextDecoder().decode(bytes);
      default:
        throw new CslError("INVALID_ARGUMENT", `Unknown target encoding: ${to}`);
    }
  }

  private prefixUserFilterFields(data: Record<string, unknown>): void {
    for (const field of ["initiate", "approve"]) {
      const val = data[field];
      if (typeof val === "string" && val !== "" && !val.includes("/")) {
        data[field] = `users/${val}`;
      }
    }
  }

  /**
   * Expand bare resource name shorthand in data fields.
   * The Rust VM's typed deserialization (Name2::from_str with single_id_allowed)
   * automatically expands e.g. "SOL" to "chains/SOL/assets/SOL" for asset fields,
   * "random" to "accounts/random" for account fields, etc.
   * We replicate that here since we send raw JSON.
   */
  private expandResourceNames(resourceType: string, data: Record<string, unknown>): void {
    // account field: bare id -> "accounts/{id}"
    const account = data.account;
    if (typeof account === "string" && account !== "" && !account.includes("/")) {
      data.account = `accounts/${account}`;
    }
    // asset field: bare id like "SOL" -> "chains/SOL/assets/SOL"
    const asset = data.asset;
    if (typeof asset === "string" && asset !== "" && !asset.includes("/")) {
      data.asset = `chains/${asset}/assets/${asset}`;
    }
    // symbol field: bare id -> expand to full symbol name
    const symbol = data.symbol;
    if (typeof symbol === "string" && symbol !== "" && !symbol.includes("/")) {
      data.symbol = `chains/${symbol}/symbols/${symbol}`;
    }
  }

  private async executeCreate(
    create: AST.Create,
    direct: boolean,
  ): Promise<ResourceRef> {
    const data = (await this.resolveInlineTable(create.data)) as Record<string, unknown>;
    if (create.variant) {
      data.variant = create.variant;
    }

    // For rule types with WhoFilter, bare IDs in initiate/approve need "users/" prefix
    // (Rust VM's typed deserialization does this via Name1::from_str → Name1::Display)
    this.prefixUserFilterFields(data);

    // Expand bare resource name shorthand (e.g. "SOL" -> "chains/SOL/assets/SOL")
    this.expandResourceNames(create.partial.resource, data);

    // Resolve variable references in IDs
    const resolvedId = this.resolveStringRef(create.partial.id);
    const resolvedParentId = this.resolveStringRef(create.partial.parentId);

    // Handle client key creation locally
    if (create.partial.resource === "client-key") {
      return this.createClientKey({ ...create, partial: { ...create.partial, id: resolvedId, parentId: resolvedParentId } });
    }

    const opName = await this.client.create(create.partial.resource, data, {
      id: resolvedId,
      parentId: resolvedParentId,
    });

    if (direct) {
      return {
        name: opName,
        partial: { resource: "operation", id: opName.replace("operations/", "") },
      };
    }

    // Wait for operation
    const op = await this.client.waitForOperation(opName);
    const state = (op.state as string) || "";
    if (state === "failed") {
      const error = (op as Record<string, unknown>)?.error as Record<string, unknown>;
      throw new CslError(
        opErrorStatus(error || {}),
        (error?.message as string) || "Operation failed",
      );
    }

    const response = (op as Record<string, unknown>)?.response as Record<string, unknown>;
    const resourceName = (response?.name as string) || opName;

    // Parse the resource name to get partial
    const parts = resourceName.split("/");
    const partial: AST.Partial = { resource: create.partial.resource };
    if (parts.length >= 2) {
      partial.id = parts[parts.length - 1];
    }

    return { name: resourceName, partial };
  }

  private variantToAlgorithm(variant?: string): NormalAlgorithm {
    switch (variant) {
      case "k256":
      case "ecdsa-k256-sha256":
        return "ecdsa-k256-sha256";
      case "p256":
      case "ecdsa-p256-sha256":
        return "ecdsa-p256-sha256";
      case "ed255":
      case "ed25519":
        return "ed25519";
      default:
        return "ecdsa-k256-sha256"; // default
    }
  }

  private createClientKey(create: AST.Create): ResourceRef {
    const name = create.partial.id || "";
    const data = create.data;
    const code = data.code as string;

    let key: SigningKey | null = null;
    if (create.variant === "invite" || code) {
      // Import from invite code
      if (code) {
        key = SK.fromInvite(code);
        if (!key) {
          const alg = this.variantToAlgorithm(create.variant);
          key = SK.fromSecretBytesHex(alg, code);
          if (!key) throw new CslError("Invalid Argument", `Invalid key material: ${code}`);
        }
      } else {
        // invite variant without code - generate ed25519
        key = SK.generate("ed25519");
      }
    } else {
      const alg = this.variantToAlgorithm(create.variant);
      key = SK.generate(alg);
    }

    this.clientKeys.set(name, key!);

    // Store in keyring
    try {
      this.keyring.store(name, key, true);
    } catch {
      // May fail if directory doesn't exist etc.
    }

    return {
      name: `client-keys/${name}`,
      partial: { resource: "client-key", id: name },
    };
  }

  private async executeUpdate(
    update: AST.Update,
    direct: boolean,
  ): Promise<ResourceRef> {
    const { resourceName, partial } = this.resolvePartialOrVariable(update.what);
    const changes = (await this.resolveInlineTable(update.data)) as Record<string, unknown>;

    // Fetch current resource and merge changes (like Rust CSL VM)
    const current = await this.client.get(resourceName);
    for (const [key, value] of Object.entries(changes)) {
      current[key] = value;
    }

    const opName = await this.client.update(resourceName, current);

    if (direct) {
      return {
        name: opName,
        partial: { resource: "operation", id: opName.replace("operations/", "") },
      };
    }

    const op = await this.client.waitForOperation(opName);
    const state = (op.state as string) || "";
    if (state === "failed") {
      const error = (op as Record<string, unknown>)?.error as Record<string, unknown>;
      throw new CslError(
        opErrorStatus(error || {}),
        (error?.message as string) || "Operation failed",
      );
    }

    return { name: resourceName, partial };
  }

  private async executeDelete(
    del: AST.Delete,
    direct: boolean,
  ): Promise<ResourceRef> {
    const { resourceName, partial } = this.resolvePartialOrVariable(del.pov);

    // Handle client key deletion locally
    if (partial.resource === "client-key") {
      const keyName = partial.id || resourceName.replace("client-keys/", "");
      this.clientKeys.delete(keyName);
      this.keyring.delete(keyName);
      return { name: resourceName, partial };
    }

    const opName = await this.client.delete(resourceName);

    if (direct) {
      return {
        name: opName,
        partial: { resource: "operation", id: opName.replace("operations/", "") },
      };
    }

    const op = await this.client.waitForOperation(opName);
    const state = (op.state as string) || "";
    if (state === "failed") {
      const error = (op as Record<string, unknown>)?.error as Record<string, unknown>;
      throw new CslError(
        opErrorStatus(error || {}),
        (error?.message as string) || "Operation failed",
      );
    }

    return { name: resourceName, partial };
  }

  private async executeCustom(
    custom: AST.Custom,
    direct: boolean,
  ): Promise<ResourceRef> {
    const { resourceName, partial } = this.resolvePartialOrVariable(custom.what);
    let body: string | undefined;
    if (custom.payload) {
      if (custom.payload.type === "object") {
        const data = (await this.resolveInlineTable(custom.payload.value)) as Record<string, unknown>;
        body = JSON.stringify(data);
      } else {
        // String payload: send as a JSON string value
        body = JSON.stringify(custom.payload.value);
      }
    }

    const opName = await this.client.custom(resourceName, custom.action, body);

    if (direct) {
      return {
        name: opName,
        partial: { resource: "operation", id: opName.replace("operations/", "") },
      };
    }

    const op = await this.client.waitForOperation(opName);
    const state = (op.state as string) || "";
    if (state === "failed") {
      const error = (op as Record<string, unknown>)?.error as Record<string, unknown>;
      throw new CslError(
        opErrorStatus(error || {}),
        (error?.message as string) || "Operation failed",
      );
    }

    return { name: resourceName, partial };
  }

  private async executeAssert(assert: AST.Assert): Promise<void> {
    const condNode = parseCondition(assert.condition);
    const varNames = extractVariables(condNode);
    const values = await this.evaluateVars(varNames);

    if (!evaluateCondition(condNode, values)) {
      throw new CslError(
        "FAILED_PRECONDITION",
        `Assertion failed: ${assert.condition}`,
      );
    }
  }

  private async executeForLoopBody(forLoop: AST.ForLoop): Promise<void> {
    // If we have multi-line body text, split and execute each command
    if (forLoop.bodyText) {
      const commands = splitCommands(forLoop.bodyText);
      if (commands.length > 1) {
        for (const cmdStr of commands) {
          if (!cmdStr.trim()) continue;
          const command = parseCommand(cmdStr, this.vars);
          await this.execute(command);
        }
        return;
      }
    }
    await this.execute(forLoop.command);
  }

  private async executeForLoop(forLoop: AST.ForLoop): Promise<void> {
    const previousValue = this.vars.getRaw(forLoop.variable);

    try {
      if (forLoop.iterable.type === "array") {
        for (const item of forLoop.iterable.value) {
          const resolved = await this.resolveValue(item);
          if (typeof resolved === "string") {
            this.vars.set(forLoop.variable, { type: "string", value: resolved });
          } else if (typeof resolved === "number") {
            this.vars.set(forLoop.variable, { type: "integer", value: resolved });
          }
          await this.executeForLoopBody(forLoop);
        }
      } else {
        // List iterable
        const list = forLoop.iterable.value;
        let parentId: string | undefined;
        if (list.parent) {
          if (list.parent.type === "variable") {
            const resource = this.vars.getResource(list.parent.value);
            if (resource) {
              const parts = resource.name.split("/");
              parentId = parts[parts.length - 1];
            }
          } else {
            parentId = list.parent.value;
          }
        }

        const result = await this.client.list(list.resource, {
          parentId,
          filter: list.filter,
          deleted: this.config.listDeleted,
        });

        // Find the array of resources in the result
        // singularToPlural imported at top
        const plural = singularToPlural(list.resource);
        const resources = (result as Record<string, unknown>)[plural] as Array<Record<string, unknown>> || [];

        for (const resource of resources) {
          const name = resource.name as string;
          if (name) {
            const parts = name.split("/");
            this.vars.set(forLoop.variable, {
              type: "resource",
              value: {
                name,
                partial: { resource: list.resource, id: parts[parts.length - 1] },
              },
            });
            await this.executeForLoopBody(forLoop);
          }
        }
      }
    } finally {
      // Restore previous variable value
      if (previousValue) {
        this.vars.set(forLoop.variable, previousValue);
      } else {
        this.vars.remove(forLoop.variable);
      }
    }
  }

  private async executeUntil(until: AST.Until): Promise<void> {
    const condNode = parseCondition(until.condition);
    const varNames = extractVariables(condNode);
    const timeout = (this.config.untilTimeout || 120) * 1000;
    const start = Date.now();

    while (true) {
      const values = await this.evaluateVars(varNames);
      if (evaluateCondition(condNode, values)) {
        return;
      }

      if (Date.now() - start > timeout) {
        throw new CslError(
          "DEADLINE_EXCEEDED",
          `until condition not met within ${this.config.untilTimeout || 120}s: ${until.condition}`,
        );
      }

      await sleep(100);
    }
  }

  private async executeAssignment(assign: AST.Assignment): Promise<void> {
    const { variable, value, direct } = assign;

    switch (value.type) {
      case "create": {
        const ref = await this.executeCreate(value.value, direct);
        this.vars.set(variable, { type: "resource", value: ref });
        return;
      }
      case "propose": {
        const ref = await this.executeCreate(value.value, direct);
        this.vars.set(variable, { type: "proposal", value: ref });
        return;
      }
      case "update": {
        const ref = await this.executeUpdate(value.value, direct);
        this.vars.set(variable, { type: "resource", value: ref });
        return;
      }
      case "delete": {
        const ref = await this.executeDelete(value.value, direct);
        this.vars.set(variable, { type: "resource", value: ref });
        return;
      }
      case "approve": {
        const { resourceName, partial } = this.resolvePartialOrVariable(value.value);
        const opName = await this.client.approve(resourceName);
        if (direct) {
          // Direct: store the operation itself
          this.vars.set(variable, {
            type: "resource",
            value: { name: opName, partial: { resource: "operation", id: opName.replace("operations/", "") } },
          });
        } else {
          // Wait and store the resulting resource
          const op = await this.client.waitForOperation(opName);
          const response = op.response as Record<string, unknown> | undefined;
          const respName = response?.name as string;
          if (respName) {
            const parts = respName.split("/");
            this.vars.set(variable, {
              type: "resource",
              value: { name: respName, partial: { resource: partial.resource, id: parts[parts.length - 1] } },
            });
          } else {
            this.vars.set(variable, {
              type: "resource",
              value: { name: opName, partial: { resource: "operation", id: opName.replace("operations/", "") } },
            });
          }
        }
        return;
      }
      case "cancel": {
        const { resourceName, partial } = this.resolvePartialOrVariable(value.value);
        const opName = await this.client.cancel(resourceName);
        this.vars.set(variable, {
          type: "resource",
          value: { name: opName, partial: { resource: "operation", id: opName.replace("operations/", "") } },
        });
        return;
      }
      case "submit": {
        const { resourceName } = this.resolvePartialOrVariable(value.value);
        await this.client.custom(resourceName, "submit");
        return;
      }
      case "get": {
        const { resourceName, partial } = this.resolvePartialOrVariable(value.value.pov);
        this.vars.set(variable, {
          type: "resource",
          value: { name: resourceName, partial },
        });
        return;
      }
      case "partial": {
        const p = value.value;
        if (p.resource === "client-key") {
          // Client keys are local-only
          const name = `client-keys/${p.id || ""}`;
          this.vars.set(variable, {
            type: "resource",
            value: { name, partial: p },
          });
          return;
        }
        // singularToPlural imported at top
        const plural = singularToPlural(p.resource);
        let name: string;
        if (p.parentId) {
          const parentMeta = getResourceTypeBySingular(p.resource);
          name = `${parentMeta.nested}/${p.parentId}/${plural}/${p.id}`;
        } else if (p.id) {
          name = `${plural}/${p.id}`;
        } else if (p.resource === "treasury") {
          // Bare "treasury" expands to "treasuries/{current treasury ID}"
          name = `${plural}/${this.client.treasuryId}`;
        } else {
          name = plural;
        }
        this.vars.set(variable, {
          type: "resource",
          value: { name, partial: p },
        });
        return;
      }
      case "string": {
        const resolved = await this.resolveValue(value.value);
        if (typeof resolved === "string") {
          this.vars.set(variable, { type: "string", value: resolved });
        } else if (typeof resolved === "number") {
          this.vars.set(variable, { type: "integer", value: resolved });
        } else {
          this.vars.set(variable, {
            type: "string",
            value: typeof resolved === "object" ? JSON.stringify(resolved) : String(resolved),
          });
        }
        return;
      }
      case "setting": {
        const settingResult = this.getSettingValue(value.value);
        if (settingResult) {
          this.vars.set(variable, { type: "string", value: settingResult });
        }
        return;
      }
      case "function": {
        // Special handling for resource() - store as resource variable
        if (value.value.name === "resource") {
          const params = [];
          for (const p of value.value.parameters) {
            params.push(await this.resolveValue(p));
          }
          let name = String(params[0]);
          // Strip JSON quotes if present
          try { name = JSON.parse(name); } catch { /* use as-is */ }
          // Parse resource name into partial
          const parts = name.split("/");
          const partial: AST.Partial = { resource: "unknown" };
          if (parts.length >= 2) {
            const pluralType = parts[parts.length - 2];
            const resourceType = pluralToSingular(pluralType);
            partial.resource = resourceType;
            partial.id = parts[parts.length - 1];
            if (parts.length >= 4) {
              partial.parentId = parts[parts.length - 3];
            }
          }
          this.vars.set(variable, {
            type: "resource",
            value: { name, partial },
          });
          return;
        }
        const result = await this.evaluateFunction(value.value);
        if (typeof result === "string") {
          this.vars.set(variable, { type: "string", value: result });
        } else {
          this.vars.set(variable, {
            type: "string",
            value: typeof result === "object" ? JSON.stringify(result) : String(result),
          });
        }
        return;
      }
      case "custom": {
        const ref = await this.executeCustom(value.value, direct);
        this.vars.set(variable, { type: "resource", value: ref });
        return;
      }
      case "convert": {
        const data = String(await this.resolveValue(value.value.data));
        const from = String(await this.resolveValue(value.value.sourceEncoding));
        const to = String(await this.resolveValue(value.value.targetEncoding));
        const converted = this.convertEncoding(data, from, to);
        this.vars.set(variable, { type: "string", value: converted });
        return;
      }
      case "replace": {
        const data = String(await this.resolveValue(value.value.data));
        const old = String(await this.resolveValue(value.value.old));
        const newVal = String(await this.resolveValue(value.value.new));
        this.vars.set(variable, { type: "string", value: this.hexReplace(data, old, newVal) });
        return;
      }
    }
  }

  private async executeFallibleAssignment(
    assign: AST.FallibleAssignment,
  ): Promise<void> {
    const { variable, error: errorVar, mutate, direct } = assign;

    try {
      let ref: ResourceRef;
      switch (mutate.type) {
        case "create":
          ref = await this.executeCreate(mutate.value, direct);
          break;
        case "propose":
          ref = await this.executeCreate(mutate.value, direct);
          break;
        case "update":
          ref = await this.executeUpdate(mutate.value, direct);
          break;
        case "delete":
          ref = await this.executeDelete(mutate.value, direct);
          break;
        case "approve": {
          const { resourceName, partial } = this.resolvePartialOrVariable(mutate.value);
          const approveOpName = await this.client.approve(resourceName);
          if (direct) {
            ref = { name: approveOpName, partial: { resource: "operation", id: approveOpName.replace("operations/", "") } };
          } else {
            const approveOp = await this.client.waitForOperation(approveOpName);
            const approveResp = approveOp.response as Record<string, unknown> | undefined;
            const approveRespName = approveResp?.name as string;
            ref = approveRespName
              ? { name: approveRespName, partial: { ...partial, id: approveRespName.split("/").pop()! } }
              : { name: approveOpName, partial: { resource: "operation", id: approveOpName.replace("operations/", "") } };
          }
          break;
        }
        case "cancel": {
          const { resourceName } = this.resolvePartialOrVariable(mutate.value);
          const cancelOpName = await this.client.cancel(resourceName);
          ref = { name: cancelOpName, partial: { resource: "operation", id: cancelOpName.replace("operations/", "") } };
          break;
        }
        case "submit": {
          const { resourceName, partial } = this.resolvePartialOrVariable(mutate.value);
          await this.client.custom(resourceName, "submit");
          ref = { name: resourceName, partial };
          break;
        }
        case "custom":
          ref = await this.executeCustom(mutate.value, direct);
          break;
        case "get": {
          const { resourceName, partial } = this.resolvePartialOrVariable(mutate.value.pov);
          await this.client.get(resourceName);
          ref = { name: resourceName, partial };
          break;
        }
      }

      this.vars.set(variable, { type: "resource", value: ref });
      this.vars.remove(errorVar);
    } catch (e) {
      let err: CslError;
      if (e instanceof CslError) {
        err = e;
      } else if (e instanceof TreasuryApiError) {
        err = new CslError(e.status, e.message);
      } else {
        err = new CslError("Unknown", e instanceof Error ? e.message : String(e));
      }
      this.vars.set(errorVar, { type: "error", value: err });
      this.vars.remove(variable);
    }
  }

  private async executeSetting(setting: AST.Setting): Promise<void> {
    const key = setting.key.join(".");
    switch (key) {
      case "sign.with": {
        const keyName = setting.value;
        const signingKey = this.clientKeys.get(keyName) || this.keyring.getSigningKey(keyName);
        if (signingKey) {
          this.client.setSigningKey(signingKey);
        } else {
          throw new CslError("NOT_FOUND", `Signing key '${keyName}' not found in keyring`);
        }
        return;
      }
      case "until.timeout":
        this.config.untilTimeout = parseInt(setting.value, 10);
        return;
      case "allow.fail":
        this.config.allowFail = setting.value === "true" || setting.value === "1";
        return;
      case "display.strip":
        this.config.displayStrip = setting.value === "true" || setting.value === "1";
        return;
      case "list.deleted":
        this.config.listDeleted = setting.value === "true" || setting.value === "1";
        return;
      case "api.base":
        // Would need to reconstruct client - skip for now
        return;
      case "treasury.id":
      case "host.id":
      case "treasury":
        // Configuration - handled elsewhere
        return;
      default:
        this.log(`Unknown setting: ${key}`);
        return;
    }
  }

  private async executeGetSetting(settingKey: AST.SettingKey): Promise<void> {
    const value = this.getSettingValue(settingKey);
    if (value) {
      this.log(value);
    }
  }

  private getSettingValue(settingKey: AST.SettingKey): string | null {
    const key = settingKey.key.join(".");
    switch (key) {
      case "host.id":
        return this.client.hostId || null;
      case "treasury.id":
        return this.client.treasuryId;
      default:
        return null;
    }
  }

  private async executeUnset(settingKey: AST.SettingKey): Promise<void> {
    const key = settingKey.key.join(".");
    switch (key) {
      case "allow.fail":
        this.config.allowFail = false;
        return;
      case "sign.with":
        // Revert to default key
        return;
      default:
        return;
    }
  }

  private async evaluateVars(varNames: string[]): Promise<Map<string, EvalValue>> {
    const values = new Map<string, EvalValue>();

    for (const name of varNames) {
      const varVal = this.vars.getRaw(name);
      if (!varVal) {
        values.set(name, { type: "null" });
        continue;
      }

      switch (varVal.type) {
        case "string":
          values.set(name, { type: "string", value: varVal.value });
          break;
        case "integer":
          values.set(name, { type: "integer", value: varVal.value });
          break;
        case "error":
          // Store error as JSON so field paths like $err.status work
          values.set(name, {
            type: "string",
            value: JSON.stringify({
              status: varVal.value.code,
              message: varVal.value.message,
              code: varVal.value.code,
            }),
          });
          break;
        case "resource":
        case "proposal": {
          // Refetch the resource to get current state
          try {
            const resource = await this.client.get(varVal.value.name);
            values.set(name, {
              type: "string",
              value: JSON.stringify(resource),
            });
          } catch {
            values.set(name, { type: "null" });
          }
          break;
        }
      }
    }

    return values;
  }

  /**
   * Run a CSL script (multiple commands).
   */
  async runScript(script: string): Promise<void> {
    const commands = splitCommands(script);

    for (const cmdStr of commands) {
      if (!cmdStr.trim()) continue;

      // Skip shebang
      if (cmdStr.startsWith("#!")) continue;

      const command = parseCommand(cmdStr, this.vars);
      try {
        await this.execute(command);
      } catch (e) {
        if (e instanceof CslError && e.code === "EXIT") {
          return;
        }
        throw e;
      }
    }
  }

  /**
   * Run a CSL script from a file.
   */
  async runFile(filePath: string): Promise<void> {
    const fs = await import("node:fs");
    const content = fs.readFileSync(filePath, "utf-8");
    await this.runScript(content);
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
