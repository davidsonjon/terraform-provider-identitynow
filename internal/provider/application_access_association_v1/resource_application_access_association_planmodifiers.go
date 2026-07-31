package application_access_association_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyApplicationAccessAssociationPlanModifiers patches this hand-written
// schema with stringplanmodifier.UseStateForUnknown() on the immutable
// Computed `id` attribute and RequiresReplace() on `application_id`, matching
// the repo-wide convention for stable, derived identifiers on hand-written
// resources.
func applyApplicationAccessAssociationPlanModifiers(s *schema.Schema) {
	patchApplicationAccessAssociationStringUseStateForUnknown(s, "id")
	patchApplicationAccessAssociationStringRequiresReplace(s, "application_id")
}

func patchApplicationAccessAssociationStringUseStateForUnknown(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}

func patchApplicationAccessAssociationStringRequiresReplace(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.RequiresReplace())
	s.Attributes[name] = a
}
