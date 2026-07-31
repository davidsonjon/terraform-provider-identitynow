// This file adds a plural "list" data source alongside the singular
// identitynow_source_v1 data source in datasource_source.go, mirroring
// role_v1's datasource_roles.go / access_profile_v1's datasource_access_profiles.go
// / governance_group_v1's datasource_governance_groups.go pattern. It queries
// GET /sources/v1 (api_beta.SourcesAPI.ListSources) instead of
// GET /sources/v1/{id}, and returns every matching Source using the exact
// same nested object shape as identitynow_source_v1 (reusing
// datasource_source's generated schema/model/value types plus this package's
// hand-added "connector_attributes"/datasourceDtoToModel converter) so
// practitioners get identical attribute names/types whether they read one
// source by id or query many by filter.
package sources_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/sources_v1/datasource_source"
)

// sourcesListMaxLimit matches GET /sources/v1's documented maximum "limit"
// value (see the shared limit.yaml parameter definition - 250, matching most
// other IdentityNow v1 list APIs).
const sourcesListMaxLimit = 250

var (
	_ datasource.DataSource              = (*sourcesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourcesDataSource)(nil)
)

func NewSourcesDataSource() datasource.DataSource {
	return &sourcesDataSource{}
}

type sourcesDataSource struct {
	client *sailpoint.APIClient
}

// SourcesDataSourceModel is hand-written (not generated) since it wraps the
// list-query parameters plus a "sources" attribute nesting the generated
// datasource_source.SourceModel shape (plus the hand-added
// "connector_attributes" field).
type SourcesDataSourceModel struct {
	Filters          types.String `tfsdk:"filters"`
	Sorters          types.String `tfsdk:"sorters"`
	Limit            types.Int64  `tfsdk:"limit"`
	Offset           types.Int64  `tfsdk:"offset"`
	ForSubadmin      types.String `tfsdk:"for_subadmin"`
	IncludeIDNSource types.Bool   `tfsdk:"include_idn_source"`
	Sources          types.List   `tfsdk:"sources"`
}

// sourcesListNestedAttributes returns the same attribute map as
// datasource_source.SourceDataSourceSchema (plus the hand-added
// "connector_attributes" field), except "id" is overridden to Computed-only.
// The singular identitynow_source_v1 data source marks "id" as Required
// because it's the lookup key for GET /sources/v1/{id}; here it's just
// another output field of a fully Computed "sources" list, so Required
// (which would otherwise render as though practitioners must set it) is
// wrong.
func sourcesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_source.SourceDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs)+1)
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Source ID.",
		MarkdownDescription: "Source ID.",
	}
	applySourceDataSourceConnectorAttributesField(&out)
	return out
}

func (d *sourcesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sources_v1"
}

func (d *sourcesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Sources from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Sources](https://documentation.sailpoint.com/saas/help/sources/index.html) from " +
			"IdentityNow/ISC via `GET /sources/v1`, optionally filtered, sorted, and paginated. Returns the same " +
			"attributes per source as the singular `identitynow_source_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_source_v1`'s (the resource) \"Known Limitations & " +
			"Live Testing Notes\" section before relying on it in production configurations; the same limitations apply " +
			"to each source returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query sources (e.g. `name eq \"Employees\"`). See " +
					"[V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the supported fields/operators for `GET /sources/v1`.",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. Sorting is supported for type, created, " +
					"modified, name, owner.name, healthy, status, id, description, owner.id, " +
					"accountCorrelationConfig.id/name, managerCorrelationRule.type/id/name, authoritative, " +
					"managementWorkgroup.id, connectorName, connectionType. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of sources to return. The API's documented maximum for this " +
					"endpoint is 250; values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"for_subadmin": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filters the returned list of sources for the identity specified by this " +
					"parameter, which is the id of an identity with the role `SOURCE_SUBADMIN`. By convention, the " +
					"value `me` indicates the identity id of the current user.",
			},
			"include_idn_source": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Include the IdentityNow source itself in the response.",
			},
			"sources": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sources matching the query, each with the same attributes as `identitynow_source_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: sourcesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *sourcesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SourcesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Sources data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.Beta.SourcesAPI.ListSources(ctx)

	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}
	if !config.ForSubadmin.IsNull() && !config.ForSubadmin.IsUnknown() {
		apiReq = apiReq.ForSubadmin(config.ForSubadmin.ValueString())
	}
	if !config.IncludeIDNSource.IsNull() && !config.IncludeIDNSource.IsUnknown() {
		apiReq = apiReq.IncludeIDNSource(config.IncludeIDNSource.ValueBool())
	}

	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		requestedLimit := config.Limit.ValueInt64()
		if requestedLimit > sourcesListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /sources/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, sourcesListMaxLimit, sourcesListMaxLimit),
			)
			apiReq = apiReq.Limit(sourcesListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Sources data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Sources", errDetail(err, httpResp))
		return
	}

	// attr.Type doesn't encode Required/Optional/Computed-ness (only the
	// underlying shape), so it's safe to build the element type from the
	// same attribute map used for each list item's schema
	// (sourcesListNestedAttributes, "id" Computed-only) even though the
	// singular data source's own schema marks "id" Required - see
	// governance_group_v1's identical elemType derivation for precedent.
	sourcesAttrSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"sources": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: sourcesListNestedAttributes(ctx),
				},
			},
		},
	}
	fullType := sourcesAttrSchema.Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["sources"].(basetypes.ListType).ElemType

	models := make([]sourceDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(ctx, &dtos[i], sourceDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	sourcesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Sources = sourcesList

	tflog.Debug(ctx, "Read Sources data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
