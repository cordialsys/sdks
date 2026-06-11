# Cordial Treasury Rust SDK

Basic Rust client SDK and CLI for the Cordial Treasury API.

This crate provides a typed client facade, keyring/signing helpers, and a
compact CSL runner for Treasury scripts.

Please note that the included `treasury-rs` CLI has less functionality than the full CLI
`treasury` available from the [download server][dl-server], and that Enterprise customers
have source code access to the internal, richer SDK.

[dl-server]: https://dl.cordial.systems/

## Install

```toml
[dependencies]
cordial-treasury-sdk = "0.1"
```

For repository development:

```sh
cd rust
just fmt lint test
```

## Create An Account

```rust,no_run
use treasury_sdk::{
    AccountVariant, Client, CreateAccountRequest, Metadata,
};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::builder("<your-treasury-id>")
        .api_key("<your-api-key>")
        .build()?;

    let op = client.create_account(
        Some("shared-account"),
        &CreateAccountRequest {
            variant: AccountVariant::Shared,
            metadata: Metadata::default(),
            data: Default::default(),
        },
    )?;

    println!("{op}");
    Ok(())
}
```

## CLI

```sh
cargo run --bin treasury-rs -- \
  --api http://localhost:8777 \
  --treasury <treasury-id> \
  list accounts
```

Run CSL scripts:

```sh
cargo run --bin treasury-rs -- \
  --api http://localhost:8777 \
  --treasury <treasury-id> \
  script ./test.csl
```

## OpenAPI Types

`build.rs` reads `../openapi/treasury.yaml` and emits documented enum types under
`treasury_sdk::types::openapi`. Hand-written request structs wrap the generated
wire values with ergonomic typed create helpers. The client keeps a raw JSON boundary
for GET/list/custom resources so newly added Treasury fields round-trip without SDK
releases.
