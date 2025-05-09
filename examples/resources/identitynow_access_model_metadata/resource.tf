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
