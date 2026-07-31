package identity_profile_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyIdentityAttributeConfigField hand-adds the
// "identity_attribute_config" attribute that
// generator_config_identity_profile_v1.yml deliberately tells
// tfplugingen-openapi/-framework to ignore (see the package doc for the full
// dynamic-shape rationale, identical to transform_v1's "attributes"/
// sources_v1's "connectorAttributes"). It is a plain schema.StringAttribute
// using jsontypes.NormalizedType{} as its CustomType, so practitioners write
// raw JSON matching the whole `{enabled, attributeTransforms: [...]}` object,
// and drift detection/equality is semantic (not textual) JSON comparison
// rather than a byte-for-byte string compare.
//
// "identityAttributeConfig" is genuinely Optional in the Identity Profile
// schema - required=false always (both the resource and data source get an
// Optional+Computed / Computed-only copy respectively).
func applyIdentityAttributeConfigField(attrs *map[string]schema.Attribute, computedOnly bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	desc := "The identity attribute mapping/transform configuration for this Identity Profile " +
		"(`{enabled, attributeTransforms: [...]}`). Each attributeTransforms entry's `transformDefinition.attributes` " +
		"shape depends entirely on the sibling `transformDefinition.type` - represented as a raw JSON object via " +
		"`jsontypes.Normalized` rather than a fixed schema, since tfplugingen-openapi/-framework cannot generate a " +
		"faithful static schema for a transform-type-discriminated union."
	if computedOnly {
		(*attrs)["identity_attribute_config"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Computed:            true,
			Description:         desc,
			MarkdownDescription: desc,
		}
		return
	}
	(*attrs)["identity_attribute_config"] = schema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Optional:            true,
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

// applyIdentityProfileUseStateForUnknown patches
// resource_identity_profile.IdentityProfileResourceSchema's generated
// Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar attributes, working around the same
// systemic tfplugingen-framework pipeline gap documented for every other
// _v1 pilot target: tfplugingen-framework never emits any PlanModifiers, so
// every unconfigured Computed attribute is proposed as Unknown on every plan
// (not just when it actually changes), producing a permanent, spurious
// "(known after apply)" diff.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves
// that are genuinely server-managed and never written by modelToDto, and
// whose value the live API does not spontaneously change on an unrelated
// Update:
//   - "id": server-assigned on Create, never configurable - safe.
//   - "created": server-assigned, never configurable - safe.
//
// Deliberately EXCLUDED (left showing "(known after apply)" until a real
// Read/Update populates them, matching sources_v1/governance_group_v1's
// established convention):
//   - "modified": genuinely volatile - refreshed unconditionally from the
//     live API on every Read/Update (identity refresh cycles, sync jobs, and
//     other out-of-Terraform's-control events can change it).
//   - "identity_count"/"identity_refresh_required"/"has_time_based_attr":
//     driven by identity aggregation/refresh cycles outside this resource's
//     own CRUD, genuinely expected to change between applies.
func applyIdentityProfileUseStateForUnknown(s *schema.Schema) {
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
