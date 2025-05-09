# Terraform 0.13+ uses the Terraform Registry:

terraform {
  required_providers {
    identitynow = {
      version = "0.3.1"
      source  = "terraform-provider-identitynow/identitynow"
    }
  }
}

provider "identitynow" {
  sail_base_url = "https://tenant.api.identitynow.com"
  sail_client_id = var.sail_client_id
  sail_client_secret = var.sail_client_secret
}
