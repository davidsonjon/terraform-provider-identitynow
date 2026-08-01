// This file adds a plural "list" data source alongside the singular
// identitynow_source_provisioning_policy_v1 data source in
// datasource_source_provisioning_policy.go, mirroring role_v1/datasource_roles.go's
// established pattern. It queries GET /sources/v1/{sourceId}/provisioning-policies
// (api_beta.SourcesAPIService.ListProvisioningPolicies) instead of
// GET /sources/v1/{sourceId}/provisioning-policies/{usageType}, and returns
// every provisioning policy defined on the source using the same nested
// object shape as identitynow_source_provisioning_policy_v1.
package source_provisioning_policy_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/source_provisioning_policy_v1/datasource_source_provisioning_policy"
)

var (
	_ datasource.DataSource              = (*sourceProvisioningPoliciesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourceProvisioningPoliciesDataSource)(nil)
)

func NewSourceProvisioningPoliciesDataSource() datasource.DataSource {
	return &sourceProvisioningPoliciesDataSource{}
}

type sourceProvisioningPoliciesDataSource struct {
	client *sailpoint.APIClient
}

// SourceProvisioningPoliciesDataSourceModel is hand-written (not generated)
// since it wraps the parent source_id plus a "provisioning_policies"
// attribute nesting the same shape as the singular data source.
type SourceProvisioningPoliciesDataSourceModel struct {
	SourceId             types.String `tfsdk:"source_id"`
	ProvisioningPolicies types.List   `tfsdk:"provisioning_policies"`
}

// sourceProvisioningPoliciesListNestedAttributes returns the same attribute
// map as the singular data source's schema (including the hand-added
// "id"/"fields" attributes), for use as the plural data source's nested
// object shape.
func sourceProvisioningPoliciesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_source_provisioning_policy.SourceProvisioningPolicyDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs)+2)
	for k, v := range attrs {
		out[k] = v
	}
	applySourceProvisioningPolicyFieldsFieldDataSource(&out)
	// source_id is Required on the singular data source (it's part of the
	// lookup key); here it's just another output field of a fully Computed
	// list, so override to Computed-only.
	out["source_id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "The Source ID.",
		MarkdownDescription: "The Source ID.",
	}
	// usage_type is likewise Required on the singular data source but a
	// plain Computed output here.
	if attr, ok := out["usage_type"].(schema.StringAttribute); ok {
		attr.Required = false
		attr.Computed = true
		out["usage_type"] = attr
	}
	return out
}

func (d *sourceProvisioningPoliciesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_provisioning_policies_v1"
}

func (d *sourceProvisioningPoliciesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Provisioning Policies defined on an existing Source in IdentityNow/ISC.",
		MarkdownDescription: "Lists all [Provisioning Policies](https://developer.sailpoint.com/docs/extensibility/transforms/guides/transforms-in-provisioning-policies) " +
			"defined on an existing Source in IdentityNow/ISC via `GET /sources/v1/{sourceId}/provisioning-policies`. " +
			"Returns the same attributes per policy as the singular `identitynow_source_provisioning_policy_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_source_provisioning_policy_v1`'s \"Known Limitations & " +
			"Live Testing Notes\" section before relying on it in production configurations; the same limitations apply " +
			"to each provisioning policy returned here.",
		Attributes: map[string]schema.Attribute{
			"source_id": schema.StringAttribute{
				Required:            true,
				Description:         "The Source ID.",
				MarkdownDescription: "The Source ID.",
			},
			"provisioning_policies": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Provisioning policies defined on the source, each with the same attributes as `identitynow_source_provisioning_policy_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: sourceProvisioningPoliciesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *sourceProvisioningPoliciesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sourceProvisioningPoliciesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SourceProvisioningPoliciesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := config.SourceId.ValueString()
	tflog.Debug(ctx, "Reading Source Provisioning Policies data source", map[string]interface{}{"source_id": sourceID})

	dtos, httpResp, err := d.client.Beta.SourcesAPI.ListProvisioningPolicies(ctx, sourceID).Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Source Provisioning Policies data source", map[string]interface{}{"source_id": sourceID, "error": err.Error()})
		resp.Diagnostics.AddError("Error listing Source Provisioning Policies", errDetail(err, httpResp))
		return
	}

	elemType := schema.NestedAttributeObject{Attributes: sourceProvisioningPoliciesListNestedAttributes(ctx)}.Type()

	models := make([]sourceProvisioningPolicyDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(&dtos[i], sourceID, sourceProvisioningPolicyDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	policiesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.ProvisioningPolicies = policiesList

	tflog.Debug(ctx, "Read Source Provisioning Policies data source", map[string]interface{}{"source_id": sourceID, "count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
