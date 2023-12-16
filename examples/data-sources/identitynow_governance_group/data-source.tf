# Example Goveranance Group data source
data "identitynow_governance_group" "group" {
  id = "19c7facb-8abb-42d9-b731-0402b05f3c6b"
}

# Example Goveranance Group data source by name
data "identitynow_governance_group" "group_by_name" {
  name = "Example Governance Group"
}
