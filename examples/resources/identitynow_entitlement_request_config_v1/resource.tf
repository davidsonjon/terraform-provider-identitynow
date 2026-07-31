# Adopt an already-existing entitlement by id first. Like
# identitynow_entitlement_v1 itself, this does not create or delete the
# upstream entitlement object.
resource "identitynow_entitlement_v1" "example" {
  id = "f45e991187dd4a9399d4f71954ec3029"
}

# Manage the adopted entitlement's request/revocation approval
# configuration. Destroy removes only Terraform state; it does not delete
# the entitlement or reset its upstream request configuration.
resource "identitynow_entitlement_request_config_v1" "example" {
  id = identitynow_entitlement_v1.example.id

  access_request_config = {
    approval_schemes = [
      {
        approver_type = "MANAGER"
      },
      {
        approver_type = "GOVERNANCE_GROUP"
        approver_id   = "00001078bd9c497a8122c6fc3f3571b1"
      },
    ]
    denial_comment_required  = true
    reauthorization_required = true
    request_comment_required = false
    require_end_date         = true
    max_permitted_access_duration = {
      value     = 30
      time_unit = "DAYS"
    }
  }

  revocation_request_config = {
    revocation_approval_schemes = [
      {
        approver_type = "WORKFLOW"
        approver_id   = "2c91808477d8f06d0177e1f4c9d000f9"
      },
    ]
  }
}
