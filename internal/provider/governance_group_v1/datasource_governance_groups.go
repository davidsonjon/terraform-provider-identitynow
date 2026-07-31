// This file adds a plural "list" data source alongside the singular
// identitynow_governance_group_v1 data source in
// datasource_governance_group.go, mirroring role_v1's datasource_roles.go
// pattern (itself adopted from upstream davidsonjon/terraform-provider-
// identitynow PR #7's identitynow_identities). It queries GET /workgroups/v1
// (api_beta.GovernanceGroupsAPI.ListWorkgroups) instead of
// GET /workgroups/v1/{id}, and returns every matching Governance Group using
// the exact same nested object shape as identitynow_governance_group_v1
// (reusing datasource_governance_group's generated schema/model/value types
// and the existing datasourceDtoToModel converter in
// datasource_governance_group.go) so practitioners get identical attribute
// names/types whether they read one governance group by id or query many by
// filter.
package governance_group_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/governance_group_v1/datasource_governance_group"
)

// governanceGroupsListMaxLimit matches GET /workgroups/v1's documented
// maximum "limit" value (see the shared limit.yaml parameter definition -
// 250, the same as most other IdentityNow v1 list APIs).
const governanceGroupsListMaxLimit = 250

var (
	_ datasource.DataSource              = (*governanceGroupsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*governanceGroupsDataSource)(nil)
)

func NewGovernanceGroupsDataSource() datasource.DataSource {
	return &governanceGroupsDataSource{}
}

type governanceGroupsDataSource struct {
	client *sailpoint.APIClient
}

// GovernanceGroupsDataSourceModel is hand-written (not generated) since it
// wraps the list-query parameters plus a "governance_groups" attribute
// nesting the generated datasource_governance_group.GovernanceGroupModel
// shape.
type GovernanceGroupsDataSourceModel struct {
	Filters          types.String `tfsdk:"filters"`
	Limit            types.Int64  `tfsdk:"limit"`
	Offset           types.Int64  `tfsdk:"offset"`
	Sorters          types.String `tfsdk:"sorters"`
	GovernanceGroups types.List   `tfsdk:"governance_groups"`
}

// governanceGroupsListNestedAttributes returns the same attribute map as
// datasource_governance_group.GovernanceGroupDataSourceSchema, except "id" is
// overridden to Computed-only. The singular identitynow_governance_group_v1
// data source marks "id" as Required because it's the lookup key for
// GET /workgroups/v1/{id}; here it's just another output field of a fully
// Computed "governance_groups" list, so Required (which would otherwise
// render as though practitioners must set it) is wrong.
func governanceGroupsListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_governance_group.GovernanceGroupDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Governance group ID.",
		MarkdownDescription: "Governance group ID.",
	}
	return out
}

func (d *governanceGroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_groups_v1"
}

func (d *governanceGroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Governance Groups from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Governance Groups](https://documentation.sailpoint.com/saas/help/common/governance_groups.html) " +
			"from IdentityNow/ISC via `GET /workgroups/v1`, optionally filtered, sorted, and paginated. Returns the same " +
			"attributes per governance group as the singular `identitynow_governance_group_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_governance_group_v1`'s \"Known Limitations & Live Testing " +
			"Notes\" section before relying on it in production configurations; the same limitations apply to each " +
			"governance group returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query governance groups (e.g. `name sw \"Test\"`, " +
					"`id in (\"2c9180...\")`). See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the supported fields/operators for `GET /workgroups/v1`.",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of governance groups to return. The API's documented maximum for " +
					"this endpoint is 250; values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. Sorting is supported for name, created, " +
					"modified, id, description. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"governance_groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Governance Groups matching the query, each with the same attributes as `identitynow_governance_group_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: governanceGroupsListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *governanceGroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *governanceGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config GovernanceGroupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Governance Groups data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.Beta.GovernanceGroupsAPI.ListWorkgroups(ctx)

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
		if requestedLimit > governanceGroupsListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /workgroups/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, governanceGroupsListMaxLimit, governanceGroupsListMaxLimit),
			)
			apiReq = apiReq.Limit(governanceGroupsListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Governance Groups data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Governance Groups", errDetail(err, httpResp))
		return
	}

	elemType := datasource_governance_group.GovernanceGroupDataSourceSchema(ctx).Type()

	models := make([]datasource_governance_group.GovernanceGroupModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(ctx, &dtos[i], datasource_governance_group.GovernanceGroupModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	groupsList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.GovernanceGroups = groupsList

	tflog.Debug(ctx, "Read Governance Groups data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
