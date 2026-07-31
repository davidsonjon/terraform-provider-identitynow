package access_model_metadata_attribute_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyAccessModelMetadataAttributeUseStateForUnknown patches the generated
// schema with the same systemic tfplugingen-framework fix already applied to
// role_v1/service_desk_integration_v1/transform_v1 (the generator never
// emits any PlanModifiers, so every unconfigured Computed attribute is
// proposed Unknown on every plan) PLUS RequiresReplace() on the 3 fields the
// API accepts at Create but will never patch afterward - see the package doc
// in resource_access_model_metadata_attribute.go for the full rationale.
//
//   - "key", "type", "object_types": accepted at Create, but the API's own
//     PATCH documentation does not list them as patchable - RequiresReplace()
//     forces a destroy+recreate on a config change instead of a silent no-op
//     that would otherwise leave permanent, undetected drift (Update() never
//     even attempts to patch them). UseStateForUnknown() is layered on top so
//     an unconfigured value doesn't show a spurious "(known after apply)" on
//     every plan once it's set.
//   - "status": Computed-only in practice (the API doesn't document any way
//     to set it meaningfully, and it isn't patchable either) - safe for
//     UseStateForUnknown() alone, no RequiresReplace() since a config change
//     to it wouldn't be actionable either way and there's no evidence it can
//     ever legitimately change out from under Terraform.
//   - "name", "description", "multiselect", "values" are all genuinely
//     patchable/practitioner-writable (see Update()) - per the same rule
//     role_v1 established for its writable Computed blocks
//     (access_profiles/entitlements/etc.), NO plan modifier is applied to
//     these: UseStateForUnknown() would incorrectly pin an intentional
//     "clear this back out" config change to the stale prior-state value.
//     Practitioners should set an explicit literal (even "" or []) rather
//     than omitting these if they want a fully converged, diff-free plan.
func applyAccessModelMetadataAttributeUseStateForUnknown(s *schema.Schema) {
	if attr, ok := s.Attributes["key"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["key"] = attr
	}
	if attr, ok := s.Attributes["type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		)
		s.Attributes["type"] = attr
	}
	if attr, ok := s.Attributes["object_types"].(schema.ListAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers,
			listplanmodifier.UseStateForUnknown(),
			listplanmodifier.RequiresReplace(),
		)
		s.Attributes["object_types"] = attr
	}
	if attr, ok := s.Attributes["status"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["status"] = attr
	}
}
