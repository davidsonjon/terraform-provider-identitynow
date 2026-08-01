package provider

import (
	"context"
	"os"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-identitynow/internal/provider/access_model_metadata_attribute_v1"
	"terraform-provider-identitynow/internal/provider/access_profile_v1"
	"terraform-provider-identitynow/internal/provider/application_access_association_v1"
	"terraform-provider-identitynow/internal/provider/application_v1"
	"terraform-provider-identitynow/internal/provider/connector_rule_v1"
	"terraform-provider-identitynow/internal/provider/entitlement_request_config_v1"
	"terraform-provider-identitynow/internal/provider/entitlement_v1"
	"terraform-provider-identitynow/internal/provider/governance_group_v1"
	"terraform-provider-identitynow/internal/provider/identity_profile_v1"
	"terraform-provider-identitynow/internal/provider/identity_v1"
	"terraform-provider-identitynow/internal/provider/role_v1"
	"terraform-provider-identitynow/internal/provider/segment_access_v1"
	"terraform-provider-identitynow/internal/provider/segment_v1"
	"terraform-provider-identitynow/internal/provider/service_desk_integration_v1"
	"terraform-provider-identitynow/internal/provider/source_load_entitlement_wait_v1"
	"terraform-provider-identitynow/internal/provider/source_provisioning_policy_v1"
	"terraform-provider-identitynow/internal/provider/source_schema_v1"
	"terraform-provider-identitynow/internal/provider/sources_v1"
	"terraform-provider-identitynow/internal/provider/transform_v1"
)

var _ provider.Provider = (*identitynowProvider)(nil)

func New() func() provider.Provider {
	return func() provider.Provider {
		return &identitynowProvider{}
	}
}

type identitynowProvider struct {
	client *sailpoint.APIClient
}

// GetClient exposes the configured SDK client to resource/data source
// implementations in subpackages (e.g. service_desk_integration_v1) via
// resp.ResourceData/resp.DataSourceData, without those subpackages needing to
// import this package (which would create an import cycle since provider.go
// imports them to register resources/data sources).
func (p identitynowProvider) GetClient() *sailpoint.APIClient {
	return p.client
}

type ProviderModel struct {
	SailBaseUrl      types.String `tfsdk:"sail_base_url"`
	SailClientId     types.String `tfsdk:"sail_client_id"`
	SailClientSecret types.String `tfsdk:"sail_client_secret"`
	HttpRetryMax     types.Int64  `tfsdk:"http_retry_max"`
}

func (p *identitynowProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The IdentityNow (Identity Security Cloud) provider is used to interact with resources supported by SailPoint's IdentityNow/ISC APIs. " +
			"The provider needs to be configured with the proper credentials before it can be used.",
		MarkdownDescription: "The IdentityNow (Identity Security Cloud) provider is used to interact with resources supported by " +
			"[SailPoint's IdentityNow/ISC APIs](https://documentation.sailpoint.com/index.html). The provider needs to be configured " +
			"with the proper credentials before it can be used.\n\n" +
			"Credentials can be provided via the `sail_client_id`/`sail_client_secret` attributes below, or via the " +
			"`SAIL_CLIENT_ID`/`SAIL_CLIENT_SECRET`/`SAIL_BASE_URL` environment variables (preferred, to avoid committing secrets to configuration).",
		Attributes: map[string]schema.Attribute{
			"sail_base_url": schema.StringAttribute{
				Optional:            true,
				Description:         "The base URL of your IdentityNow/ISC tenant API, e.g. https://your-tenant.api.identitynow.com. May also be set via the SAIL_BASE_URL environment variable.",
				MarkdownDescription: "The base URL of your IdentityNow/ISC tenant API, e.g. `https://your-tenant.api.identitynow.com`. May also be set via the `SAIL_BASE_URL` environment variable.",
			},
			"sail_client_id": schema.StringAttribute{
				Optional:            true,
				Description:         "The OAuth client ID for a personal access token or API client on your tenant. May also be set via the SAIL_CLIENT_ID environment variable.",
				MarkdownDescription: "The OAuth client ID for a [personal access token or API client](https://developer.sailpoint.com/docs/api/authentication/) on your tenant. May also be set via the `SAIL_CLIENT_ID` environment variable.",
			},
			"sail_client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "The OAuth client secret paired with sail_client_id. May also be set via the SAIL_CLIENT_SECRET environment variable.",
				MarkdownDescription: "The OAuth client secret paired with `sail_client_id`. May also be set via the `SAIL_CLIENT_SECRET` environment variable.",
			},
			"http_retry_max": schema.Int64Attribute{
				Optional:            true,
				Description:         "Override number of retries for the retryablehttp client - default is 20",
				MarkdownDescription: "Override number of retries for the retryablehttp client - default is 20.",
			},
		},
	}
}

