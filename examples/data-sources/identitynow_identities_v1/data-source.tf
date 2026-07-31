# Lists identities whose alias starts with "D12", then reshapes the result
# into an alias=>id map for downstream use.
data "identitynow_identities_v1" "example" {
  filters = "alias sw \"D12\""
  sorters = "alias"
  limit   = 10
}

locals {
  identity_ids_by_alias = {
    for i in data.identitynow_identities_v1.example.identities : i.alias => i.id
  }
}

output "identity_ids_by_alias" {
  value = local.identity_ids_by_alias
}
