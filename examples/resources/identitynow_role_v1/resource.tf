resource "identitynow_role_v1" "example" {
  name        = "example-role"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  # owner is Required (matches the API's required: [name, owner]).
  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }
}
