
---
layout: ""
page_title: "Provider: IdentityNow"
description: |-
  The IdentityNow provider provides the resources to interact with SailPoint IdentityNow platform.
---

# IdentityNow Provider

The IdentityNow provider provides the resources to interact with [SailPoint IdentityNow](https://www.sailpoint.com/products/identitynow).

The provider uses Terraform [protocol version 6](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol#protocol-version-6) that is compatible with Terraform CLI version 1.0 and later.

## Authentication

[IdentityNow Authentication Personal Access Tokens](https://developer.sailpoint.com/docs/api/authentication/#generate-a-personal-access-token)

### Environment Variables

The provider can leverage the [SailPoint CLI](https://github.com/sailpoint-oss/sailpoint-cli) Environment Variables.

## Example Usage

```terraform
# Terraform 1.+ uses the Terraform Registry:

terraform {
  required_providers {
    identitynow = {
      version = "0.1.0"
      source  = "davidsonjon/identitynow"
    }
  }
}

provider "identitynow" {
  sail_base_url      = "https://tenant.api.identitynow.com"
  sail_client_id     = var.sail_client_id
  sail_client_secret = var.sail_client_secret
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `http_retry_max` (Number) Override number of retries for the retryablehttp client - default is 20
- `sail_base_url` (String)
- `sail_client_id` (String)
- `sail_client_secret` (String)
