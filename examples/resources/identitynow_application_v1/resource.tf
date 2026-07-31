resource "identitynow_application_v1" "example" {
  name        = "Example Application"
  description = "Managed by Terraform."

  # account_source.id is Required (matches the API's SourceAppCreateDto.accountSource.id).
  account_source = {
    id = "2c91808576ddc7060176de5040574aa0"
  }

  # owner is Optional+Computed: the create API doesn't accept it, so the
  # provider sets it via a follow-up PATCH immediately after create. Omit it
  # to let IdentityNow assign a default owner.
  owner = {
    id   = "2c91808576ddc7060176de5040574ab0"
    type = "IDENTITY"
  }

  enabled                   = true
  provision_request_enabled = false
  app_center_enabled        = true
  match_all_accounts        = false

  # access_profile_ids is hand-added (not part of the source-app GET/POST
  # response) - it is read via a separate access-profiles list call and
  # written via a JSON Patch "replace" on /accessProfiles.
  access_profile_ids = [
    "2c91808576ddc7060176de5040574ac0",
  ]
}
