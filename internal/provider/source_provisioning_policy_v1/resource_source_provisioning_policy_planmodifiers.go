package source_provisioning_policy_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// sourceProvisioningPolicyFieldsDescription is shared between the resource
// and data source schema descriptions for the hand-added "fields" attribute.
const sourceProvisioningPolicyFieldsDescription = "The list of fields (attribute-to-transform mappings) that make up this " +
	"provisioning policy's template, as a raw JSON array. Each element's shape matches the API's FieldDetailsDto - see " +
	"https://developer.sailpoint.com/docs/extensibility/transforms/guides/transforms-in-provisioning-policies for examples. " +
	"This is a raw JSON string (via jsontypes.Normalized, semantic not textual equality) because each field's own " +
	"\"transform\"/\"attributes\" sub-properties are a discriminated union keyed by a sibling \"type\" - the same shape as " +
	"identitynow_transform_v1's \"attributes\" attribute."

// applySourceProvisioningPolicyFieldsField hand-adds the "fields" attribute
// that generator_config_source_provisioning_policy_v1.yml deliberately tells
// tfplugingen-openapi/-framework to ignore (see the package doc for the full
// dynamic-shape rationale), plus the "id" attribute the generator never
// produced at all (ProvisioningPolicyDto has no native id - see the package
// doc's v1-vs-v2 decision).
func applySourceProvisioningPolicyFieldsField(attrs *map[string]schema.Attribute, resourceMode bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	(*attrs)["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Synthesized composite id in the form \"source_id/usage_type\" (ProvisioningPolicyDto has no native id).",
		MarkdownDescription: "Synthesized composite id in the form `source_id/usage_type` (`ProvisioningPolicyDto` has no native id).",
	}
	if resourceMode {
		(*attrs)["fields"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Optional:            true,
			Computed:            true,
			Description:         sourceProvisioningPolicyFieldsDescription,
			MarkdownDescription: sourceProvisioningPolicyFieldsDescription,
		}
	} else {
		(*attrs)["fields"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Computed:            true,
			Description:         sourceProvisioningPolicyFieldsDescription,
			MarkdownDescription: sourceProvisioningPolicyFieldsDescription,
		}
	}
}

// applySourceProvisioningPolicyPlanModifiers patches
// resource_source_provisioning_policy.SourceProvisioningPolicyResourceSchema's
// generated Attributes map with plan modifiers, working around the systemic
// tfplugingen-framework gap where it never emits any PlanModifiers - see the
// identical, already-confirmed role_v1/service_desk_integration_v1/
// transform_v1 cases.
//   - "id" is Computed-only with no schema Default - needs
//     UseStateForUnknown to avoid a perpetual Unknown-on-every-plan diff.
//   - "source_id"/"usage_type" are Required (forced via schema_overrides_
//     source_provisioning_policy_v1.yml) and identify which underlying API
//     object this resource manages - changing either means a completely
//     different provisioning policy, so both get RequiresReplace.
//   - "description" is Optional+Computed with no Default - also needs
//     UseStateForUnknown so an unconfigured value doesn't plan as Unknown
//     forever.
//   - "fields" (hand-added above) is Optional+Computed but intentionally
//     does NOT get UseStateForUnknown, mirroring transform_v1's identical
//     choice for "attributes": jsontypes.Normalized's own
//     StringSemanticEquals already prevents false diffs from
//     whitespace/key-ordering alone, so a real content change should still
//     surface as a diff.
func applySourceProvisioningPolicyPlanModifiers(s *schema.Schema) {
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["source_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["source_id"] = attr
	}
	if attr, ok := s.Attributes["usage_type"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["usage_type"] = attr
	}
	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["description"] = attr
	}
}
