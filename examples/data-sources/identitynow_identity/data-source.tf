# Example Identity data source
data "identitynow_identity" "identity" {
  id = "3072631"
}

# Example Identity data source by alias
data "identitynow_identity" "identity_by_alias" {
  alias = "D12345678"
}

# Example Identity data source by email
data "identitynow_identity" "identity_by_email" {
  email_address = "last.first@example.com"
}
