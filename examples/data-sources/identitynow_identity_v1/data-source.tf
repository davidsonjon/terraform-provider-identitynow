# Look up a single identity directly by its ISC identity id.
data "identitynow_identity_v1" "by_id" {
  id = "2c91808576ddc7060176de5040574ab0"
}

# Alternatively, look up the identity by its exact alias.
data "identitynow_identity_v1" "by_alias" {
  alias = "D12345678"
}

# Or look it up by exact email address.
data "identitynow_identity_v1" "by_email" {
  email_address = "last.first@example.com"
}
