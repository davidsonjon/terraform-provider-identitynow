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
#   terraform apply -invoke=action.identitynow_aggregate_entitlements.example
action "identitynow_aggregate_entitlements" "example" {
  config {
    source_id = identitynow_source_v1.example.id
    timeout   = "45m"
  }
}

# Recommended: bind the action to the *group*-type schema's lifecycle (the
# schema whose attributes actually populate entitlement data), not to the
# account-type schema or to identitynow_source_v1 directly.
resource "identitynow_source_schema_v1" "groups" {
  source_id          = identitynow_source_v1.example.id
  name               = "group"
  native_object_type = "Group"
  identity_attribute = "distinguishedName"
  display_attribute  = "name"

  attributes = [
    {
      name           = "distinguishedName"
      native_name    = "distinguishedName"
      type           = "STRING"
      description    = "Group DN"
      is_multi       = false
      is_entitlement = false
      is_group       = false
    },
  ]

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.identitynow_aggregate_entitlements.example]
    }
  }
}
