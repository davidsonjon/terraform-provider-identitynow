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

- `access_model_metadata` (Attributes List) (see [below for nested schema](#nestedatt--access_model_metadata))
- `attribute` (String) The entitlement attribute name
- `cloud_governed` (Boolean) True if the entitlement is cloud governed
- `created` (String) Time when the entitlement was created
- `description` (String) The description of the entitlement, due to API limitations, may be set to an empty string (`""`) but not **null**. Note: this attribute can be initially aggregated in from some sources and will be overwritten if set
- `modified` (String) Time when the entitlement was last modified
- `name` (String) The entitlement name
- `owner` (Attributes) The Owner of the entitlement (see [below for nested schema](#nestedatt--owner))
- `privileged` (Boolean) True if the entitlement is privileged
- `requestable` (Boolean) True if the entitlement is requestable
- `source_id` (String) The Source ID of the entitlement
- `source_schema_object_type` (String) The object type of the entitlement from the source schema
- `value` (String) The value of the entitlement

<a id="nestedatt--access_model_metadata"></a>
### Nested Schema for `access_model_metadata`

Read-Only:

- `description` (String) The description of the Attribute.
- `key` (String) Technical name of the Attribute. This is unique and cannot be changed after creation.
- `multiselect` (Boolean) Indicates whether the attribute can have multiple values.
- `name` (String) The display name of the key.
- `object_types` (List of String) An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.
- `status` (String) The status of the Attribute.
- `type` (String) The type of the Attribute. This can be either `custom` or `governance`.
- `values` (Attributes List) (see [below for nested schema](#nestedatt--access_model_metadata--values))

<a id="nestedatt--access_model_metadata--values"></a>
### Nested Schema for `access_model_metadata.values`

Read-Only:

- `name` (String) The display name of the Attribute value.
- `status` (String) The status of the Attribute value.
- `value` (String) Technical name of the Attribute value. This is unique and cannot be changed after creation.



<a id="nestedatt--owner"></a>
### Nested Schema for `owner`

Read-Only:

- `id` (String) Identity id
- `name` (String) Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.
- `type` (String) The type of the Source, will always be `IDENTITY`
