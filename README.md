# Terraform Provider for SailPoint IdentityNow

A [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
provider for SailPoint IdentityNow / Identity Security Cloud (ISC), built on
top of the official [`golang-sdk`](https://github.com/sailpoint-oss/golang-sdk).

## Scope

This provider currently covers 21 resources and 32 data sources across IdentityNow
access-governance surfaces (roles, access profiles, entitlements, sources, workflows,
segments, governance groups, SOD policies, transforms, and more). See
[`docs/index.md`](docs/index.md) for the categorized, up-to-date list of every
resource/data source — that page (not this README) is the source of truth, so it
doesn't need to be duplicated or kept in sync here.

See [`examples/`](examples/) for real, sanitized HCL for every
resource/data source, and [`docs/`](docs/) for the full generated reference
(schema, examples, and hand-written "Known Limitations & Live Testing Notes"
per target).

### Roadmap

Candidate future additions (SOD policy schedule/evaluate sub-endpoints,
Certification Campaigns, richer Access Request configuration) are tracked as
GitHub issues, not in this README — see the repo's
[issues](https://github.com/davidsonjon/terraform-provider-identitynow/issues)
for concrete, tracked feature requests.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) — see `go.mod` for the exact pinned
  toolchain version
- [Node.js](https://nodejs.org/) (only needed for the per-service `v1`
  codegen pipeline's `swagger-cli bundle` step — not needed to just build or
  use the provider)

## Building & installing locally

```shell
make build    # go build ./...
make install  # go install — puts the binary in $GOPATH/bin
```

To exercise a locally-built binary with real Terraform configs without
publishing to a registry, point `~/.terraformrc`'s `dev_overrides` at the
installed binary (see [`docs/TESTING.md`](docs/TESTING.md) for the exact
convention this repo uses).

## Development

This provider is built via a two-stage OpenAPI code-generation pipeline
([`terraform-plugin-codegen-openapi`](https://github.com/hashicorp/terraform-plugin-codegen-openapi)
+ [`terraform-plugin-codegen-framework`](https://github.com/hashicorp/terraform-plugin-codegen-framework))
followed by hand-written CRUD logic, per target (one resource/data-source
family at a time):

1. **Bundle/dereference the OpenAPI spec** for the target's service —
   `make bundle-spec-v1 TARGET=<target> SERVICE=<service-folder>` (one-time
   per target; the result is committed under `api-specs/dereferenced/`).
2. **Author a generator config** (`generator_config/generator_config_<target>_v1.yml`)
   mapping OpenAPI paths/operations to resource/data-source schema keys.
3. **`make gen-api-v1 TARGET=<target> SERVICE=<service-folder>`** — produces
   an intermediate "code spec" JSON.
4. **Schema post-processing & type linking** — apply any needed
   `generator_config/schema_overrides_<target>_v1.yml` corrections and
   `generator_config/type_mappings_<target>_v1.yml` SDK type bindings
   (both via checked-in `scripts/apply_codespec_*.py` scripts, never
   hand-patched inline).
5. **`make gen-framework-api-v1 TARGET=<target>`** — generates Go
   schema/model types into `internal/provider/<target>_v1/` (schema/model
   only; no CRUD).
6. **Hand-write CRUD** — `resource_<name>.go` / `datasource_<name>.go`
   implement Create/Read/Update/Delete/Configure/ImportState against the
   SDK client. These files are never regenerated and are expected to be
   hand-maintained.
7. **Build, wire into `internal/provider/provider.go`, validate.**

Two validation phases apply to every target:

- **Phase A (offline, always run)**: `go build`/`go vet`/`go test ./...`,
  `make lint`, `make docs` (regenerate + diff-check), `make validate-examples`,
  `make tflint`. No vendor credentials required.
- **Phase B (live, requires a real sandbox tenant)**: `make plan TARGET=<folder>`
  / `make apply TARGET=<folder>` against a config under `test/<folder>/`
  (gitignored — see [`docs/TESTING.md`](docs/TESTING.md) for the full
  convention, including credential handling via `test/.env`).

### Other useful `make` targets

```shell
make lint              # golangci-lint (https://golangci-lint.run)
make tflint             # tflint over examples/ (https://github.com/terraform-linters/tflint)
make validate-examples  # terraform validate over examples/
make docs               # regenerate docs/ via tfplugindocs (https://github.com/hashicorp/terraform-plugin-docs)
make test               # go test ./...
make testacc            # TF_ACC=1 acceptance tests — real side effects, costs money/time
make fmt                # gofmt -s -w .
```

### Adding a Go dependency

```shell
go get github.com/author/dependency
go mod tidy
```

Commit the resulting `go.mod`/`go.sum` changes.

### AI agent context

Contributors (human or AI) extending this provider should start with
[`.github/copilot-instructions.md`](.github/copilot-instructions.md), which
points to the fuller agent/knowledge files under
[`.github/agents/`](.github/agents/) — these capture the pipeline in detail,
hand-written-CRUD patterns, known SDK quirks, and a running log of lessons
learned from building out each target.

## Release status

CI (`.github/workflows/ci.yml`) runs build/vet/test, lint, a docs-drift
check, `tflint`, and `terraform validate` on every push/PR.

This provider is published on the [Terraform
Registry](https://registry.terraform.io/providers/davidsonjon/identitynow)
as `davidsonjon/identitynow` (community tier) and has shipped several
tagged releases (currently v0.5.3). Releases are cut by pushing a `v*` tag,
which triggers `.github/workflows/release.yml` to run
[GoReleaser](https://goreleaser.com/) per `.goreleaser.yml` — cross-compiling
binaries for all supported `GOOS`/`GOARCH` targets, GPG-signing the
checksums manifest, and publishing a GitHub Release that the Registry then
picks up automatically.

This is an actively developed, community-maintained provider (not an
official SailPoint product). Every resource/data source has been exercised
against a real sandbox tenant via `terraform apply`/`plan`/`destroy` (see
each target's own "Known Limitations & Live Testing Notes" section in
[`docs/`](docs/)), but as with any community provider you should validate
behavior against your own tenant and use case before relying on it in
production.
