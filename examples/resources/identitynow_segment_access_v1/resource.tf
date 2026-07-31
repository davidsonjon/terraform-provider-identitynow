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

resource "identitynow_role_v1" "example" {
  name        = "example-role"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }
}

resource "identitynow_access_profile_v1" "example" {
  name        = "example-access-profile"
  description = "Managed by Terraform."
  enabled     = true
  requestable = false

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  source = {
    id   = "2c9180866166b5b0016167c32ef31f3c"
    type = "SOURCE"
  }
}

resource "identitynow_segment_access_v1" "example" {
  segment_id = identitynow_segment_v1.example.id

  assignments = [
    {
      type = "ROLE"
      id   = identitynow_role_v1.example.id
    },
    {
      type = "ACCESS_PROFILE"
      id   = identitynow_access_profile_v1.example.id
    },
  ]
}
