// See resource_workflow.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package workflow_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/workflow_v1/datasource_workflow"
)

var (
	_ datasource.DataSource              = (*workflowDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workflowDataSource)(nil)
)

func NewWorkflowDataSource() datasource.DataSource {
	return &workflowDataSource{}
}

type workflowDataSource struct {
	client *sailpoint.APIClient
}

// workflowDataSourceModel mirrors datasource_workflow.WorkflowModel's
// generator-managed fields plus the two hand-added fields ("definition",
// "trigger") - see workflowResourceModel's doc comment in resource_workflow.go
// for why this can't just embed the generated model type. Also reused
// directly by datasource_workflows.go's plural list (one row per matching
// workflow), so both data sources return identical attribute shapes.
type workflowDataSourceModel struct {
	Created        types.String                        `tfsdk:"created"`
	Creator        datasource_workflow.CreatorValue    `tfsdk:"creator"`
	Definition     jsontypes.Normalized                `tfsdk:"definition"`
	Description    types.String                        `tfsdk:"description"`
	Enabled        types.Bool                          `tfsdk:"enabled"`
	ExecutionCount types.Int64                         `tfsdk:"execution_count"`
	FailureCount   types.Int64                         `tfsdk:"failure_count"`
	Id             types.String                        `tfsdk:"id"`
	Modified       types.String                        `tfsdk:"modified"`
	ModifiedBy     datasource_workflow.ModifiedByValue `tfsdk:"modified_by"`
	Name           types.String                        `tfsdk:"name"`
	Owner          datasource_workflow.OwnerValue      `tfsdk:"owner"`
	Trigger        types.Object                        `tfsdk:"trigger"`
}

func (d *workflowDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_v1"
}

func (d *workflowDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_workflow.WorkflowDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Workflow from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Workflow](https://developer.sailpoint.com/docs/extensibility/workflows/) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations.\n\n" +
		workflowGuidanceMarkdown
	applyWorkflowDataSourceDefinitionField(&resp.Schema.Attributes)
	applyWorkflowDataSourceTriggerField(&resp.Schema.Attributes)
}

func (d *workflowDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workflowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workflowDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Workflow data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.WorkflowsAPI.
		GetWorkflow(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Workflow data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Workflow", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Workflow data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors workflowDtoToModel in resource_workflow.go
// but against the data source's (Go-distinct) generated value types.
func datasourceDtoToModel(ctx context.Context, dto *api_beta.Workflow) (workflowDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := workflowDataSourceModel{
		Creator:    datasource_workflow.NewCreatorValueNull(),
		ModifiedBy: datasource_workflow.NewModifiedByValueNull(),
		Owner:      datasource_workflow.NewOwnerValueNull(),
		Trigger:    types.ObjectNull(triggerAttrTypes()),
	}
	if dto == nil {
		return model, diags
	}

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		model.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	} else {
		model.Description = types.StringNull()
	}
	if dto.Enabled != nil {
		model.Enabled = types.BoolValue(*dto.Enabled)
	}
	if dto.ExecutionCount != nil {
		model.ExecutionCount = types.Int64Value(int64(*dto.ExecutionCount))
	}
	if dto.FailureCount != nil {
		model.FailureCount = types.Int64Value(int64(*dto.FailureCount))
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	if dto.Owner != nil {
		owner, d := datasource_workflow.NewOwnerValueNull().FromApi_betaWorkflowBodyOwner(ctx, dto.Owner)
		diags.Append(d...)
		model.Owner = owner
	}
	if dto.Creator != nil {
		creator, d := datasource_workflow.NewCreatorValueNull().FromApi_betaWorkflowAllOfCreator(ctx, dto.Creator)
		diags.Append(d...)
		model.Creator = creator
	}
	if dto.ModifiedBy != nil {
		modifiedBy, d := datasource_workflow.NewModifiedByValueNull().FromApi_betaWorkflowModifiedBy(ctx, dto.ModifiedBy)
		diags.Append(d...)
		model.ModifiedBy = modifiedBy
	}

	def, d := definitionFromAPI(dto.Definition)
	diags.Append(d...)
	model.Definition = def

	trg, d := triggerAPIToObject(ctx, dto.Trigger)
	diags.Append(d...)
	model.Trigger = trg

	return model, diags
}
