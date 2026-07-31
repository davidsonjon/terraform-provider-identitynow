package segment_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applySegmentUseStateForUnknown patches segment_resource.SegmentResourceSchema's
// generated Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar, server-managed Computed attributes,
// working around the same systemic tfplugingen-framework gap documented in
// role_v1/resource_role_planmodifiers.go: the generator never emits any
// PlanModifiers, so every unconfigured Computed attribute (id/created/
// modified/name/description/active/owner - all Optional+Computed per the
// generated schema) is proposed as Unknown on every plan that changes
// anything else, producing a permanent "(known after apply)" diff even for
// attributes that plainly did not change.
//
// Scope is intentionally narrow, matching role_v1's precedent:
//   - "id" never changes across an Update (PATCH targets the same id), so
//     it's always safe to pin to prior state.
//   - "modified" is genuinely volatile (the API bumps it on every real
//     Update, even when the practitioner's config is unchanged) -
//     UseStateForUnknown would pin the plan to a stale value and produce a
//     hard "Provider produced inconsistent result after apply" error.
//     Acceptance tests must instead use a `lifecycle { ignore_changes =
//     [modified] }` block.
//   - "created" never changes after initial creation, so it is also safe to
//     pin.
//   - "name", "description", "active", and "owner" all have real write
//     support in segmentModelToAPI/segmentPatchRequestBody (a practitioner
//     can configure or change them), so UseStateForUnknown must NOT be
//     applied to them - per the framework's own documented caveat, doing so
//     would reuse the prior state value whenever the proposed value is
//     Unknown, preventing a practitioner from ever changing these back out
//     via config in the same plan cycle where some other attribute forces a
//     replan.
func applySegmentUseStateForUnknown(s *schema.Schema) {
	patchSegmentString(s, "id")
	patchSegmentString(s, "created")
}

func patchSegmentString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
