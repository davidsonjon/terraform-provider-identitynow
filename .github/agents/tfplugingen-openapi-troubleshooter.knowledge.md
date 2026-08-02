# tfplugingen-openapi Troubleshooting Knowledge

Use this file to reinforce the troubleshooting agent over time.

## Entry Template
For a troubleshooting entry (fixed a generation failure):
- Date: YYYY-MM-DD
- Symptom: Short failure summary
- Root cause: Concrete technical cause
- Fix pattern: Reusable steps that solved it
- Validation command: Command(s) used to confirm resolution
- Guardrail update: What should change in agent behavior/checklist

For a review/authoring-pass entry (no specific failure being fixed):
- Date: YYYY-MM-DD
- Task: review/cleanup/authoring scope of the pass
- Summary: what was checked and what (if anything) was changed
- Validation: how the change was confirmed correct
Use whichever template fits the entry - do not force a review pass into the Symptom/Root-cause fields, or vice versa.

## Known Patterns
- Date: 2026-05-25
- Symptom: `rolodex has no file systems configured` while running `tfplugingen-openapi generate`
- Root cause: unresolved external `$ref` paths in dereferenced spec and resolver base-path mismatch
- Fix pattern: locate unresolved refs, rewrite or inline final unresolved schemas into base spec used by generation
- Validation command: `make gen-api TARGET=role`
- Guardrail update: always run unresolved `$ref` scan and fail fast before generation

- Date: 2026-05-25
- Symptom: troubleshooting starts from pasted logs and misses active runtime failure context
- Root cause: no deterministic command execution step before diagnosis
- Fix pattern: run `make gen-api TARGET=<name>` first, then diagnose from the current terminal run; optionally run `make gen-framework-api TARGET=<name>`
- Validation command: `make gen-api TARGET=<name>` and `make gen-framework-api TARGET=<name>`
- Guardrail update: enforce runner-first workflow when target is provided

- Date: 2026-05-30
- Symptom: user requested runtime-aware runner execution for role target
- Root cause: none (command succeeded on first execution)
- Fix pattern: keep runner-first flow and perform artifact sanity checks (`ls` + `jq keys`) even when no error is thrown
- Validation command: `make gen-api TARGET=role`
- Guardrail update: on successful runs, still record artifact size and top-level key check in validation output

- Date: 2026-07-24
- Symptom: `make gen-api-v1 TARGET=transform SERVICE=transforms` succeeded (exit 0) but emitted `WARN msg="skipping mapping of create/read operation response body"` with `err="found 2 allOf subschema(s), schema composition is currently not supported"` — the resulting code spec had a resource with NO attributes read from the response at all.
- Root cause: `transforms`' spec wraps the *entire top-level* response schema (not just a nested field) in a 2-member `allOf`. `tfplugingen-openapi` cannot decompose `allOf` composition at all — when the whole response is behind `allOf`, it silently skips mapping the whole operation, not just one field.
- Fix pattern: hand-editing was impractical (one `allOf` member's sibling content, `attributes`, is a ~3000-line `oneOf` across 35 transform types). Used `scripts/flatten_openapi_allof.py` — a small, checked-in `pyyaml` script that structurally walks the bundled `api-specs/dereferenced/deref-<service>.v1.yaml`, matches an `allOf` node by its members' sibling-key shape (not line/path position), merges `properties`/`required` into one flat object schema, and re-dumps the file. Run once against the bundled/dereferenced output (never the raw upstream spec).
- Validation command: `grep -c allOf api-specs/dereferenced/deref-transforms.v1.yaml` (0 after, 4 before), then `make gen-api-v1 TARGET=transform SERVICE=transforms` (clean, no WARN), then `jq` sanity-check of the resulting code spec.
- Guardrail update: a "skipping mapping of ... response body" WARN (whole-operation, not per-field) means the `allOf` is likely at the entire-response level — prefer `scripts/flatten_openapi_allof.py` over manual reindentation. Must be re-run every time `make bundle-spec-v1` re-derives that service's spec (bundling always re-derives from raw upstream, discarding any hand-flattening) — note this as a forward-compatibility trap in the target's package doc comment (see `transform_v1`).


- Date: 2026-07-25
- Task: review/cleanup pass across all agents/prompts
- Summary: No unused/orphaned agent, knowledge, or prompt files found - all 3 agent pairs (`identitynow-terraform-provider-developer`, `tfplugingen-openapi-troubleshooter`, `tfplugingen-openapi-type-reviewer`) have exactly one matching `.knowledge.md` and one matching `.prompt.md`, and every referenced `make` target exists in `GNUmakefile`. Found real drift instead: this agent's Workflow (step 0 runtime preflight, step 4 validate) and its paired prompt (`tfplugingen-openapi-runner.prompt.md`) only referenced the legacy `make gen-api`/`make gen-framework-api` commands, even though the per-service v1 pipeline (`make bundle-spec-v1`/`make gen-api-v1`/`make gen-framework-api-v1`) is now the preferred pipeline for new targets (used by `role_v1`, `service_desk_integration_v1`, `transform_v1`). Fixed by adding pipeline-selection guidance (ask which pipeline applies if ambiguous, don't default to legacy) to both the agent workflow and the prompt, plus a `pipeline`/`service` input to the prompt.
- Validation: confirmed `gen-api`, `gen-framework-api`, `bundle-spec-v1`, `gen-api-v1`, `gen-framework-api-v1` all exist as `GNUmakefile` targets; re-parsed YAML front matter for both the agent and prompt files after editing (clean).
