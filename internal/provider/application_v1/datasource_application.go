// See resource_application.go in this package for design notes and known
// limitations shared by both the resource and these data sources.
package application_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/application_v1/datasource_application"
)

var (
	_ datasource.DataSource              = (*applicationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*applicationDataSource)(nil)
)

func NewApplicationDataSource() datasource.DataSource {
	return &applicationDataSource{}
}

type applicationDataSource struct {
	client *sailpoint.APIClient
}

// applicationDataSourceModel mirrors datasource_application.ApplicationModel
// plus the hand-added access_profile_ids field.
type applicationDataSourceModel struct {
	AccountSource           datasource_application.AccountSourceValue `tfsdk:"account_source"`
	AccessProfileIds        types.Set                                 `tfsdk:"access_profile_ids"`
	AppCenterEnabled        types.Bool                                `tfsdk:"app_center_enabled"`
	CloudAppId              types.String                              `tfsdk:"cloud_app_id"`
	Created                 types.String                              `tfsdk:"created"`
	Description             types.String                              `tfsdk:"description"`
	Enabled                 types.Bool                                `tfsdk:"enabled"`
	Id                      types.String                              `tfsdk:"id"`
	MatchAllAccounts        types.Bool                                `tfsdk:"match_all_accounts"`
	Modified                types.String                              `tfsdk:"modified"`
	Name                    types.String                              `tfsdk:"name"`
	Owner                   datasource_application.OwnerValue         `tfsdk:"owner"`
	ProvisionRequestEnabled types.Bool                                `tfsdk:"provision_request_enabled"`
}

func (d *applicationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_v1"
}

func (d *applicationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Reads an Application (source app) from IdentityNow/ISC by id.",
		MarkdownDescription: "Reads an Application (source app) from IdentityNow/ISC by `id`. Returns the same attributes as " +
			"the `identitynow_application_v1` resource, including `access_profile_ids` via an additional access-profiles lookup.",
		Attributes: applicationDataSourceAttributes(ctx),
	}
}

func applicationDataSourceAttributes(ctx context.Context) map[string]datasourceschema.Attribute {
	attrs := datasource_application.ApplicationDataSourceSchema(ctx).Attributes
	out := make(map[string]datasourceschema.Attribute, len(attrs)+1)
	for k, v := range attrs {
		out[k] = v
	}
	out["access_profile_ids"] = datasourceschema.SetAttribute{
		Computed:    true,
		ElementType: types.StringType,
		Description: "Set of access profile IDs assigned to the application.",
		MarkdownDescription: "Set of access profile IDs assigned to the application, read from the separate " +
			"`GET /source-apps/v1/{id}/access-profiles` endpoint.",
	}
	return out
}

func (d *applicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *applicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config applicationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Application data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.AppsAPI.
		GetSourceApp(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Application data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Application", applicationErrDetail(err, httpResp))
		return
	}

	accessProfileIDs, diags := listApplicationAccessProfileIDs(ctx, d.client, config.Id.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, diags := applicationDatasourceDtoToModel(ctx, dto, accessProfileIDs, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func applicationDatasourceDtoToModel(ctx context.Context, dto *api_beta.SourceApp, accessProfileIDs []string, fallback applicationDataSourceModel) (applicationDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	} else {
		model.Id = types.StringNull()
	}
	model.Name = types.StringPointerValue(dto.Name)
	model.Description = types.StringPointerValue(dto.Description)
	model.CloudAppId = types.StringPointerValue(dto.CloudAppId)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)
	model.Enabled = types.BoolPointerValue(dto.Enabled)
	model.ProvisionRequestEnabled = types.BoolPointerValue(dto.ProvisionRequestEnabled)
	model.AppCenterEnabled = types.BoolPointerValue(dto.AppCenterEnabled)
	model.MatchAllAccounts = types.BoolPointerValue(dto.MatchAllAccounts)

	owner, d := datasourceOwnerFromAPI(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	accountSource, d := datasourceAccountSourceFromAPI(ctx, dto.AccountSource.Get())
	diags.Append(d...)
	model.AccountSource = accountSource

	accessProfileSet, d := types.SetValueFrom(ctx, types.StringType, accessProfileIDs)
	diags.Append(d...)
	model.AccessProfileIds = accessProfileSet

	return model, diags
}

func datasourceOwnerFromAPI(ctx context.Context, dto *api_beta.BaseReferenceDto) (datasource_application.OwnerValue, diag.Diagnostics) {
	if dto == nil {
		return datasource_application.NewOwnerValueNull(), nil
	}
	attrs := map[string]attr.Value{
		"id":   types.StringPointerValue(dto.Id),
		"name": types.StringPointerValue(dto.Name),
		"type": types.StringNull(),
	}
	if dto.Type != nil {
		attrs["type"] = types.StringValue(string(*dto.Type))
	}
	return datasource_application.NewOwnerValue(datasource_application.OwnerValue{}.AttributeTypes(ctx), attrs)
}

func datasourceAccountSourceFromAPI(ctx context.Context, dto *api_beta.SourceAppAccountSource) (datasource_application.AccountSourceValue, diag.Diagnostics) {
	if dto == nil {
		return datasource_application.NewAccountSourceValueNull(), nil
	}

	passwordPolicies, diags := datasourcePasswordPoliciesFromAPI(ctx, dto.PasswordPolicies)
	if diags.HasError() {
		return datasource_application.NewAccountSourceValueUnknown(), diags
	}

	attrs := map[string]attr.Value{
		"id":                          types.StringPointerValue(dto.Id),
		"name":                        types.StringPointerValue(dto.Name),
		"password_policies":           passwordPolicies,
		"type":                        types.StringPointerValue(dto.Type),
		"use_for_password_management": types.BoolPointerValue(dto.UseForPasswordManagement),
	}
	return datasource_application.NewAccountSourceValue(datasource_application.AccountSourceValue{}.AttributeTypes(ctx), attrs)
}

func datasourcePasswordPoliciesFromAPI(ctx context.Context, policies []api_beta.BaseReferenceDto) (types.List, diag.Diagnostics) {
	if len(policies) == 0 {
		return types.ListNull(datasource_application.PasswordPoliciesValue{}.Type(ctx)), nil
	}

	var diags diag.Diagnostics
	values := make([]datasource_application.PasswordPoliciesValue, 0, len(policies))
	for i := range policies {
		attrs := map[string]attr.Value{
			"id":   types.StringPointerValue(policies[i].Id),
			"name": types.StringPointerValue(policies[i].Name),
			"type": types.StringNull(),
		}
		if policies[i].Type != nil {
			attrs["type"] = types.StringValue(string(*policies[i].Type))
		}
		v, d := datasource_application.NewPasswordPoliciesValue(datasource_application.PasswordPoliciesValue{}.AttributeTypes(ctx), attrs)
		diags.Append(d...)
		values = append(values, v)
	}
	if diags.HasError() {
		return types.ListNull(datasource_application.PasswordPoliciesValue{}.Type(ctx)), diags
	}
	listVal, d := types.ListValueFrom(ctx, datasource_application.PasswordPoliciesValue{}.Type(ctx), values)
	diags.Append(d...)
	return listVal, diags
}
