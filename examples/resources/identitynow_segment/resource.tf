# Example Segment
resource "identitynow_segment" "segment" {
  name        = "terraform test segment"
  description = "test segment"
  active      = false
  visibility_criteria = {
    operator = "AND"
    children = [
      {
        operator  = "EQUALS"
        attribute = "displayName"
        value = {
          type  = "STRING"
          value = "Doe, John"
        }
      },
      {
        operator  = "EQUALS"
        attribute = "lastname"
        value = {
          type  = "STRING"
          value = "Doe"
      } }
    ]
  }
}
