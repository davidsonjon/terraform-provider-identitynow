resource "identitynow_source_v1" "example" {
  name      = "example-source"
  connector = "delimited-file"

  description = "Managed by Terraform."

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  # "connector_attributes" is connector-type-specific raw JSON - see this
  # resource's "Known Limitations & Live Testing Notes" documentation section
  # for why it's modeled as a JSON string rather than a fixed schema.
  connector_attributes = jsonencode({
    fileLocationType = "S3"
  })

  # Optional attributes below - all confirmed against a real Delimited File
  # source in a live sandbox tenant unless noted otherwise.

  # "cluster" is intentionally omitted here: it only applies to
  # "direct"-connection sources (e.g. Active Directory, SCIM, JDBC) - a
  # "file"-connection source like this Delimited File example always has
  # `cluster = null` regardless of configuration (confirmed live).

  # "account_correlation_config" cannot be set to reference an arbitrary
  # existing correlation config: live testing confirmed each config is
  # generated per-source (its own display name embeds the owning source's
  # id, e.g. "Directory [source-62867] Account Correlation" per the spec's
  # own example) and the API returns "HTTP 404, ... target resource" if you
  # try to create a new source referencing another source's correlation
  # config id. Left unset here; if you have a verified valid id scoped to
  # this exact source (e.g. captured via import after an out-of-band
  # change), it can be set the same shape as shown in the commented
  # "account_correlation_rule" alternative below.
  # account_correlation_config = {
  #   id   = "<a correlation config id scoped to this specific source>"
  #   type = "ACCOUNT_CORRELATION_CONFIG"
  # }

  # "account_correlation_rule" is a mutually exclusive alternative to
  # "account_correlation_config" above - only use one or the other. Shown
  # here commented out for illustration; do not set both in a real config.
  # account_correlation_rule = {
  #   id   = "2c9180835d2e5168015d32f8ce440e88"
  #   type = "RULE"
  # }

  # "management_workgroup" references a governance group - if it's managed
  # by Terraform too, this provider's own identitynow_governance_group_v1
  # resource can supply its id, e.g.:
  #   management_workgroup = {
  #     id   = identitynow_governance_group_v1.example.id
  #     type = "GOVERNANCE_GROUP"
  #   }
  management_workgroup = {
    id   = "74fd5c69-f542-48fe-8909-60ae705adce4"
    type = "GOVERNANCE_GROUP"
  }

  # Optional features supported by this connector - modifying this is only
  # recommended for connectors that document specific feature support.
  features = [
    "DIRECT_PERMISSIONS",
    "DISCOVER_SCHEMA",
  ]

  delete_threshold            = 10
  credential_provider_enabled = false
  # "category" is intentionally omitted - it's Computed-only. Live testing
  # confirmed the API silently ignores any configured value (every source in
  # this tenant returns category = null regardless of connector type), so
  # Terraform can only read it, never set it.
}

