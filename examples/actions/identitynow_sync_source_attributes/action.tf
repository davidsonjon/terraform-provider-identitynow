# Manual, one-off invocation:
#   terraform apply -invoke=action.identitynow_sync_source_attributes.example
action "identitynow_sync_source_attributes" "example" {
  config {
    source_id = identitynow_source_v1.example.id
    timeout   = "15m"
  }
}

# Recommended: trigger re-synchronization after connector configuration
# changes (connector_attributes, cluster, etc.), since those are exactly the
# inputs this operation re-reads. `before_destroy`/`after_destroy` triggers
# are not supported by Terraform as of the actions feature's 1.14 release, so
# there is no equivalent "sync before removing a source" lifecycle hook.
resource "identitynow_source_v1" "example" {
  name      = "example-source"
  connector = "active-directory"

  connector_attributes = jsonencode({
    host = "ad.example.com"
  })

  owner = {
    id   = "2c91808576ddc7060176de5040574aa0"
    type = "IDENTITY"
  }

  lifecycle {
    action_trigger {
      events  = [after_update]
      actions = [action.identitynow_sync_source_attributes.example]
    }
  }
}
