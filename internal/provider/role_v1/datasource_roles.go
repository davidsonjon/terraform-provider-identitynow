// This file adds a plural "list" data source alongside the singular
// identitynow_role_v1 data source in datasource_role.go, mirroring the
// upstream davidsonjon/terraform-provider-identitynow PR #7 pattern
// (identitynow_identities: a separate plural data source wrapping a List*
// SDK call with filter/limit support, rather than overloading the singular
// data source). It queries GET /roles/v1 (roles.RolesAPI.ListRolesV1)
// instead of GET /roles/v1/{id}, and returns every matching Role using the
// exact same nested object shape as identitynow_role_v1 (reusing
// datasource_role's generated schema/model/value types and the existing
// roleDatasourceDtoToModel converter in datasource_role.go) so practitioners
// get identical attribute names/types whether they read one role by id or
// query many by filter.
package role_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/role_v1/datasource_role"
)

// rolesListMaxLimit matches GET /roles/v1's documented maximum "limit" value
// (note this differs from other IdentityNow list APIs, e.g. identities' 250 -
// see the spec's parameter description for /roles/v1).
const rolesListMaxLimit = 50

var (
	_ datasource.DataSource              = (*rolesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*rolesDataSource)(nil)
)

func NewRolesDataSource() datasource.DataSource {
	return &rolesDataSource{}
}

type rolesDataSource struct {
	client *sailpoint.APIClient
}

// RolesDataSourceModel is hand-written (not generated) since it wraps the
// list-query parameters plus a "roles" attribute nesting the generated
// datasource_role.RoleModel shape.
type RolesDataSourceModel struct {
	Filters            types.String `tfsdk:"filters"`
	Limit              types.Int64  `tfsdk:"limit"`
	Offset             types.Int64  `tfsdk:"offset"`
	Sorters            types.String `tfsdk:"sorters"`
	ForSubadmin        types.String `tfsdk:"for_subadmin"`
	ForSegmentIds      types.String `tfsdk:"for_segment_ids"`
	IncludeUnsegmented types.Bool   `tfsdk:"include_unsegmented"`
	Roles              types.List   `tfsdk:"roles"`
}

// rolesListNestedAttributes returns the same attribute map as
// datasource_role.RoleDataSourceSchema, except "id" is overridden to
// Computed-only. The singular identitynow_role_v1 data source marks "id" as
// Required because it's the lookup key for GET /roles/v1/{id}; here it's just
// another output field of a fully Computed "roles" list, so Required (which
// would otherwise render as though practitioners must set it) is wrong.
func rolesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_role.RoleDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "ID of the Role",
		MarkdownDescription: "ID of the Role",
	}
	return out
}

func (d *rolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles_v1"
}

func (d *rolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Roles from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Roles](https://documentation.sailpoint.com/saas/help/access/roles.html) from " +
			"IdentityNow/ISC via `GET /roles/v1`, optionally filtered, sorted, and paginated. Returns the same " +
			"attributes per role as the singular `identitynow_role_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_role_v1`'s \"Known Limitations & Live Testing " +
			"Notes\" section before relying on it in production configurations; the same limitations apply to each " +
			"role returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query roles (e.g. `name sw \"Engineering\"`, " +
					"`enabled eq true`). See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the supported fields/operators for `GET /roles/v1`.",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of roles to return. The API's documented maximum for this " +
					"endpoint is 50; values above 50 are capped to 50 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"for_subadmin": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "If provided, filters the returned list according to what is visible to the " +
					"indicated ROLE_SUBADMIN identity. The value is either an identity id or the special value `me`.",
			},
			"for_segment_ids": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "If present and not empty, additionally filters roles to those assigned to the given comma-separated Segment id(s).",
			},
			"include_unsegmented": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether the response should include unsegmented roles. Only meaningful when `for_segment_ids` is set.",
			},
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Roles matching the query, each with the same attributes as `identitynow_role_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: rolesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *rolesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *rolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config RolesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Roles data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.RolesAPI.ListRolesV1(ctx)

	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}
	if !config.ForSubadmin.IsNull() && !config.ForSubadmin.IsUnknown() {
		apiReq = apiReq.ForSubadmin(config.ForSubadmin.ValueString())
	}
	if !config.ForSegmentIds.IsNull() && !config.ForSegmentIds.IsUnknown() {
		apiReq = apiReq.ForSegmentIds(config.ForSegmentIds.ValueString())
	}
	if !config.IncludeUnsegmented.IsNull() && !config.IncludeUnsegmented.IsUnknown() {
		apiReq = apiReq.IncludeUnsegmented(config.IncludeUnsegmented.ValueBool())
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}

	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		requestedLimit := config.Limit.ValueInt64()
		if requestedLimit > rolesListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /roles/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, rolesListMaxLimit, rolesListMaxLimit),
			)
			apiReq = apiReq.Limit(rolesListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Roles data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Roles", roleErrDetail(err, httpResp))
		return
	}

	elemType := datasource_role.RoleDataSourceSchema(ctx).Type()

	models := make([]datasource_role.RoleModel, 0, len(dtos))
	for i := range dtos {
		model, diags := roleDatasourceDtoToModel(ctx, &dtos[i], datasource_role.RoleModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	rolesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Roles = rolesList

	tflog.Debug(ctx, "Read Roles data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
