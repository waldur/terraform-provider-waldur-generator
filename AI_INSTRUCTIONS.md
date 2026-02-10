# AI Development Guidelines: Waldur Terraform Provider Generator

This document provides instructions and context for AI agents working on this project to ensure consistency, maintainability, and deterministic code generation.

## Project Overview

A Go-based generator that consumes a Waldur OpenAPI specification and produces a Terraform provider based on the **Terraform Plugin Framework**.

- **Core Repo**: `terraform-waldur-provider`
- **Generated Code**: Root `output/` directory (git-ignored).
- **Config**: `config.yaml` defines what gets generated.

## Architecture & Core Modules

| Module | Purpose |
| :--- | :--- |
| `internal/config` | Configuration parsing and defaults. **Avoid hardcoding rules here.** |
| `internal/generator/common/schema.go` | Logic for extracting `FieldInfo` from OpenAPI schemas. Supports nesting (depth 3). |
| `internal/generator/builders/` | Modular logic for resource types (`Standard`, `Order`, `Link`). |
| `internal/openapi/` | Wrapper around `kin-openapi` for schema/operation retrieval. |
| `internal/generator/templates/` | Go `text/template` files for code generation. |

## Key Development Principles

### 1. Configuration Over Code

Hardcoded field lists or rules are strictly forbidden.

- **Exclusions**: Use `excluded_fields` in `config.yaml` to skip fields globally.
- **Typing**: Use `set_fields` in `config.yaml` to force `types.Set` instead of `types.List`.
- **Implementation**: Always pass `common.SchemaConfig` from `config.yaml` through the extraction logic.

### 2. Determinism is Mandatory

The generator must produce the **exact same code** on every run.

- **Sorting**: Never iterate over maps directly when generating code. Always collect keys, sort them lexicographical, and iterate over sorted keys.
- **Affected areas**: `Resource.UpdateActions`, `Components.Schemas`, and any field property iterations.

### 3. Verification Workflow

Always verify changes using this progression:

1. **Unit Tests**: `go test ./internal/...`
2. **Determinism Check**: Run the generator twice and ensure no diffs exist in `output/` between runs.
3. **Regression Check**: Compare current `output/` against a baseline (e.g., `git stash` -> generate baseline -> `git stash pop` -> generate current -> `diff -r`).
4. **Acceptance Tests**: Run `TF_ACC=1 WALDUR_POLL_DELAY=1ms WALDUR_POLL_MIN_TIMEOUT=1ms go test -v ./e2e_test/...` inside the `output/` directory. (Note: These may have pre-existing VCR mismatch issues).

### 4. Code Generation Pipeline

When modifying the generator, favor moving logic out of templates and into Go code.

- Extract data into `FieldInfo` or specialized Data structs.
- Use the `builders` pattern to encapsulate resource-specific logic.
- Use `common.AssignMissingAttrTypeRefs` to ensure unique naming for nested structs.

## Ongoing Refactoring (Context)

We are currently in a multi-phase refactoring effort:

- **Direction 4 (COMPLETED)**: Externalized configurations.
- **Direction 1 (PLANNED)**: Centralized Type Registry & Mapping (move type logic out of templates).
- **Direction 2 & 3 (PLANNED)**: Pipeline-based processing and moving schema string generation to Go.

---
*Note: This document should be updated as new architecture patterns are established.*
