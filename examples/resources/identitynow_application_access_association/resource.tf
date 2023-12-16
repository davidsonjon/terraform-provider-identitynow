# Example Application resource
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

  # NOTE - when using the application_access_association resource use ignore_changes lifecycle block
  lifecycle {
    ignore_changes = [access_profile_ids]
  }
}

# Example Application Access Profile assocation data source
resource "identitynow_application_access_association" "application_access_association" {
  application_id = identitynow_application.application.id
  access_profile_ids = [
    "4c9180867817ac4d0178243bb74b2d90", "4c9180867817ac4d0178243cc9372daa"
  ]
}
