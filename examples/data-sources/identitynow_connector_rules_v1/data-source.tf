# Lists every connector rule in the tenant - this endpoint has no
# filtering/pagination support, so "connector_rules" always contains all of
# them. Each entry has the same attributes as identitynow_connector_rule_v1.
data "identitynow_connector_rules_v1" "example" {}

output "connector_rule_ids" {
  value = [for r in data.identitynow_connector_rules_v1.example.connector_rules : r.id]
}
