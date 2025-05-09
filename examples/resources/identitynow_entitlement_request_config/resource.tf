# Example Entitlement resource by value
resource "identitynow_entitlement" "entitlement" {
  id         = "f45e991187dd4a9399d4fk1954ec3029"
}

# Example Entitlement Request Configuration resource
resource "identitynow_entitlement_request_config" "entitlement_request_config" {
  id         = identitynow_entitlement.entitlement.id
  access_request_config = {
    approval_schemes = [
      {
        approver_type = "MANAGER",
        approver_id   = null
      },
    ]
    comments_required        = true
    denial_comments_required = false
  }
}
