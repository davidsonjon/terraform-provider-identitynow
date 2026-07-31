# terraform-provider-identitynow

A Terraform Plugin Framework provider for SailPoint IdentityNow / Identity
Security Cloud, built via HashiCorp's OpenAPI-driven codegen pipeline
(`terraform-plugin-codegen-openapi` + `terraform-plugin-codegen-framework`)
plus hand-written CRUD.

**Before doing any codegen-pipeline work (adding/refreshing a resource or
data source), read the specialist agent docs first** — they contain far more
detail than this file and are kept up to date as the pipeline evolves:
- [`.github/agents/terraform-provider-developer.agent.md`](agents/terraform-provider-developer.agent.md) — vendor-agnostic pipeline shape, offline-vs-live validation split, enforcement rules.
- [`.github/agents/identitynow-terraform-provider-developer.agent.md`](agents/identitynow-terraform-provider-developer.agent.md) — IdentityNow-specific spec source, SDK, and command deltas.
- [`.github/agents/identitynow-terraform-provider-developer.sdk-issues.md`](agents/identitynow-terraform-provider-developer.sdk-issues.md) — confirmed `golang-sdk` defects/workarounds (check before assuming a new bug).
- [`.github/agents/identitynow-terraform-provider-developer.sdk-type-reference.md`](agents/identitynow-terraform-provider-developer.sdk-type-reference.md) — confirmed SDK struct field shapes.

## Build, lint, test

- `make build` / `make install` — build/install the provider binary (`go build`/`go install`).
- `make lint` — `golangci-lint run` (config: `.golangci.yml`).
- `make fmt` — `gofmt -s -w -e .` + `terraform fmt -recursive examples`.
- `make test` — `go test -v -cover -timeout=120s -parallel=10 ./...` (unit tests only, no live credentials).
  - Single package: `go test ./internal/provider/role_v1/...`
  - Single test: `go test -run TestRoleResource_readback ./internal/provider/role_v1/...`
- `make testacc` — `TF_ACC=1 go test -v -cover -timeout 120m ./...`. **Live acceptance tests against a real sandbox tenant — always ask for explicit user confirmation before running.**
- `make docs` — regenerates `./docs` via `tfplugindocs` from schema descriptions + `./examples` + `./templates`. CI fails if `docs/` drifts from a fresh `make docs` run — regenerate and commit after any schema/example change.
- `make tflint` — lints `examples/` (the only committed Terraform HCL; `test/**/main.tf` is gitignored).
- `make validate-examples` — runs `terraform validate` against every `examples/{resources,data-sources}/<name>/*.tf` snippet using a locally-built dev-override binary (no network/credentials needed).
- `make plan TARGET=<folder>` — builds/installs, then `terraform plan` in `test/<folder>` against a real sandbox tenant (see Live testing below). `make apply TARGET=<folder>` does the same for `apply` — **live side effects, confirm with the user first.**

CI (`.github/workflows/ci.yml`) runs build/vet/test, `golangci-lint`, a docs-drift
check, `tflint`, and `validate-examples` on every push/PR — never `testacc`.

### Offline (Phase A) vs. live (Phase B) validation
Phase A (`go build`, `go vet`, `go test`, `make lint`, `make docs` diff-check,
`make validate-examples`, `make tflint`) needs no tenant access and should
always be run in full. Phase B (`make plan`/`make apply` against
`test/<folder>/main.tf`, or `make testacc`) requires real sandbox credentials
and always needs explicit user confirmation before any write/delete side
effect. If no `test/<folder>/main.tf` exists and you have no sandbox
credentials, that's expected — complete Phase A and leave Phase B as an
explicit pending item; never fabricate a plan/apply result you didn't
actually observe.

## Architecture

### Codegen pipeline (per resource/data-source "target")
Each API target goes through the same pipeline stages — see the agent docs
for full commands:
1. **Bundle spec** — `make bundle-spec-v1 TARGET=<target> SERVICE=<service-folder>` dereferences `api-specs/idn/apis/<service>/openapi.yaml` (read from an external `API_SPECS_SOURCE` clone of `sailpoint-oss/api-specs`, default sibling dir `../api-specs`) into the committed `api-specs/dereferenced/deref-<service>.v1.yaml`. This is a **one-time** step per target — later stages only need the committed output, not `API_SPECS_SOURCE`.
2. **Generator config** — hand-authored `generator_config/generator_config_<target>_v1.yml` maps OpenAPI paths/operations to a resource/data-source key (must match `^[a-z_][a-z0-9_]*$`).
3. **`make gen-api-v1 TARGET=<target> SERVICE=<service-folder>`** — produces `openapi_code_spec/openapi_code_spec_<target>_v1.json`. Watch WARN output for silently-skipped response mappings (e.g. whole-operation `allOf`).
4. **Schema post-processing** — before/alongside type linking, apply any needed `generator_config/schema_overrides_<target>_v1.yml` corrections (e.g. `required_overrides`, `strip_defaults`) via `scripts/apply_codespec_schema_overrides.py`, and flatten metadata-only `allOf` wrappers via `scripts/flatten_openapi_allof.py` where the spec uses them — both are recurring, checked-in, replayable steps, not one-off fixes.
5. **Type linking** — map `associated_external_type`/`custom_type` to `github.com/sailpoint-oss/golang-sdk/v2` types (usually `api_beta` package). Persist mappings/renames in `generator_config/type_mappings_<target>_v1.yml`, applied via `python3 scripts/apply_codespec_type_mappings.py --target <target>_v1` (not one-off inline edits, so refreshes are replayable).
6. **`make gen-framework-api-v1 TARGET=<target>`** — emits generated Go schema/model types into `internal/provider/<target>_v1/<resource|datasource>_<name>/*_gen.go`. This step never generates CRUD logic.
7. **Hand-write CRUD** — `internal/provider/<target>_v1/resource_<name>.go` / `datasource_<name>.go` implement Create/Read/Update/Delete/Configure/ImportState against the SDK client via a small `clientProvider` interface (satisfied by `identitynowProvider.GetClient()`) to avoid an import cycle with the root `provider` package.
8. **Build & wire** — `make build`, then register the new `New<X>Resource`/`New<X>DataSource` constructors in `internal/provider/provider.go`'s `Resources()`/`DataSources()`.
9. **Validate** — Phase A always; Phase B when credentials/test config are available.

