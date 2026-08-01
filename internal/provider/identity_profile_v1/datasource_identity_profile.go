// See resource_identity_profile.go in this package for design notes and
// known limitations shared by both the resource and this data source.
package identity_profile_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/identity_profiles"

	"terraform-provider-identitynow/internal/provider/identity_profile_v1/datasource_identity_profile"
)

var (
	_ datasource.DataSource              = (*identityProfileDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*identityProfileDataSource)(nil)
)

func NewIdentityProfileDataSource() datasource.DataSource {
	return &identityProfileDataSource{}
}

type identityProfileDataSource struct {
	client *sailpoint.APIClient
}

// identityProfileDataSourceModel mirrors
// datasource_identity_profile.IdentityProfileModel plus the hand-added
// "identity_attribute_config" field - see identityProfileResourceModel in
// resource_identity_profile.go for the full rationale (identical here).
type identityProfileDataSourceModel struct {
	AuthoritativeSource              datasource_identity_profile.AuthoritativeSourceValue              `tfsdk:"authoritative_source"`
	Created                          types.String                                                      `tfsdk:"created"`
	Description                      types.String                                                      `tfsdk:"description"`
	HasTimeBasedAttr                 types.Bool                                                        `tfsdk:"has_time_based_attr"`
	Id                               types.String                                                      `tfsdk:"id"`
	IdentityAttributeConfig          jsontypes.Normalized                                              `tfsdk:"identity_attribute_config"`
	IdentityCount                    types.Int64                                                       `tfsdk:"identity_count"`
	IdentityExceptionReportReference datasource_identity_profile.IdentityExceptionReportReferenceValue `tfsdk:"identity_exception_report_reference"`
	IdentityRefreshRequired          types.Bool                                                        `tfsdk:"identity_refresh_required"`
	Modified                         types.String                                                      `tfsdk:"modified"`
	Name                             types.String                                                      `tfsdk:"name"`
	Owner                            datasource_identity_profile.OwnerValue                            `tfsdk:"owner"`
	Priority                         types.Int64                                                       `tfsdk:"priority"`
}

func (d *identityProfileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_profile_v1"
}

func (d *identityProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_identity_profile.IdentityProfileDataSourceSchema(ctx)
	resp.Schema.Description = "Reads an Identity Profile from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads an [Identity Profile](https://documentation.sailpoint.com/saas/help/setup/identity_profiles.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see `identitynow_identity_profile_v1`'s (the resource) \"Known " +
		"Limitations & Live Testing Notes\" section before relying on it in production configurations."
	applyIdentityAttributeConfigDataSourceField(&resp.Schema.Attributes)
	applyIdentityProfileDataSourceIdRequired(&resp.Schema)
}

func (d *identityProfileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *identityProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config identityProfileDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Identity Profile data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.IdentityProfilesAPI.
		GetIdentityProfileV1(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Identity Profile data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Identity Profile", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Identity Profile data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_identity_profile.go but
// against the data source's generated model/value types (a separate Go
// package emitted by tfplugingen-framework, so the types are not identical
// even though they're structurally the same).
func datasourceDtoToModel(ctx context.Context, dto *identity_profiles.IdentityProfile, fallback identityProfileDataSourceModel) (identityProfileDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringPointerValue(dto.Name.Get())
	model.Description = types.StringPointerValue(dto.Description.Get())
	if dto.Priority != nil {
		model.Priority = types.Int64Value(*dto.Priority)
	} else {
		model.Priority = types.Int64Null()
	}
	model.IdentityRefreshRequired = types.BoolPointerValue(dto.IdentityRefreshRequired)
	model.HasTimeBasedAttr = types.BoolPointerValue(dto.HasTimeBasedAttr)
	if dto.IdentityCount != nil {
		model.IdentityCount = types.Int64Value(int64(*dto.IdentityCount))
	} else {
		model.IdentityCount = types.Int64Null()
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	authoritativeSource, d := datasource_identity_profile.AuthoritativeSourceValue{}.FromIdentity_profilesIdentityProfileAllOfAuthoritativeSource(ctx, &dto.AuthoritativeSource)
	diags.Append(d...)
	model.AuthoritativeSource = authoritativeSource

	owner, d := datasource_identity_profile.OwnerValue{}.FromIdentity_profilesIdentityProfileAllOfOwner(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	exceptionRef, d := datasource_identity_profile.IdentityExceptionReportReferenceValue{}.FromIdentity_profilesIdentityExceptionReportReference(ctx, dto.IdentityExceptionReportReference.Get())
	diags.Append(d...)
	model.IdentityExceptionReportReference = exceptionRef

	cfg, d := identityAttributeConfigFromAPI(dto.IdentityAttributeConfig)
	diags.Append(d...)
	model.IdentityAttributeConfig = cfg

	return model, diags
}
