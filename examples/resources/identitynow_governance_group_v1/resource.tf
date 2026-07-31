resource "identitynow_governance_group_v1" "example" {
  name        = "example-governance-group"
  description = "Managed by Terraform."

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }
}
