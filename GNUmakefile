TARGET = base
SERVICE = $(TARGET)

# ./api-specs/ is git-tracked in this repo but only holds specs/outputs this
# project has produced or modified itself (currently just the per-service v1
# bundled+dereferenced outputs under api-specs/dereferenced/deref-<service>.v1.yaml).
# It is NOT a copy of the full upstream github.com/sailpoint-oss/api-specs
# tree. Raw/unmodified upstream specs are read directly from an external,
# separately git-managed checkout of that repo via API_SPECS_SOURCE.
#
# Portable default: a sibling directory next to this repo checkout
# (../api-specs, e.g. .../wip/api-specs alongside .../wip/terraform-provider-identitynow),
# matching the layout used by every contributor/session so far - but this is
# only a convenience default, not a requirement. If you don't have that
# sibling clone (or it's somewhere else), override per-invocation:
#   API_SPECS_SOURCE=/path/to/your/sailpoint-oss/api-specs/clone make bundle-spec-v1 ...
# Targets that actually need this (bundle-spec-v1, gen-api) fail fast with a
# clear error if the resolved path doesn't exist, rather than silently
# passing a broken path to a downstream tool. See
# .github/agents/identitynow-terraform-provider-developer.agent.md for the
# full pipeline this feeds into, and api-specs/README.md for what this repo's
# own ./api-specs/ directory holds instead.
API_SPECS_SOURCE ?= $(abspath $(CURDIR)/../api-specs)

# check-api-specs-source is a prerequisite for any target that reads directly
# from $(API_SPECS_SOURCE) (the external upstream spec clone) - it is NOT
# needed by targets that only read this repo's own committed
# api-specs/dereferenced/*.yaml output (gen-api-v1, gen-framework-api-v1,
# etc.), since those are self-contained once bundled. See the Project
# Context's "one-time API_SPECS_SOURCE dependency" note in the IdentityNow
# agent file.
check-api-specs-source:
	@test -d "$(API_SPECS_SOURCE)" || { \
		echo "ERROR: API_SPECS_SOURCE ($(API_SPECS_SOURCE)) does not exist."; \
		echo "This target reads the raw upstream github.com/sailpoint-oss/api-specs checkout directly - it is not committed in this repo."; \
		echo "Clone it locally, then either:"; \
		echo "  - place it at $(abspath $(CURDIR)/../api-specs) (the default sibling-directory convention), or"; \
		echo "  - override explicitly: API_SPECS_SOURCE=/path/to/your/clone make $(MAKECMDGOALS)"; \
		exit 1; \
	}

default: fmt lint install generate

gen-api: check-api-specs-source
	tfplugingen-openapi generate \
	--config generator_config/generator_config_$(TARGET).yml \
	--output openapi_code_spec/openapi_code_spec_$(TARGET).json \
	$(API_SPECS_SOURCE)/dereferenced/deref-sailpoint-api.beta.yaml

gen-framework-api:
	tfplugingen-framework generate resources \
	--input openapi_code_spec/openapi_code_spec_$(TARGET).json \
	--output internal/provider/$(TARGET)
	tfplugingen-framework generate data-sources \
	--input openapi_code_spec/openapi_code_spec_$(TARGET).json \
	--output internal/provider/$(TARGET)

# --- Per-service v1 pipeline ($(API_SPECS_SOURCE)/idn/apis/<service>) ---
# TARGET matches generator_config_<target>_v1.yml (underscores, e.g. service_desk_integration)
# SERVICE matches the $(API_SPECS_SOURCE)/idn/apis/<service> folder name (dashes, e.g. service-desk-integration)
# Override both when they differ, e.g. make bundle-spec-v1 TARGET=service_desk_integration SERVICE=service-desk-integration
# The raw per-service openapi.yaml is read directly from the external
# API_SPECS_SOURCE checkout; only the bundled+dereferenced *output* (this
# project's own derived artifact) is written locally to
# api-specs/dereferenced/ and committed.
bundle-spec-v1: check-api-specs-source
	npx --yes @apidevtools/swagger-cli bundle --dereference \
	$(API_SPECS_SOURCE)/idn/apis/$(SERVICE)/openapi.yaml \
	-t yaml \
	-o api-specs/dereferenced/deref-$(SERVICE).v1.yaml

gen-api-v1:
	tfplugingen-openapi generate \
	--config generator_config/generator_config_$(TARGET)_v1.yml \
	--output openapi_code_spec/openapi_code_spec_$(TARGET)_v1.json \
	api-specs/dereferenced/deref-$(SERVICE).v1.yaml

gen-framework-api-v1:
	tfplugingen-framework generate resources \
	--input openapi_code_spec/openapi_code_spec_$(TARGET)_v1.json \
	--output internal/provider/$(TARGET)_v1
	tfplugingen-framework generate data-sources \
	--input openapi_code_spec/openapi_code_spec_$(TARGET)_v1.json \
	--output internal/provider/$(TARGET)_v1

gen-framework:
	tfplugingen-framework generate resources \
	--input custom_code_spec/provider_code_spec_$(TARGET).json \
	--output internal/provider/$(TARGET)
	tfplugingen-framework generate data-sources \
	--input custom_code_spec/provider_code_spec_$(TARGET).json \
	--output internal/provider/$(TARGET)

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

# tflint runs the core `terraform` ruleset (no cloud-provider-specific
# ruleset exists for this custom IdentityNow provider) against examples/ —
# the only committed Terraform HCL in this repo (test/**/main.tf is
# gitignored; validate that manually via `make plan TARGET=<folder>`).
# Requires `tflint` on PATH (https://github.com/terraform-linters/tflint).
tflint:
	tflint --init --config "$(CURDIR)/.tflint.hcl"
	tflint --recursive --config "$(CURDIR)/.tflint.hcl" --chdir examples/

# validate-examples runs `terraform validate` against every
# examples/{resources,data-sources}/<name>/*.tf snippet, using a freshly
# built local provider binary via dev_overrides (no registry/network access,
# no real tenant credentials needed — validate never calls the API). See
# scripts/validate-examples.sh for details.
validate-examples:
	./scripts/validate-examples.sh

generate: fmt docs

# docs runs tfplugindocs to (re)generate ./docs from the provider's schema
# (descriptions in resp.Schema.Description/MarkdownDescription in the
# hand-written Schema() wrappers), ./examples (real, sanitized HCL snippets -
# no live credentials), and ./templates (custom per-resource/data-source
# pages with "Known Limitations & Live Testing Notes" sections distilled from
# terraform apply/testacc runs against a real sandbox tenant - see
# .github/agents/identitynow-terraform-provider-developer.knowledge.md).
docs:
	go tool tfplugindocs generate --rendered-provider-name "identitynow"

fmt:
	gofmt -s -w -e .
	terraform fmt -recursive examples

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

# plan runs `terraform plan` against a self-contained manual test config under
# test/$(TARGET) (see docs/TESTING.md), after building and installing the
# provider so ~/.terraformrc dev_overrides pick up the latest local build.
# Example: make plan TARGET=service_desk_integration
plan: install
	cd test/$(TARGET); terraform plan

# apply runs `terraform apply` against the same self-contained manual test
# config as `plan`, after building/installing the provider. Use with caution -
# this makes live create/update/delete calls against whatever tenant is
# configured in test/$(TARGET)/main.tf. Example: make apply TARGET=role
apply: install
	cd test/$(TARGET); terraform apply

.PHONY: fmt lint tflint validate-examples test testacc build install generate docs plan apply check-api-specs-source
