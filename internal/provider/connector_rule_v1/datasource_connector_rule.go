// See resource_connector_rule.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package connector_rule_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/connector_rule_v1/datasource_connector_rule"
)

var (
	_ datasource.DataSource              = (*connectorRuleDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectorRuleDataSource)(nil)
)

func NewConnectorRuleDataSource() datasource.DataSource {
	return &connectorRuleDataSource{}
}

type connectorRuleDataSource struct {
	client *sailpoint.APIClient
}

// connectorRuleDataSourceModel mirrors datasource_connector_rule.ConnectorRuleModel
// plus the hand-added "attributes" field - see connectorRuleResourceModel's
// doc comment in resource_connector_rule.go for why this can't just embed the
// generated model type.
type connectorRuleDataSourceModel struct {
	Created     types.String                              `tfsdk:"created"`
	Description types.String                              `tfsdk:"description"`
	Id          types.String                              `tfsdk:"id"`
	Modified    types.String                              `tfsdk:"modified"`
	Name        types.String                              `tfsdk:"name"`
	Signature   datasource_connector_rule.SignatureValue  `tfsdk:"signature"`
	SourceCode  datasource_connector_rule.SourceCodeValue `tfsdk:"source_code"`
	Type        types.String                              `tfsdk:"type"`
	Attributes  jsontypes.Normalized                      `tfsdk:"attributes"`
}

func (d *connectorRuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_rule_v1"
}

func (d *connectorRuleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_connector_rule.ConnectorRuleDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Connector Rule from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Connector Rule](https://developer.sailpoint.com/docs/extensibility/rules/) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations.\n\n" +
		connectorRuleGuidanceMarkdown
	applyConnectorRuleAttributesFieldDataSource(&resp.Schema.Attributes)
}

// applyConnectorRuleAttributesFieldDataSource mirrors applyConnectorRuleAttributesField
// (resource_connector_rule_planmodifiers.go) but against a
// datasource/schema.Attribute map (a distinct Go type from
// resource/schema.Attribute, so it can't share the same function signature).
func applyConnectorRuleAttributesFieldDataSource(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	desc := "A raw JSON object of arbitrary metadata about the connector rule. Unlike identitynow_transform_v1's " +
		"\"attributes\" (a discriminated union keyed by \"type\"), this has no fixed shape."
	(*attrs)["attributes"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

func (d *connectorRuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectorRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config connectorRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Connector Rule data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.ConnectorRuleManagementAPI.
		GetConnectorRule(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Connector Rule data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Connector Rule", errDetail(err, httpResp))
		return
	}

	state, diags := connectorRuleDatasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Connector Rule data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// connectorRuleDatasourceDtoToModel mirrors connectorRuleResponseToModel in
// resource_connector_rule.go but against the data source's generated model
// type (a distinct Go type from the resource's, per package).
func connectorRuleDatasourceDtoToModel(ctx context.Context, dto *api_beta.ConnectorRuleResponse, fallback connectorRuleDataSourceModel) (connectorRuleDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.Id = types.StringValue(dto.Id)
	model.Created = types.StringValue(dto.Created)
	model.Name = types.StringValue(dto.Name)
	model.Type = types.StringValue(dto.Type)

	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	} else {
		model.Description = types.StringNull()
	}

	if dto.Modified.IsSet() && dto.Modified.Get() != nil {
		model.Modified = types.StringValue(*dto.Modified.Get())
	} else {
		model.Modified = types.StringNull()
	}

	sourceCode, d := datasourceSourceCodeFromAPI(ctx, dto.SourceCode)
	diags.Append(d...)
	model.SourceCode = sourceCode

	signature, d := datasourceSignatureFromAPI(ctx, dto.Signature)
	diags.Append(d...)
	model.Signature = signature

	model.Attributes, d = normalizedAttributesFromAPI(dto.Attributes)
	diags.Append(d...)

	return model, diags
}
