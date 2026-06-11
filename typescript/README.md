# Cordial Treasury TypeScript SDK

TypeScript client SDK for the Cordial Treasury API.

The examples below use the hosted Treasury API at `https://treasury.cordialapis.com/`.

## Install

```sh
npm install @cordialsys/treasury-sdk
```

For repository development:

```sh
pnpm install
pnpm build
pnpm test
```

## Create An Address

```ts
import { Keyring, TreasuryClient } from "@cordialsys/treasury-sdk";

const treasuryId = "<your-treasury-id>";
const apiKey = "<your-api-key>";

const keyring = new Keyring(treasuryId);
const signingKey = keyring.getSigningKey("root-key");
if (!signingKey) {
  throw new Error("root-key not found");
}

const client = new TreasuryClient({
  treasuryId,
  apiKey,
  signingKey,
});

const operation = await client.createResource(
  "address",
  { variant: "internal" },
  { parentId: "SOL" },
);

console.log(operation);
```

## Sign A Message

```ts
const operation = await client.createResource("signature", {
  key: "keys/123",
  message: "68656c6c6f", // "hello"
});

console.log(operation);
```

## OpenAPI Types

The TypeScript SDK uses generated resource types from `../openapi/treasury.yaml` plus hand-written client helpers.

Regenerate TypeScript resource types with:

```sh
pnpm generate:types
```
