# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known connector rule id looked up directly, as shown
# here.
data "identitynow_connector_rule_v1" "example" {
  id = "2c9180835d191059015d29ee6d660f22"
}
