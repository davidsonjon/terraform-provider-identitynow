# Lists roles whose name starts with "Engineering", capped at 10 results.
# Each entry in `roles` has the same attributes as identitynow_role_v1.
data "identitynow_roles_v1" "example" {
  filters = "name sw \"Engineering\""
  limit   = 10
}

output "engineering_role_ids" {
  value = [for r in data.identitynow_roles_v1.example.roles : r.id]
}
