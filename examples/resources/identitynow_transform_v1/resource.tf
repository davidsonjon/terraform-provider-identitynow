# A simple, single-level transform. "attributes" is a raw JSON string - its
# shape depends entirely on "type" (see the resource documentation for the
# full list of supported types and a link to SailPoint's Transform Operations
# reference).
resource "identitynow_transform_v1" "lower_department" {
  name = "Lowercase Department"
  type = "lower"

  # No "input" key below means an *implicit* input: the system supplies
  # whatever value is mapped to this transform (e.g. from an identity
  # profile's attribute mapping).
  attributes = jsonencode({})
}

# A nested/explicit-input example: a "lower" transform whose input is itself
# a "concat" transform joining two identity attributes. Nested transform
# objects (anything passed via "input"/"values") must NOT have their own
# "name" - only the root-level transform is named.
resource "identitynow_transform_v1" "lower_concat_example" {
  name = "Lowercase Concat Example"
  type = "lower"

  attributes = jsonencode({
    input = {
      type = "concat"
      attributes = {
        values = [
          { type = "identityAttribute", attributes = { name = "firstname" } },
          "-",
          { type = "identityAttribute", attributes = { name = "lastname" } },
        ]
      }
    }
  })
}
