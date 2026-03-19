# Barometer

Barometer is a Go package, CLI, and GitHub Action for **contract testing** and validation of APIs using **OpenAPI** (2.0, 3.0.x, 3.1.x, 3.2) and **Arazzo** workflow documents. It runs requests against a live API and validates responses against the spec (status, headers, body schema) and executes multi-step Arazzo workflows.

## Features

- **OpenAPI contract tests**: Load a spec, hit every (or filtered) operation, validate response status and body schema.
- **Arazzo workflows**: Run multi-step API workflows with runtime expressions, success criteria, and outputs.
- **CLI**: `barometer openapi validate|test`, `barometer arazzo validate|run`, `barometer contract test`.
- **Output**: Human, JUnit XML, or versioned JSON for CI and tools (e.g. [Telescope](https://github.com/sailpoint-oss/telescope)).
- **Async API**: `barometer.Start(ctx, config)` returns a `Job` for IDE/LSP integration without blocking.

## Install

```bash
go install github.com/sailpoint-oss/barometer/cmd/barometer@latest
```

## Quick start

```bash
# Validate an OpenAPI spec
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
    base-url: 'https://api.example.com'
    output-format: 'junit'
```

## Telescope integration

Barometer uses the **Telescope Go rewrite** ([schemas-and-ci-pipeline](https://github.com/sailpoint-oss/telescope/tree/schemas-and-ci-pipeline)) OpenAPI IR: the same `*openapi.Index` and typed model (Document, Operation, Schema, etc.) produced by Telescope's tree-sitter and standalone parsers. This allows:

- **Go library**: Telescope (or any Go tool) can pass a pre-parsed `*openapi.Index` to `barometer.RunWithIndex(ctx, idx, baseURL, opts)` or `barometer.StartWithIndex(ctx, idx, baseURL, opts)` so parsing is done once and contract tests run against the same IR.
- **SDK analyzer**: Register contract tests as a Telescope rule via `barometer.ContractTestAnalyzer(baseURL, opts)` in a `sdk.Rule(...).Custom(...).Register(plugin)` chain; failures appear as diagnostics at each operation's source location.
- **CLI / async**: Run `barometer contract test --output json` or `barometer.Start(ctx, config)` for config-driven runs. Use `barometer schema` to print the versioned JSON report schema for type generation.

Development requires the Telescope server module (e.g. clone [telescope](https://github.com/sailpoint-oss/telescope) next to barometer and use `replace github.com/sailpoint-oss/telescope/server => ../telescope/server` in go.mod).

## License

See [LICENSE](LICENSE).
