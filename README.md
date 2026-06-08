# Cordial Treasury SDKs

Client SDKs for the Cordial Treasury API.

This repository currently contains:

- Go SDK: `github.com/cordialsys/sdk-go`
- TypeScript SDK: `@cordialsys/treasury-sdk`

The examples below use the base Treasury API at `http://127.0.0.1:8777`.

## Go

### Install

```sh
go get github.com/cordialsys/sdk-go
```

### Create An Address

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cordialsys/sdk-go/treasury/client"
	"github.com/cordialsys/sdk-go/treasury/types"
)

func main() {
	ctx := context.Background()
	baseURL := "http://127.0.0.1:8777"

	treasuryID, err := client.LookupTreasuryID(ctx, baseURL, "")
	if err != nil {
		log.Fatal(err)
	}

	c, err := client.NewClient(treasuryID, client.WithBaseURL(baseURL))
	if err != nil {
		log.Fatal(err)
	}

	keyring, err := client.NewKeyring(treasuryID)
	if err != nil {
		log.Fatal(err)
	}
	rootKey, err := keyring.GetKey("root-key")
	if err != nil {
		log.Fatal(err)
	}
	c.SetSigner(rootKey)

	op, err := c.CreateAddress("SOL", "", client.CreateAddressRequest{
		Variant: types.AddressVariantInternal,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(op)
}
```

### Sign A Message

```go
message := types.Hex("68656c6c6f") // "hello"
key := types.KeyName("keys/123")

op, err := c.CreateSignature("", client.CreateSignatureRequest{
	SignatureData: types.SignatureData{
		Key:     &key,
		Message: &message,
	},
})
if err != nil {
	log.Fatal(err)
}

fmt.Println(op)
```

### Local Development

```sh
cd go
go test ./...
```

## TypeScript

### Install

```sh
npm install @cordialsys/treasury-sdk
```

For repository development:

```sh
cd typescript
pnpm install
pnpm build
pnpm test
```

### Create An Address

```ts
import { Keyring, TreasuryClient, lookupTreasuryId } from "@cordialsys/treasury-sdk";

const baseUrl = "http://127.0.0.1:8777";
const treasuryId = await lookupTreasuryId(baseUrl);

const keyring = new Keyring(treasuryId);
const signingKey = keyring.getSigningKey("root-key");
if (!signingKey) {
  throw new Error("root-key not found");
}

const client = new TreasuryClient({
  baseUrl,
  treasuryId,
  signingKey,
});

const operation = await client.createResource(
  "address",
  { variant: "internal" },
  { parentId: "SOL" },
);

console.log(operation);
```

### Sign A Message

```ts
const operation = await client.createResource("signature", {
  key: "keys/123",
  message: "68656c6c6f", // "hello"
});

console.log(operation);
```

## OpenAPI Types

The SDKs are generated from `openapi/treasury.yaml` plus hand-written client helpers.

Regenerate TypeScript resource types with:

```sh
cd typescript
pnpm generate:types
```
