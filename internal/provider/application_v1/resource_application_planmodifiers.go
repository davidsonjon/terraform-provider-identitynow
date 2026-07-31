package application_v1

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyApplicationUseStateForUnknown patches
// resource_application.ApplicationResourceSchema's generated Attributes map with
// stringplanmodifier.UseStateForUnknown() on a narrow set of server-managed
// string leaves, mirroring role_v1/access_profile_v1. This avoids a permanent
// spurious "(known after apply)" diff from tfplugingen-framework's generated
// schemas, which currently never emit plan modifiers.
//
// Scope is intentionally narrow:
//   - only plain schema.StringAttribute leaves are patched, never parent object
//     attributes, to avoid the generated CustomType/object-plan-modifier
//     interaction documented in role_v1/access_profile_v1.
//   - "modified" is deliberately not patched because it changes on every real
//     update and pinning it to prior state would cause an inconsistent-result
//     error after apply.
//   - "access_profile_ids" is handled separately as a set attribute in
//     resource_application.go.
func applyApplicationUseStateForUnknown(s *resourceschema.Schema) {
	patchApplicationString(s, "id")
	patchApplicationString(s, "created")

	if owner, ok := s.Attributes["owner"].(resourceschema.SingleNestedAttribute); ok {
		if name, ok := owner.Attributes["name"].(resourceschema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			owner.Attributes["name"] = name
		}
		s.Attributes["owner"] = owner
	}

	if accountSource, ok := s.Attributes["account_source"].(resourceschema.SingleNestedAttribute); ok {
		if name, ok := accountSource.Attributes["name"].(resourceschema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			accountSource.Attributes["name"] = name
		}
		s.Attributes["account_source"] = accountSource
	}
}

func patchApplicationString(s *resourceschema.Schema, name string) {
	a, ok := s.Attributes[name].(resourceschema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
