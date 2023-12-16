---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "identitynow_application Resource - terraform-provider-identitynow"
subcategory: ""
description: |-
  Application data source
---

# identitynow_application (Resource)

Application data source

## Example Usage

```terraform
# Example Source data source
data "identitynow_source" "source" {
  id = "3c91808677e0502f0177eee68e943f6f"
}

# Example Access Profile data source
data "identitynow_access_profile" "access_profile" {
  id = "3c9180847817ac4f0178221df4391f75"
}

# Example Identity data source
data "identitynow_identity" "identity" {
  id = "4072631"
}

# Example Applicaiton resource
resource "identitynow_application" "application" {
  name        = "example terraform application"
  description = "example application"

  owner = {
    id = data.identitynow_identity.identity.cc_id
  }
  account_service_id        = data.identitynow_source.source.connector_attributes.cloud_external_id
  launchpad_enabled         = true
  app_center_enabled        = false
  provision_request_enabled = false
  access_profile_ids = [
    data.identitynow_access_profile.access_profile.id,
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `account_service_id` (String) account_service_id of application
- `name` (String) Name of the application

### Optional

- `access_profile_ids` (List of String) List of access profile id's
- `app_center_enabled` (Boolean) Determines if application is enabled in app center
- `description` (String) Description of the application
- `launchpad_enabled` (Boolean) Launchpad enabled
- `owner` (Attributes) Owner information (see [below for nested schema](#nestedatt--owner))
- `provision_request_enabled` (Boolean) Determines if application is requestable in app center

### Read-Only

- `app_id` (String) app_id of the application
- `date_created` (String) date created
- `id` (String) id of the application
- `service_app_id` (String) service_app_id of the application
- `service_id` (String) service_id of the application

<a id="nestedatt--owner"></a>
### Nested Schema for `owner`

Required:

- `id` (String) Owner id

Read-Only:

- `name` (String) Owner name

## Import

Import is supported using the following syntax:

```shell
# Syntax: <Application ID>
terraform import identitynow_application.application <Application ID>
```