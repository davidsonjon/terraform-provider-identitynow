// See resource_access_profile.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package access_profile_v1

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/access_profile_v1/datasource_access_profile"
)

var (
	_ datasource.DataSource              = (*accessProfileDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accessProfileDataSource)(nil)
)

func NewAccessProfileDataSource() datasource.DataSource {
	return &accessProfileDataSource{}
}

type accessProfileDataSource struct {
	client *sailpoint.APIClient
}

func (d *accessProfileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_profile_v1"
}

func (d *accessProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_access_profile.AccessProfileDataSourceSchema(ctx)
	resp.Schema.Description = "Reads an Access Profile from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads an [Access Profile](https://documentation.sailpoint.com/saas/help/access/access-profiles.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."
}

func (d *accessProfileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accessProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Access Profile data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.AccessProfilesAPI.
		GetAccessProfile(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Access Profile data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Access Profile", accessProfileErrDetail(err, httpResp))
		return
	}

	state, diags := accessProfileDatasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Access Profile data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// accessProfileDatasourceDtoToModel mirrors accessProfileDtoToModel in
// resource_access_profile.go but against the data source's generated
// model/value types (a separate Go package emitted by tfplugingen-framework,
// so the types are not identical even though they're structurally the same).
// Every data source attribute is Computed-only, so - unlike the resource -
// there is no "practitioner configured it" case to preserve: every block
// always gets a real read-back from the API response.
func accessProfileDatasourceDtoToModel(ctx context.Context, dto *api_beta.AccessProfile, fallback datasource_access_profile.AccessProfileModel) (datasource_access_profile.AccessProfileModel, diag.Diagnostics) {
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

	owner, d := datasource_access_profile.OwnerValue{}.FromApi_betaOwnerReference(ctx, &dto.Owner)
	diags.Append(d...)
	model.Owner = owner

	source, d := datasource_access_profile.SourceValue{}.FromApi_betaAccessProfileSourceRef(ctx, &dto.Source)
	diags.Append(d...)
	model.Source = source

	if dto.Segments != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.Segments)
		diags.Append(d...)
		model.Segments = listVal
	}

	if dto.Entitlements != nil {
		values := make([]datasource_access_profile.EntitlementsValue, 0, len(dto.Entitlements))
		for _, e := range dto.Entitlements {
			values = append(values, datasource_access_profile.EntitlementsValue{
				Id:               types.StringPointerValue(e.Id),
				Name:             types.StringPointerValue(e.Name.Get()),
				EntitlementsType: types.StringPointerValue(e.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, datasource_access_profile.EntitlementsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Entitlements = listVal
	}

	if dto.AdditionalOwners != nil {
		values := make([]datasource_access_profile.AdditionalOwnersValue, 0, len(dto.AdditionalOwners))
		for _, o := range dto.AdditionalOwners {
			values = append(values, datasource_access_profile.AdditionalOwnersValue{
				Id:                   types.StringPointerValue(o.Id),
				Name:                 types.StringPointerValue(o.Name.Get()),
				AdditionalOwnersType: types.StringPointerValue(o.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, datasource_access_profile.AdditionalOwnersValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AdditionalOwners = listVal
	}

	v, d := accessProfileDatasourceAccessModelMetadataFromApi(ctx, dto.AccessModelMetadata)
	diags.Append(d...)
	model.AccessModelMetadata = v

	arc, d := accessProfileDatasourceAccessRequestConfigFromApi(ctx, dto.AccessRequestConfig.Get())
	diags.Append(d...)
	model.AccessRequestConfig = arc

	rrc, d := accessProfileDatasourceRevocationRequestConfigFromApi(ctx, dto.RevocationRequestConfig.Get())
	diags.Append(d...)
	model.RevocationRequestConfig = rrc

	pc, d := accessProfileDatasourceProvisioningCriteriaFromApi(ctx, dto.ProvisioningCriteria.Get())
	diags.Append(d...)
	model.ProvisioningCriteria = pc

	return model, diags
}
