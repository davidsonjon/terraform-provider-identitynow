# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known workflow id looked up directly, as shown here.
data "identitynow_workflow_v1" "example" {
  id = "c17bea3a-574d-453c-9e04-4365fbf5af0b"
}
