// This file adds a plural "list" data source alongside the singular
// identitynow_identity_profile_v1 data source in datasource_identity_profile.go,
// mirroring sources_v1's datasource_sources.go / role_v1's
// datasource_roles.go pattern. It queries GET /identity-profiles/v1
// (identity_profiles.IdentityProfilesAPI.ListIdentityProfilesV1) instead of GET
// /identity-profiles/v1/{id}, and returns every matching Identity Profile
// using the exact same nested object shape as identitynow_identity_profile_v1
// (reusing datasource_identity_profile's generated schema/model/value types
// plus this package's hand-added "identity_attribute_config"/
// datasourceDtoToModel converter) so practitioners get identical attribute
// names/types whether they read one profile by id or query many by filter.
package identity_profile_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/identity_profile_v1/datasource_identity_profile"
)

// identityProfilesListMaxLimit matches GET /identity-profiles/v1's documented
// maximum "limit" value (see the shared limit.yaml parameter definition -
// 250, matching most other IdentityNow v1 list APIs).
const identityProfilesListMaxLimit = 250

var (
	_ datasource.DataSource              = (*identityProfilesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*identityProfilesDataSource)(nil)
)

func NewIdentityProfilesDataSource() datasource.DataSource {
	return &identityProfilesDataSource{}
}

type identityProfilesDataSource struct {
	client *sailpoint.APIClient
}

// IdentityProfilesDataSourceModel is hand-written (not generated) since it
// wraps the list-query parameters plus an "identity_profiles" attribute
// nesting the generated datasource_identity_profile.IdentityProfileModel
// shape (plus the hand-added "identity_attribute_config" field).
type IdentityProfilesDataSourceModel struct {
	Filters          types.String `tfsdk:"filters"`
	Sorters          types.String `tfsdk:"sorters"`
	Limit            types.Int64  `tfsdk:"limit"`
	Offset           types.Int64  `tfsdk:"offset"`
	IdentityProfiles types.List   `tfsdk:"identity_profiles"`
}

// identityProfilesListNestedAttributes returns the same attribute map as
// datasource_identity_profile.IdentityProfileDataSourceSchema (plus the
// hand-added "identity_attribute_config" field), except "id" is left
// Computed-only (unlike the singular data source, which hand-patches "id" to
// Required as its lookup key - here it's just another output field of a
// fully Computed "identity_profiles" list).
func identityProfilesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_identity_profile.IdentityProfileDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs)+1)
	for k, v := range attrs {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "System-generated unique ID of the Object",
		MarkdownDescription: "System-generated unique ID of the Object",
	}
	applyIdentityAttributeConfigDataSourceField(&out)
	return out
}

func (d *identityProfilesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_profiles_v1"
}

func (d *identityProfilesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Identity Profiles from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Identity Profiles](https://documentation.sailpoint.com/saas/help/setup/identity_profiles.html) " +
			"from IdentityNow/ISC via `GET /identity-profiles/v1`, optionally filtered, sorted, and paginated. Returns " +
			"the same attributes per profile as the singular `identitynow_identity_profile_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_identity_profile_v1`'s (the resource) \"Known " +
			"Limitations & Live Testing Notes\" section before relying on it in production configurations; the same " +
			"limitations apply to each profile returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query identity profiles (e.g. `name eq \"Employees\"`). " +
					"Filtering is supported for `id`, `name` (eq, ne, ge, gt, in, le, sw), and `priority` (eq, ne). See " +
					"[V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results).",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. Sorting is supported for id, name, priority, " +
					"created, modified, owner.id, owner.name. See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of identity profiles to return. The API's documented maximum for " +
					"this endpoint is 250; values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"identity_profiles": schema.ListNestedAttribute{
				Computed: true,
				MarkdownDescription: "Identity Profiles matching the query, each with the same attributes as " +
					"`identitynow_identity_profile_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: identityProfilesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *identityProfilesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *identityProfilesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IdentityProfilesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Identity Profiles data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.IdentityProfilesAPI.ListIdentityProfilesV1(ctx)

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
		if requestedLimit > identityProfilesListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /identity-profiles/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, identityProfilesListMaxLimit, identityProfilesListMaxLimit),
			)
			apiReq = apiReq.Limit(identityProfilesListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Identity Profiles data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Identity Profiles", errDetail(err, httpResp))
		return
	}

	// attr.Type doesn't encode Required/Optional/Computed-ness (only the
	// underlying shape), so it's safe to build the element type from the
	// same attribute map used for each list item's schema
	// (identityProfilesListNestedAttributes, "id" Computed-only) even though
	// the singular data source's own schema marks "id" Required - see
	// sources_v1/governance_group_v1's identical elemType derivation for
	// precedent.
	identityProfilesAttrSchema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"identity_profiles": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: identityProfilesListNestedAttributes(ctx),
				},
			},
		},
	}
	fullType := identityProfilesAttrSchema.Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["identity_profiles"].(basetypes.ListType).ElemType

	models := make([]identityProfileDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(ctx, &dtos[i], identityProfileDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	identityProfilesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.IdentityProfiles = identityProfilesList

	tflog.Debug(ctx, "Read Identity Profiles data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