### Directory layout this implies
- `internal/provider/<target>_v1/` — one package per target. `*_gen.go` files under a nested `resource_<name>/` or `datasource_<name>/` subpackage are **generated — never hand-edit them**; fix the spec/generator config/type-mapping script instead. Sibling `resource_<name>.go`, `datasource_<name>.go`, `resource_<name>_planmodifiers.go`, `resource_<name>_readback.go`, `sdk_fallback.go` files are hand-written and expected to be hand-maintained.
- `_v1` suffix marks the per-service versioned pipeline (preferred for new targets, paths like `/service-desk-integrations/v1/{id}`); targets without it use the legacy monolithic spec pipeline (unversioned paths, `generator_config_<target>.yml`, `make gen-api`/`make gen-framework-api`). Promoting a `_v1` pilot by dropping the suffix is a deliberate separate step, not implied by wiring it into the provider.
- Computed-only server-managed attributes with no schema `Default` (e.g. `id`) typically need a hand-added `UseStateForUnknown()` plan modifier in `resource_<name>_planmodifiers.go`, or repeated `plan`/`apply` cycles show a perpetual spurious diff.
- Dynamic/discriminated-union fields (e.g. `transform`'s `attributes`) are excluded from codegen via `schema.ignores` in the generator config, then hand-added as a `jsontypes.Normalized` (`terraform-plugin-framework-jsontypes`) JSON-string `CustomType` with a hand-written model struct — see `transform_v1` for the reference implementation.
- `api-specs/` is git-tracked but holds only this project's own derived `dereferenced/deref-<service>.v1.yaml` outputs, never the full upstream spec tree.
- `docs/` is generated by `tfplugindocs` from schema descriptions, `examples/` (real sanitized HCL, no live credentials), and `templates/` (custom per-page sections, e.g. "Known Limitations & Live Testing Notes").

### Manual live test configs (`test/`)
`test/<folder>/main.tf` is a self-contained, **gitignored** Terraform config
(one per resource/data-source family) that exercises the locally-built
provider against a real sandbox tenant via `~/.terraformrc` `dev_overrides`
(no `terraform init` needed). Full conventions (folder layout, credentials,
Phase A vs. Phase B, gotchas) are tracked in
[`docs/TESTING.md`](../docs/TESTING.md) — `test/` itself is gitignored, so
that file (not `test/README.md`, a local-only pointer stub) is canonical.
Convention: chain a paired data source's lookup key from its resource's own
unknown-until-apply output attribute, so `plan` alone (not just `apply`)
validates schema/wiring without needing a pre-existing live object.
**Exception:** a plural/list data source (e.g. `identitynow_roles_v1`) with
a fully-known filter invokes the live API during `plan` itself, not just
`apply` — this makes that target's `plan` a Phase B (live) operation.
Sandbox credentials live only in `test/.env` (gitignored, `source`d before
any Phase B command) — every `main.tf`'s own `provider` block stays empty
and holds no secrets, just fixture object ids/config. Never `cat`/print/log
`test/.env`; a `main.tf` file may be viewed/edited normally.

## Key conventions
- Hand-written CRUD files are never regenerated — fix the OpenAPI spec, generator config, or `scripts/apply_codespec_type_mappings.py` config instead of hand-patching `*_gen.go` output.
- Resolve tool/SDK locations portably (e.g. `go list -m -f '{{.Dir}}' github.com/sailpoint-oss/golang-sdk/v2`), never via a hardcoded absolute path from a prior session.
- Any live `apply`/`destroy`/`testacc` run requires explicit user confirmation each time, even repeating a previously successful run — state what will be created/changed/destroyed first.
- If spec/SDK output suggests a target is missing an operation (e.g. no `Delete*` method), treat it as a hypothesis to confirm with the user before designing a permanent workaround (no-op delete, "immutable" doc claim) — don't silently assume spec/SDK silence means a real API limitation.
- A spec/schema YAML file existing on disk does not prove its endpoint is live — trace it to a real top-level `paths:` entrypoint or confirm via a live curl call before designing a write path around it (an unverified path once invalidated an entire resource's design).
- Resources managing an association/sub-resource of another object (not a fully-owned lifecycle) need live verification via a *direct* API call on the parent object at each CRUD step — a clean `terraform plan`/`apply` alone is not sufficient, since Terraform's state can look consistent while the write path is non-functional.
- Never trust a delegated background agent's self-reported build/test/lint/live-test results without independently re-running them yourself.
- An `Optional: true, Computed: true` field with no `PlanModifier` is reported **Unknown** (not Null) when a practitioner omits it from config — check `!field.IsNull() && !field.IsUnknown()` in hand-written conversion functions, not just `IsNull()`.
