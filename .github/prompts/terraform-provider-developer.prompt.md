---
description: "Drive the generic OpenAPI-codegen-based Terraform provider pipeline end-to-end for a vendor that has no dedicated specialist agent yet, or author/review the vendor-agnostic base agent itself"
agent: terraform-provider-developer
model: GPT-5 (copilot)
tools: [execute, read, search, edit, todo]
argument-hint: "target=<name> task=<pipeline|review|author-agent|author-prompt|author-skill>"
---
Inputs:
- target: ${input:target}
- task: ${input:task}

Execution policy:
1. If `task` is `pipeline`:
   - Confirm whether a vendor-specific specialist agent already exists for this provider (e.g. `<vendor>-terraform-provider-developer.agent.md`). If one exists, prefer invoking it directly instead of this generic prompt - it will carry the exact command names/paths this generic file only describes abstractly.
   - If no vendor specialist exists yet, either proceed directly using this file's generic pipeline-stage descriptions (stages 1-9 in the agent file), substituting this repo's actual build-tool commands/paths as you discover them, or pause to author a new vendor specialist first (see `task=author-agent`) so vendor-specific facts land in the right place for future sessions.
   - Run Phase A (Offline-safe validation) in full regardless of live access. Run Phase B (Live verification) only if credentials/a real test configuration are available; otherwise leave it as an explicit pending task in your response rather than blocking or fabricating a result.
2. If `task` is `review`:
   - Read this agent file, its knowledge file, and every vendor-specific specialist's `.agent.md`/`.knowledge.md`/`.prompt.md` file.
   - Verify every referenced build command, file path, and tool name still exists in the repo.
   - Verify no vendor-specific fact has crept into this generic file, and no genuinely vendor-agnostic pattern is duplicated across multiple vendor specialists instead of living here.
   - Unless the invocation explicitly asks for a **read-only** review (e.g. "review and report, don't edit" or a cross-model second-opinion pass), apply minimal, surgical fixes for any drift found and report items intentionally left unchanged. If read-only was requested, make no edits at all — report findings only, organized by category (consistency/staleness/redundancy/gaps/clarity/knowledge-hygiene or similar), with specific file:line citations.
3. If `task` is `author-agent`, `author-prompt`, or `author-skill`:
   - Confirm no existing agent/prompt/skill (generic or vendor-specific) already covers the requested scope.
   - Create a narrowly-scoped `.agent.md` (with Mission/Scope/Non-Goals/Workflow/Enforcement Rules/Reinforcement Loop/Response Format), a paired `.knowledge.md` with an Entry Template, and a matching `.prompt.md`, all in the same change.
   - If the new agent is vendor-specific, have it explicitly compose this generic file (read it first, supply only deltas) rather than duplicating pipeline/validation/enforcement content.
4. Append a reinforcement entry to `.github/agents/terraform-provider-developer.knowledge.md` (for vendor-agnostic findings) and/or the relevant vendor specialist's knowledge file (for vendor-specific findings) summarizing the run (skip only for an explicitly read-only review pass that made no repo changes — note that explicitly instead).

Return format (edit-mode `pipeline`/`review`/`author-*`):
1. Diagnosis/Objective
2. Changes made (files touched)
3. Validation commands and outcomes (Phase A / Phase B split where applicable)
4. Delegation summary
5. Reinforcement entry added

Return format (read-only `review`, no edits made):
1. Objective/scope of the review
2. Findings by category, each with file:line citations
3. Explicit confirmation that no edits were made
4. Recommended next actions for a human or a follow-up edit-mode pass to act on
