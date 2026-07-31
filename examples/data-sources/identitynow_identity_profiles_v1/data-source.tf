# Lists identity profiles whose name starts with "Test", capped at 10
# results. Each entry in `identity_profiles` has the same attributes as
# identitynow_identity_profile_v1.
data "identitynow_identity_profiles_v1" "example" {
  filters = "name sw \"Test\""
  limit   = 10
}

output "test_identity_profile_ids" {
  value = [for p in data.identitynow_identity_profiles_v1.example.identity_profiles : p.id]
}
