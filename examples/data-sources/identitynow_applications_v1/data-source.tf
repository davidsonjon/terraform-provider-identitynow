# Lists applications (source apps) whose name starts with "Engineering",
# capped at 10 results. Each entry in `applications` has the same attributes
# as identitynow_application_v1, including access_profile_ids.
data "identitynow_applications_v1" "example" {
  filters = "name sw \"Engineering\""
  limit   = 10
}

output "engineering_application_ids" {
  value = [for a in data.identitynow_applications_v1.example.applications : a.id]
}
