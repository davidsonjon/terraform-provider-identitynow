# Lists access profiles whose name starts with "Engineering", capped at 10
# results. Each entry in `access_profiles` has the same attributes as
# identitynow_access_profile_v1.
data "identitynow_access_profiles_v1" "example" {
  filters = "name sw \"Engineering\""
  limit   = 10
}

output "engineering_access_profile_ids" {
  value = [for ap in data.identitynow_access_profiles_v1.example.access_profiles : ap.id]
}
