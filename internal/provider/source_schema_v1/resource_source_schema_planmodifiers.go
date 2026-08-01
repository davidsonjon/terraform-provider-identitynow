package source_schema_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// sourceSchemaConfigurationDescription is shared between the resource and
// data source schema descriptions for the hand-added "configuration"
// attribute.
const sourceSchemaConfigurationDescription = "Holds any extra configuration data that the schema may require, as a raw " +
	"JSON object (via jsontypes.Normalized, semantic not textual equality). There is no fixed set of keys - contents " +
	"vary by connector/schema type."

// applySourceSchemaConfigurationField hand-adds the "configuration"
// attribute that generator_config_source_schema_v1.yml deliberately tells
// tfplugingen-openapi/-framework to ignore (see the package doc for the full
// dynamic-shape rationale).
func applySourceSchemaConfigurationField(attrs *map[string]schema.Attribute, resourceMode bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	if resourceMode {
		(*attrs)["configuration"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Optional:            true,
			Computed:            true,
			Description:         sourceSchemaConfigurationDescription,
			MarkdownDescription: sourceSchemaConfigurationDescription,
		}
	} else {
		(*attrs)["configuration"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Computed:            true,
			Description:         sourceSchemaConfigurationDescription,
			MarkdownDescription: sourceSchemaConfigurationDescription,
		}
	}
}

// applySourceSchemaPlanModifiers patches
// resource_source_schema.SourceSchemaResourceSchema's generated Attributes
// map with plan modifiers, working around the systemic
// tfplugingen-framework gap where it never emits any PlanModifiers - see the
// identical, already-confirmed role_v1/service_desk_integration_v1/
// transform_v1/source_provisioning_policy_v1 cases.
//   - "id"/"schema_id" are Computed-only (forced via schema_overrides_
//     source_schema_v1.yml's computed_only_overrides) with no schema
//     Default - need UseStateForUnknown to avoid a perpetual
//     Unknown-on-every-plan diff.
//   - "source_id" is Required (forced via required_overrides) and identifies
//     which Source this schema belongs to - changing it means an entirely
//     different parent object, so it gets RequiresReplace.
//   - "name" is Required (forced via required_overrides) and, per
//     putSourceSchemaV1's own documented immutable-fields list ("id", "name",
//     "created", "modified" cannot be updated - attempting to do so is a
//     400), also gets RequiresReplace.
//   - Every other Computed-optional scalar attribute with no Default
//     (created, modified, display_attribute, hierarchy_attribute,
//     identity_attribute, native_object_type) gets UseStateForUnknown for
//     the same reason as "id"/"schema_id". "include_permissions" already has
//     a generated `Default: booldefault.StaticBool(false)`, so it needs no
//     modifier.
//   - "attributes"/"features" (Computed-optional lists with no Default) get
//     listplanmodifier.UseStateForUnknown() for the same reason.
//   - "configuration" (hand-added above) is Optional+Computed but
//     intentionally does NOT get UseStateForUnknown, mirroring
//     transform_v1's identical choice for "attributes"/
//     source_provisioning_policy_v1's "fields": jsontypes.Normalized's own
//     StringSemanticEquals already prevents false diffs from
//     whitespace/key-ordering alone, so a real content change should still
//     surface as a diff.
func applySourceSchemaPlanModifiers(s *schema.Schema) {
	useStateForUnknownStrings := []string{
		"id", "schema_id", "created", "modified", "display_attribute",
		"hierarchy_attribute", "identity_attribute", "native_object_type",
	}
	for _, name := range useStateForUnknownStrings {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			s.Attributes[name] = attr
		}
	}

	useStateForUnknownLists := []string{"attributes", "features"}
	for _, name := range useStateForUnknownLists {
		if attr, ok := s.Attributes[name].(schema.ListNestedAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, listplanmodifier.UseStateForUnknown())
			s.Attributes[name] = attr
		} else if attr, ok := s.Attributes[name].(schema.ListAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, listplanmodifier.UseStateForUnknown())
			s.Attributes[name] = attr
		}
	}

	if attr, ok := s.Attributes["source_id"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["source_id"] = attr
	}
	if attr, ok := s.Attributes["name"].(schema.StringAttribute); ok {
		attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
		s.Attributes["name"] = attr
	}
}
