resource "identitynow_service_desk_integration_v1" "example" {
  name        = "example-sdi"
  description = "Managed by Terraform."
  type        = "ServiceNowSDIM"

  # NOTE: the OpenAPI spec only lists name/description/type/attributes as
  # required, but live testing against a real tenant showed the "ServiceNowSDIM"
  # (and "Generic SDIM") connector types actually reject Create without
  # cluster_ref set - include it even though the schema treats it as optional.
  cluster_ref = {
    id = "2c91808576ddc7060176de5040574ac0"
  }

  attributes = {}
}
