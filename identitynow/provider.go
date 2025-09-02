package identitynow

import (
	"context"
	"os"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/accessprofile"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/application"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/applicationaccessassocation"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/entitlement"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/governancegroup"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/identity"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/metadataattribute"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/role"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/segment"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/source"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &Provider{}

// provider satisfies the tfsdk.Provider interface and usually is included
// with all Resource and DataSource implementations.
type Provider struct {
	// client can contain the upstream provider SDK or HTTP client used to
	// communicate with the upstream service. Resource and DataSource
	// implementations can then make calls using this client.
	//
	// TODO: If appropriate, implement upstream provider SDK or HTTP client.
	client *sailpoint.APIClient

	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ProviderModel can be used to store data from the Terraform configuration.
type ProviderModel struct {
	SailBaseUrl      types.String `tfsdk:"sail_base_url"`
	SailClientId     types.String `tfsdk:"sail_client_id"`
	SailClientSecret types.String `tfsdk:"sail_client_secret"`
	HttpRetryMax     types.Int64  `tfsdk:"http_retry_max"`
}

func (p *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "identitynow"
	resp.Version = p.version
}

func (p *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"sail_base_url": schema.StringAttribute{
				Optional: true,
			},
			"sail_client_id": schema.StringAttribute{
				Optional: true,
			},
			"sail_client_secret": schema.StringAttribute{
				Optional: true,
			},
			"http_retry_max": schema.Int64Attribute{
				Optional:    true,
				Description: "Override number of retries for the retryablehttp client - default is 20",
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var provider ProviderModel
	// var providerConfig ProviderConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &provider)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !provider.SailBaseUrl.IsNull() {
		os.Setenv("SAIL_BASE_URL", provider.SailBaseUrl.ValueString())
	}

	if !provider.SailClientId.IsNull() {
		os.Setenv("SAIL_CLIENT_ID", provider.SailClientId.ValueString())
	}

	if !provider.SailClientSecret.IsNull() {
		os.Setenv("SAIL_CLIENT_SECRET", provider.SailClientSecret.ValueString())
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

	providerConfig := config.ProviderConfig{}

	providerConfig.APIClient = apiClient

	resp.DataSourceData = providerConfig
	resp.ResourceData = providerConfig
}

func (p *Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		accessprofile.NewAccessProfileResource,
		applicationaccessassocation.NewAccessProfileAssociationResource,
		application.NewApplicationResource,
		entitlement.NewEntitlementResource,
		entitlement.NewEntitlementRequestConfigResource,
		governancegroup.NewGovernanceGroupResource,
		source.NewSourceLoadWaitResource,
		segment.NewSegmentResource,
		segment.NewSegmentAccessResource,
		metadataattribute.NewMetadataAttributeResource,
		role.NewRoleResource,
	}
}

func (p *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		entitlement.NewEntitlementDataSource,
		entitlement.NewEntitlementsDataSource,
		source.NewSourceDataSource,
		identity.NewIdentityDataSource,
		identity.NewIdentitiesDataSource,
		accessprofile.NewAccessProfileDataSource,
		application.NewApplicationDataSource,
		governancegroup.NewGovernanceGroupDataSource,
		segment.NewSegmentDataSource,
		metadataattribute.NewAccessModelMetadataDataSource,
		role.NewRoleDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{
			version: version,
		}
	}
}
