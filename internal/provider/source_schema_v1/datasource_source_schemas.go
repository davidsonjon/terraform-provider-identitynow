// This file adds a plural "list" data source alongside the singular
// identitynow_source_schema_v1 data source in datasource_source_schema.go,
// mirroring role_v1/datasource_roles.go and
// source_provisioning_policy_v1/datasource_source_provisioning_policies.go's
// established pattern. It queries GET /sources/v1/{sourceId}/schemas
// (api_beta.SourcesAPIService.GetSourceSchemas) instead of
// GET /sources/v1/{sourceId}/schemas/{schemaId}, and returns every schema
// defined on the source using the same nested object shape as
// identitynow_source_schema_v1.
package source_schema_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/source_schema_v1/datasource_source_schema"
)

var (
	_ datasource.DataSource              = (*sourceSchemasDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourceSchemasDataSource)(nil)
)

func NewSourceSchemasDataSource() datasource.DataSource {
	return &sourceSchemasDataSource{}
}

type sourceSchemasDataSource struct {
	client *sailpoint.APIClient
}

// SourceSchemasDataSourceModel is hand-written (not generated) since it
// wraps the list-query parameters plus a "schemas" attribute nesting the
// same shape as the singular data source.
type SourceSchemasDataSourceModel struct {
	SourceId     types.String `tfsdk:"source_id"`
	IncludeTypes types.String `tfsdk:"include_types"`
	IncludeNames types.String `tfsdk:"include_names"`
	Schemas      types.List   `tfsdk:"schemas"`
}

// sourceSchemasListNestedAttributes returns the same attribute map as the
// singular data source's schema (including the hand-added "configuration"
// attribute), with "source_id"/"schema_id" overridden to Computed-only (they
// are just output fields of a fully Computed list here, not lookup keys).
func sourceSchemasListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_source_schema.SourceSchemaDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	applySourceSchemaConfigurationFieldDataSource(&out)
	out["source_id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The Source id.",
		MarkdownDescription: "The Source id.",
	}
	out["schema_id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The Schema id.",
		MarkdownDescription: "The Schema id.",
	}
	return out
}

func (d *sourceSchemasDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_schemas_v1"
}

func (d *sourceSchemasDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Schemas defined on an existing Source in IdentityNow/ISC.",
		MarkdownDescription: "Lists all [Schemas](https://documentation.sailpoint.com/saas/help/accounts/schema.html) " +
			"defined on an existing Source in IdentityNow/ISC via `GET /sources/v1/{sourceId}/schemas`. Returns the same " +
			"attributes per schema as the singular `identitynow_source_schema_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_source_schema_v1`'s \"Known Limitations & Live " +
			"Testing Notes\" section before relying on it in production configurations; the same limitations apply to " +
			"each schema returned here.",
		Attributes: map[string]schema.Attribute{
			"source_id": schema.StringAttribute{
				Required:            true,
				Description:         "The Source id.",
				MarkdownDescription: "The Source id.",
			},
			"include_types": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "If set to `group`, only group schemas are returned. Only a value of `group` is " +
					"recognized presently.",
			},
			"include_names": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A comma-separated list of schema names to filter the result.",
			},
			"schemas": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Schemas defined on the source, each with the same attributes as `identitynow_source_schema_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: sourceSchemasListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *sourceSchemasDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sourceSchemasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SourceSchemasDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := config.SourceId.ValueString()
	tflog.Debug(ctx, "Reading Source Schemas data source", map[string]interface{}{"source_id": sourceID})

	apiReq := d.client.Beta.SourcesAPI.GetSourceSchemas(ctx, sourceID)
	if !config.IncludeTypes.IsNull() && !config.IncludeTypes.IsUnknown() {
		apiReq = apiReq.IncludeTypes(config.IncludeTypes.ValueString())
	}
	if !config.IncludeNames.IsNull() && !config.IncludeNames.IsUnknown() {
		apiReq = apiReq.IncludeNames(config.IncludeNames.ValueString())
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Source Schemas data source", map[string]interface{}{"source_id": sourceID, "error": err.Error()})
		resp.Diagnostics.AddError("Error listing Source Schemas", errDetail(err, httpResp))
		return
	}

	elemType := schema.NestedAttributeObject{Attributes: sourceSchemasListNestedAttributes(ctx)}.Type()

	models := make([]sourceSchemaDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(ctx, &dtos[i], sourceID, sourceSchemaDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	schemasList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Schemas = schemasList

	tflog.Debug(ctx, "Read Source Schemas data source", map[string]interface{}{"source_id": sourceID, "count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
