---
description: "Run make-based tfplugingen-openapi troubleshooting (legacy or per-service v1 pipeline) using target and live terminal output"
agent: tfplugingen-openapi-troubleshooter
model: GPT-5 (copilot)
tools: [execute, read, search, edit, todo]
argument-hint: "target=<name> run-framework=<true|false> pipeline=<legacy|v1> service=<service-folder (v1 only)>"
---
Use agent `tfplugingen-openapi-troubleshooter`.

Inputs:
- target: ${input:target}
- run-framework: ${input:run_framework}
- pipeline: ${input:pipeline} (legacy or v1; ask if not provided and ambiguous - v1 is preferred for new targets)
- service: ${input:service} (per-service v1 folder name under api-specs/idn/apis/, required when pipeline=v1)

Execution policy:
1. If `pipeline` is `v1`: run `make bundle-spec-v1 TARGET=${input:target} SERVICE=${input:service}` then `make gen-api-v1 TARGET=${input:target} SERVICE=${input:service}` from repo root and capture the active terminal output. Otherwise (legacy): run `make gen-api TARGET=${input:target}`.
2. If `${input:run_framework}` is `true`, run the matching framework-generation command after the OpenAPI-generation step succeeds (`make gen-framework-api-v1 TARGET=${input:target}` for v1, `make gen-framework-api TARGET=${input:target}` for legacy).
3. If either command fails, troubleshoot using the agent playbooks and apply minimal fixes.
4. Re-run failed command(s) to validate.
5. Append reinforcement notes to `.github/agents/tfplugingen-openapi-troubleshooter.knowledge.md`.

Return format:
1. Diagnosis
2. Fixes applied
3. Validation commands and outcomes
4. Reinforcement entry added
