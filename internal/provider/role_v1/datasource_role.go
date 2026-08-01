// See resource_role.go in this package for design notes and known limitations
// shared by both the resource and this data source.
package role_v1

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/sailpoint-oss/golang-sdk/v3/roles"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/role_v1/datasource_role"
)

var (
	_ datasource.DataSource              = (*roleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*roleDataSource)(nil)
)

func NewRoleDataSource() datasource.DataSource {
	return &roleDataSource{}
}

type roleDataSource struct {
	client *sailpoint.APIClient
}

func (d *roleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_v1"
}

func (d *roleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_role.RoleDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Role from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Role](https://documentation.sailpoint.com/saas/help/access/roles.html) from " +
		"IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."
}

func (d *roleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_role.RoleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Role data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.RolesAPI.
		GetRoleV1(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Role data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Role", roleErrDetail(err, httpResp))
		return
	}

	state, diags := roleDatasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Role data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// roleDatasourceDtoToModel mirrors roleDtoToModel in resource_role.go but
// against the data source's generated model/value types (a separate Go package
// emitted by tfplugingen-framework, so the types are not identical even though
// they're structurally the same).
func roleDatasourceDtoToModel(ctx context.Context, dto *roles.Role, fallback datasource_role.RoleModel) (datasource_role.RoleModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringValue(dto.Name)
	if dto.Description.IsSet() {
		model.Description = types.StringPointerValue(dto.Description.Get())
	}
	if dto.Created != nil {
		model.Created = types.StringValue(dto.Created.Format(time.RFC3339))
	}
	if dto.Modified != nil {
		model.Modified = types.StringValue(dto.Modified.Format(time.RFC3339))
	}
	if dto.Enabled != nil {
		model.Enabled = types.BoolPointerValue(dto.Enabled)
	}
	if dto.Requestable != nil {
		model.Requestable = types.BoolPointerValue(dto.Requestable)
	}
	if dto.PrivilegeLevel.IsSet() {
		model.PrivilegeLevel = types.StringPointerValue(dto.PrivilegeLevel.Get())
	}
	if dto.Dimensional.IsSet() {
		model.Dimensional = types.BoolPointerValue(dto.Dimensional.Get())
	}

	owner, d := datasource_role.OwnerValue{}.FromApi_betaOwnerReference(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	if dto.Segments != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.Segments)
		diags.Append(d...)
		model.Segments = listVal
	}

	if dto.AccessProfiles != nil {
		values := make([]datasource_role.AccessProfilesValue, 0, len(dto.AccessProfiles))
		for i := range dto.AccessProfiles {
			v, d := datasource_role.AccessProfilesValue{}.FromApi_betaAccessProfileRef(ctx, &dto.AccessProfiles[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, datasource_role.AccessProfilesValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AccessProfiles = listVal
	}

	if dto.DimensionRefs != nil {
		values := make([]datasource_role.DimensionRefsValue, 0, len(dto.DimensionRefs))
		for i := range dto.DimensionRefs {
			v, d := datasource_role.DimensionRefsValue{}.FromApi_betaDimensionRef(ctx, &dto.DimensionRefs[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, datasource_role.DimensionRefsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.DimensionRefs = listVal
	}

	if dto.Entitlements != nil {
		values := make([]datasource_role.EntitlementsValue, 0, len(dto.Entitlements))
		for _, e := range dto.Entitlements {
			values = append(values, datasource_role.EntitlementsValue{
				Id:               types.StringPointerValue(e.Id),
				Name:             types.StringPointerValue(e.Name.Get()),
				EntitlementsType: types.StringPointerValue(e.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, datasource_role.EntitlementsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Entitlements = listVal
	}

	if dto.AdditionalOwners != nil {
		values := make([]datasource_role.AdditionalOwnersValue, 0, len(dto.AdditionalOwners))
		for _, o := range dto.AdditionalOwners {
			values = append(values, datasource_role.AdditionalOwnersValue{
				Id:                   types.StringPointerValue(o.Id),
				Name:                 types.StringPointerValue(o.Name.Get()),
				AdditionalOwnersType: types.StringPointerValue(o.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, datasource_role.AdditionalOwnersValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AdditionalOwners = listVal
	}

	// access_model_metadata, access_request_config, revocation_request_config,
	// and membership now get a real deeper read-back from the API response
	// (see datasource_role_readback.go) instead of just resolving Unknown to
	// Null - every data source attribute is Computed-only, so this always
	// runs (there's no "practitioner configured it" case to preserve here,
	// unlike the resource).
	v, d := roleDatasourceAccessModelMetadataFromApi(ctx, dto.AccessModelMetadata)
	diags.Append(d...)
	model.AccessModelMetadata = v

	arc, d := roleDatasourceAccessRequestConfigFromApi(ctx, dto.AccessRequestConfig)
	diags.Append(d...)
	model.AccessRequestConfig = arc

	rrc, d := roleDatasourceRevocationRequestConfigFromApi(ctx, dto.RevocationRequestConfig)
	diags.Append(d...)
	model.RevocationRequestConfig = rrc

	membership, d := roleDatasourceMembershipFromApi(ctx, dto.Membership.Get())
	diags.Append(d...)
	model.Membership = membership

	// legacy_membership_info's generated schema has zero attributes (nothing
	// to populate from the API's arbitrary map[string]interface{} shape - see
	// datasource_role_readback.go's doc comment), so it remains fully
	// pass-through: just resolve Unknown to Null.
	if model.LegacyMembershipInfo.IsUnknown() {
		model.LegacyMembershipInfo = datasource_role.NewLegacyMembershipInfoValueNull()
	}

	return model, diags
}
