# Example Identity data source
data "identitynow_identity" "identity1" {
  id = "2072631"
}

# Example Identity data source
data "identitynow_identity" "identity2" {
  id = "354940"
}

# Example Goverance Group resource
resource "identitynow_governance_group" "group" {
  name        = "terraform example goverance group"
  description = "example goverance group"
  owner = {
    id = data.identitynow_identity.identity1.id
  }
  membership = [
    {
      id = data.identitynow_identity.identity1.id
    },
    {
      id = data.identitynow_identity.identity2.id
    },
  ]
}
