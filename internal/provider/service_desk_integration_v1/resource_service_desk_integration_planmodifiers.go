package service_desk_integration_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyServiceDeskIntegrationUseStateForUnknown patches
// resource_service_desk_integration.ServiceDeskIntegrationResourceSchema's
// generated Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar attributes, working around the same
// systemic tfplugingen-framework pipeline gap documented for role_v1 (see the
// identitynow-terraform-provider-developer knowledge file, 2026-07-24
// entries): tfplugingen-framework never emits any PlanModifiers, so every
// unconfigured Computed attribute is proposed as Unknown on every plan (not
// just when it actually changes), producing a permanent, spurious
// "(known after apply)" diff.
//
// Confirmed via `grep -n PlanModifiers` that this generated schema has zero
// PlanModifiers, same as role_v1 before its fix. Live acceptance-test
// verification of the resulting idempotency-check failure was attempted
// (TestAccServiceDeskIntegrationV1Resource) but is currently blocked by a
// tenant licensing gap unrelated to this bug ("Application template for
// ServiceNowSDIM integration was not found" - HTTP 400 on Create, confirmed
// via a direct SDK call bypassing Terraform entirely) - this fix is applied
// on the strength of the schema-level evidence and the identical, already-
// proven-correct reasoning from role_v1, not a second live repro.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves,
// mirroring role_v1's resource_role_planmodifiers.go:
//   - "id" and "created" are Optional+Computed strings with no write support
//     in modelToDto (server-managed) - safe.
//   - "modified" would normally be excluded (see role_v1's "modified" caveat:
//     a field the API genuinely changes on every real Update cannot safely
//     receive this modifier). However, THIS package's dtoToModel currently
//     hard-codes `model.Created = types.StringPointerValue(nil)` and
//     `model.Modified = types.StringPointerValue(nil)` unconditionally -
//     api_beta.ServiceDeskIntegrationDto has no typed Created/Modified
//     fields at all, so neither is actually read back from the API today
//     (a separate, pre-existing gap - not fixed here, out of scope for this
//     plan-modifier pass). Since "modified" is therefore always a constant
//     null in this implementation (never genuinely volatile the way role's
//     "modified" is), it is safe to protect here too. If/when Created/
//     Modified read-back is implemented from AdditionalProperties (same
//     technique as dtoID), "modified" MUST be re-evaluated and likely
//     dropped from this list, exactly as it was excluded for role_v1.
//   - "cluster", "cluster_ref", "owner_ref", "before_provisioning_rule",
//     "managed_sources", and "provisioning_config" all have real write
//     support in modelToDto (a practitioner can configure, change, or remove
//     them), so UseStateForUnknown must not be applied for the same
//     config-removal correctness reason documented in role_v1's
//     resource_role_planmodifiers.go. Object-level UseStateForUnknown on
//     "cluster_ref"/"owner_ref"/"before_provisioning_rule"/
//     "provisioning_config" (all schema.SingleNestedAttribute with further
//     nested Computed attributes) is also NOT attempted here, following the
//     confirmed role_v1 finding that this reproducibly breaks even a
//     first-ever `terraform plan` - see role_v1's knowledge file entry.
func applyServiceDeskIntegrationUseStateForUnknown(s *schema.Schema) {
	patchString(s, "id")
	patchString(s, "created")
	patchString(s, "modified")
}

func patchString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
