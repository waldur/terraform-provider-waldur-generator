# terraform-provider-waldur-generator

A **code generator** (Go) that turns the Waldur OpenAPI schema + `config.yaml` into a complete
**Terraform provider** built on the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
(protocol 6.0). The generated provider is **not** committed here — it is a build artifact pushed to
the downstream repo `waldur/terraform-provider-waldur` for publication to the Terraform Registry.

> You are almost always editing the **generator** (`internal/...`, `config.yaml`, templates), not
> generated provider code. Never hand-edit anything under `output/` — it is regenerated and discarded.

> **Detailed source of truth: `docs/AI_INSTRUCTIONS.md`.** Also see `docs/CONFIGURATION_GUIDE.md`,
> `docs/DEVELOPER_GUIDE.md`, `docs/E2E_TEST_SETUP.md`. This file is the quick orientation; read those
> before non-trivial work.

## Source vs. output

| Path | Role |
|------|------|
| `main.go`, `internal/` | The generator (edit here) |
| `config.yaml` | Resources, data sources, and global generation rules (`excluded_fields`, `set_fields`) |
| `waldur_api.yaml` | The Waldur OpenAPI schema — source of truth for provider content |
| `output/`, `dist/` | Generated provider — **gitignored**, ephemeral |

Because `output/` is gitignored and content is schema-driven, changes to the generated provider
leave **no git diff and no changelog entry** here. The deploy workflow
(`.github/workflows/deploy.yml`) generates into `output/`, then **wipes the downstream
`waldur/terraform-provider-waldur` repo and rsyncs the output in, committing with a single flat
message `"Update generated provider code"`** — so the published history carries no record of *what*
changed. There is no changelog tooling in either repo. See `../CLAUDE.md` for the workspace-level
discussion; the fix that captures these changes is to diff the generated resource/field set between
schema revisions and write a real commit message / changelog into the downstream repo.

## Architecture (Component + Plugin)

**Components** (`internal/generator/components/`) — each owns its template-data prep + rendering:

- `resource` — full CRUD Terraform resources
- `datasource` — read-only data sources
- `list` — plural listing (ListResources)
- `action` — standalone POST actions on existing resources

**Plugin builders** (`internal/generator/plugins/`) — resource "flavors":

- `standard` — base implementation for simple resources
- `order` — async Waldur marketplace orders
- `link` — linking resources (e.g. volume attachment)

**Shared**: `internal/generator/common/schema.go` (extracts `FieldInfo` from OpenAPI, deep nesting +
type resolution); `internal/generator/templates.go` (template helper funcs like `ToAttrType`,
`formatValidator`); Go templates under `internal/generator/templates/`.

## Key principles (do not violate)

- **Configuration over code** — no hardcoded field lists or service-specific rules in Go. Use
  `excluded_fields` / `set_fields` in `config.yaml`; thread `common.SchemaConfig` through extraction.
- **Determinism is mandatory** — output must be byte-identical across runs. Never range over maps
  when emitting code; collect keys, sort lexicographically, iterate sorted.
- **Logic in Go, not templates** — do transformations/humanization/type-mapping in Go; keep
  templates structural and minimal.

## Commands

```bash
go test ./internal/...                       # unit tests
go run main.go -config config.yaml           # generate provider into output/

cd output && go mod tidy && go build ./...    # MANDATORY: verify generated code compiles

# E2E (acceptance) — replays VCR cassettes by default
cd output && TF_ACC=1 \
  WALDUR_ACCESS_TOKEN=<token> WALDUR_API_URL=http://... \
  WALDUR_POLL_DELAY=1ms WALDUR_POLL_MIN_TIMEOUT=1ms \
  go test -v ./e2e_test/...
```

Go 1.25 (`go.mod`). Note `go.mod` has `replace github.com/waldur/terraform-provider-waldur => ./output`.

### Test & fixture persistence (important)

`output/` is ephemeral. New tests or recorded cassettes there are **lost on regeneration** unless you
move them back into the generator:

- new e2e tests → `internal/generator/templates/e2e/*.go.tmpl`
- new VCR cassettes → `internal/generator/templates/fixtures/*.yaml`

## Conventions & pitfalls

- **Branch / remote**: default branch `main`, GitHub (`github.com/waldur/terraform-provider-waldur-generator`).
- **Auth**: use `WALDUR_ACCESS_TOKEN` (not `WALDUR_AUTH_TOKEN` — that yields `401`).
- **Empty `output/`**: if `output/go.mod` is missing, `go run main.go` failed — fix that first.
- **`map has no entry for key`** template errors: ensure data passed to `RenderTemplate` is
  initialized and exported (capitalized).
- A prebuilt `generator` binary is committed at the repo root; prefer `go run main.go` / `go build`
  from source and don't commit rebuilt binaries.
</content>
