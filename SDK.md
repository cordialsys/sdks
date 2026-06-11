
TARGET LANGUAGE is _______

I want you to create/maintain/update a create a client for our Treasury API/product for the target language.

  * make a branch in git@github.com:cordialsys/sdk.git to work out of (empty repo to start)
  * use Treasury git@github.com:cordialsys/treasury.git to test against and to use the existing rust client as a reference.
  * Download our openapi doc from https://api.stoplight.io/projects/cHJqOjIzOTcxNw/branches/main/export/reference/treasury.yaml
  * Evaluate a good tool to generate types from openapi-doc to use, and to re-use the existing docs in the openapi doc.

First you should start by familiarizing yourself with the existing SDK, development environment, and tests.
The SDK will be a library product for others to consume in their programs, and you will work by
using it as a library for your own CLI based in this language that replicates the rust treasury CLI.
The SDK should be designed to be verbose and very usable by other people not familiar with Treasury.

You will test it by running it on the existing CSL tests in ./treasury/api/tests and confirming they all pass.
It will probably make sense to work resource by resource incrementally rather than try all at once.

Part of this work will be re-implementing the CSL interpret in the target language just so you can actually run the existing tests.
being able to run this .csl tests is important so we can have a reliable way to test the library.

  goals:
  * run a "1/1" treasury instance using the tilt/docker-compose dev environment
  * look at treasury/README.md for how to setup the dev environment, using the justfile recipes.
    * Use soft-reset when you need to reset, so you keep the cache
  * install all of the dependencies you need along the way and update the paths as needed (docker, go, rust, melange, apko, git-semver, etc)
  * study existing rust CLI + tests
  * run tests against the dev env, using the CSL interpreter implemented in the target language
  * develop a complete SDK library + CLI replicating the rust CLI
  * Use generated types from the open-api doc to avoid re-defining types (dont need to use generated clients, just the resource related types).
  * Retain comments from open-api so that documentation trickles through.

SDK requirements:
* Do not fallback to using typless `any` objects.
* The CSL interpreter should use a clear & concise method for every kind of function.
Example:

```csl
create shared account { fee_payer = "allow" }
```

The CSL interpreter should use something like:

```pseudocode
Enum AccountVariant = "shared" | "internal" | "external" | "contract" | "validator"
Enum FeePayerPolicy = "allow" | "require" | "deny"
CreateAccount(variant = "shared", fee_payer = "allow")
```

Where `variant` and `fee_payer` are correctly typed to enums, not plain strings.

Do NOT fallback to something like:

```pseudocode
Do("POST", "/accounts", {"variant": "shared", "fee_payer": "allow"})
```

It's okay to use a generic "Do" function internally, but the goal is to have more
helpful functions on the SDK, and we force that by requiring the CSL interpreter to use those functions directly.

* Client constructor should take following arguments:
  * Optional base URL for the treasury API.
    * Defaults to `https://treasury.cordialapis.com/`
  * required treasury ID, which should be used as `Treasury: <treasury-id>` header for all requests.
  * Helper function(s) to aid looking up the treasury ID automatically from the base URL.
  * An optional API key, which should be used as `Authorization: Bearer <api-key>` header for all requests.  Should be base64 encoded if it's not already.
    * If no API key is provided, and the default base URL is used, an error should be returned.
  * Optional generic signer implementation, to be used for signing requests.

* Updates (PUT) should be read-modify-write (no PATCH semantics).

After starting work, do not stop until completion, unless you are blocked on something important.
