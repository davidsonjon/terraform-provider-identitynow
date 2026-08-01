# Lists every Schema (account, and for hierarchical/group sources, group)
# defined on a source.
data "identitynow_source_schemas_v1" "example" {
  source_id = "9e99be10dcf24aa9bbe83902dece8738"
}

output "example_schema_names" {
  value = [for s in data.identitynow_source_schemas_v1.example.schemas : s.name]
}
