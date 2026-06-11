use cordial_treasury::client::{ClientBuilder, normalize_api_key};
use cordial_treasury::csl::{Command, parse_line};
use cordial_treasury::{AccountVariant, Keyring, ResourceType, SigningAlgorithm};
use httpmock::Method::{GET, POST, PUT};
use httpmock::MockServer;
use serde_json::json;

#[test]
fn normalizes_raw_api_keys() {
    assert_eq!(normalize_api_key("secret"), "c2VjcmV0");
    assert_eq!(normalize_api_key("c2VjcmV0"), "c2VjcmV0");
}

#[test]
fn parses_create_with_typed_variant_and_fields() {
    let cmd = parse_line(
        1,
        r#"create shared account named acct-1 { fee_payer = "allow", threshold = 2 }"#,
    )
    .expect("parse");
    let Command::Create(create) = cmd else {
        panic!("expected create command");
    };
    assert_eq!(create.resource_type, ResourceType::Account);
    assert_eq!(create.variant.as_deref(), Some("shared"));
    assert_eq!(create.id.as_deref(), Some("acct-1"));
    assert_eq!(create.fields["fee_payer"], json!("allow"));
    assert_eq!(create.fields["threshold"], json!(2));
    assert_eq!(
        create.variant.unwrap().parse::<AccountVariant>().unwrap(),
        AccountVariant::Shared
    );
}

#[test]
fn keyring_roundtrips_ed25519_keys() {
    let dir = tempfile::tempdir().unwrap();
    let keyring = Keyring::from_dir(dir.path());
    let created = keyring
        .generate_key("admin", SigningAlgorithm::Ed25519)
        .unwrap();
    let loaded = keyring.get_key("admin").unwrap();
    assert_eq!(created.public_key_hex, loaded.public_key_hex);
    assert_eq!(keyring.list_keys().unwrap(), vec!["admin"]);
}

#[test]
fn create_account_sends_treasury_and_authorization_headers() {
    let server = MockServer::start();
    let mock = server.mock(|when, then| {
        when.method(POST)
            .path("/v1/accounts/acct-1")
            .header("Treasury", "t1")
            .header("Authorization", "Bearer c2VjcmV0")
            .json_body(json!({
                "variant": "shared",
                "fee_payer": "allow"
            }));
        then.status(200)
            .header("content-type", "application/json")
            .json_body(json!({ "name": "operations/op-1" }));
    });

    let client = ClientBuilder::new("t1")
        .base_url(server.base_url())
        .api_key("secret")
        .build()
        .unwrap();

    let mut data = serde_json::Map::new();
    data.insert("fee_payer".to_string(), json!("allow"));
    let op = client
        .create_account(
            Some("acct-1"),
            &cordial_treasury::CreateAccountRequest {
                variant: AccountVariant::Shared,
                metadata: Default::default(),
                data,
            },
        )
        .unwrap();

    mock.assert();
    assert_eq!(op, "operations/op-1");
}

#[test]
fn update_is_read_modify_write() {
    let server = MockServer::start();
    let get = server.mock(|when, then| {
        when.method(GET).path("/v1/accounts/acct-1");
        then.status(200)
            .header("content-type", "application/json")
            .json_body(json!({
                "name": "accounts/acct-1",
                "variant": "shared",
                "fee_payer": "allow"
            }));
    });
    let put = server.mock(|when, then| {
        when.method(PUT)
            .path("/v1/accounts/acct-1")
            .json_body(json!({
                "name": "accounts/acct-1",
                "variant": "shared",
                "fee_payer": "deny"
            }));
        then.status(200)
            .header("content-type", "application/json")
            .json_body(json!({ "name": "operations/op-2" }));
    });

    let client = ClientBuilder::new("t1")
        .base_url(server.base_url())
        .build()
        .unwrap();
    let op = client
        .update("accounts/acct-1", &json!({ "fee_payer": "deny" }))
        .unwrap();

    get.assert();
    put.assert();
    assert_eq!(op, "operations/op-2");
}

#[test]
fn update_replaces_nested_objects_instead_of_patch_merging() {
    let server = MockServer::start();
    let get = server.mock(|when, then| {
        when.method(GET).path("/v1/accounts/acct-1");
        then.status(200)
            .header("content-type", "application/json")
            .json_body(json!({
                "name": "accounts/acct-1",
                "variant": "shared",
                "metadata": {
                    "keep": "old",
                    "replace": "old"
                },
                "fee_payer": "allow"
            }));
    });
    let put = server.mock(|when, then| {
        when.method(PUT)
            .path("/v1/accounts/acct-1")
            .json_body(json!({
                "name": "accounts/acct-1",
                "variant": "shared",
                "metadata": {
                    "replace": "new"
                },
                "fee_payer": "allow"
            }));
        then.status(200)
            .header("content-type", "application/json")
            .json_body(json!({ "name": "operations/op-3" }));
    });

    let client = ClientBuilder::new("t1")
        .base_url(server.base_url())
        .build()
        .unwrap();
    let op = client
        .update(
            "accounts/acct-1",
            &json!({ "metadata": { "replace": "new" } }),
        )
        .unwrap();

    get.assert();
    put.assert();
    assert_eq!(op, "operations/op-3");
}
