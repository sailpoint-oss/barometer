# Barometer

Barometer is a Go package, CLI, and GitHub Action for **contract testing** and validation of APIs using **OpenAPI** (3.0.x, 3.1.x, 3.2) and **Arazzo** workflow documents. It runs requests against a live API and validates responses against the spec (status, headers, body schema) and executes multi-step Arazzo workflows. Swagger/OAS 2.0 inputs are detected and rejected until dedicated runtime support is added.

## Features

- **OpenAPI contract tests**: Load a spec, hit every (or filtered) operation, validate response status and body schema.
- **Arazzo workflows**: Run multi-step API workflows with runtime expressions, success criteria, and outputs.
- **CLI**: `barometer openapi validate|test`, `barometer arazzo validate|run`, `barometer contract test`.
- **Output**: Human, JUnit XML, or versioned JSON with one shared report shape for CLI, action, and Go integrations.
- **Async API**: `barometer.Start(ctx, config)` returns a `Job` for IDE/LSP integration without blocking.

## Install

```bash
go install github.com/sailpoint-oss/barometer/cmd/barometer@latest

# Go library
go get github.com/sailpoint-oss/barometer
```

## Quick start

```bash
# Validate an OpenAPI 3.x spec
barometer openapi validate openapi.yaml

# Run contract tests against a base URL
barometer openapi test openapi.yaml https://api.example.com

# Validate an Arazzo document
barometer arazzo validate arazzo.yaml

# Run Arazzo workflows
barometer arazzo run arazzo.yaml https://api.example.com

# Unified run from config
barometer contract test --config barometer.yaml
```

## Config file

Create `barometer.yaml`:

```yaml
baseUrl: https://api.example.com
openapi:
  spec: ./openapi.yaml
  tags: [pet, store]
arazzo:
  doc: ./arazzo.yaml
  workflows: []   # empty = all
output: json      # human, junit, json
```

## GitHub Action

```yaml
- uses: sailpoint-oss/barometer/.github/actions/barometer@v1
  with:
    openapi-spec: './openapi.yaml'
    openapi-tags: 'pet,store'
    base-url: 'https://api.example.com'
    arazzo-doc: './arazzo.yaml'
    arazzo-workflows: 'syncWidgets'
    output-format: 'json'
```

When `config` is omitted, the action now builds one temporary contract config from the flat OpenAPI/Arazzo inputs and still executes `barometer contract test`, so report shaping stays consistent across OpenAPI-only and mixed runs.
The action exposes `result`, `report-path`, `report-json`, and `report-junit` outputs so downstream CI steps can consume the exact artifact they need.

## Go integration

Import the module root package for the stable public API:

```go
input := barometer.ContractInput{
    BaseURL:     "https://api.example.com",
    OpenAPISpec: "./openapi.yaml",
    ArazzoDoc:   "./arazzo.yaml",
    Output:      barometer.FormatJSON,
}

result, err := barometer.RunInput(ctx, input, barometer.NewClient(nil))
report := barometer.BuildReport(result, time.Second)
```

## Local sibling development

When changing Barometer with other Go repos in the toolchain, prefer a workspace `go.work` file:

```bash
go work init .
go work use ../navigator ../barrelman ../telescope/server
```

This keeps Barometer pointed at sibling checkouts without editing `go.mod`.

## Release coordination

- `.github/workflows/release.yml` publishes Barometer from pushed `v*` tags after running `go test -race -count=1 ./...`.
- Run `go test ./internal/openapi ./...` locally before tagging, especially after Navigator resolver or document-model changes.
- For shared compatibility and bump order, use `navigator/TOOLCHAIN_BOUNDARIES.md`.
- For runtime smoke fixtures and parity anchors, use `navigator/TOOLCHAIN_FIXTURE_MATRIX.md`.

## Toolchain role

Barometer is the runtime contract-testing layer in the shared OpenAPI toolchain:

- `navigator` provides the canonical OpenAPI and Arazzo document-loading and validation layer.
- `barometer` uses those static contracts to execute live HTTP validations and Arazzo workflows.
- `barometer` does **not** own static parsing, semantic linting, or editor UX.

In other words: Navigator owns the static document contract; Barometer owns runtime execution against that contract. Telescope and Meridian may surface or compose Barometer runs, but Barometer remains the runtime engine.

## License

See [LICENSE](LICENSE).
