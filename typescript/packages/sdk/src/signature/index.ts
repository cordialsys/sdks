/**
 * HTTP Message Signatures (RFC 9421) implementation for Treasury API.
 */
import { sha256 } from "../crypto/digest.js";
import { base64 } from "@scure/base";
import { randomBytes } from "@noble/hashes/utils";
import type { SigningKey } from "../crypto/keys.js";
import type { NormalAlgorithm } from "../crypto/algorithms.js";

const encoder = new TextEncoder();

// Header names
export const CONTENT_DIGEST = "content-digest";
export const SIGNATURE_INPUT = "signature-input";
export const SIGNATURE = "signature";
export const TREASURY = "treasury";
export const TREASURY_HOST = "treasury-host";

export const SIGNATURE_NAME = "iam";

/**
 * Components signed for Treasury API requests.
 */
export function defaultComponents(hasHost: boolean): string[] {
  const components = [
    "@method",
    "@path",
    "@query",
    CONTENT_DIGEST,
    TREASURY,
  ];
  if (hasHost) {
    components.push(TREASURY_HOST);
  }
  return components;
}

/**
 * Generate a Content-Digest header value.
 * Format: sha-256=:<base64(sha256(body))>:
 */
export function contentDigest(body: Uint8Array): string {
  const digest = sha256(body);
  const encoded = base64.encode(digest);
  return `sha-256=:${encoded}:`;
}

/**
 * Generate a 12-digit random nonce (~40 bits entropy).
 */
export function generateNonce(): string {
  const min = 1_000_000_000_000;
  const max = 10_000_000_000_000;
  const buf = randomBytes(8);
  const view = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  const n = view.getBigUint64(0);
  const range = BigInt(max) - BigInt(min);
  const value = Number(BigInt(min) + (n % range));
  return value.toString();
}

/**
 * Format a component line for the signature base.
 */
function componentLine(
  name: string,
  method: string,
  path: string,
  query: string,
  headers: Record<string, string>,
): string {
  if (name.startsWith("@")) {
    switch (name) {
      case "@method":
        return `"@method": ${method}`;
      case "@path":
        return `"@path": ${path}`;
      case "@query":
        return `"@query": ?${query}`;
      case "@authority":
        return `"@authority": ${headers["host"] || ""}`;
      case "@scheme":
        return `"@scheme": ${headers["scheme"] || "https"}`;
      default:
        throw new Error(`Unknown derived component: ${name}`);
    }
  } else {
    const value = headers[name];
    if (value === undefined) {
      throw new Error(`Required header ${name} missing`);
    }
    return `${name}: ${value}`;
  }
}

export interface SignatureParams {
  algorithm: NormalAlgorithm;
  created: number;
  keyid: string;
  nonce: string;
  tag: string;
}

/**
 * Build the Signature-Input parameter string.
 * Format: ("@method" "@path" "@query" "content-digest" "treasury");alg="ed25519";created=N;keyid="K";nonce="N";tag="T"
 */
export function serializeSignatureParams(
  components: string[],
  params: SignatureParams,
): string {
  const componentList = components
    .map((c) => (c.startsWith("@") ? `"${c}"` : `"${c}"`))
    .join(" ");
  return (
    `(${componentList})` +
    `;alg="${params.algorithm}"` +
    `;created=${params.created}` +
    `;keyid="${params.keyid}"` +
    `;nonce="${params.nonce}"` +
    `;tag="${params.tag}"`
  );
}

/**
 * Build the signature base string that will be signed.
 */
export function buildSignatureBase(
  components: string[],
  params: SignatureParams,
  method: string,
  path: string,
  query: string,
  headers: Record<string, string>,
): string {
  const lines: string[] = [];
  for (const name of components) {
    lines.push(componentLine(name, method, path, query, headers));
  }

  const sigParamsStr = serializeSignatureParams(components, params);
  lines.push(`"@signature-params": ${sigParamsStr}`);

  // Lines joined with \n, trailing \n at the end
  return lines.join("\n") + "\n";
}

export interface SignedHeaders {
  "content-digest": string;
  "signature-input": string;
  signature: string;
}

/**
 * Sign an HTTP request for Treasury API.
 *
 * Returns the three headers that need to be added to the request:
 * content-digest, signature-input, signature.
 */
export function signRequest(opts: {
  signingKey: SigningKey;
  method: string;
  path: string;
  query: string;
  body: string;
  treasuryId: string;
  hostId?: string;
  tag?: string;
}): SignedHeaders {
  const bodyBytes = encoder.encode(opts.body);
  const digest = contentDigest(bodyBytes);

  const hasHost = !!opts.hostId;
  const components = defaultComponents(hasHost);

  const headers: Record<string, string> = {
    [CONTENT_DIGEST]: digest,
    [TREASURY]: opts.treasuryId,
  };
  if (opts.hostId) {
    headers[TREASURY_HOST] = opts.hostId;
  }

  const now = Math.floor(Date.now() / 1000);
  const nonce = generateNonce();
  const keyid = opts.signingKey.toPublicBytesHex();
  const tag = opts.tag ?? "";

  const params: SignatureParams = {
    algorithm: opts.signingKey.algorithm,
    created: now,
    keyid,
    nonce,
    tag,
  };

  const sigBase = buildSignatureBase(
    components,
    params,
    opts.method,
    opts.path,
    opts.query,
    headers,
  );

  const rawSignature = opts.signingKey.sign(encoder.encode(sigBase));
  const encodedSignature = base64.encode(rawSignature);

  const sigParamsStr = serializeSignatureParams(components, params);

  return {
    "content-digest": digest,
    "signature-input": `${SIGNATURE_NAME}=${sigParamsStr}`,
    signature: `${SIGNATURE_NAME}=:${encodedSignature}:`,
  };
}
