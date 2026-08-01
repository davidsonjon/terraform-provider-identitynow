# Lists every Provisioning Policy (all usage types) defined on a source.
data "identitynow_source_provisioning_policies_v1" "example" {
  source_id = "9e99be10dcf24aa9bbe83902dece8738"
}

output "example_usage_types" {
  value = [for p in data.identitynow_source_provisioning_policies_v1.example.provisioning_policies : p.usage_type]
}
