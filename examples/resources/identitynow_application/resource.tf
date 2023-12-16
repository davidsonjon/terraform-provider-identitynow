# Example Source data source
data "identitynow_source" "source" {
  id = "3c91808677e0502f0177eee68e943f6f"
}

# Example Access Profile data source
data "identitynow_access_profile" "access_profile" {
  id = "3c9180847817ac4f0178221df4391f75"
}

# Example Identity data source
data "identitynow_identity" "identity" {
  id = "4072631"
}

# Example Applicaiton resource
resource "identitynow_application" "application" {
  name        = "example terraform application"
  description = "example application"

  owner = {
    id = data.identitynow_identity.identity.cc_id
  }
  account_service_id        = data.identitynow_source.source.connector_attributes.cloud_external_id
  launchpad_enabled         = true
  app_center_enabled        = false
  provision_request_enabled = false
  access_profile_ids = [
    data.identitynow_access_profile.access_profile.id,
  ]
}
