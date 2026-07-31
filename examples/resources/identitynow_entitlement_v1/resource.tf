# Adopt an already-existing entitlement by id. This never creates or
# deletes anything upstream - it only manages the handful of writable
# fields (requestable, segments, owner, name, description) on an
# entitlement that already exists in IdentityNow/ISC (aggregated in from a
# source), and forgets it (removes from Terraform state only) on destroy.
resource "identitynow_entitlement_v1" "example" {
  id          = "f45e991187dd4a9399d4f71954ec3029"
  requestable = true
}

# Adopt the same kind of existing entitlement, but looked up by
# source_id + value instead of a known id - useful when entitlements are
# referenced by their source-native value rather than IdentityNow's
# internal id (e.g. right after a source aggregation, chained from a
# source_load_entitlement_wait_v1 resource in the future).
data "identitynow_source_v1" "example" {
  id = "01f28e7f21804bef8565673ed668f36e"
}

resource "identitynow_entitlement_v1" "by_value" {
  source_id   = data.identitynow_source_v1.example.id
  value       = "CN=Engineering,OU=Groups,DC=example,DC=com"
  requestable = true
  owner = {
    id   = "00001078bd9c497a8122c6fc3f3571b1"
    type = "IDENTITY"
  }
}
