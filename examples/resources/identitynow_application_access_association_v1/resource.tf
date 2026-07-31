# When mixing identitynow_application_v1 with the additive
# identitynow_application_access_association_v1 helper below, ignore direct
# application-level access_profile_ids drift on the Application resource so
# Terraform doesn't fight over the same underlying /accessProfiles list.
resource "identitynow_application_v1" "example" {
  name        = "Example Application"
  description = "Managed by Terraform."

  account_source = {
    id = "2c91808576ddc7060176de5040574aa0"
  }

  owner = {
    id   = "2c91808576ddc7060176de5040574ab0"
    type = "IDENTITY"
  }

  enabled                   = true
  provision_request_enabled = false
  app_center_enabled        = true
  match_all_accounts        = false

  lifecycle {
    ignore_changes = [access_profile_ids]
  }
}

resource "identitynow_access_profile_v1" "example_1" {
  name        = "example-access-profile-1"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  source = {
    id   = "2c9180866166b5b0016167c32ef31f3c"
    type = "SOURCE"
  }
}

resource "identitynow_access_profile_v1" "example_2" {
  name        = "example-access-profile-2"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  source = {
    id   = "2c9180866166b5b0016167c32ef31f3c"
    type = "SOURCE"
  }
}

resource "identitynow_application_access_association_v1" "example" {
  application_id = identitynow_application_v1.example.id
  access_profile_ids = [
    identitynow_access_profile_v1.example_1.id,
    identitynow_access_profile_v1.example_2.id,
  ]
}
