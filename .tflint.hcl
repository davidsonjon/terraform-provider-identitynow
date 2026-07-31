# tflint configuration for this repo.
#
# Scope: lints examples/**/*.tf (the only committed Terraform HCL in this
# repo — test/**/main.tf is gitignored, see .gitignore, and is validated
# locally/manually via `make plan TARGET=<folder>` instead).
#
# No provider-specific tflint ruleset plugin exists for this custom
# IdentityNow provider (rulesets are only published for major cloud
# providers - aws/azurerm/google/etc.), so only the core `terraform`
# ruleset runs here: deprecated syntax, unused declarations, naming
# conventions, and other provider-agnostic HCL checks.
config {
  format = "compact"
  call_module_type = "none"
}

plugin "terraform" {
  enabled = true
  preset  = "recommended"
}

# These files are intentionally standalone documentation snippets (one
# resource/data-source block each, no `terraform`/`required_providers` block
# of their own — see examples/provider/provider.tf for the shared one, and
# scripts/validate-examples.sh for how they're schema-validated), matching
# the HashiCorp tfplugindocs convention. Disable the rules that only make
# sense for a real, standalone root module; keep everything else (deprecated
# syntax, naming conventions, comment style) since those catch real
# documentation mistakes.
rule "terraform_required_version" {
  enabled = false
}
rule "terraform_required_providers" {
  enabled = false
}
rule "terraform_unused_declarations" {
  enabled = false
}
