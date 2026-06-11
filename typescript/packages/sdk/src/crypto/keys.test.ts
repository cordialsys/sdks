import { describe, it } from "node:test";
import * as assert from "node:assert/strict";
import { SigningKey, VerifyingKey } from "./keys.js";

describe("SigningKey", () => {
  describe("generate", () => {
    it("generates ed25519 key", () => {
      const key = SigningKey.generate("ed25519");
      assert.equal(key.algorithm, "ed25519");
      assert.equal(key.toPublicBytes().length, 32);
      assert.equal(key.toSecretBytes().length, 32);
    });

    it("generates k256 key", () => {
      const key = SigningKey.generate("ecdsa-k256-sha256");
      assert.equal(key.algorithm, "ecdsa-k256-sha256");
      assert.equal(key.toPublicBytes().length, 33); // compressed
      assert.equal(key.toSecretBytes().length, 32);
    });

    it("generates p256 key", () => {
      const key = SigningKey.generate("ecdsa-p256-sha256");
      assert.equal(key.algorithm, "ecdsa-p256-sha256");
      assert.equal(key.toPublicBytes().length, 33); // compressed
      assert.equal(key.toSecretBytes().length, 32);
    });
  });

  describe("fromInvite", () => {
    it("creates key from short invite code", () => {
      const key = SigningKey.fromInvite("1");
      assert.ok(key);
      assert.equal(key.algorithm, "ed25519");
      // "1" padded to 64 chars → 0000...0001
      const secretHex = key.toSecretBytesHex();
      assert.equal(secretHex, "0000000000000000000000000000000000000000000000000000000000000001");
    });

    it("creates key from full hex invite code", () => {
      const key = SigningKey.fromInvite("abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890");
      assert.ok(key);
      assert.equal(key.toSecretBytesHex(), "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890");
    });

    it("creates key from string invite like 'root'", () => {
      // "root" gets hex-encoded first: 726f6f74, then padded to 64 chars
      const key = SigningKey.fromInvite("root");
      assert.ok(key);
      assert.equal(key.algorithm, "ed25519");
    });
  });

  describe("sign and verify", () => {
    it("ed25519 sign and verify round-trip", () => {
      const key = SigningKey.generate("ed25519");
      const message = new TextEncoder().encode("hello world");
      const signature = key.sign(message);
      assert.equal(signature.length, 64);
      assert.ok(key.verifyingKey().verify(message, signature));
    });

    it("k256 sign and verify round-trip", () => {
      const key = SigningKey.generate("ecdsa-k256-sha256");
      const message = new TextEncoder().encode("hello world");
      const signature = key.sign(message);
      assert.equal(signature.length, 64);
      assert.ok(key.verifyingKey().verify(message, signature));
    });

    it("p256 sign and verify round-trip", () => {
      const key = SigningKey.generate("ecdsa-p256-sha256");
      const message = new TextEncoder().encode("hello world");
      const signature = key.sign(message);
      assert.equal(signature.length, 64);
      assert.ok(key.verifyingKey().verify(message, signature));
    });

    it("rejects tampered message", () => {
      const key = SigningKey.generate("ed25519");
      const message = new TextEncoder().encode("hello world");
      const signature = key.sign(message);
      const tampered = new TextEncoder().encode("hello world!");
      assert.ok(!key.verifyingKey().verify(tampered, signature));
    });
  });

  describe("serialization", () => {
    it("round-trips through JSON", () => {
      const key = SigningKey.generate("ed25519");
      const json = key.toJSON();
      const restored = SigningKey.fromJSON(json);
      assert.ok(restored);
      assert.equal(restored.toSecretBytesHex(), key.toSecretBytesHex());
      assert.equal(restored.toPublicBytesHex(), key.toPublicBytesHex());
    });

    it("round-trips through hex", () => {
      const key = SigningKey.generate("ecdsa-k256-sha256");
      const hexStr = key.toSecretBytesHex();
      const restored = SigningKey.fromSecretBytesHex("ecdsa-k256-sha256", hexStr);
      assert.ok(restored);
      assert.equal(restored.toPublicBytesHex(), key.toPublicBytesHex());
    });
  });
});

describe("VerifyingKey", () => {
  it("round-trips through hex", () => {
    const key = SigningKey.generate("ed25519");
    const vk = key.verifyingKey();
    const hexStr = vk.toPublicBytesHex();
    const restored = VerifyingKey.fromPublicBytesHex("ed25519", hexStr);
    assert.ok(restored);
    assert.equal(restored.toPublicBytesHex(), hexStr);
  });

  it("rejects invalid public key", () => {
    const vk = VerifyingKey.fromPublicBytesHex("ed25519", "deadbeef");
    assert.equal(vk, null);
  });
});
