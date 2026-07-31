# Look up a single entitlement by id. Returns the same attributes as the
# identitynow_entitlement_v1 resource, all Computed.
data "identitynow_entitlement_v1" "example" {
  id = "f45e991187dd4a9399d4f71954ec3029"
}

output "entitlement_owner" {
  value = data.identitynow_entitlement_v1.example.owner
}
