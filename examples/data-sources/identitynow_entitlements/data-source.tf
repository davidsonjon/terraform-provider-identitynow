# Example Entitlements data source
data "identitynow_entitlements" "source_entitlements" {
  source_id = "8b649d6daee1407ca0df6b491c82f74b"
  limit     = 10
}
