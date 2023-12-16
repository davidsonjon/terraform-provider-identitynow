# Example Entitlement resource by value
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
