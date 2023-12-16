---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "identitynow_entitlement Data Source - terraform-provider-identitynow"
subcategory: ""
description: |-
  Entitlement data source
---

# identitynow_entitlement (Data Source)

Entitlement data source

## Example Usage

```terraform
# Example Entitlement data source
data "identitynow_entitlement" "entitlement" {
  id = "73d3ef265a964023849a2e9173078462"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) The entitlement id

### Read-Only

- `attribute` (String) The entitlement attribute name
- `cloud_governed` (Boolean) True if the entitlement is cloud governed
- `created` (String) Time when the entitlement was created
- `description` (String) The description of the entitlement
- `modified` (String) Time when the entitlement was last modified
- `name` (String) The entitlement name
- `owner_id` (String) The Owner ID of the entitlement
- `privileged` (Boolean) True if the entitlement is privileged
- `requestable` (Boolean) True if the entitlement is requestable
- `source_id` (String) The Source ID of the entitlement
- `source_schema_object_type` (String) The object type of the entitlement from the source schema
- `value` (String) The value of the entitlement