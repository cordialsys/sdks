# List all recipes
list:
    just --list

# Fetch all latest OpenAPI specifications
fetch-all: (fetch "admin") (fetch "auth") (fetch "connector") (fetch "oracle") (fetch "treasury")

# Fetch latest OpenAPI specification for an API
fetch api:
    curl -sSO --output-dir openapi https://api.stoplight.io/projects/$(just api-key {{ api }})/branches/main/export/reference/{{ api }}.yaml

[private]
fmt-just:
    just --fmt --unstable

[private]
api-key api:
    #!/usr/bin/env bash
    declare -A -r api_keys=(
        ["admin"]="cHJqOjIzOTcxNQ"
        ["auth"]="cHJqOjIzOTk5MQ"
        ["connector"]="cHJqOjIzOTcxOA"
        ["oracle"]="cHJqOjIzOTcxOQ"
        ["treasury"]="cHJqOjIzOTcxNw"
    )
    echo ${api_keys[{{ api }}]}
