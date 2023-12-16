---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "Resource identitynow_source_load_entitlement_wait - terraform-provider-identitynow"
subcategory: ""
description: |-
  Source Load Entitlement Wait resource. On create will call /loadentitlement endpoint for a Source
---

# Resource (identitynow_source_load_entitlement_wait)

Source Load Entitlement Wait resource. On create will call /loadentitlement endpoint for a Source

#### *Warning*

This will trigger entitlement aggregation, changes to `triggers` will cause resource to be re-created and cause another entitlement aggregation.

## Example Usage

```terraform
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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `source_id` (String) Source ID

### Optional

- `triggers` (Map of String) (Optional) Arbitrary map of values that, when changed, will run any creation or destroy delays again.
- `wait_for_active_jobs` (Boolean) Wait for any active jobs to finish before starting

## Import

Import is supported using the following syntax:

```shell
# Syntax: <Source ID>
terraform import identitynow_source_load_entitlement_wait.wait <Source ID>,<trigger_key1>:<trigger_value1>/<trigger_key2>:<trigger_value2>,<wait_for_active_jobs>
```