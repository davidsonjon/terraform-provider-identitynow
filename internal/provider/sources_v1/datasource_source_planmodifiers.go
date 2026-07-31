package sources_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// applySourceDataSourceConnectorAttributesField hand-adds the
// "connector_attributes" attribute to the singular identitynow_source_v1
// data source's generated schema - see applySourceConnectorAttributesField in
// resource_source_planmodifiers.go for the full rationale. The data source's
// copy is always Computed-only (a read has nothing to configure).
func applySourceDataSourceConnectorAttributesField(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	desc := "Connector-specific configuration. This configuration differs from connector type to connector type " +
		"(e.g. Active Directory vs. Workday vs. a Delimited File source each expect different sub-keys) - represented " +
		"as a raw JSON object via `jsontypes.Normalized` rather than a fixed schema, since tfplugingen-openapi/-framework " +
		"cannot generate a faithful static schema for a connector-type-discriminated union."
	(*attrs)["connector_attributes"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}
