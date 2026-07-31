// See resource_application.go in this package for design notes and known
// limitations shared by both the resource and these data sources.
package application_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
)

var (
	_ datasource.DataSource              = (*applicationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*applicationsDataSource)(nil)
)

func NewApplicationsDataSource() datasource.DataSource {
	return &applicationsDataSource{}
}

type applicationsDataSource struct {
	client *sailpoint.APIClient
}

type ApplicationsDataSourceModel struct {
	Filters      types.String `tfsdk:"filters"`
	Sorters      types.String `tfsdk:"sorters"`
	Limit        types.Int64  `tfsdk:"limit"`
	Offset       types.Int64  `tfsdk:"offset"`
	Applications types.List   `tfsdk:"applications"`
}

func applicationsListNestedAttributes(ctx context.Context) map[string]datasourceschema.Attribute {
	attrs := applicationDataSourceAttributes(ctx)
	out := make(map[string]datasourceschema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = datasourceschema.StringAttribute{
		Computed:            true,
		Description:         "ID of the application.",
		MarkdownDescription: "ID of the application.",
	}
	return out
}

func (d *applicationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_applications_v1"
}

func (d *applicationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Lists Applications (source apps) from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists Applications (source apps) from IdentityNow/ISC via `GET /source-apps/v1/all`, optionally filtered, sorted, and paginated. " +
			"Returns the same attributes per application as the singular `identitynow_application_v1` data source, including `access_profile_ids`.",
		Attributes: map[string]datasourceschema.Attribute{
			"filters": datasourceschema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query applications (e.g. `name sw \"Engineering\"`, `enabled eq true`). See " +
					"[V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the supported fields/operators for `GET /source-apps/v1/all`.",
			},
			"sorters": datasourceschema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. Sorting is supported for `id`, `name`, `created`, `modified`, `owner.id`, and `accountSource.id`. " +
					"See [V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"limit": datasourceschema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of applications to return. The API's documented maximum for this endpoint is 250; " +
					"values above 250 are capped to 250 with a warning.",
			},
			"offset": datasourceschema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"applications": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Applications matching the query, each with the same attributes as `identitynow_application_v1`.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: applicationsListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *applicationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *applicationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ApplicationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := d.client.Beta.AppsAPI.ListAllSourceApp(ctx)
	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		requestedLimit := config.Limit.ValueInt64()
		if requestedLimit > applicationListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /source-apps/v1/all's documented maximum of %d. Using %d instead.", requestedLimit, applicationListMaxLimit, applicationListMaxLimit),
			)
			apiReq = apiReq.Limit(applicationListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	tflog.Debug(ctx, "Reading Applications data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Applications data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Applications", applicationErrDetail(err, httpResp))
		return
	}

	applicationsAttrSchema := datasourceschema.Schema{
		Attributes: map[string]datasourceschema.Attribute{
			"applications": datasourceschema.ListNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: applicationsListNestedAttributes(ctx),
				},
			},
		},
	}
	fullType := applicationsAttrSchema.Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["applications"].(basetypes.ListType).ElemType

	models := make([]applicationDataSourceModel, 0, len(dtos))
	for i := range dtos {
		id := ""
		if dtos[i].Id != nil {
			id = *dtos[i].Id
		}
		accessProfileIDs, diags := listApplicationAccessProfileIDs(ctx, d.client, id)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		model, diags := applicationDatasourceDtoToModel(ctx, &dtos[i], accessProfileIDs, applicationDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	applicationsList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Applications = applicationsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
