// See resource_source.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package sources_v1

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/sources_v1/datasource_source"
)

var (
	_ datasource.DataSource              = (*sourceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourceDataSource)(nil)
)

func NewSourceDataSource() datasource.DataSource {
	return &sourceDataSource{}
}

type sourceDataSource struct {
	client *sailpoint.APIClient
}

// sourceDataSourceModel mirrors datasource_source.SourceModel plus the
// hand-added "connector_attributes" field - see sourceResourceModel in
// resource_source.go for the full rationale (identical here).
type sourceDataSourceModel struct {
	AccountCorrelationConfig  datasource_source.AccountCorrelationConfigValue  `tfsdk:"account_correlation_config"`
	AccountCorrelationRule    datasource_source.AccountCorrelationRuleValue    `tfsdk:"account_correlation_rule"`
	Authoritative             types.Bool                                       `tfsdk:"authoritative"`
	BeforeProvisioningRule    datasource_source.BeforeProvisioningRuleValue    `tfsdk:"before_provisioning_rule"`
	Category                  types.String                                     `tfsdk:"category"`
	Cluster                   datasource_source.ClusterValue                   `tfsdk:"cluster"`
	ConnectionType            types.String                                     `tfsdk:"connection_type"`
	Connector                 types.String                                     `tfsdk:"connector"`
	ConnectorAttributes       jsontypes.Normalized                             `tfsdk:"connector_attributes"`
	ConnectorClass            types.String                                     `tfsdk:"connector_class"`
	ConnectorId               types.String                                     `tfsdk:"connector_id"`
	ConnectorImplementationId types.String                                     `tfsdk:"connector_implementation_id"`
	ConnectorName             types.String                                     `tfsdk:"connector_name"`
	Created                   types.String                                     `tfsdk:"created"`
	CredentialProviderEnabled types.Bool                                       `tfsdk:"credential_provider_enabled"`
	DeleteThreshold           types.Int64                                      `tfsdk:"delete_threshold"`
	Description               types.String                                     `tfsdk:"description"`
	Features                  types.List                                       `tfsdk:"features"`
	Healthy                   types.Bool                                       `tfsdk:"healthy"`
	Id                        types.String                                     `tfsdk:"id"`
	ManagementWorkgroup       datasource_source.ManagementWorkgroupValue       `tfsdk:"management_workgroup"`
	ManagerCorrelationMapping datasource_source.ManagerCorrelationMappingValue `tfsdk:"manager_correlation_mapping"`
	ManagerCorrelationRule    datasource_source.ManagerCorrelationRuleValue    `tfsdk:"manager_correlation_rule"`
	Modified                  types.String                                     `tfsdk:"modified"`
	Name                      types.String                                     `tfsdk:"name"`
	Owner                     datasource_source.OwnerValue                     `tfsdk:"owner"`
	PasswordPolicies          types.List                                       `tfsdk:"password_policies"`
	Schemas                   types.List                                       `tfsdk:"schemas"`
	Since                     types.String                                     `tfsdk:"since"`
	Status                    types.String                                     `tfsdk:"status"`
	Type                      types.String                                     `tfsdk:"type"`
}

func (d *sourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_v1"
}

func (d *sourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_source.SourceDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Source from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Source](https://documentation.sailpoint.com/saas/help/sources/index.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see `identitynow_source_v1`'s (the resource) \"Known Limitations & Live " +
		"Testing Notes\" section before relying on it in production configurations."
	applySourceDataSourceConnectorAttributesField(&resp.Schema.Attributes)
}

