package connector_rule_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applyConnectorRuleAttributesField hand-adds the "attributes" attribute that
// generator_config_connector_rule_v1.yml deliberately tells tfplugingen-openapi/
// -framework to ignore (see the package doc for the full rationale). It is a
// plain schema.StringAttribute using jsontypes.NormalizedType{} as its
// CustomType, mirroring transform_v1's identical "attributes" treatment -
// practitioners write raw JSON, and drift detection/equality is semantic
// (not textual) JSON comparison.
//
// resource=true gets a plain Required (not Computed) attribute. Confirmed
// live against a sandbox tenant that CreateConnectorRule/UpdateConnectorRule
// silently inject a server-computed "sourceVersion" key (mirroring
// "source_code.version") into the returned "attributes" object even when the
// practitioner's config sent "{}". Terraform requires a Required attribute's
// post-apply value to exactly equal the practitioner's configured value, and
// - critically - does not permit Optional(+Computed) attributes to be
// planned Unknown when configured either ("provider produced invalid plan"),
// so there is no schema-level way to let the provider freely echo back the
// API's augmented value here. Instead, this resource deliberately never
// writes the API's returned "attributes" into state (see
// connectorRuleResponseToModel in resource_connector_rule.go) - state always
// reflects the practitioner's own configured value. This means out-of-band
// changes to "attributes" made outside Terraform (including the API's own
// "sourceVersion" injection) are not detected as drift - a documented `_v1`
// pilot limitation, not a bug.
//
// The data source gets a Computed-only copy instead (a read has nothing to
// configure, and a Computed-only attribute has no config value to conflict
// with, so it can freely reflect whatever the API actually returns).
func applyConnectorRuleAttributesField(attrs *map[string]schema.Attribute, resource bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	desc := "A raw JSON object of arbitrary metadata about the connector rule. Unlike identitynow_transform_v1's " +
		"\"attributes\" (a discriminated union keyed by \"type\"), this has no fixed shape."
	if resource {
		desc += " Note: the API is known to add a server-computed \"sourceVersion\" key (mirroring " +
			"source_code.version) to the stored value; this resource does not reflect that (or any other " +
			"out-of-band change to \"attributes\") back into state - state always mirrors what was last configured."
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

// applyConnectorRuleUseStateForUnknown patches
// resource_connector_rule.ConnectorRuleResourceSchema's generated Attributes
// map with stringplanmodifier.UseStateForUnknown() on the Computed-only
// "id"/"created"/"modified" attributes - none of them have a schema Default,
// so without this each would be proposed Unknown on every plan after
// creation, not just when it actually changes (the same systemic
// tfplugingen-framework gap already fixed for role_v1/transform_v1/
// service_desk_integration_v1 - it never emits any PlanModifiers itself).
//
//   - "description" is Optional+Computed (practitioner-settable) - it must
//     NOT get UseStateForUnknown, or a real config change would be masked.
//   - "signature" is likewise Optional+Computed and practitioner-settable
//     (its "output" sub-attribute is also Optional+Computed for the same
//     reason) - no UseStateForUnknown here either, matching role_v1/
//     transform_v1's precedent of never blanket-applying this to
//     user-configurable attributes.
func applyConnectorRuleUseStateForUnknown(s *schema.Schema) {
	for _, name := range []string{"id", "created", "modified"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			s.Attributes[name] = attr
		}
	}
}

// applyConnectorRuleRequiresReplace adds stringplanmodifier.RequiresReplace()
// to "name" and "type", both plain Required strings the connector-rule API
// documents as immutable once a rule is created ("Attempting to change
// [name/type] will result in an error" - unlike transform_v1's analogous
// situation, which left this as an undocumented follow-up rather than
// enforcing it, this is a fresh _v1 target so the enforcement is added
// proactively here rather than deferred).
func applyConnectorRuleRequiresReplace(s *schema.Schema) {
	for _, name := range []string{"name", "type"} {
		if attr, ok := s.Attributes[name].(schema.StringAttribute); ok {
			attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.RequiresReplace())
			s.Attributes[name] = attr
		}
	}
}
