package access_profile_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyAccessProfileUseStateForUnknown patches
// resource_access_profile.AccessProfileResourceSchema's generated Attributes
// map with stringplanmodifier.UseStateForUnknown() on a deliberately narrow
// set of scalar, server-managed Computed attributes, working around a
// systemic gap in the tfplugingen-framework pipeline: it never emits any
// PlanModifiers, so every unconfigured Computed attribute is proposed as
// Unknown on every plan (not just when it actually changes), producing a
// permanent, spurious "(known after apply)" diff. See role_v1's
// resource_role_planmodifiers.go for the full investigation this mirrors.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves,
// never a schema.SingleNestedAttribute/ListNestedAttribute:
//   - "modified" is genuinely volatile (changes on every real Update, even
//     when unrelated fields change) - UseStateForUnknown would pin the plan
//     to the stale value and cause a hard "Provider produced inconsistent
//     result after apply" error on Update.
//   - "entitlements", "additional_owners", "segments", and
//     "provisioning_criteria" have real write support in
//     accessProfileModelToDto (a practitioner can configure, change, or
//     remove them), so UseStateForUnknown must not be applied - it would
//     prevent a practitioner from ever clearing one of these attributes back
//     out via config.
//   - "access_model_metadata", "access_request_config", and
//     "revocation_request_config" are NOT protected here, mirroring role_v1's
//     documented finding that combining an object-level plan modifier with a
//     SingleNestedAttribute whose nested schema contains further nested
//     Computed attributes with their own CustomTypes reproducibly breaks even
//     a first-ever `terraform plan` with a hard "<leaf-attribute-name> is
//     missing from object" error from the generated CustomType's
//     `fromObject` conversion function.
func applyAccessProfileUseStateForUnknown(s *schema.Schema) {
	patchString(s, "id")
	patchString(s, "created")

	if owner, ok := s.Attributes["owner"].(schema.SingleNestedAttribute); ok {
		if name, ok := owner.Attributes["name"].(schema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			owner.Attributes["name"] = name
		}
		s.Attributes["owner"] = owner
	}

	if source, ok := s.Attributes["source"].(schema.SingleNestedAttribute); ok {
		if name, ok := source.Attributes["name"].(schema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			source.Attributes["name"] = name
		}
		s.Attributes["source"] = source
	}
}

func patchString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
