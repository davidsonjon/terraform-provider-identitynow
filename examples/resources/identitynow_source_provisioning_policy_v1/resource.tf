# A Provisioning Policy defines which fields are requested/set for a given
# usage type (e.g. CREATE, UPDATE, ENABLE, DISABLE) when IdentityNow
# provisions to this source. usage_type + source_id together form this
# resource's identity - the API has no separate real "id" of its own (see
# "Known Limitations & Live Testing Notes" for why this uses v1, not v2,
# endpoints).
resource "identitynow_source_provisioning_policy_v1" "example" {
  source_id   = "9e99be10dcf24aa9bbe83902dece8738"
  usage_type  = "CREATE"
  name        = "Create Account"
  description = "Managed by Terraform."

  # "fields" is connector/usage-type-specific and can itself nest a full
  # transform definition per field - modeled as a raw JSON string (see this
  # resource's "Known Limitations & Live Testing Notes" documentation
  # section for the full rationale, matching identitynow_transform_v1's
  # "attributes" and identitynow_source_v1's "connector_attributes").
  fields = jsonencode([
    {
      name          = "userName"
      transform     = { type = "accountAttribute", attributes = { attributeName = "userName", sourceName = "Employees" } }
      attributes    = {}
      isRequired    = true
      type          = "string"
      isMultiValued = false
    },
  ])
}
