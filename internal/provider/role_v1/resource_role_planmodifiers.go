package role_v1

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyRoleUseStateForUnknown patches resource_role.RoleResourceSchema's generated
// Attributes map with stringplanmodifier.UseStateForUnknown() on a deliberately
// narrow set of scalar, server-managed Computed attributes, working around a
// systemic gap in the tfplugingen-framework pipeline: it never emits any
// PlanModifiers, so every unconfigured Computed attribute is proposed as
// Unknown on every plan (not just when it actually changes), producing a
// permanent, spurious "(known after apply)" diff and failing
// terraform-plugin-testing's built-in post-apply empty-plan check - see
// TestAccRoleV1Resource and the 2026 dated knowledge entry for the full
// investigation.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves,
// never a schema.SingleNestedAttribute/ListNestedAttribute:
//   - "modified" is genuinely volatile (changes on every real Update, even
//     when unrelated fields change) - UseStateForUnknown would pin the plan to
//     the stale value and cause a hard "Provider produced inconsistent result
//     after apply" error on Update. Acceptance tests must instead use a
//     `lifecycle { ignore_changes = [modified] }` block in the test config.
//   - "access_profiles", "dimension_refs", "entitlements", "additional_owners",
//     "segments", and "privilege_level" have real write support in
//     roleModelToDto (a practitioner can configure, change, or remove them),
//     so UseStateForUnknown must not be applied - per the framework's own
//     documented caveat, the modifier reuses the prior state value whenever
//     the proposed value is Unknown *regardless of whether config is null vs.
//     unknown*, which would prevent a practitioner from ever clearing one of
//     these attributes back out via config. Acceptance tests should instead
//     configure these attributes to an explicit (even empty) literal so Core
//     never proposes them as Unknown in the first place.
//   - "access_model_metadata", "access_request_config",
//     "revocation_request_config", "membership", and "legacy_membership_info"
//     were ALSO tried with objectplanmodifier.UseStateForUnknown() on the
//     parent schema.SingleNestedAttribute, but this reproducibly broke even a
//     first-ever `terraform plan` (no prior state at all) with a hard
//     "<leaf-attribute-name> is missing from object" error surfaced from the
//     generated CustomType's `fromObject` conversion function - confirmed via
//     isolated testing (enabling only one such patch at a time) to be a
//     framework/codegen interaction bug specific to combining an
//     object-level plan modifier with a SingleNestedAttribute whose nested
//     schema contains further nested Computed attributes with their own
//     CustomTypes (matches the objectplanmodifier.UseStateForUnknown() godoc's
//     own warning to prefer UseNonNullStateForUnknown or leaf-level modifiers
//     "for use-cases like a child attribute of a nested attribute"). These 5
//     blocks remain unprotected here by design; acceptance tests must use
//     `ExpectNonEmptyPlan: true` to account for them instead.
func applyRoleUseStateForUnknown(s *schema.Schema) {
	patchString(s, "id")
	patchString(s, "created")

	if owner, ok := s.Attributes["owner"].(schema.SingleNestedAttribute); ok {
		if name, ok := owner.Attributes["name"].(schema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			owner.Attributes["name"] = name
		}
		s.Attributes["owner"] = owner
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
