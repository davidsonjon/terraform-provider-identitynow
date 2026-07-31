# NOTE: reading a *pre-existing* service desk integration whose
# provisioningConfig.managedResourceRefs is non-empty currently fails due to an
# upstream golang-sdk bug (see the resource documentation's "Live Testing
# Notes" section) - the entries' type/id/name fields are mistyped as
# map[string]interface{} instead of string, so the SDK cannot decode the real
# API response for such objects. This does not affect newly created
# integrations, which start with no managedResourceRefs.
data "identitynow_service_desk_integration_v1" "example" {
  id = "2c91808576ddc7060176de5040574ac0"
}
