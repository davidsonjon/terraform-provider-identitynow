package identity_profile_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// applyIdentityAttributeConfigDataSourceField hand-adds the
// "identity_attribute_config" attribute to the singular
// identitynow_identity_profile_v1 data source's generated schema - see
// applyIdentityAttributeConfigField in resource_identity_profile_planmodifiers.go
// for the full rationale. The data source's copy is always Computed-only (a
// read has nothing to configure).
func applyIdentityAttributeConfigDataSourceField(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	desc := "The identity attribute mapping/transform configuration for this Identity Profile " +
		"(`{enabled, attributeTransforms: [...]}`). Represented as a raw JSON object via `jsontypes.Normalized` rather " +
		"than a fixed schema, since tfplugingen-openapi/-framework cannot generate a faithful static schema for a " +
		"transform-type-discriminated union."
	(*attrs)["identity_attribute_config"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

// applyIdentityProfileDataSourceIdRequired hand-patches the singular
// identitynow_identity_profile_v1 data source's generated "id" attribute
// from Computed-only to Required.
//
// Unlike sources_v1/role_v1/governance_group_v1 (whose per-service v1 specs
// name their read path parameter literally "{id}", which tfplugingen-openapi
// auto-correlates with the response body's own "id" property and marks
// Required for free), identity-profiles' read path parameter is named
// "{identity-profile-id}" - a different name than the response body's "id"
// property, so tfplugingen-openapi does NOT recognize the correlation and
// leaves "id" as a plain Computed-only output attribute here. Without this
// hand-patch, the singular data source would have no way for a practitioner
// to specify which Identity Profile to look up. See the package doc's
// "Codegen notes" for the sibling "identityprofileid" duplicate-attribute
// symptom of this same root cause (fixed via schema_overrides'
// drop_attributes instead, since that one affects real generated
// attributes, not this hand-written schema mutation).
func applyIdentityProfileDataSourceIdRequired(s *dsschema.Schema) {
	a, ok := s.Attributes["id"].(dsschema.StringAttribute)
	if !ok {
		return
	}
	a.Required = true
	a.Computed = false
	s.Attributes["id"] = a
}
