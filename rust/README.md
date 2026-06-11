# Cordial Treasury Rust SDK

Rust client SDK and CLI for the Cordial Treasury API.

## Install

```toml
[dependencies]
cordial-treasury = "0.1"
```

For repository development:

```sh
cd rust
cargo test
```

## Create An Account

```rust
use cordial_treasury::{
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
`cordial_treasury::types::openapi`. Hand-written request structs wrap the generated
wire values with ergonomic typed create helpers. The client keeps a raw JSON boundary
for GET/list/custom resources so newly added Treasury fields round-trip without SDK
releases.
