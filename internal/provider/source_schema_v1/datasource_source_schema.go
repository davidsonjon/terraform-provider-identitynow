// See resource_source_schema.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package source_schema_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/source_schema_v1/datasource_source_schema"
)

var (
	_ datasource.DataSource              = (*sourceSchemaDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourceSchemaDataSource)(nil)
)

func NewSourceSchemaDataSource() datasource.DataSource {
	return &sourceSchemaDataSource{}
}

type sourceSchemaDataSource struct {
	client *sailpoint.APIClient
}

// sourceSchemaDataSourceModel mirrors
// datasource_source_schema.SourceSchemaModel plus the hand-added
// "configuration" field - see sourceSchemaResourceModel's doc comment in
// resource_source_schema.go for why this can't just embed the generated
// model type.
type sourceSchemaDataSourceModel struct {
	Attributes         types.List           `tfsdk:"attributes"`
	Configuration      jsontypes.Normalized `tfsdk:"configuration"`
	Created            types.String         `tfsdk:"created"`
	DisplayAttribute   types.String         `tfsdk:"display_attribute"`
	Features           types.List           `tfsdk:"features"`
	HierarchyAttribute types.String         `tfsdk:"hierarchy_attribute"`
	Id                 types.String         `tfsdk:"id"`
	IdentityAttribute  types.String         `tfsdk:"identity_attribute"`
	IncludePermissions types.Bool           `tfsdk:"include_permissions"`
	Modified           types.String         `tfsdk:"modified"`
	Name               types.String         `tfsdk:"name"`
	NativeObjectType   types.String         `tfsdk:"native_object_type"`
	SchemaId           types.String         `tfsdk:"schema_id"`
	SourceId           types.String         `tfsdk:"source_id"`
}

func (d *sourceSchemaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_schema_v1"
}

func (d *sourceSchemaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_source_schema.SourceSchemaDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Schema on an existing Source from IdentityNow/ISC by source_id + schema_id."
	resp.Schema.MarkdownDescription = "Reads a [Schema](https://documentation.sailpoint.com/saas/help/accounts/schema.html) " +
		"on an existing Source from IdentityNow/ISC, identified by `source_id` + `schema_id`.\n\n" +
		"~> This is a `_v1` pilot data source - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations."
	applySourceSchemaConfigurationFieldDataSource(&resp.Schema.Attributes)
}

// applySourceSchemaConfigurationFieldDataSource mirrors
// applySourceSchemaConfigurationField (resource_source_schema_planmodifiers.go)
// but against a datasource/schema.Attribute map (a distinct Go type from
// resource/schema.Attribute, so it can't share the same function signature).
func applySourceSchemaConfigurationFieldDataSource(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	(*attrs)["configuration"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         sourceSchemaConfigurationDescription,
		MarkdownDescription: sourceSchemaConfigurationDescription,
	}
}

func (d *sourceSchemaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = cp.GetClient()
}

func (d *sourceSchemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sourceSchemaDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := config.SourceId.ValueString()
	schemaID := config.SchemaId.ValueString()
	tflog.Debug(ctx, "Reading Source Schema data source", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})

	dto, httpResp, err := d.client.Beta.SourcesAPI.
		GetSourceSchema(ctx, sourceID, schemaID).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Source Schema data source", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID, "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source Schema", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto, sourceID, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_source_schema.go but
// against the data source's model type.
func datasourceDtoToModel(ctx context.Context, dto *api_beta.Schema, sourceID string, fallback sourceSchemaDataSourceModel) (sourceSchemaDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.SourceId = types.StringValue(sourceID)
	model.Id = types.StringValue(dto.GetId())
	model.SchemaId = types.StringValue(dto.GetId())
	model.Name = types.StringPointerValue(dto.Name)
	model.NativeObjectType = types.StringPointerValue(dto.NativeObjectType)
	model.IdentityAttribute = types.StringPointerValue(dto.IdentityAttribute)
	model.DisplayAttribute = types.StringPointerValue(dto.DisplayAttribute)
	model.HierarchyAttribute = types.StringPointerValue(dto.HierarchyAttribute.Get())
	model.IncludePermissions = types.BoolPointerValue(dto.IncludePermissions)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified.Get())

	if dto.Features != nil {
		featuresList, d := types.ListValueFrom(ctx, types.StringType, dto.Features)
		diags.Append(d...)
		model.Features = featuresList
	} else {
		model.Features = types.ListNull(types.StringType)
	}

	config, d := normalizedConfigurationFromAPI(dto.Configuration)
	diags.Append(d...)
	model.Configuration = config

	attrsList, d := dsAttributesListFromAPI(ctx, dto.Attributes)
	diags.Append(d...)
	model.Attributes = attrsList

	return model, diags
}

// dsAttributesListFromAPI mirrors attributesListFromAPI in
// resource_source_schema.go but builds datasource_source_schema.AttributesValue
// elements (a distinct generated Go type from the resource package's, even
// though structurally identical) for this data source's "attributes" list.
func dsAttributesListFromAPI(ctx context.Context, defs []api_beta.AttributeDefinition) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_source_schema.AttributesValue{}.Type(ctx)
	if defs == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_source_schema.AttributesValue, 0, len(defs))
	for _, ad := range defs {
		schemaObj, d := schemaRefToObjectValue(ad.Schema)
		diags.Append(d...)

		attrType := ""
		if ad.Type != nil {
			attrType = string(*ad.Type)
		}

		v, d := datasource_source_schema.NewAttributesValue(
			datasource_source_schema.AttributesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"description":    types.StringPointerValue(ad.Description),
				"is_entitlement": types.BoolPointerValue(ad.IsEntitlement),
				"is_group":       types.BoolPointerValue(ad.IsGroup),
				"is_multi":       types.BoolPointerValue(ad.IsMulti),
				"name":           types.StringPointerValue(ad.Name),
				"native_name":    nullableStringToStringValue(ad.NativeName),
				"schema":         schemaObj,
				"type":           types.StringValue(attrType),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}
