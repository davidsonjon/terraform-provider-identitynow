// See resource_source_provisioning_policy.go in this package for design
// notes and known limitations shared by both the resource and this data
// source.
package source_provisioning_policy_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/source_provisioning_policy_v1/datasource_source_provisioning_policy"
)

var (
	_ datasource.DataSource              = (*sourceProvisioningPolicyDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sourceProvisioningPolicyDataSource)(nil)
)

func NewSourceProvisioningPolicyDataSource() datasource.DataSource {
	return &sourceProvisioningPolicyDataSource{}
}

type sourceProvisioningPolicyDataSource struct {
	client *sailpoint.APIClient
}

// sourceProvisioningPolicyDataSourceModel mirrors
// datasource_source_provisioning_policy.SourceProvisioningPolicyModel plus
// the hand-added "id"/"fields" fields - see
// sourceProvisioningPolicyResourceModel's doc comment in
// resource_source_provisioning_policy.go for why this can't just embed the
// generated model type.
type sourceProvisioningPolicyDataSourceModel struct {
	Id          types.String         `tfsdk:"id"`
	SourceId    types.String         `tfsdk:"source_id"`
	UsageType   types.String         `tfsdk:"usage_type"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Fields      jsontypes.Normalized `tfsdk:"fields"`
}

func (d *sourceProvisioningPolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_provisioning_policy_v1"
}

func (d *sourceProvisioningPolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_source_provisioning_policy.SourceProvisioningPolicyDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Provisioning Policy on an existing Source from IdentityNow/ISC by source_id + usage_type."
	resp.Schema.MarkdownDescription = "Reads a [Provisioning Policy](https://developer.sailpoint.com/docs/extensibility/transforms/guides/transforms-in-provisioning-policies) " +
		"on an existing Source from IdentityNow/ISC, identified by `source_id` + `usage_type`.\n\n" +
		"~> This is a `_v1` pilot data source - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations."
	applySourceProvisioningPolicyFieldsFieldDataSource(&resp.Schema.Attributes)
}

// applySourceProvisioningPolicyFieldsFieldDataSource mirrors
// applySourceProvisioningPolicyFieldsField (resource_source_provisioning_policy_planmodifiers.go)
// but against a datasource/schema.Attribute map (a distinct Go type from
// resource/schema.Attribute, so it can't share the same function signature).
func applySourceProvisioningPolicyFieldsFieldDataSource(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	(*attrs)["id"] = dsschema.StringAttribute{
		Computed:            true,
		Description:         "Synthesized composite id in the form \"source_id/usage_type\" (ProvisioningPolicyDto has no native id).",
		MarkdownDescription: "Synthesized composite id in the form `source_id/usage_type` (`ProvisioningPolicyDto` has no native id).",
	}
	(*attrs)["fields"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         sourceProvisioningPolicyFieldsDescription,
		MarkdownDescription: sourceProvisioningPolicyFieldsDescription,
	}
}

func (d *sourceProvisioningPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sourceProvisioningPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sourceProvisioningPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := config.SourceId.ValueString()
	usageType := config.UsageType.ValueString()
	tflog.Debug(ctx, "Reading Source Provisioning Policy data source", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})

	dto, httpResp, err := d.client.Beta.SourcesAPI.
		GetProvisioningPolicy(ctx, sourceID, api_beta.UsageType(usageType)).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Source Provisioning Policy data source", map[string]interface{}{"source_id": sourceID, "usage_type": usageType, "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source Provisioning Policy", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(dto, sourceID, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_source_provisioning_policy.go
// but against the data source's model type.
func datasourceDtoToModel(dto *api_beta.ProvisioningPolicyDto, sourceID string, fallback sourceProvisioningPolicyDataSourceModel) (sourceProvisioningPolicyDataSourceModel, diag.Diagnostics) {
	model := fallback

	usageType := ""
	if dto.UsageType != nil {
		usageType = string(*dto.UsageType)
	}

	model.Id = types.StringValue(idFromParts(sourceID, usageType))
	model.SourceId = types.StringValue(sourceID)
	model.UsageType = types.StringValue(usageType)
	model.Name = types.StringValue(dto.GetName())
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	} else {
		model.Description = types.StringNull()
	}

	fields, diags := normalizedFieldsFromAPI(dto.Fields)
	model.Fields = fields
	return model, diags
}
