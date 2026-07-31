# Trigger an entitlement aggregation ("load entitlements") on a source and
# wait for the launched background task to finish before this resource
# reports as created. This is a hand-written action resource with
# null_resource-style replacement behavior, not a CRUD wrapper around a
# persistent upstream object - there is nothing to "read back" afterwards.
resource "identitynow_source_load_entitlement_wait_v1" "example" {
  source_id = "9e99be10dcf24aa9bbe83902dece8738"

  # Any change to any key/value here forces this resource to be replaced,
  # re-triggering aggregation - the same idea as null_resource.triggers or
  # terraform_data.triggers_replace. A common use is referencing the id of
  # an upstream group/object that, once created, causes the source to pick
  # up new entitlements on its next aggregation.
  triggers = {
    reason = "initial aggregation"
  }

  # If true, Create first waits for any already-running entitlement
  # aggregation jobs on this source to finish before launching a new one,
  # avoiding overlapping/duplicate aggregation runs.
  wait_for_active_jobs = true
}

# Entitlements only exist in IdentityNow/ISC once a source aggregation has
# imported them. Adopting an entitlement by source_id + value right after
# triggering aggregation is only safe once that aggregation has actually
# completed - depends_on this resource to sequence that correctly.
resource "identitynow_entitlement_v1" "example" {
  source_id = identitynow_source_load_entitlement_wait_v1.example.source_id
  value     = "ODQ-IND3201"

  depends_on = [identitynow_source_load_entitlement_wait_v1.example]
}
