# Manual (live) testing conventions

This document describes the `test/` directory: manual Terraform
configurations used to exercise the locally-built provider against a real
IdentityNow sandbox tenant. `test/` itself is gitignored (it holds real
sandbox credentials and fixture object ids), but the conventions below are
tracked here so contributors and agents without direct access to that
directory still understand its intent and how it's structured.

Each subdirectory under `test/` is a **self-contained** Terraform
configuration used to manually exercise the locally-built provider (via
`make install` + `~/.terraformrc` `dev_overrides`) against a real IdentityNow
sandbox tenant.

## Convention

- One folder per resource/data-source family under test, e.g.
  `test/service_desk_integration/`.
- Each folder has its own `terraform { required_providers { ... } }` block and
  an **empty** `provider "identitynow" {}` block — folders are independent
  and can be `plan`/`apply`ed on their own without needing the others to
  exist. Credentials are **not** stored per-folder; see "Credentials" below.
- When adding a new pilot resource/data source, copy an existing folder's
  `main.tf` (e.g. `cp service_desk_integration/main.tf <new_folder>/main.tf`)
  rather than retyping the `terraform`/`provider` boilerplate.
- Prefer chaining a data source's lookup key from the paired resource's own
  (unknown-until-apply) output attribute, e.g. `id =
  identitynow_service_desk_integration_v1.test.id`. This defers the data
  source's `Read` to apply time so `terraform plan` never needs a real,
  pre-existing object id in the sandbox tenant to produce a clean plan.

## Credentials

Sandbox credentials live in `test/.env` (gitignored, alongside the rest of
this directory), **not** hardcoded into any individual `main.tf`'s `provider`
block. `test/.env` exports `SAIL_BASE_URL`/`SAIL_CLIENT_ID`/
`SAIL_CLIENT_SECRET` — the same environment variables the provider itself
already reads natively (see `internal/provider/provider.go`) — so every
`provider "identitynow" {}` block across every folder can stay empty.

`source test/.env` before running any Phase B command (`make plan`/
`make apply`, or `TF_ACC=1 go test ...` for acceptance tests). Because the
CLI's bash tool runs each command in a fresh, non-interactive process, shell
profile files (`.zshrc`/`.zshenv`/etc.) set in a human's interactive shell
are **not** visible to an AI agent's tool calls — `test/.env` must be
`source`d explicitly in each command that needs it.

If `test/.env` doesn't exist yet or its credentials have been rotated, ask a
human to populate/update it — do not fabricate or guess sandbox credentials,
and do not ask a human to paste secrets directly into a chat/agent transcript
(paste them into the file instead).

## Running

Use `make plan TARGET=<folder>` from the repo root (builds, installs, and runs
`terraform plan` in `test/<folder>`) or manually:

```sh
source test/.env
make install
cd test/<folder>
terraform plan
```

`terraform init` is not required — `~/.terraformrc` `dev_overrides` for
`local.dev/identitynow/identitynow` (and `davidsonjon/identitynow`) point
directly at the locally-installed provider binary in `$GOBIN`.

## Offline (Phase A) vs. Live (Phase B) validation

Not every check in this repo requires sandbox credentials or a `test/<folder>/`
config to exist. Per the
[terraform-provider-developer](../.github/agents/terraform-provider-developer.agent.md)
base agent's Offline-vs-Live Validation split:

- **Phase A (offline, always run, no tenant access needed)**: `go build ./...`,
  `go vet ./...`, `go test ./...`, `make lint`, `make docs` (+ confirm no diff
  against committed `docs/`), `make validate-examples`, `make tflint`. These
  are exactly what CI (`.github/workflows/ci.yml`) runs on every push/PR —
  none of them touch a live tenant.
- **Phase B (live, requires a `test/<folder>/main.tf` with real sandbox
  credentials)**: `make plan TARGET=<folder>` (and, with explicit user
  confirmation only, `terraform apply`). This directory (`test/`) is entirely
  Phase B — it's gitignored precisely because it holds real credentials, and
  a fresh clone or cloud-hosted agent session won't have it populated.

**If no `test/<folder>/main.tf` exists yet for a target and you have no
sandbox credentials to create one** (e.g. a cloud-hosted agent, or a
contributor without tenant access): this is expected and not a blocker.
Complete every Phase A check in full, then explicitly call out Phase B as a
pending/deferred item (e.g. a todo) for a human or credentialed session to
finish later. Never fabricate a `plan` result you didn't actually observe.

**Gotcha — plural/list data sources are not always Phase-A-safe even with a
config present**: a data source's `Read` normally only runs at `apply` time
if its lookup key is unknown-until-apply (see the chaining convention above).
But a **plural/list data source** (e.g. `identitynow_roles_v1`,
`identitynow_access_profiles_v1`) configured with a fully-known filter (no
unknown attributes referenced) has its `Read` invoked by Terraform Core
**during `plan` itself** — confirmed on both `roles_v1` and
`access_profiles_v1`. This means `make plan TARGET=<folder>` for a target
with such a data source block is a live (Phase B) operation even though it's
"just" a plan, not an apply. If you need a genuinely credential-free
Phase A check for such a target, omit or comment out the plural data source
block, or give it a filter that's unknown until apply.

## Security

`main.tf` files in these folders no longer contain real credentials (see
"Credentials" above) — only `test/.env` does. Never print, `cat`, or log the
contents of `test/.env`. When it must be inspected or edited by an AI agent,
use non-printing operations (`source`, redirection into env vars) rather
than `cat`/`view` on the whole file. `main.tf` files may still reference
real (sandbox) object ids (source ids, owner ids, entitlement ids, etc.) —
these are not secrets and may be freely read/printed/edited, only the
credentials in `test/.env` need this care.