func (p *identitynowProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var provider ProviderModel
	// var providerConfig ProviderConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &provider)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !provider.SailBaseUrl.IsNull() {
		if err := os.Setenv("SAIL_BASE_URL", provider.SailBaseUrl.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to set SAIL_BASE_URL", err.Error())
			return
		}
	}

	if !provider.SailClientId.IsNull() {
		if err := os.Setenv("SAIL_CLIENT_ID", provider.SailClientId.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to set SAIL_CLIENT_ID", err.Error())
			return
		}
	}

	if !provider.SailClientSecret.IsNull() {
		if err := os.Setenv("SAIL_CLIENT_SECRET", provider.SailClientSecret.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to set SAIL_CLIENT_SECRET", err.Error())
			return
		}
	}

	defer func() {
		if err := recover(); err != nil {
			resp.Diagnostics.AddError(
				"Panic during provider configuration",
				"This is usually caused by not having correct SailPoint credentials configured",
			)
			return
		}
	}()

	configuration := sailpoint.NewDefaultConfiguration()
	httpClient := retryablehttp.NewClient()

	httpClient.RetryMax = 20

	if !provider.HttpRetryMax.IsNull() {
		httpClient.RetryMax = int(provider.HttpRetryMax.ValueInt64())
	}

	configuration.HTTPClient = httpClient
	apiClient := sailpoint.NewAPIClient(configuration)
	p.client = apiClient

	providerConfig := identitynowProvider{}

	providerConfig.client = apiClient

	resp.DataSourceData = providerConfig
	resp.ResourceData = providerConfig
}

func (p *identitynowProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "identitynow"
}

func (p *identitynowProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		access_model_metadata_attribute_v1.NewAccessModelMetadataAttributeDataSource,
		access_profile_v1.NewAccessProfileDataSource,
		access_profile_v1.NewAccessProfilesDataSource,
		application_v1.NewApplicationDataSource,
		application_v1.NewApplicationsDataSource,
		connector_rule_v1.NewConnectorRuleDataSource,
		connector_rule_v1.NewConnectorRulesDataSource,
		entitlement_v1.NewEntitlementDataSource,
		entitlement_v1.NewEntitlementsDataSource,
		governance_group_v1.NewGovernanceGroupConnectionsDataSource,
		governance_group_v1.NewGovernanceGroupDataSource,
		governance_group_v1.NewGovernanceGroupsDataSource,
		identity_v1.NewIdentityDataSource,
		identity_v1.NewIdentitiesDataSource,
		identity_profile_v1.NewIdentityProfileDataSource,
		identity_profile_v1.NewIdentityProfilesDataSource,
		role_v1.NewRoleDataSource,
		segment_v1.NewSegmentDataSource,
		segment_v1.NewSegmentsDataSource,
		role_v1.NewRolesDataSource,
		service_desk_integration_v1.NewServiceDeskIntegrationDataSource,
		source_provisioning_policy_v1.NewSourceProvisioningPolicyDataSource,
		source_provisioning_policy_v1.NewSourceProvisioningPoliciesDataSource,
		source_schema_v1.NewSourceSchemaDataSource,
		source_schema_v1.NewSourceSchemasDataSource,
		sources_v1.NewSourceDataSource,
		sources_v1.NewSourcesDataSource,
		transform_v1.NewTransformDataSource,
	}
}

func (p *identitynowProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		access_model_metadata_attribute_v1.NewAccessModelMetadataAttributeResource,
		access_profile_v1.NewAccessProfileResource,
		application_access_association_v1.NewApplicationAccessAssociationResource,
		application_v1.NewApplicationResource,
		connector_rule_v1.NewConnectorRuleResource,
		entitlement_request_config_v1.NewEntitlementRequestConfigResource,
		entitlement_v1.NewEntitlementResource,
		governance_group_v1.NewGovernanceGroupResource,
		governance_group_v1.NewGovernanceGroupMembersResource,
		identity_profile_v1.NewIdentityProfileResource,
		role_v1.NewRoleResource,
		segment_access_v1.NewSegmentAccessResource,
		segment_v1.NewSegmentResource,
		service_desk_integration_v1.NewServiceDeskIntegrationResource,
		source_load_entitlement_wait_v1.NewSourceLoadEntitlementWaitResource,
		source_provisioning_policy_v1.NewSourceProvisioningPolicyResource,
		source_schema_v1.NewSourceSchemaResource,
		sources_v1.NewSourceResource,
		transform_v1.NewTransformResource,
	}
}
