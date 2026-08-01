# A Schema defines the attributes IdentityNow aggregates for a given
# object type (account or, for group/hierarchical sources, entitlement) on
# a source. Composite id = source_id + schema_id (this resource's own "id"
# and "schema_id" outputs are always equal - see "Known Limitations & Live
# Testing Notes").
resource "identitynow_source_schema_v1" "example" {
  source_id          = "9e99be10dcf24aa9bbe83902dece8738"
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
    {
      name           = "memberOf"
      native_name    = "memberOf"
      type           = "STRING"
      description    = "Group membership"
      is_multi       = true
      is_entitlement = true
      is_group       = true
      schema = {
        id   = "2c91808a7813090a017814121919ffff"
        name = "group"
        type = "CONNECTOR_SCHEMA"
      }
    },
  ]

  # "configuration" is a free-form JSON object with no fixed set of keys -
  # see this resource's "Known Limitations & Live Testing Notes"
  # documentation section for the full rationale.
  configuration = jsonencode({})
}
