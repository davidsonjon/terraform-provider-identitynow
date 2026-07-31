terraform {
  required_providers {
    identitynow = {
      source = "hashicorp.com/edu/identitynow"
    }
  }
}

# Credentials should come from environment/CI secrets, not hardcoded literals.
# See https://developer.sailpoint.com/docs/api/authentication/ for how to
# create a personal access token/client id+secret pair for your tenant.
provider "identitynow" {
  sail_base_url      = var.sail_base_url
  sail_client_id     = var.sail_client_id
  sail_client_secret = var.sail_client_secret
  http_retry_max     = 1
}
