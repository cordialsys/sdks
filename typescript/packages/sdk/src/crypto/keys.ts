/**
 * Signing and verifying key implementations for Ed25519, K256, and P256.
 */
import { ed25519 } from "@noble/curves/ed25519";
import { secp256k1 } from "@noble/curves/secp256k1";
import { p256 } from "@noble/curves/p256";
import { randomBytes } from "@noble/hashes/utils";
import { hex } from "@scure/base";
import { sha256 } from "./digest.js";
import type { NormalAlgorithm } from "./algorithms.js";

export class SigningKey {
  private constructor(
    private readonly _algorithm: NormalAlgorithm,
    private readonly _secretBytes: Uint8Array,
  ) {}

  get algorithm(): NormalAlgorithm {
    return this._algorithm;
  }

  static generate(algorithm: NormalAlgorithm): SigningKey {
    switch (algorithm) {
      case "ed25519":
        return SigningKey.generateEd25519();
      case "ecdsa-k256-sha256":
        return SigningKey.generateK256();
      case "ecdsa-p256-sha256":
        return SigningKey.generateP256();
    }
  }

  static generateEd25519(): SigningKey {
    const secret = randomBytes(32);
    return new SigningKey("ed25519", secret);
  }

  static generateK256(): SigningKey {
    const secret = secp256k1.utils.randomPrivateKey();
    return new SigningKey("ecdsa-k256-sha256", secret);
  }

  static generateP256(): SigningKey {
    const secret = p256.utils.randomPrivateKey();
    return new SigningKey("ecdsa-p256-sha256", secret);
  }

  /**
   * Create a signing key from an invite code.
   * The invite code is hex-padded to 64 characters and used as an Ed25519 seed.
   */
  static fromInvite(invite: string): SigningKey | null {
    const decode = (inv: string): Uint8Array | null => {
      const padded = inv.padStart(64, "0");
      try {
        return hex.decode(padded);
      } catch {
        return null;
      }
    };

    let seed = decode(invite);
    if (!seed) {
      // Try hex-encoding the invite string first, then decode
      const hexEncoded = hex.encode(new TextEncoder().encode(invite));
      seed = decode(hexEncoded);
    }
    if (!seed || seed.length !== 32) {
      return null;
    }

    return new SigningKey("ed25519", seed);
  }

  static fromSecretBytes(algorithm: NormalAlgorithm, material: Uint8Array): SigningKey | null {
    try {
      // Validate the key material by attempting to derive the public key
      switch (algorithm) {
        case "ed25519":
          if (material.length !== 32) return null;
          ed25519.getPublicKey(material);
          break;
        case "ecdsa-k256-sha256":
          secp256k1.getPublicKey(material);
          break;
        case "ecdsa-p256-sha256":
          p256.getPublicKey(material);
          break;
      }
      return new SigningKey(algorithm, new Uint8Array(material));
    } catch {
      return null;
    }
  }

  static fromSecretBytesHex(algorithm: NormalAlgorithm, hexStr: string): SigningKey | null {
    try {
      const material = hex.decode(hexStr.trim());
      return SigningKey.fromSecretBytes(algorithm, material);
    } catch {
      return null;
    }
  }

  sign(message: Uint8Array): Uint8Array {
    switch (this._algorithm) {
      case "ed25519": {
        const sig = ed25519.sign(message, this._secretBytes);
        return sig;
      }
      case "ecdsa-k256-sha256": {
        // noble-curves treats input as raw hash; Rust k256 hashes with SHA-256 internally
        const msgHash = sha256(message);
        const sig = secp256k1.sign(msgHash, this._secretBytes);
        return new Uint8Array(sig.toCompactRawBytes());
      }
      case "ecdsa-p256-sha256": {
        const msgHash = sha256(message);
        const sig = p256.sign(msgHash, this._secretBytes);
        return new Uint8Array(sig.toCompactRawBytes());
      }
    }
  }

  verifyingKey(): VerifyingKey {
    return new VerifyingKey(this._algorithm, this.toPublicBytes());
  }

  toSecretBytes(): Uint8Array {
    return new Uint8Array(this._secretBytes);
  }

  toSecretBytesHex(): string {
    return hex.encode(this._secretBytes);
  }

  toSecretInvite(): string {
    return hex.encode(this._secretBytes);
  }

  toPublicBytes(): Uint8Array {
    switch (this._algorithm) {
      case "ed25519":
        return ed25519.getPublicKey(this._secretBytes);
      case "ecdsa-k256-sha256":
        // Compressed format (33 bytes, 0x02 or 0x03 prefix)
        return secp256k1.getPublicKey(this._secretBytes, true);
      case "ecdsa-p256-sha256":
        // Compressed format (33 bytes, 0x02 or 0x03 prefix)
        return p256.getPublicKey(this._secretBytes, true);
    }
  }

  toPublicBytesHex(): string {
    return hex.encode(this.toPublicBytes());
  }

  toJSON(): { algorithm: NormalAlgorithm; secret_key: string } {
    return {
      algorithm: this._algorithm,
      secret_key: this.toSecretBytesHex(),
    };
  }

  static fromJSON(json: { algorithm: NormalAlgorithm; secret_key: string }): SigningKey | null {
    return SigningKey.fromSecretBytesHex(json.algorithm, json.secret_key);
  }
}

export class VerifyingKey {
  constructor(
    private readonly _algorithm: NormalAlgorithm,
    private readonly _publicBytes: Uint8Array,
  ) {}

  get algorithm(): NormalAlgorithm {
    return this._algorithm;
  }

  verify(message: Uint8Array, signature: Uint8Array): boolean {
    try {
      switch (this._algorithm) {
        case "ed25519":
          return ed25519.verify(signature, message, this._publicBytes);
        case "ecdsa-k256-sha256": {
          const msgHash = sha256(message);
          const k256Sig = secp256k1.Signature.fromCompact(signature);
          return secp256k1.verify(k256Sig as unknown as Uint8Array, msgHash, this._publicBytes);
        }
        case "ecdsa-p256-sha256": {
          const msgHash = sha256(message);
          const p256Sig = p256.Signature.fromCompact(signature);
          return p256.verify(p256Sig as unknown as Uint8Array, msgHash, this._publicBytes);
        }
      }
    } catch {
      return false;
    }
  }

  toPublicBytes(): Uint8Array {
    return new Uint8Array(this._publicBytes);
  }

  toPublicBytesHex(): string {
    return hex.encode(this._publicBytes);
  }

  static fromPublicBytesHex(algorithm: NormalAlgorithm, hexStr: string): VerifyingKey | null {
    try {
      const bytes = hex.decode(hexStr.trim());
      return VerifyingKey.fromPublicBytes(algorithm, bytes);
    } catch {
      return null;
    }
  }

  static fromPublicBytes(algorithm: NormalAlgorithm, bytes: Uint8Array): VerifyingKey | null {
    try {
      // Validate by checking the point is on the curve
      switch (algorithm) {
        case "ed25519":
          if (bytes.length !== 32) return null;
          break;
        case "ecdsa-k256-sha256":
          secp256k1.ProjectivePoint.fromHex(bytes);
          break;
        case "ecdsa-p256-sha256":
          p256.ProjectivePoint.fromHex(bytes);
          break;
      }
      return new VerifyingKey(algorithm, new Uint8Array(bytes));
    } catch {
      return null;
    }
  }

  toJSON(): { algorithm: NormalAlgorithm; public_key: string } {
    return {
      algorithm: this._algorithm,
      public_key: this.toPublicBytesHex(),
    };
  }
}
