---
description: "Drive the IdentityNow Terraform provider pipeline end-to-end, or author/review the agents, prompts, and skills that support this codebase"
agent: identitynow-terraform-provider-developer
model: GPT-5 (copilot)
tools: [execute, read, search, edit, todo]
argument-hint: "target=<name> task=<pipeline|review|author-agent|author-prompt|author-skill>"
---
Inputs:
- target: ${input:target}
- task: ${input:task}

Execution policy:
1. If `task` is `pipeline`:
   - Determine which pipeline applies: legacy monolithic vs. per-service v1 (preferred for new targets) — ask if ambiguous, do not default to legacy.
   - Legacy: run `make gen-api TARGET=${input:target}`. Per-service v1: run `make bundle-spec-v1 TARGET=${input:target} SERVICE=<service-folder>` (**skip this if `api-specs/dereferenced/deref-<service>.v1.yaml` already exists and you're not explicitly refreshing from upstream** — see the agent's Project Context bullet on one-time `API_SPECS_SOURCE` dependency), then `make gen-api-v1 TARGET=${input:target} SERVICE=<service-folder>`. On failure, delegate to `tfplugingen-openapi-troubleshooter`, then re-run.
   - If needed, apply schema post-processing (`generator_config/schema_overrides_<target>_v1.yml` via `scripts/apply_codespec_schema_overrides.py`, and/or `scripts/flatten_openapi_allof.py` for metadata-only `allOf` wrappers) before/alongside type linking.
   - Delegate type-mapping review of `openapi_code_spec/openapi_code_spec_${input:target}.json` (legacy) or `openapi_code_spec/openapi_code_spec_${input:target}_v1.json` (v1) to `tfplugingen-openapi-type-reviewer`; persist its output as `generator_config/type_mappings_<target>_v1.yml` and apply via `scripts/apply_codespec_type_mappings.py` — never hand-patch the generated code-spec JSON directly.
   - Legacy: run `make gen-framework-api TARGET=${input:target}`. V1: run `make gen-framework-api-v1 TARGET=${input:target}` (schema/model only — CRUD is hand-written next). On failure, loop back to the prior steps or fix the schema-overrides/type-mappings config, never the generated JSON/Go output directly.
   - For v1 targets: hand-write CRUD (see the agent's per-service v1 pipeline step 6) and wire into `internal/provider/provider.go`.
   - Run Offline Validation (Phase A — `go build`/`go vet`/`go test ./...`, `make lint`, `make docs`, `make validate-examples`, `make tflint`) in full, every time, regardless of live access.
   - If a `test/<folder>/main.tf` with real sandbox credentials (`test/.env`) is available, run Live Verification (Phase B — `make plan TARGET=<folder>`, and `make apply`/direct-API cross-check for association/sub-resource targets, with explicit user confirmation first). Otherwise leave Phase B as an explicit pending item rather than blocking.
2. If `task` is `review`:
   - Treat this as a fresh invocation with no prior conversation memory (per the base agent's fresh-context review discipline) — re-derive every factual claim from the current repo state rather than trusting earlier-session assumptions.
   - Read `.github/agents/terraform-provider-developer.agent.md` (the vendor-agnostic base agent) FIRST, then all existing `.github/agents/*.agent.md`, `*.knowledge.md`, and `.github/prompts/*.prompt.md`.
   - Verify every referenced `make` target, file path, and tool name still exists in the repo.
   - Verify this file's IdentityNow-specific content stays cleanly separated from the base agent: no generic codegen-pipeline-shape/authoring-convention prose duplicated here that belongs there, and no IdentityNow-specific fact (SDK types, spec URLs, exact command names) missing here that got incorrectly generalized into the base agent.
   - Unless the invocation explicitly asks for a **read-only** review (e.g. "review and report, don't edit" or a cross-model second-opinion pass), apply minimal, surgical fixes for any drift found and report items intentionally left unchanged. If read-only was requested, make no edits at all — report findings only, organized by category (consistency/staleness/redundancy/gaps/clarity/knowledge-hygiene or similar), with specific file:line citations.
3. If `task` is `author-agent`, `author-prompt`, or `author-skill`:
   - Confirm no existing agent/prompt/skill already covers the requested scope.
   - Create a narrowly-scoped `.agent.md` (with Mission/Scope/Non-Goals/Workflow/Enforcement Rules/Reinforcement Loop/Response Format), a paired `.knowledge.md` with an Entry Template, and a matching `.prompt.md`, all in the same change.
4. Append a reinforcement entry to `.github/agents/identitynow-terraform-provider-developer.knowledge.md` summarizing the run (skip only for an explicitly read-only review pass that made no repo changes — note that explicitly instead).

Return format (edit-mode `pipeline`/`review`/`author-*`):
1. Diagnosis/Objective
2. Changes made (files touched)
3. Validation commands and outcomes
4. Delegation summary
5. Reinforcement entry added

Return format (read-only `review`, no edits made):
1. Objective/scope of the review
2. Findings by category, each with file:line citations
3. Explicit confirmation that no edits were made
4. Recommended next actions for a human or a follow-up edit-mode pass to act on
