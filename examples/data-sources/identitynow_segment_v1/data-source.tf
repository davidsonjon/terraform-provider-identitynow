# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known segment id looked up directly, as shown here.
data "identitynow_segment_v1" "example" {
  id = "2c91808576ddc7060176de5040574ab0"
}

# Alternatively, look the segment up by its exact (unique) name instead of
# id. Exactly one of `id` or `name` must be set.
data "identitynow_segment_v1" "example_by_name" {
  name = "example-segment"
}