func (d *sourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sourceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Source data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.SourcesAPI.
		GetSource(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Source data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Source data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_source.go but against
// the data source's generated model/value types (a separate Go package
// emitted by tfplugingen-framework, so the types are not identical even
// though they're structurally the same).
func datasourceDtoToModel(ctx context.Context, dto *api_beta.Source, fallback sourceDataSourceModel) (sourceDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringValue(dto.Name)
	model.Connector = types.StringValue(dto.Connector)
	model.Description = types.StringPointerValue(dto.Description)
	model.Type = types.StringPointerValue(dto.Type)
	model.ConnectorClass = types.StringPointerValue(dto.ConnectorClass)
	model.ConnectorId = types.StringPointerValue(dto.ConnectorId)
	model.ConnectorName = types.StringPointerValue(dto.ConnectorName)
	model.ConnectionType = types.StringPointerValue(dto.ConnectionType)
	model.ConnectorImplementationId = types.StringPointerValue(dto.ConnectorImplementationId)
	model.Authoritative = types.BoolPointerValue(dto.Authoritative)
	model.Healthy = types.BoolPointerValue(dto.Healthy)
	model.Status = types.StringPointerValue(dto.Status)
	model.Since = types.StringPointerValue(dto.Since)
	model.CredentialProviderEnabled = types.BoolPointerValue(dto.CredentialProviderEnabled)
	if dto.DeleteThreshold != nil {
		model.DeleteThreshold = types.Int64Value(int64(*dto.DeleteThreshold))
	} else {
		model.DeleteThreshold = types.Int64Null()
	}
	model.Category = types.StringPointerValue(dto.Category.Get())
	if dto.Created != nil {
		model.Created = types.StringValue(dto.Created.Format(time.RFC3339))
	}
	if dto.Modified != nil {
		model.Modified = types.StringValue(dto.Modified.Format(time.RFC3339))
	}

	connAttrs, d := normalizedConnectorAttributesFromAPI(dto.ConnectorAttributes)
	diags.Append(d...)
	model.ConnectorAttributes = connAttrs

	if dto.Features != nil {
		featuresList, d := types.ListValueFrom(ctx, types.StringType, dto.Features)
		diags.Append(d...)
		model.Features = featuresList
	} else {
		model.Features = types.ListNull(types.StringType)
	}

	owner, d := datasource_source.OwnerValue{}.FromApi_betaSourceOwner(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	clusterVal, d := datasourceClusterFromAPI(ctx, dto.Cluster.Get())
	diags.Append(d...)
	model.Cluster = clusterVal

	accountCorrelationConfig, d := datasource_source.AccountCorrelationConfigValue{}.FromApi_betaMultiHostSourcesAccountCorrelationConfig(ctx, dto.AccountCorrelationConfig.Get())
	diags.Append(d...)
	model.AccountCorrelationConfig = accountCorrelationConfig

	accountCorrelationRule, d := datasource_source.AccountCorrelationRuleValue{}.FromApi_betaMultiHostSourcesAccountCorrelationRule(ctx, dto.AccountCorrelationRule.Get())
	diags.Append(d...)
	model.AccountCorrelationRule = accountCorrelationRule

	managerCorrelationMapping, d := datasource_source.ManagerCorrelationMappingValue{}.FromApi_betaManagerCorrelationMapping(ctx, dto.ManagerCorrelationMapping.Get())
	diags.Append(d...)
	model.ManagerCorrelationMapping = managerCorrelationMapping

	managerCorrelationRule, d := datasource_source.ManagerCorrelationRuleValue{}.FromApi_betaMultiHostSourcesManagerCorrelationRule(ctx, dto.ManagerCorrelationRule.Get())
	diags.Append(d...)
	model.ManagerCorrelationRule = managerCorrelationRule

	beforeProvisioningRule, d := datasource_source.BeforeProvisioningRuleValue{}.FromApi_betaMultiHostSourcesBeforeProvisioningRule(ctx, dto.BeforeProvisioningRule.Get())
	diags.Append(d...)
	model.BeforeProvisioningRule = beforeProvisioningRule

	managementWorkgroup, d := datasource_source.ManagementWorkgroupValue{}.FromApi_betaMultiHostIntegrationsManagementWorkgroup(ctx, dto.ManagementWorkgroup.Get())
	diags.Append(d...)
	model.ManagementWorkgroup = managementWorkgroup

	if len(dto.Schemas) > 0 {
		values := make([]datasource_source.SchemasValue, 0, len(dto.Schemas))
		for i := range dto.Schemas {
			v, d := datasource_source.SchemasValue{}.FromApi_betaMultiHostSourcesSchemasInner(ctx, &dto.Schemas[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, datasource_source.SchemasValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Schemas = listVal
	} else {
		model.Schemas = types.ListNull(datasource_source.SchemasValue{}.Type(ctx))
	}

	if len(dto.PasswordPolicies) > 0 {
		values := make([]datasource_source.PasswordPoliciesValue, 0, len(dto.PasswordPolicies))
		for i := range dto.PasswordPolicies {
			v, d := datasource_source.PasswordPoliciesValue{}.FromApi_betaMultiHostSourcesPasswordPoliciesInner(ctx, &dto.PasswordPolicies[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, datasource_source.PasswordPoliciesValue{}.Type(ctx), values)
		diags.Append(d...)
		model.PasswordPolicies = listVal
	} else {
		model.PasswordPolicies = types.ListNull(datasource_source.PasswordPoliciesValue{}.Type(ctx))
	}

	return model, diags
}

// datasourceClusterFromAPI mirrors clusterFromAPI in resource_source.go
// against the data source's generated ClusterValue type.
func datasourceClusterFromAPI(ctx context.Context, cluster *api_beta.MultiHostIntegrationsCluster) (datasource_source.ClusterValue, diag.Diagnostics) {
	if cluster == nil {
		return datasource_source.NewClusterValueNull(), nil
	}
	return datasource_source.NewClusterValue(
		datasource_source.ClusterValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(cluster.Id),
			"name": types.StringValue(cluster.Name),
			"type": types.StringValue(cluster.Type),
		},
	)
}
