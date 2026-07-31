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
- Symptom: `make gen-api-v1 TARGET=transform SERVICE=transforms` succeeded (exit 0) but emitted `WARN msg="skipping mapping of create/read operation response body"` / `"skipping data source schema mapping"` with `err="found 2 allOf subschema(s), schema composition is currently not supported"` - the resulting `openapi_code_spec_transform_v1.json` had a resource with NO attributes read from the response at all (would have silently produced an empty/near-empty generated schema if not caught).
- Root cause: unlike `service_desk_integration`'s ~37 `allOf` occurrences (all nested inside sub-schemas, so only those specific sub-objects lost fidelity), `transforms`' spec wraps the *entire top-level* response schema for create/get/put (and each list item) in a 2-member `allOf` (`[{title: Transform, properties: {name,type,attributes,...}}, {required: [id,internal], properties: {id,internal}}]`). `tfplugingen-openapi` cannot decompose `allOf` schema composition at all (confirmed both here and in the pre-existing troubleshooter Playbook B note) - when the *entire* response is behind `allOf` (not just a nested field), the tool skips mapping the whole operation's response body, not just one field.
- Fix pattern: manual/inline hand-editing (Playbook B's normal narrow-flatten approach) is impractical here because each `allOf` member's sibling content is huge (the `attributes` property alone is a ~3000-line `oneOf` across 35 transform types) - reindenting that by hand via `sed`/`edit` is high-risk. Instead, wrote a small one-off Python script (using `pyyaml`, `yaml.safe_load`/`yaml.dump(..., sort_keys=False)`) that: (1) parses the whole bundled `api-specs/dereferenced/deref-<service>.v1.yaml`, (2) recursively walks every dict looking for an `allOf` key whose 2-member list matches this exact base-object + `{id,internal}`-wrapper shape (matched structurally via presence of the `id`/`internal`/`name`/`type`/`attributes` keys, not by node path/line number, so it's robust to the exact line positions), (3) merges the two subschemas' `properties`/`required` into one flat object schema (dropping any field the generator config was going to `schema.ignores` anyway, e.g. `attributes`, from the merged `required` list, so a hand-added custom field never appears in a place the generator would complain about a required-but-undeclared attribute), (4) deletes the `allOf` key and replaces the node's content with the merged schema in place, (5) re-dumps the entire file. Ran this once, directly against the *bundled/dereferenced* output (not the raw upstream spec - that stays untouched in the externally-managed `API_SPECS_SOURCE` clone), then re-ran `make gen-api-v1` and confirmed zero `allOf`/warning output and a correctly populated code spec.
- Validation command: `grep -c allOf api-specs/dereferenced/deref-transforms.v1.yaml` (0 after the fix, 4 before) then `make gen-api-v1 TARGET=transform SERVICE=transforms` (clean, no WARN lines) then `python3 -m json.tool openapi_code_spec/openapi_code_spec_transform_v1.json` to visually confirm `id`/`internal`/`name`/`type` all present in both the resource and data source schemas.
- Guardrail update: when a `SchemaCompatibility` failure's WARN/ERROR text says "skipping mapping of ... response body" (as opposed to a narrower per-field skip), check whether the `allOf` is at the *entire operation response* level rather than nested inside one sub-field - if so, prefer a scripted structural merge (pyyaml walk-and-merge, matched by sibling-key shape rather than brittle line numbers) over manual `sed`/`edit` reindentation, since the surrounding content is often too large to safely hand-edit. This scripted flattening must be re-applied any time `make bundle-spec-v1` is re-run for that service (bundling does not preserve hand-flattened `allOf` - it always re-derives from the raw upstream spec), so note this explicitly in the target's hand-written CRUD package doc comment (see `transform_v1`'s package doc) as a forward-compatibility trap for whoever next re-syncs that service's spec.


- Date: 2026-07-25
- Task: review/cleanup pass across all agents/prompts
- Summary: No unused/orphaned agent, knowledge, or prompt files found - all 3 agent pairs (`identitynow-terraform-provider-developer`, `tfplugingen-openapi-troubleshooter`, `tfplugingen-openapi-type-reviewer`) have exactly one matching `.knowledge.md` and one matching `.prompt.md`, and every referenced `make` target exists in `GNUmakefile`. Found real drift instead: this agent's Workflow (step 0 runtime preflight, step 4 validate) and its paired prompt (`tfplugingen-openapi-runner.prompt.md`) only referenced the legacy `make gen-api`/`make gen-framework-api` commands, even though the per-service v1 pipeline (`make bundle-spec-v1`/`make gen-api-v1`/`make gen-framework-api-v1`) is now the preferred pipeline for new targets (used by `role_v1`, `service_desk_integration_v1`, `transform_v1`). Fixed by adding pipeline-selection guidance (ask which pipeline applies if ambiguous, don't default to legacy) to both the agent workflow and the prompt, plus a `pipeline`/`service` input to the prompt.
- Validation: confirmed `gen-api`, `gen-framework-api`, `bundle-spec-v1`, `gen-api-v1`, `gen-framework-api-v1` all exist as `GNUmakefile` targets; re-parsed YAML front matter for both the agent and prompt files after editing (clean).
