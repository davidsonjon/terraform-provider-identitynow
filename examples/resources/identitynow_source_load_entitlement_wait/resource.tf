# Example Source data source
data "identitynow_source" "source" {
  id = "1c91808677e0502f0177eee68e943f6f"
}

# AzureAD entitlement example
resource "azuread_group" "group" {
  display_name = "example group"

  # Ignore changes to membership
  lifecycle {
    ignore_changes = [
      members,
    ]
  }
}

# Example Load Entitlement Wait resource
resource "identitynow_source_load_entitlement_wait" "wait" {
  source_id            = data.identitynow_source.source.connector_attributes["cloud_external_id"]
  wait_for_active_jobs = false
  triggers = {
    "test" = azuread_group.group.id
  }
}

# Example Entitlement reource by value that has not been aggregated yet
resource "identitynow_entitlement" "entitlement" {
  privileged = true
  source_id  = data.identitynow_source.source.id
  value      = azuread_group.group.id

  # Use depends_on to wait for entitlement aggregation to complete
  depends_on = [identitynow_source_load_entitlement_wait.wait]
}
