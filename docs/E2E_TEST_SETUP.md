# E2E VCR Testing Guide

This guide explains how to use the automated End-to-End (E2E) testing workflow with VCR (Go-VCR). The generator automatically sets up the test environment, including test helpers and fixture directories.

## Workflow Overview

The E2E testing workflow consists of four main stages:

1. **Generate**: The generator creates the provider and copies e2e tests/fixtures.
2. **Replay**: Run tests using existing cassettes (default).
3. **Record**: Run tests against a live API to create new cassettes.
4. **Migrate**: Copy new cassettes back to templates to persist them.

## 1. Quick Start (Replay Mode)

After running the generator, you can immediately run E2E tests in replay mode. No extra setup is required.

```bash
# 1. Generate the provider
go run main.go -config config.yaml

# 2. Go to output directory
cd output

# 3. Install dependencies
go mod tidy

# 4. Run tests
TF_ACC=1 go test ./e2e_test -v
```

## 2. Recording New Cassettes

To add new tests or update existing ones, you need to record interactions with a real Waldur API.

### A. Environment Setup

Export the necessary environment variables:

```bash
export WALDUR_API_URL="https://api.waldur.example.com/"
export WALDUR_API_TOKEN="your-real-token"
export VCR_RECORD=true
export TF_ACC=1
```

### B. Run the Test

Run the specific test you want to record:

```bash
cd output
go test ./e2e_test -v -run TestOpenstackInstance_CRUD
```

This will:

1. Execute real API calls.
2. Create a new cassette file at `output/testdata/fixtures/<cassette-name>.yaml`.

### C. Sanitize the Cassette

**CRITICAL**: Before committing, you **MUST** sanitize the cassette to remove sensitive data.

1. Open `output/testdata/fixtures/<cassette-name>.yaml`.
2. Replace your real token with a placeholder (e.g., `test-token`).
3. Replace any sensitive real IDs or URLs if necessary.

## 3. Persisting Cassettes (Migrate & Commit)

Since the `output/` directory is ephemeral (it gets overwritten on regeneration), you must move new cassettes back to the generator's templates.

### A. Migrate to Templates

Copy the recorded and sanitized cassette from the output directory to the templates directory:

```bash
# From project root
cp output/testdata/fixtures/your_new_cassette.yaml internal/generator/templates/fixtures/
```

### B. Commit Changes

Commit the new cassette file to the generator repository:

```bash
git add internal/generator/templates/fixtures/your_new_cassette.yaml
git commit -m "Add VCR cassette for <feature>"
```

Now, the next time you run the generator, this cassette will be automatically included in the output.

## 4. Writing New Tests

To add a new E2E test:

1. Create a new file in `internal/generator/templates/e2e/` (e.g., `my_resource_test.go`).
2. Follow the pattern in `openstack_instance_test.go`:
    * Use `testhelpers.SetupVCR(t, "my_cassette_name")`.
    * Use `provider.NewWithHTTPClient` to inject the VCR transport.
3. Re-run the generator (`go run main.go ...`) to propagate the new test file to `output/e2e_test/`.
4. Follow the **Recording** steps above to generate the initial cassette.
