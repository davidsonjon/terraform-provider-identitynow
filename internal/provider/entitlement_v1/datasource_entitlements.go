// See resource_entitlement.go in this package for the adopt-existing design
// notes and hand-added schema field rationale shared by the resource and these
// data sources.
package entitlement_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
)

const entitlementsListMaxLimit = 250

var (
	_ datasource.DataSource              = (*entitlementsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*entitlementsDataSource)(nil)
)

func NewEntitlementsDataSource() datasource.DataSource {
	return &entitlementsDataSource{}
}

type entitlementsDataSource struct {
	client *sailpoint.APIClient
}

type EntitlementsDataSourceModel struct {
	Filters      types.String `tfsdk:"filters"`
	Sorters      types.String `tfsdk:"sorters"`
	Limit        types.Int64  `tfsdk:"limit"`
	Offset       types.Int64  `tfsdk:"offset"`
	Entitlements types.List   `tfsdk:"entitlements"`
}

func entitlementsListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := entitlementDataSourceAttributes(ctx)
	out := make(map[string]schema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "ID of the Entitlement.",
		MarkdownDescription: "ID of the Entitlement.",
	}
	return out
}

func (d *entitlementsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlements_v1"
}

func (d *entitlementsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Entitlements from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists Entitlements from IdentityNow/ISC via `GET /entitlements/v1`, optionally filtered, sorted, and paginated. " +
			"Returns the same attributes per entitlement as the singular `identitynow_entitlement_v1` data source.",
		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query entitlements. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results).",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of entitlements to return. The API's documented maximum for this endpoint is 250; " +
					"values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"entitlements": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Entitlements matching the query, each with the same attributes as `identitynow_entitlement_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: entitlementsListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *entitlementsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *entitlementsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config EntitlementsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := d.client.EntitlementsAPI.ListEntitlementsV1(ctx)
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
		if requestedLimit > entitlementsListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /entitlements/v1's documented maximum of %d. Using %d instead.", requestedLimit, entitlementsListMaxLimit, entitlementsListMaxLimit),
			)
			apiReq = apiReq.Limit(entitlementsListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	tflog.Debug(ctx, "Reading Entitlements data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Entitlements data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Entitlements", entitlementErrDetail(err, httpResp))
		return
	}

	entitlementsAttrSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"entitlements": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: entitlementsListNestedAttributes(ctx),
				},
			},
		},
	}
	fullType := entitlementsAttrSchema.Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["entitlements"].(basetypes.ListType).ElemType

	models := make([]entitlementDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := entitlementDataSourceDtoToModel(ctx, &dtos[i], entitlementDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	entitlementsList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Entitlements = entitlementsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
