/**
 * Cryptographic algorithm types used for HTTP message signatures.
 */

export type NormalAlgorithm = "ed25519" | "ecdsa-k256-sha256" | "ecdsa-p256-sha256";

export type Algorithm =
  | NormalAlgorithm
  | "rsa-pss-sha512"
  | "rsa-v1_5-sha256"
  | "hmac-sha256"
  | "ecdsa-p384-sha384"
  | "web-authn"
  | "web-authn-uv"
  | "open-pubkey";

export function parseAlgorithm(s: string): Algorithm {
  switch (s) {
    case "rsa-pss-sha512":
    case "rsa-v1_5-sha256":
    case "hmac-sha256":
    case "ecdsa-p256-sha256":
    case "ecdsa-p384-sha384":
    case "ed25519":
    case "ecdsa-k256-sha256":
    case "web-authn":
    case "web-authn-uv":
    case "open-pubkey":
      return s;
    case "p256":
      return "ecdsa-p256-sha256";
    case "ed255":
      return "ed25519";
    case "k256":
    case "k256-sha2":
      return "ecdsa-k256-sha256";
    default:
      throw new Error(`Unknown signature algorithm: ${s}`);
  }
}

export function parseNormalAlgorithm(s: string): NormalAlgorithm {
  const alg = parseAlgorithm(s);
  if (alg === "ed25519" || alg === "ecdsa-k256-sha256" || alg === "ecdsa-p256-sha256") {
    return alg;
  }
  throw new Error(`Non-normal algorithm where one is expected: ${s}`);
}

export function algorithmToNormal(alg: Algorithm): NormalAlgorithm {
  if (alg === "ed25519" || alg === "ecdsa-k256-sha256" || alg === "ecdsa-p256-sha256") {
    return alg;
  }
  throw new Error(`Cannot convert ${alg} to NormalAlgorithm`);
}
