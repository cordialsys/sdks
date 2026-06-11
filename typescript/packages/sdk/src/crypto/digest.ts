/**
 * Cryptographic digest functions.
 */
import { sha256 as nobleSha256 } from "@noble/hashes/sha256";
import { sha512 as nobleSha512 } from "@noble/hashes/sha512";
import { keccak_256 } from "@noble/hashes/sha3";

export function sha256(data: Uint8Array): Uint8Array {
  return nobleSha256(data);
}

export function sha512(data: Uint8Array): Uint8Array {
  return nobleSha512(data);
}

export function keccak256(data: Uint8Array): Uint8Array {
  return keccak_256(data);
}
