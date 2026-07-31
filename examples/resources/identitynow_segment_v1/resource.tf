resource "identitynow_segment_v1" "example" {
  name        = "example-segment"
  description = "Managed by Terraform."
  active      = true

  visibility_criteria = {
    expression = {
      operator = "AND"
      children = [
        {
          operator  = "EQUALS"
          attribute = "location"
          value = {
            type  = "STRING"
            value = "Philadelphia"
          }
        },
        {
          operator  = "EQUALS"
          attribute = "department"
          value = {
            type  = "STRING"
            value = "HR"
          }
        }
      ]
    }
  }
}
