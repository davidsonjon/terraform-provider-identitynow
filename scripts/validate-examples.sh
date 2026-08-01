#!/usr/bin/env bash
# validate-examples.sh — runs `terraform validate` against every
# examples/resources/<name>/resource.tf and examples/data-sources/<name>/data-source.tf
# snippet in this repo.
#
# Each example file is a documentation snippet (no `terraform`/`provider` block
# of its own — see examples/provider/provider.tf for the shared one), so this
# script assembles a scratch directory per example containing a copy of
# examples/provider/provider.tf plus the single example file, then runs
# `terraform validate` against that scratch directory using a throwaway
# CLI config that dev_overrides the provider address to a freshly built local
# binary. This avoids needing network access to the Terraform Registry and
# avoids needing real IdentityNow tenant credentials (validate never contacts
# the tenant — it only checks HCL syntax and schema-level attribute
# correctness against the provider's own schema).
#
# Usage: scripts/validate-examples.sh
# Requires: `terraform` on PATH, and the provider already built via
# `make build` (this script builds it itself into a scratch GOBIN if not
# already present, so it also works standalone in CI).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Must match the `source` used in examples/provider/provider.tf (the real
# published registry address, since that file is also what's rendered on the
# Terraform Registry docs) - NOT the "local.dev/identitynow/identitynow"
# placeholder used by test/*/main.tf for local manual `make plan`/`apply`
# testing (see docs/TESTING.md), which is a separate, unrelated convention.
PROVIDER_ADDRESS="davidsonjon/identitynow"
SCRATCH_ROOT="$(mktemp -d)"
BIN_DIR="$SCRATCH_ROOT/bin"
mkdir -p "$BIN_DIR"

cleanup() {
  rm -rf "$SCRATCH_ROOT"
}
trap cleanup EXIT

echo "==> Building provider binary for validation (scratch GOBIN: $BIN_DIR)"
GOBIN="$BIN_DIR" go install ./...

CLI_CONFIG="$SCRATCH_ROOT/dev.tfrc"
cat > "$CLI_CONFIG" <<EOF
provider_installation {
  dev_overrides {
    "$PROVIDER_ADDRESS" = "$BIN_DIR"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$CLI_CONFIG"

fail=0
count=0

validate_dir() {
  local example_dir="$1"
  local example_file="$2"

  local name
  name="$(basename "$example_dir")"
  local work_dir="$SCRATCH_ROOT/work/$name-$(basename "$example_file" .tf)"
  mkdir -p "$work_dir"
  cp "$REPO_ROOT/examples/provider/provider.tf" "$work_dir/"
  cp "$example_file" "$work_dir/"

  # provider.tf references var.sail_* (real values come from env/CI secrets
  # at apply time) — declare them here as empty-default strings purely so
  # `terraform validate` can resolve the reference; validate never uses the
  # actual values since it makes no API calls.
  cat > "$work_dir/variables.tf" <<'VARSEOF'
variable "sail_base_url" {
  type    = string
  default = "https://example.api.identitynow.com"
}
variable "sail_client_id" {
  type    = string
  default = "validate-placeholder"
}
variable "sail_client_secret" {
  type      = string
  default   = "validate-placeholder"
  sensitive = true
}
VARSEOF

  count=$((count + 1))
  echo "==> terraform validate: $example_file"
  if ! (cd "$work_dir" && terraform validate -no-color); then
    echo "    FAILED: $example_file"
    fail=1
  fi
}

shopt -s nullglob
for dir in examples/resources/*/; do
  for f in "$dir"*.tf; do
    validate_dir "$dir" "$f"
  done
done
for dir in examples/data-sources/*/; do
  for f in "$dir"*.tf; do
    validate_dir "$dir" "$f"
  done
done

echo
echo "Validated $count example file(s)."
if [ "$fail" -ne 0 ]; then
  echo "One or more examples failed terraform validate." >&2
  exit 1
fi
echo "All examples passed terraform validate."
