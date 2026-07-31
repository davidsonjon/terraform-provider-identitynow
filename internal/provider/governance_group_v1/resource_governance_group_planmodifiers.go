package governance_group_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyGovernanceGroupUseStateForUnknown patches
// resource_governance_group.GovernanceGroupResourceSchema's generated
// Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar attributes, working around the same
// systemic tfplugingen-framework pipeline gap documented for role_v1/
// service_desk_integration_v1/transform_v1 (see the
// identitynow-terraform-provider-developer knowledge file, 2026-07-24
// entries): tfplugingen-framework never emits any PlanModifiers, so every
// unconfigured Computed attribute is proposed as Unknown on every plan (not
// just when it actually changes), producing a permanent, spurious
// "(known after apply)" diff.
//
// Confirmed via `grep -n PlanModifiers` that this generated schema has zero
// PlanModifiers, same as every other _v1 pilot target before its fix.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves
// that are genuinely server-managed and never written by modelToDto:
//   - "id": server-assigned on Create, never configurable - safe.
//   - "created": server-assigned, never configurable - safe.
//   - "modified": unlike role_v1's "modified" (which the API genuinely
//     changes on every real Update, so it must NOT get this modifier - see
//     resource_role_planmodifiers.go), this resource's dtoToModel always
//     refreshes "modified" from the live API response on every Read/Update
//     (api_beta.WorkgroupDto.Modified is a real typed *SailPointTime field,
//     read back unconditionally - not a constant/never-populated field the
//     way service_desk_integration_v1's is). Applying UseStateForUnknown
//     here would therefore suppress a legitimate diff after a real
//     server-side modification (e.g. changing the owner). Deliberately
//     EXCLUDED from this list - "modified" is expected to show
//     "(known after apply)" on every plan until Update/Read actually runs,
//     matching role_v1's documented behavior/caveat.
//   - "name", "description", and "owner" all have real write support in
//     modelToDto (a practitioner can configure/change them), so
//     UseStateForUnknown must not be applied to them either, for the same
//     config-removal correctness reason documented in role_v1's
//     resource_role_planmodifiers.go.
//   - "member_count"/"connection_count" are genuinely volatile
//     (membership/connection changes happen outside this resource's own
//     CRUD, e.g. via the members/connections sub-resource endpoints noted in
//     the package doc as out of scope) and must also be excluded.
func applyGovernanceGroupUseStateForUnknown(s *schema.Schema) {
	patchString(s, "id")
	patchString(s, "created")
}

func patchString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
