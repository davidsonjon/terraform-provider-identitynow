// This file adds a plural "list" data source alongside the singular
// identitynow_workflow_v1 data source in datasource_workflow.go, mirroring
// governance_group_v1's datasource_governance_groups.go / role_v1's
// datasource_roles.go pattern. It queries GET /workflows/v1
// (api_beta.WorkflowsAPI.ListWorkflows) instead of GET /workflows/v1/{id},
// and returns every matching Workflow using the exact same nested object
// shape as identitynow_workflow_v1 (reusing datasource_workflow's generated
// schema/model/value types, the hand-added "definition"/"trigger" fields,
// and the existing datasourceDtoToModel converter in datasource_workflow.go)
// so practitioners get identical attribute names/types whether they read
// one workflow by id or query many by filter.
//
// A fully-known "filters" value WILL invoke a live API call during
// `terraform plan` itself (confirmed on role_v1/access_profile_v1's
// identically-shaped plural data sources) - be aware of this if wiring a
// test/workflow/main.tf block that must stay plan-time credential-free.
package workflow_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"

	"terraform-provider-identitynow/internal/provider/workflow_v1/datasource_workflow"
)

// workflowsListMaxLimit matches GET /workflows/v1's documented maximum
// "limit" value (see the shared limit.yaml parameter definition - 250, the
// same as most other IdentityNow v1 list APIs).
const workflowsListMaxLimit = 250

var (
	_ datasource.DataSource              = (*workflowsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workflowsDataSource)(nil)
)

func NewWorkflowsDataSource() datasource.DataSource {
	return &workflowsDataSource{}
}

type workflowsDataSource struct {
	client *sailpoint.APIClient
}

// WorkflowsDataSourceModel is hand-written (not generated) since it wraps
// the list-query parameters plus a "workflows" attribute nesting the
// hand-written workflowDataSourceModel shape (datasource_workflow.go).
type WorkflowsDataSourceModel struct {
	Filters   types.String `tfsdk:"filters"`
	Limit     types.Int64  `tfsdk:"limit"`
	Offset    types.Int64  `tfsdk:"offset"`
	Sorters   types.String `tfsdk:"sorters"`
	Workflows types.List   `tfsdk:"workflows"`
}

// workflowsListNestedAttributes returns the same attribute map as
// datasource_workflow.WorkflowDataSourceSchema (plus the hand-added
// "definition"/"trigger" attributes), except "id" is overridden to
// Computed-only. The singular identitynow_workflow_v1 data source marks
// "id" as Required because it's the lookup key for GET /workflows/v1/{id};
// here it's just another output field of a fully Computed "workflows" list.
func workflowsListNestedAttributes(ctx context.Context) map[string]schema.Attribute {
	s := datasource_workflow.WorkflowDataSourceSchema(ctx)
	applyWorkflowDataSourceDefinitionField(&s.Attributes)
	applyWorkflowDataSourceTriggerField(&s.Attributes)

	out := make(map[string]schema.Attribute, len(s.Attributes))
	for k, v := range s.Attributes {
		out[k] = v
	}
	out["id"] = schema.StringAttribute{
		Computed:            true,
		Description:         "Workflow ID.",
		MarkdownDescription: "Workflow ID.",
	}
	return out
}

func (d *workflowsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflows_v1"
}

func (d *workflowsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Workflows from IdentityNow/ISC, optionally filtered, sorted, and paginated.",
		MarkdownDescription: "Lists [Workflows](https://developer.sailpoint.com/docs/extensibility/workflows/) " +
			"from IdentityNow/ISC via `GET /workflows/v1`, optionally filtered, sorted, and paginated. Returns the same " +
			"attributes per workflow as the singular `identitynow_workflow_v1` data source.\n\n" +
			"~> This is a `_v1` pilot data source - see `identitynow_workflow_v1`'s \"Known Limitations & Live Testing " +
			"Notes\" section before relying on it in production configurations; the same limitations apply to each " +
			"workflow returned here.",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Filter expression used to query workflows. Filtering is supported for `enabled` " +
					"(`eq`), `connectorInstanceId` (`eq`), `triggerId` (`eq`). See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#filtering-results) " +
					"for the general syntax.",
			},
			"limit": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Maximum number of workflows to return. The API's documented maximum for this " +
					"endpoint is 250; values above 250 are capped to 250 with a warning.",
			},
			"offset": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Offset into the full result set, usually used with `limit` to paginate.",
			},
			"sorters": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Sort expression for the results. Sorting is supported for `modified`, `name`. " +
					"See [V3 API Standard Collection Parameters]" +
					"(https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results).",
			},
			"workflows": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Workflows matching the query, each with the same attributes as `identitynow_workflow_v1`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: workflowsListNestedAttributes(ctx),
				},
			},
		},
	}
}

func (d *workflowsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workflowsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config WorkflowsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Workflows data source", map[string]interface{}{"filters": config.Filters.ValueString()})

	apiReq := d.client.Beta.WorkflowsAPI.ListWorkflows(ctx)

	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}

	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		requestedLimit := config.Limit.ValueInt64()
		if requestedLimit > workflowsListMaxLimit {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				fmt.Sprintf("The requested limit (%d) exceeds GET /workflows/v1's documented maximum of %d. Using %d instead.",
					requestedLimit, workflowsListMaxLimit, workflowsListMaxLimit),
			)
			apiReq = apiReq.Limit(workflowsListMaxLimit)
		} else {
			apiReq = apiReq.Limit(int32(requestedLimit))
		}
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Workflows data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Workflows", errDetail(err, httpResp))
		return
	}

	rowType := workflowRowElemType(ctx)

	models := make([]workflowDataSourceModel, 0, len(dtos))
	for i := range dtos {
		model, diags := datasourceDtoToModel(ctx, &dtos[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		models = append(models, model)
	}

	workflowsList, diags := types.ListValueFrom(ctx, rowType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Workflows = workflowsList

	tflog.Debug(ctx, "Read Workflows data source", map[string]interface{}{"count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// workflowRowElemType returns the attr.Type matching workflowDataSourceModel's
// Go struct shape, used as the "workflows" list's element type for
// types.ListValueFrom. Derived from the (hand-added field-augmented)
// nested attribute map's own Type(ctx), rather than hand-duplicated, so it
// always stays in sync with workflowsListNestedAttributes.
func workflowRowElemType(ctx context.Context) attr.Type {
	return schema.NestedAttributeObject{Attributes: workflowsListNestedAttributes(ctx)}.Type()
}
