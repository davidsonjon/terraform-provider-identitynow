# Lists governance groups whose name starts with "Test", capped at 10 results.
# Each entry in `governance_groups` has the same attributes as
# identitynow_governance_group_v1.
data "identitynow_governance_groups_v1" "example" {
  filters = "name sw \"Test\""
  limit   = 10
}

output "test_governance_group_ids" {
  value = [for g in data.identitynow_governance_groups_v1.example.governance_groups : g.id]
}
