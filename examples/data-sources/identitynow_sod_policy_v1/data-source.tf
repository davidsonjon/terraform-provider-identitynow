# Look up an existing SOD policy by id. Unlike the `identitynow_segment_v1`
# data source, `identitynow_sod_policy_v1` currently only supports lookup by
# `id` (the sod-policies v1 API has no exact-name-match GET) - see the
# resource docs' Known Limitations for details.
data "identitynow_sod_policy_v1" "example" {
  id = "2c9180835d191305015d28d181fc1234"
}
