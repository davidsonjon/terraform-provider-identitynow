---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "identitynow_identity Data Source - terraform-provider-identitynow"
subcategory: ""
description: |-
  Identity data source
---

# identitynow_identity (Data Source)

Identity data source

## Example Usage

```terraform
# Example Identity data source
data "identitynow_identity" "identity" {
  id = "3072631"
}

# Example Identity data source by alias
data "identitynow_identity" "identity_by_alias" {
  alias = "D12345678"
}

# Example Identity data source by email
data "identitynow_identity" "identity_by_email" {
  email_address = "last.first@example.com"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `alias` (String) Alternate unique identifier for the identity
- `email_address` (String) The email address of the identity
- `id` (String) System-generated unique ID of the Object
- `use_caller_identity` (Boolean) **beware** user with caution. Use the caller's identity if no user is found, to support lifecycle outside of terraform

### Read-Only

- `caller_identity_used` (Boolean) Helper flag to indicate if the caller's identity is being used
- `created` (String) Creation date of the Object
- `identity_status` (String) The identity's status in the system
- `modified` (String) Last modification date of the Object
- `name` (String) Name of the Object
- `processing_state` (String) The processing state of the identity
