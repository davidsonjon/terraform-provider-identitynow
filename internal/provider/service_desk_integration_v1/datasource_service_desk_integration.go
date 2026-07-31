// See resource_service_desk_integration.go in this package for design notes and
// known limitations shared by both the resource and this data source.
package service_desk_integration_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/service_desk_integration_v1/datasource_service_desk_integration"
)

var (
	_ datasource.DataSource              = (*serviceDeskIntegrationDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serviceDeskIntegrationDataSource)(nil)
)

func NewServiceDeskIntegrationDataSource() datasource.DataSource {
	return &serviceDeskIntegrationDataSource{}
}

type serviceDeskIntegrationDataSource struct {
	client *sailpoint.APIClient
}

func (d *serviceDeskIntegrationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_desk_integration_v1"
}

func (d *serviceDeskIntegrationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_service_desk_integration.ServiceDeskIntegrationDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Service Desk Integration from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Service Desk Integration](https://documentation.sailpoint.com/saas/help/integration/help/landing-service-desk.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."
}

func (d *serviceDeskIntegrationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceDeskIntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Service Desk Integration data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.ServiceDeskIntegrationAPI.
		GetServiceDeskIntegration(ctx, config.Id.ValueString()).
		Execute()
	dto, httpResp, err = withManagedResourceRefsFallback(ctx, dto, httpResp, err)
	if err != nil {
		tflog.Error(ctx, "Error reading Service Desk Integration data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Service Desk Integration", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Service Desk Integration data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_service_desk_integration.go
// but against the data source's generated model/value types (a separate Go
// package emitted by tfplugingen-framework, so the types are not identical even
// though they're structurally the same).
func datasourceDtoToModel(ctx context.Context, dto *api_beta.ServiceDeskIntegrationDto, fallback datasource_service_desk_integration.ServiceDeskIntegrationModel) (datasource_service_desk_integration.ServiceDeskIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if id := dtoID(dto); id != "" {
		model.Id = types.StringValue(id)
	}
	model.Name = types.StringValue(dto.Name)
	model.Description = types.StringValue(dto.Description)
	model.Type = types.StringValue(dto.Type)
	if dto.Cluster != nil {
		model.Cluster = types.StringPointerValue(dto.Cluster)
	}

	beforeProvisioningRule, d := datasource_service_desk_integration.BeforeProvisioningRuleValue{}.FromApi_betaBeforeProvisioningRuleDto(ctx, dto.BeforeProvisioningRule)
	diags.Append(d...)
	model.BeforeProvisioningRule = beforeProvisioningRule

	clusterRef, d := datasource_service_desk_integration.ClusterRefValue{}.FromApi_betaSourceClusterDto(ctx, dto.ClusterRef)
	diags.Append(d...)
	model.ClusterRef = clusterRef

	ownerRef, d := datasource_service_desk_integration.OwnerRefValue{}.FromApi_betaOwnerDto(ctx, dto.OwnerRef)
	diags.Append(d...)
	model.OwnerRef = ownerRef

	if dto.ManagedSources != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.ManagedSources)
		diags.Append(d...)
		model.ManagedSources = listVal
	}

	attrsVal, d := datasource_service_desk_integration.NewAttributesValue(map[string]attr.Type{}, map[string]attr.Value{})
	diags.Append(d...)
	model.Attributes = attrsVal

	if dto.ProvisioningConfig != nil {
		model.ProvisioningConfig.UniversalManager = types.BoolPointerValue(dto.ProvisioningConfig.UniversalManager)
		model.ProvisioningConfig.NoProvisioningRequests = types.BoolPointerValue(dto.ProvisioningConfig.NoProvisioningRequests)
		if dto.ProvisioningConfig.ProvisioningRequestExpiration != nil {
			model.ProvisioningConfig.ProvisioningRequestExpiration = types.Int64Value(int64(*dto.ProvisioningConfig.ProvisioningRequestExpiration))
		}
	}

	return model, diags
}
