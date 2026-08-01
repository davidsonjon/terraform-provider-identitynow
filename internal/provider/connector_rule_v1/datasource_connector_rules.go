// This file adds a plural "list" data source alongside the singular
// identitynow_connector_rule_v1 data source in datasource_connector_rule.go,
// mirroring role_v1's identitynow_roles_v1 pattern. It queries
// GET /connector-rules/v1 (connector_rule_management.ConnectorRuleManagementAPI.GetConnectorRuleListV1)
// instead of GET /connector-rules/v1/{id}, and returns every connector rule
// using the exact same nested object shape as identitynow_connector_rule_v1.
//
// Unlike role_v1's plural data source, this one has no filters/limit/offset/
// sorters attributes: although the OpenAPI spec for GET /connector-rules/v1
// declares "limit"/"offset"/"count" query parameters, the vendored SDK's
// ApiGetConnectorRuleListRequest builder exposes none of them - only
// ctx/Execute() - so there is nothing for this data source to forward. This
// is a new SDK/spec mismatch distinct from the previously documented issues;
// see the connector_rule_v1 knowledge.md entry.
package connector_rule_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/connector_rule_v1/datasource_connector_rule"
)

var (
	_ datasource.DataSource              = (*connectorRulesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectorRulesDataSource)(nil)
)

func NewConnectorRulesDataSource() datasource.DataSource {
	return &connectorRulesDataSource{}
}

type connectorRulesDataSource struct {
	client *sailpoint.APIClient
}

// ConnectorRulesDataSourceModel is hand-written (not generated) since it
// wraps a "connector_rules" attribute nesting the generated
// datasource_connector_rule.ConnectorRuleModel shape.
type ConnectorRulesDataSourceModel struct {
	ConnectorRules types.List `tfsdk:"connector_rules"`
}

// connectorRulesListNestedAttributes returns the same attribute map as
// datasource_connector_rule.ConnectorRuleDataSourceSchema plus the hand-added
// "attributes" field (see applyConnectorRuleAttributesFieldDataSource),
// except "id" is overridden to Computed-only. The singular
// identitynow_connector_rule_v1 data source marks "id" as Required because
// it's the lookup key for GET /connector-rules/v1/{id}; here it's just
// another output field of a fully Computed "connector_rules" list.
func connectorRulesListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	attrs := datasource_connector_rule.ConnectorRuleDataSourceSchema(ctx).Attributes
	out := make(map[string]schema.Attribute, len(attrs)+1)
	for k, v := range attrs {
		out[k] = v
	}
	applyConnectorRuleAttributesFieldDataSource(&out)
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "the ID of the rule",
		MarkdownDescription: "the ID of the rule",
	}
	return out
}

func (d *connectorRulesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_rules_v1"
}

func (d *connectorRulesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all Connector Rules from IdentityNow/ISC.",
		MarkdownDescription: "Lists all [Connector Rules](https://developer.sailpoint.com/docs/extensibility/rules/) " +
			"from IdentityNow/ISC via `GET /connector-rules/v1`. Returns the same attributes per rule as the singular " +
			"`identitynow_connector_rule_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_connector_rule_v1`'s \"Known Limitations & Live " +
			"Testing Notes\" section before relying on it in production configurations; the same limitations apply to " +
			"each rule returned here. Note this endpoint has no filtering/pagination support in the current SDK - every " +
			"connector rule in the tenant is always returned.",
		Attributes: map[string]schema.Attribute{
			"connector_rules": schema.ListNestedAttribute{
				Computed: true,
				MarkdownDescription: "All Connector Rules in the tenant, each with the same attributes as " +
					"`identitynow_connector_rule_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: connectorRulesListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *connectorRulesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectorRulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ConnectorRulesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Connector Rules data source", nil)

	dtos, httpResp, err := d.client.ConnectorRuleManagementAPI.
		GetConnectorRuleListV1(ctx).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Connector Rules data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Connector Rules", errDetail(err, httpResp))
		return
	}

	elemType := schema.NestedAttributeObject{Attributes: connectorRulesListNestedAttributes(ctx)}.Type()

	models := make([]connectorRuleDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := connectorRuleDatasourceDtoToModel(ctx, &dtos[i], connectorRuleDataSourceModel{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	connectorRulesList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.ConnectorRules = connectorRulesList

	tflog.Debug(ctx, "Read Connector Rules data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
