# Example Entitlement resource by id
resource "identitynow_entitlement" "entitlement" {
  id         = "f45e991187dd4a9399d4f71954ec3029"
  privileged = true
}

# Example Source data source
data "identitynow_source" "source" {
  id = "3c91808677e0502f0177eee68e943f6f"
}

# Example Entitlement resource by value
resource "identitynow_entitlement" "entitlement_by_value" {
  privileged = true
  source_id  = data.identitynow_source.source.id
  value      = "e2638045-4d39-4c79-8623-b0e82778b432"
}

# Example Entitlement resource by value with metadata
resource "identitynow_entitlement" "entitlement_by_value" {
  privileged = true
  source_id  = data.identitynow_source.source.id
  value      = "e2638045-4d39-4c79-8623-b0e82778b432"
  access_model_metadata = [
    {
      description = "Specifies the degree of Risk represented by an access item."
      key         = "iscRisk"
      multiselect = false
      name        = "Risk"
      object_types = [
        "general",
      ]
      status = "active"
      type   = "governance"
      values = [
        {
          name   = "Low"
          status = "active"
          value  = "low"
        },
      ]
    },
  ]
}
