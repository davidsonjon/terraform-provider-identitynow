# NOTE: this resource's Delete uses a hand-rolled HTTP call, not the
# golang-sdk, because SailPoint's published spec omits the (real, working)
# DELETE endpoint for this API - see the resource documentation's "Known
# Limitations & Live Testing Notes" for details.
resource "identitynow_access_model_metadata_attribute_v1" "example" {
  key         = "exampleDataClassification"
  name        = "Example Data Classification"
  description = "Classifies the sensitivity of data accessible via this object."
  multiselect = false

  # "all" applies to every access-item object type; alternatively scope to
  # specific types, e.g. ["entitlement", "role"].
  object_types = ["all"]

  values = [
    {
      value = "public"
      name  = "Public"
    },
    {
      value = "internal"
      name  = "Internal"
    },
    {
      value = "confidential"
      name  = "Confidential"
    },
  ]
}
