package entitlement_request_config_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyEntitlementRequestConfigUseStateForUnknown patches the generated schema
// with stringplanmodifier.UseStateForUnknown() on the immutable Computed `id`.
// This resource adopts an existing entitlement by id and all subsequent
// Create/Read/Update operations continue targeting that same entitlement, so
// re-planning unrelated changes must not degrade `id` to "(known after apply)".
func applyEntitlementRequestConfigUseStateForUnknown(s *schema.Schema) {
	a, ok := s.Attributes["id"].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes["id"] = a
}
