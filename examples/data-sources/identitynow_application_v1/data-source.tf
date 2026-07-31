# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known application id looked up directly, as shown here.
data "identitynow_application_v1" "example" {
  id = "2c91808576ddc7060176de5040574aa0"
}
