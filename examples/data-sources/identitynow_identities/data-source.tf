# Example Identity data source
data "identitynow_identities" "identities" {
  filters = "alias sw \"alice\"'"
}

data "identitynow_identities" "identities" {
  filters = "email eq \"test@example.com\""
}

data "identitynow_identities" "identities" {
  filters = "firstname eq \"John\""
}
