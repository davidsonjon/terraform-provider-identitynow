# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known identity profile id looked up directly, as
# shown here.
data "identitynow_identity_profile_v1" "example" {
  id = "2c9180857756ddc70177574aa0576ab1"
}
