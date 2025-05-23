---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "identitynow_access_model_metadata Resource - terraform-provider-identitynow"
subcategory: ""
description: |-
  Access Profile resource
---

# identitynow_access_model_metadata (Resource)

Access Profile resource

## Example Usage

```terraform
# Example Source data source
resource "identitynow_access_model_metadata" "access_model_metadata" {
  name         = "EXAMPLE"
  object_types = ["entitlement"]
  description  = "example description"
  values = [
    {
      name  = "abc"
      value = "abc"
    },
    {
      name  = "def"
      value = "def"
  }]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The display name of the key.
- `object_types` (List of String) An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.

### Optional

- `description` (String) The description of the Attribute.
- `multiselect` (Boolean) Indicates whether the attribute can have multiple values.
- `values` (Attributes List) (see [below for nested schema](#nestedatt--values))

### Read-Only

- `key` (String) Technical name of the Attribute. This is unique and cannot be changed after creation.
- `status` (String) The status of the Attribute.
- `type` (String) The type of the Attribute. This can be either `custom` or `governance`.

<a id="nestedatt--values"></a>
### Nested Schema for `values`

Required:

- `name` (String) The display name of the Attribute value.
- `value` (String) Technical name of the Attribute value. This is unique and cannot be changed after creation.

Read-Only:

- `status` (String) The status of the Attribute value.

## Import

Import is supported using the following syntax:

```shell
# Syntax: <Metadata Key>
terraform import identitynow_access_model_metadata.access_model_metadata <Metadata Key>
```
