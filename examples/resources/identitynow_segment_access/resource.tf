# Example Identity data source
data "identitynow_identity" "identity" {
  id = "2072631"
}

# Example Application data source
data "identitynow_application" "application" {
  id = "38383"
}

# Example Source data source
data "identitynow_source" "source" {
  id = "4c91808677e0502f0177eee68e943f6f"
}

# Example Entitlement data source
data "identitynow_entitlement" "entitlement" {
  id = "53d3ef265a964023849a2e9173078462"
}

# Example Access Profile resource
resource "identitynow_access_profile" "access_profile" {
  name        = "Example terraform access profile"
  description = "Example Access Profile"
  requestable = true
  enabled     = true
  owner = {
    id   = data.identitynow_identity.identity.id
    name = data.identitynow_identity.identity.name
    type = "IDENTITY"
  }
  source = {
    id   = data.identitynow_source.source.id
    name = data.identitynow_source.source.name
    type = "SOURCE"
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

# Example Segment by name
data "identitynow_segment" "segment_name" {
  name = "Example Segment"
}

# Example Segement Access
resource "identitynow_segment_access" "segment_access" {
  segment_id        = data.identitynow_segment.segment_name.id
  assignments = [
    {
      type = "ACCESS_PROFILE"
      id   = identitynow_access_profile.access_profile.id
    },
  ]
}
