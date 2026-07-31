---
description: "Review openapi_code_spec_<target>.json (legacy) or openapi_code_spec_<target>_v1.json (per-service v1) and add associated_external_type/custom_type mappings"
agent: tfplugingen-openapi-type-reviewer
model: GPT-5 (copilot)
tools: [execute, read, search, edit, todo]
argument-hint: "target=<name> pipeline=<legacy|v1> run-framework=<true|false>"
---
Inputs:
- target: ${input:target}
- pipeline: ${input:pipeline} (legacy or v1; ask if not provided and ambiguous - v1 is preferred for new targets)
- run-framework: ${input:run_framework}

Execution policy:
1. Review `openapi_code_spec/openapi_code_spec_${input:target}.json` (legacy) or `openapi_code_spec/openapi_code_spec_${input:target}_v1.json` (per-service v1), per `pipeline`.
2. Add conservative `associated_external_type` mappings first, then `custom_type` only when justified. Run the mandatory Go symbol-collision scan (agent Workflow step 2a) every pass, even when zero mapping changes are made.
3. Validate JSON.
4. If `${input:run_framework}` is `true`, run `make gen-framework-api TARGET=${input:target}` (legacy) or `make gen-framework-api-v1 TARGET=${input:target}` (v1).
5. Return changed JSON paths with evidence for each mapping.
