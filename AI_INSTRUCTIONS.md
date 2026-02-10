# AI Development Guidelines: Waldur Terraform Provider Generator

This document provides a comprehensive source of truth for AI agents working on the Waldur Terraform Provider Generator, ensuring consistency, maintainability, and deterministic code generation.

## 1. Project Overview

The generator is a Go application that consumes a Waldur OpenAPI specification and produces a Terraform provider based on the **Terraform Plugin Framework**.

- **Source Repository**: `terraform-waldur-provider`
- **Output Directory**: `output/` (git-ignored, ephemeral).
- **Core Configuration**: `config.yaml` defines resources, data sources, and global generation rules.

## 2. Architecture & Core Modules

The generator follows a modular **Component** and **Plugin** architecture.

### 2.1. Component Packages (`internal/generator/components/`)

Each component encapsulates its own template data preparation and rendering logic.

- **`resource`**: Handles full CRUD Terraform resources.
- **`datasource`**: Handles Read-only data sources.
- **`list`**: Handles plural resource listing (ListResources).
- **`action`**: Handles standalone API actions (POST operations on existing resources).

### 2.2. Plugin Builders (`internal/generator/plugins/`)

Modular logic for different resource "flavors":

- **`standard`**: Base implementation for simple resources.
- **`order`**: Specialized implementation for async Waldur orders (Marketplace).
- **`link`**: Implementation for linking resources (e.g., volume attachment).

### 2.3. Shared Modules

- **`internal/generator/common/schema.go`**: Centralized logic for extracting `FieldInfo` from OpenAPI. Supports deep nesting and type resolution.
- **`internal/generator/templates.go`**: Contains helper functions (`ToAttrType`, `formatValidator`, etc.) registered with the template engine.

## 3. Key Development Principles

### 3.1. Configuration Over Code

Hardcoded field lists or service-specific rules are strictly forbidden in Go code.

- **Global Rules**: Use `excluded_fields` in `config.yaml` to skip metadata fields like `created`, `modified`.
- **Type Overrides**: Use `set_fields` in `config.yaml` to force `types.Set` for specific API fields.
- **Metadata**: Always pass `common.SchemaConfig` through extraction logic.

### 3.2. Determinism is Mandatory

The generator must produce **byte-identical code** on every run.

- **Sorting**: Never iterate over maps directly when generating code. Always collect keys, sort them lexicographically, and iterate over the sorted keys.
- **Strict Areas**: Iterating over `Resource.UpdateActions`, `Components.Schemas`, or field properties.

### 3.3. Logic in Go, Not Templates

Templates should be kept minimal and focused on structure.

- **Data Preparation**: Perform all complex transformations, field humanization, and type mapping in Go before passing data to templates.
- **Specialized Structs**: Use `FieldInfo` or component-specific `TemplateData` structs to enrich data in the generation pipeline.

### 3.4. Metadata Automation

- **Descriptions**: If OpenAPI descriptions are missing, use humanization logic to generate a fallback from the field name (e.g., `article_code` -> `Article Code`).
- **Standardization**: Consistently use `MarkdownDescription` in schema definitions.

### 3.5. Shared Template Logic

- **`shared.tmpl`**: Centralizes common rendering logic.
- **`renderValidators`**: Use the shared `renderValidators` template definition for all attribute types (String, Int64, Float64). It supports `OneOf`, `AtLeast`, `AtMost`, and `RegexMatches`.

## 4. Verification & Testing Workflow

### 4.1. The Development Progression

1. **Unit Tests**: `go test ./internal/...`
2. **Generation**: Run `go run main.go`.
3. **Compilation**: `cd output && go mod tidy && go build ./...` **(MANDATORY: Always build after generation to verify code validity)**
4. **E2E Tests**: `cd output && go test -v ./e2e_test/...` (Use acceptance testing flags below).

### 4.2. Acceptance Testing Flags

```bash
cd output
TF_ACC=1 \
WALDUR_ACCESS_TOKEN=<your_token> \
WALDUR_API_URL=http://... \
WALDUR_POLL_DELAY=1ms \
WALDUR_POLL_MIN_TIMEOUT=1ms \
[VCR_RECORD=true] \
go test -v -run <TestName> ./e2e_test/
```

### 4.3. Test & Fixture Persistence

The `output/` directory is ephemeral. If you create new tests or record VCR cassettes, you **must persist them**:

- **Test Templates**: Move new tests from `output/e2e_test/*.go` to `internal/generator/templates/e2e/*.go.tmpl`.
- **VCR Fixtures**: Move new cassettes from `output/e2e_test/testdata/*.yaml` to `internal/generator/templates/fixtures/*.yaml`.

## 5. Common Pitfalls & Troubleshooting

- **Authentication**: Always use `WALDUR_ACCESS_TOKEN`. `WALDUR_AUTH_TOKEN` is incorrect and results in `401 Unauthorized`.
- **Empty `output/`**: If `output/go.mod` is missing, the generator likely wasn't run or failed silently. Always ensure `go run main.go` succeeds first.
- **Template Errors**: If you encounter `map has no entry for key`, ensure all data passed to `RenderTemplate` is correctly initialized and capitalized for visibility.
