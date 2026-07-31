package segment_access_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applySegmentAccessPlanModifiers patches this hand-written schema with
// stringplanmodifier.UseStateForUnknown() on the immutable Computed `id`
// attribute, following the same repo-wide pattern as the generated-resource
// *_planmodifiers.go fixes: without this, every plan that changes anything
// else would propose `id = (known after apply)` even though this resource's id
// is always just the stable Segment id. `segment_id` itself gets
// RequiresReplace() because changing which Segment is managed is a different
// resource instance, not an in-place migration.
func applySegmentAccessPlanModifiers(s *schema.Schema) {
	patchSegmentAccessStringUseStateForUnknown(s, "id")
	patchSegmentAccessStringRequiresReplace(s, "segment_id")
}

func patchSegmentAccessStringUseStateForUnknown(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}

func patchSegmentAccessStringRequiresReplace(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.RequiresReplace())
	s.Attributes[name] = a
}
