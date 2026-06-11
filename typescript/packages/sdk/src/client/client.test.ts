import { describe, it, afterEach } from "node:test";
import * as assert from "node:assert/strict";
import { TreasuryClient, lookupTreasuryId, normalizeApiKey } from "./index.js";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("normalizeApiKey", () => {
  it("base64 encodes plain API keys", () => {
    assert.equal(normalizeApiKey("secret-token"), "c2VjcmV0LXRva2Vu");
  });

  it("keeps already-base64 API keys", () => {
    assert.equal(normalizeApiKey("c2VjcmV0LXRva2Vu"), "c2VjcmV0LXRva2Vu");
  });
});

describe("TreasuryClient constructor", () => {
  it("requires an API key for the default production URL", () => {
    assert.throws(
      () => new TreasuryClient({ treasuryId: "treasury-1" }),
      /apiKey is required/,
    );
  });

  it("allows unauthenticated local development clients", () => {
    const client = new TreasuryClient({
      baseUrl: "http://127.0.0.1:8777",
      treasuryId: "treasury-1",
    });
    assert.equal(client.baseUrl, "http://127.0.0.1:8777");
    assert.equal(client.treasuryId, "treasury-1");
  });
});

describe("TreasuryClient requests", () => {
  it("sends treasury and bearer headers", async () => {
    let observedHeaders: Headers;
    globalThis.fetch = (async (_url, init) => {
      observedHeaders = new Headers(init?.headers);
      return jsonResponse({ name: "accounts/main" });
    }) as typeof fetch;

    const client = new TreasuryClient({
      baseUrl: "https://treasury.cordialapis.com",
      treasuryId: "treasury-1",
      apiKey: "secret-token",
    });

    await client.getAccount("main");

    assert.equal(observedHeaders!.get("Treasury"), "treasury-1");
    assert.equal(observedHeaders!.get("Authorization"), "Bearer c2VjcmV0LXRva2Vu");
  });

  it("performs read-modify-write updates", async () => {
    const calls: Array<{ method?: string; url: string; body?: unknown }> = [];
    globalThis.fetch = (async (url, init) => {
      calls.push({ method: init?.method, url: String(url), body: init?.body });
      if (init?.method === "GET") {
        return jsonResponse({
          name: "accounts/main",
          variant: "internal",
          description: "old",
          labels: { env: "dev" },
        });
      }
      return jsonResponse("operations/op-1");
    }) as typeof fetch;

    const client = new TreasuryClient({
      baseUrl: "http://127.0.0.1:8777",
      treasuryId: "treasury-1",
    });

    const op = await client.updateAccount("main", { description: "new" });

    assert.equal(op, "operations/op-1");
    assert.equal(calls.length, 2);
    assert.equal(calls[0].method, "GET");
    assert.equal(calls[1].method, "PUT");
    assert.deepEqual(JSON.parse(String(calls[1].body)), {
      name: "accounts/main",
      variant: "internal",
      description: "new",
      labels: { env: "dev" },
    });
  });

  it("supports typed generic resource helpers", async () => {
    const calls: Array<{ method?: string; url: string; body?: unknown }> = [];
    globalThis.fetch = (async (url, init) => {
      calls.push({ method: init?.method, url: String(url), body: init?.body });
      return jsonResponse("operations/op-2");
    }) as typeof fetch;

    const client = new TreasuryClient({
      baseUrl: "http://127.0.0.1:8777",
      treasuryId: "treasury-1",
    });

    const op = await client.createResource("account", {
      variant: "internal",
      description: "main account",
    });

    assert.equal(op, "operations/op-2");
    assert.equal(calls[0].method, "POST");
    assert.equal(new URL(calls[0].url).pathname, "/v1/accounts");
    assert.deepEqual(JSON.parse(String(calls[0].body)), {
      variant: "internal",
      description: "main account",
    });
  });
});

describe("lookupTreasuryId", () => {
  it("extracts the id from the treasury singleton name", async () => {
    globalThis.fetch = (async () => jsonResponse({ name: "treasuries/install-1" })) as typeof fetch;

    assert.equal(
      await lookupTreasuryId("http://127.0.0.1:8777"),
      "install-1",
    );
  });
});

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}
