resource "identitynow_source_v1" "example" {
  name      = "example-source"
  connector = "active-directory"

  connector_attributes = jsonencode({
    host = "ad.example.com"
  })

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }
}

# Manual, one-off invocation:
#   terraform apply -invoke=action.identitynow_aggregate_accounts.example
action "identitynow_aggregate_accounts" "example" {
  config {
    source_id = identitynow_source_v1.example.id
    timeout   = "45m"
  }
}

# Recommended: bind the action to the account schema's lifecycle instead of
# triggering it manually or off unrelated source attribute changes. Account
# schema changes (added/removed attributes, correlation keys, etc.) are the
# natural signal that a re-aggregation is needed; binding directly to
# identitynow_source_v1's after_update would also fire on incidental changes
# (e.g. a description edit) that don't affect account data.
resource "identitynow_source_schema_v1" "accounts" {
  source_id          = identitynow_source_v1.example.id
  name               = "account"
  native_object_type = "User"
  identity_attribute = "sAMAccountName"
  display_attribute  = "sAMAccountName"

  attributes = [
    {
      name           = "sAMAccountName"
      native_name    = "sAMAccountName"
      type           = "STRING"
      description    = "Account ID"
      is_multi       = false
      is_entitlement = false
      is_group       = false
    },
  ]

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.identitynow_aggregate_accounts.example]
    }
  }
}
