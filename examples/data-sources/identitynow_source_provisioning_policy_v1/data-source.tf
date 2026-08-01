# Looks up a single Provisioning Policy by source_id + usage_type.
data "identitynow_source_provisioning_policy_v1" "example" {
  source_id  = "9e99be10dcf24aa9bbe83902dece8738"
  usage_type = "CREATE"
}
