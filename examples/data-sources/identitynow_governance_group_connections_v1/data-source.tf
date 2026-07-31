# Lists the objects (roles, access profiles, SOD policies, sources)
# connected to a governance group as owner/reviewer.
data "identitynow_governance_group_connections_v1" "example" {
  governance_group_id = "2c91808a7813090a017814121919ecca"
}

output "example_connection_object_ids" {
  value = [for c in data.identitynow_governance_group_connections_v1.example.connections : c.object_id]
}
