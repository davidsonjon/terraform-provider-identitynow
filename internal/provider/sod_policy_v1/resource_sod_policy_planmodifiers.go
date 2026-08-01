package sod_policy_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applySodPolicyUseStateForUnknown patches resource_sod_policy.SodPolicyResourceSchema's
// generated Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar, server-managed Computed attributes -
// mirroring the same systemic tfplugingen-framework gap documented in
// role_v1/resource_role_planmodifiers.go and segment_v1/resource_segment_planmodifiers.go:
// the generator never emits any PlanModifiers, so every unconfigured
// Computed attribute is proposed as Unknown on every plan that changes
// anything else, producing a permanent "(known after apply)" diff even for
// attributes that plainly did not change.
//
// Scope is intentionally narrow:
//   - "id" never changes across an Update (PUT targets the same id), so it's
//     always safe to pin to prior state.
//   - "created" never changes after initial creation, so it is also safe to
//     pin.
//   - "creator_id" is set once at creation time and never mutated by any
//     Update, so it is also safe to pin.
//   - "modified" is genuinely volatile (the API bumps it on every real
//     Update, even when the practitioner's config is unchanged) -
//     UseStateForUnknown would pin the plan to a stale value and produce a
//     hard "Provider produced inconsistent result after apply" error.
//   - "modifier_id" changes on every real Update to reflect whoever/whatever
//     made the change, so it is likewise left Unknown rather than pinned.
//   - Every other Computed(-optional) attribute (name, description,
//     policy_query, compensating_controls, correction_advice, state, tags,
//     scheduled, type, owner_ref, conflicting_access_criteria,
//     violation_owner_assignment_config) has real write support in
//     sodPolicyModelToDTO, so UseStateForUnknown must NOT be applied to them -
//     per the framework's own documented caveat, doing so would reuse the
//     prior state value whenever the proposed value is Unknown, preventing a
//     practitioner from ever changing these back out via config in the same
//     plan cycle where some other attribute forces a replan.
func applySodPolicyUseStateForUnknown(s *schema.Schema) {
	patchSodPolicyString(s, "id")
	patchSodPolicyString(s, "created")
	patchSodPolicyString(s, "creator_id")
}

func patchSodPolicyString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
