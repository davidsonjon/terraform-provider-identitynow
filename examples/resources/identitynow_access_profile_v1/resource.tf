resource "identitynow_access_profile_v1" "example" {
  name        = "example-access-profile"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  # owner and source are both Required (matches the API's required:
  # [name, owner, source]).
  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  source = {
    id   = "2c9180866166b5b0016167c32ef31f3c"
    type = "SOURCE"
  }
}
