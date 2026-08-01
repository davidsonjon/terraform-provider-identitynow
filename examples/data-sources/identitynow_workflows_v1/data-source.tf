# Lists enabled workflows, capped at 10 results. Each entry in "workflows"
# has the same attributes as identitynow_workflow_v1.
data "identitynow_workflows_v1" "example" {
  filters = "enabled eq true"
  limit   = 10
}

output "enabled_workflow_ids" {
  value = [for w in data.identitynow_workflows_v1.example.workflows : w.id]
}
