resource "identitynow_identity_profile_v1" "example" {
  name        = "Employees"
  description = "Managed by Terraform."

  # "owner" references an identity that governs this profile. If the owning
  # identity is also referenced elsewhere in this configuration (e.g. via the
  # read-only identitynow_identity_v1 data source, once implemented), its id
  # can be supplied the same way as any other reference attribute here.
  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  # "authoritative_source" binds this profile to the source that aggregates
  # identities into it - almost always required by the live API even though
  # the schema itself only forces the surrounding object, not its nested
  # "id" (see this resource's "Known Limitations & Live Testing Notes" for
  # the full spec-vs-API mismatch discussion). Reference an
  # identitynow_source_v1 resource's id directly if it's also Terraform
  # managed:
  #   authoritative_source = {
  #     id   = identitynow_source_v1.example.id
  #     type = "SOURCE"
  #   }
  authoritative_source = {
    id   = "2c9180857756ddc70177574aa0576ab1"
    type = "SOURCE"
  }

  priority = 10

  # "identity_attribute_config" is a raw JSON object - see this resource's
  # "Known Limitations & Live Testing Notes" documentation section for why
  # it's modeled as a JSON string (each attributeTransforms entry's
  # transformDefinition.attributes shape depends on the sibling
  # transformDefinition.type, exactly like identitynow_transform_v1's
  # "attributes").
  identity_attribute_config = jsonencode({
    enabled = true
    attributeTransforms = [
      {
        identityAttributeName = "email"
        transformDefinition = {
          type = "accountAttribute"
          attributes = {
            sourceName    = "Employees"
            attributeName = "mail"
          }
        }
      },
      {
        identityAttributeName = "displayName"
        transformDefinition = {
          type = "accountAttribute"
          attributes = {
            sourceName    = "Employees"
            attributeName = "displayName"
          }
        }
      },
    ]
  })
}
