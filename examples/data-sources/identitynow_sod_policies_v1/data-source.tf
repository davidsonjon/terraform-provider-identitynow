# List SOD policies, optionally filtered/sorted server-side via the
# standard V3 collection parameters syntax:
# https://developer.sailpoint.com/idn/api/standard-collection-parameters
data "identitynow_sod_policies_v1" "example" {
  filters = "state eq \"ENFORCED\""
  sorters = "name"
  limit   = 50
}

output "sod_policy_ids_by_name" {
  value = { for p in data.identitynow_sod_policies_v1.example.sod_policies : p.name => p.id }
}
