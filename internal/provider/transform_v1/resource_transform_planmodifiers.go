package transform_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyTransformAttributesField hand-adds the "attributes" attribute that
// generator_config_transform_v1.yml deliberately tells tfplugingen-openapi/
// -framework to ignore (see the package doc for the full dynamic-shape
// rationale). It is a plain schema.StringAttribute using
// jsontypes.NormalizedType{} as its CustomType, so practitioners write raw
// JSON matching whatever shape the transform's "type" requires, and drift
// detection/equality is semantic (not textual) JSON comparison rather than a
// byte-for-byte string compare.
//
// required=true for the resource (the API's own requestBody marks "attributes"
// as required - every transform type needs at least "{}"); the data source
// gets a Computed-only copy instead (a read has nothing to configure).
func applyTransformAttributesField(attrs *map[string]schema.Attribute, required bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	desc := "Meta-data about the transform, as a raw JSON object. Values are specific to the transform's \"type\" - " +
		"see https://developer.sailpoint.com/docs/extensibility/transforms/operations for the shape each \"type\" expects."
	if required {
		(*attrs)["attributes"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Required:            true,
			Description:         desc,
			MarkdownDescription: desc,
		}
	} else {
		(*attrs)["attributes"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Computed:            true,
			Description:         desc,
			MarkdownDescription: desc,
		}
	}
}

// applyTransformUseStateForUnknown patches resource_transform.TransformResourceSchema's
// generated Attributes map with a plan modifier on "id" (and, defensively,
// "internal") to work around the systemic tfplugingen-framework gap - it
// never emits any PlanModifiers, so every unconfigured Computed attribute is
// proposed as Unknown on every plan, not just when it actually changes - see
// the identical, already-confirmed-and-fixed role_v1/service_desk_integration_v1
// cases and the 2026-07-24 knowledge entries for the full investigation.
//
//   - "id" has no schema Default and is Computed-only, so without this it
//     would be proposed Unknown on every plan after creation - a hard
//     UseStateForUnknown fix, exactly like role_v1's "id"/"created" and
//     service_desk_integration_v1's "id"/"created"/"modified".
//   - "internal" is also Computed-only, but the generator already emitted
//     `Default: booldefault.StaticBool(false)` for it (from the OpenAPI
//     schema's `default: false`) - a Default plan modifier already resolves
//     an unconfigured proposal to a known value at plan time, so this is
//     purely defensive/for consistency with the other two _v1 targets, not
//     required to pass `make plan`/acceptance testing.
//   - "name" and "type" are plain Required (never Computed), so Core never
//     proposes them Unknown in the first place; no modifier is needed or
//     applied here.
//   - "attributes" (hand-added above) is likewise Required (resource) - not
//     Computed - so it needs no plan modifier either. It intentionally does
//     NOT get UseStateForUnknown even on the data source's Computed copy,
//     since jsontypes.Normalized's own StringSemanticEquals already prevents
//     spurious diffs from whitespace/key-ordering differences between
//     config/state and the API's response - a real content change SHOULD
//     still show as a diff, unlike a field this pipeline can't write at all.
func applyTransformUseStateForUnknown(s *schema.Schema) {
	if attr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
		s.Attributes["id"] = attr
	}
	if attr, ok := s.Attributes["internal"].(schema.BoolAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, boolplanmodifier.UseStateForUnknown())
		s.Attributes["internal"] = attr
	}
}
