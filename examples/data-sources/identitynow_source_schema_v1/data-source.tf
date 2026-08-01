# Looks up a single Schema by source_id + schema_id.
data "identitynow_source_schema_v1" "example" {
  source_id = "9e99be10dcf24aa9bbe83902dece8738"
  schema_id = "2c91808a7813090a0178141219190000"
}
