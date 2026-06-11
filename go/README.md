# Cordial Treasury Go SDK

Go client SDK for the Cordial Treasury API.

The examples below use the hosted Treasury API at `https://treasury.cordialapis.com/`.

## Install

```sh
go get github.com/cordialsys/sdk-go
```

## Create An Address

```go
package main

import (
	"fmt"
	"log"

	"github.com/cordialsys/sdk-go/treasury/client"
	"github.com/cordialsys/sdk-go/treasury/types"
)

func main() {
	treasuryID := "<your-treasury-id>"
	apiKey := "<your-api-key>"

	c, err := client.NewClient(treasuryID, client.WithAPIKey(apiKey))
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

## Sign A Message

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

## Local Development

```sh
go test ./...
```
