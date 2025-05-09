# Example Identity data source
data "identitynow_identity" "identity" {
  id = "2072631"
}

# Example Entitlement data source
data "identitynow_entitlement" "entitlement" {
  id = "53d3ef265a964023849a2e9173078462"
}

# Example Access Profile resource
resource "identitynow_role" "role" {
  name        = "Example terraform Role"
  description = "Example Role"
  enabled     = true
  requestable = true
  owner = {
    id   = data.identitynow_identity.identity.id
    type = "IDENTITY"
  }
  entitlements = [
    {
      id   = data.identitynow_entitlement.entitlement.id,
      type = "ENTITLEMENT",
    },
  ]
  access_request_config = {
    approval_schemes = [
      {
        approver_type = "MANAGER",
        approver_id   = null
      }
    ]
    comments_required        = true
    denial_comments_required = true
  }
}
