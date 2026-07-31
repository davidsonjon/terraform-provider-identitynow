# Lists entitlements filtered to a single source, then reshapes the result
# with a `for` expression into a value=>entitlement map keyed by the
# entitlement's source-native value (handy for referencing entitlements from
# other resources without needing to know their IdentityNow ids).
data "identitynow_entitlements_v1" "example" {
  filters = "source.id eq \"01f28e7f21804bef8565673ed668f36e\""
  limit   = 250
}

locals {
  entitlements_by_value = {
    for e in data.identitynow_entitlements_v1.example.entitlements : e.value => e
  }
}

output "entitlement_ids_by_value" {
  value = { for k, e in local.entitlements_by_value : k => e.id }
}

# --- Eventual consistency note ---
#
# Entitlements only exist after a source aggregation has imported them - see
# `POST /entitlements/v1/aggregate/sources/{id}` (exercised by the
# `identitynow_source_load_entitlement_wait_v1` helper resource, not yet
# implemented as of this writing). Terraform data sources are read exactly
# once per plan/apply and have NO built-in retry/backoff: if this data
# source's Read happens to run before a just-triggered aggregation has
# finished, newly-imported entitlements will simply be missing from
# `entitlements` that apply - the data source will not "wait" for them
# to show up.
#
# The correct fix is NOT to retry inside the data source, but to make sure
# the data source's Read only happens after aggregation has verifiably
# completed, by depending on a resource that itself polls until the
# aggregation finishes:
#
#   resource "identitynow_source_load_entitlement_wait_v1" "aggregate" {
#     source_id = "01f28e7f21804bef8565673ed668f36e"
#   }
#
#   data "identitynow_entitlements_v1" "example" {
#     filters    = "source.id eq \"01f28e7f21804bef8565673ed668f36e\""
#     depends_on = [identitynow_source_load_entitlement_wait_v1.aggregate]
#   }
#
# As a defense-in-depth measure (e.g. if a source aggregates asynchronously
# on IdentityNow's side even after the wait resource above returns), a
# `postcondition` can turn a silently-incomplete result into a hard failure
# instead of a plan that quietly provisions against too few entitlements:
#
#   data "identitynow_entitlements_v1" "example" {
#     filters = "source.id eq \"01f28e7f21804bef8565673ed668f36e\""
#
#     lifecycle {
#       postcondition {
#         condition     = length(self.entitlements) > 0
#         error_message = "No entitlements found for this source yet - source aggregation may still be in progress."
#       }
#     }
#   }
