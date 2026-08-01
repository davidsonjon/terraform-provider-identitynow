// This file adds a plural "list" data source alongside the singular
// identitynow_access_profile_v1 data source in access_profile_data_source.go,
// mirroring the same pattern used by role_v1's datasource_roles.go (itself
// modeled on the upstream davidsonjon/terraform-provider-identitynow PR #7's
// identitynow_identities: a separate plural data source wrapping a List*
// SDK call with filter/sort/pagination support, rather than overloading the
// singular data source). It queries GET /access-profiles/v1
// (access_profiles.AccessProfilesAPI.ListAccessProfilesV1) instead of
// GET /access-profiles/v1/{id}, and returns every matching Access Profile
// using the exact same nested object shape as identitynow_access_profile_v1
// (reusing datasource_access_profile's generated schema/model/value types and
// the existing accessProfileDatasourceDtoToModel converter in
// access_profile_data_source.go) so practitioners get identical attribute
// names/types whether they read one access profile by id or query many by
// filter.
package access_profile_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/access_profile_v1/datasource_access_profile"
)

// accessProfilesListMaxLimit matches GET /access-profiles/v1's documented
// maximum "limit" value (the shared limit.yaml parameter's maximum: 250 -
// note this differs from role's endpoint-specific 50; always check the
// target endpoint's own parameter definition rather than assuming a common
// default).
const accessProfilesListMaxLimit = 250

var (
	_ datasource.DataSource              = (*accessProfilesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accessProfilesDataSource)(nil)
)

func NewAccessProfilesDataSource() datasource.DataSource {
	return &accessProfilesDataSource{}
}

type accessProfilesDataSource struct {
	client *sailpoint.APIClient
}

// AccessProfilesDataSourceModel is hand-written (not generated) since it
// wraps the list-query parameters plus an "access_profiles" attribute
// nesting the generated datasource_access_profile.AccessProfileModel shape.
type AccessProfilesDataSourceModel struct {
	Filters            types.String `tfsdk:"filters"`
	Limit              types.Int64  `tfsdk:"limit"`
	Offset             types.Int64  `tfsdk:"offset"`
	IncludeCount       types.Bool   `tfsdk:"include_count"`
	Sorters            types.String `tfsdk:"sorters"`
	ForSubadmin        types.String `tfsdk:"for_subadmin"`
	ForSegmentIds      types.String `tfsdk:"for_segment_ids"`
	IncludeUnsegmented types.Bool   `tfsdk:"include_unsegmented"`
	AccessProfiles     types.List   `tfsdk:"access_profiles"`
}

// accessProfilesListNestedAttributes returns the same attribute map as
// datasource_access_profile.AccessProfileDataSourceSchema, except "id" is
// overridden to Computed-only. The singular identitynow_access_profile_v1
// data source marks "id" as Required because it's the lookup key for
// GET /access-profiles/v1/{id}; here it's just another output field of a
// fully Computed "access_profiles" list, so Required (which would otherwise
// render as though practitioners must set it) is wrong.
func accessProfilesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_access_profile.AccessProfileDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs))
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "ID of the Access Profile",
		MarkdownDescription: "ID of the Access Profile",
	}
	return out
}

func (d *accessProfilesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_profiles_v1"
}

func (d *accessProfilesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Access Profiles from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Access Profiles](https://documentation.sailpoint.com/saas/help/access/access-profiles.html) " +
			"from IdentityNow/ISC via `GET /access-profiles/v1`, optionally filtered, sorted, and paginated. Returns " +
			"the same attributes per access profile as the singular `identitynow_access_profile_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_access_profile_v1`'s \"Known Limitations & Live " +
			"Testing Notes\" section before relying on it in production configurations; the same limitations apply " +
			"to each access profile returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query access profiles (e.g. `name sw \"Engineering\"`, " +
					"`enabled eq true`). See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the supported fields/operators for `GET /access-profiles/v1`.",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of access profiles to return. The API's documented maximum for " +
					"this endpoint is 250; values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"include_count": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "If `true`, populates `X-Total-Count` response header with the number of " +
					"results that would be returned if `limit`/`offset` were ignored. This provider does not " +
					"currently surface that header's value as an attribute; it only affects the underlying API call.",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"for_subadmin": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "If provided, filters the returned list according to what is visible to the " +
					"indicated ROLE_SUBADMIN or SOURCE_SUBADMIN identity. The value is either an identity id or the " +
					"special value `me`.",
			},
			"for_segment_ids": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "If present and not empty, additionally filters access profiles to those assigned to the given comma-separated Segment id(s).",
			},
			"include_unsegmented": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether the response should include unsegmented access profiles. Only meaningful when `for_segment_ids` is set.",
			},
			"access_profiles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Access Profiles matching the query, each with the same attributes as `identitynow_access_profile_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: accessProfilesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *accessProfilesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accessProfilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config AccessProfilesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Access Profiles data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.AccessProfilesAPI.ListAccessProfilesV1(ctx)

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
	if !config.IncludeCount.IsNull() && !config.IncludeCount.IsUnknown() {
		apiReq = apiReq.Count(config.IncludeCount.ValueBool())
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}

	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		requestedLimit := config.Limit.ValueInt64()
		if requestedLimit > accessProfilesListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /access-profiles/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, accessProfilesListMaxLimit, accessProfilesListMaxLimit),
			)
			apiReq = apiReq.Limit(accessProfilesListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Access Profiles data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Access Profiles", accessProfileErrDetail(err, httpResp))
		return
	}

	elemType := datasource_access_profile.AccessProfileDataSourceSchema(ctx).Type()

	models := make([]datasource_access_profile.AccessProfileModel, 0, len(dtos))
	for i := range dtos {
		model, diags := accessProfileDatasourceDtoToModel(ctx, &dtos[i], datasource_access_profile.AccessProfileModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	accessProfilesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.AccessProfiles = accessProfilesList

	tflog.Debug(ctx, "Read Access Profiles data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
