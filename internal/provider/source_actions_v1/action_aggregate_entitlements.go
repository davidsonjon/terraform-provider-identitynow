package source_actions_v1

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/util"
)

var (
	_ action.Action              = (*AggregateEntitlementsAction)(nil)
	_ action.ActionWithConfigure = (*AggregateEntitlementsAction)(nil)
)

func NewAggregateEntitlementsAction() action.Action {
	return &AggregateEntitlementsAction{}
}

// AggregateEntitlementsAction triggers SailPoint entitlement aggregation
// (load entitlements) for a source and waits for the background task to
// complete. It supersedes source_load_entitlement_wait_v1's Create logic
// with a native Terraform action, invocable manually
// (`terraform apply -invoke=action.identitynow_aggregate_entitlements.x`) or
// via an action_trigger lifecycle block.
type AggregateEntitlementsAction struct {
	client *sailpoint.APIClient
}

type aggregateEntitlementsActionModel struct {
	SourceID types.String `tfsdk:"source_id"`
	Timeout  types.String `tfsdk:"timeout"`
}

func (a *AggregateEntitlementsAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aggregate_entitlements"
}

func (a *AggregateEntitlementsAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Triggers SailPoint entitlement aggregation (load entitlements) for a source and waits for the background task to complete.",
		MarkdownDescription: "Triggers SailPoint entitlement aggregation (`load entitlements`) for a source and waits for the " +
			"background task to complete. Reports progress periodically while polling. Prefer invoking this via an " +
			"`action_trigger` on the resource that changes entitlement-affecting configuration (for example a group-type " +
			"`identitynow_source_schema_v1`), or manually via `terraform apply -invoke`.",
		Attributes: map[string]actionschema.Attribute{
			"source_id": actionschema.StringAttribute{
				Required:            true,
				Description:         "Plain IdentityNow/ISC source id to aggregate.",
				MarkdownDescription: "Plain IdentityNow/ISC source id to aggregate.",
			},
			"timeout": actionschema.StringAttribute{
				Optional: true,
				Description: "Maximum time to wait for the aggregation task to complete, as a Go duration string " +
					"(for example \"30m\"). Defaults to 30m when unset.",
				MarkdownDescription: "Maximum time to wait for the aggregation task to complete, as a Go duration string " +
					"(for example `\"30m\"`). Defaults to `30m` when unset.",
			},
		},
	}
}

func (a *AggregateEntitlementsAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if client := configureClient(req, resp); client != nil {
		a.client = client
	}
}

func (a *AggregateEntitlementsAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config aggregateEntitlementsActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, err := parseTimeout(config.Timeout.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid timeout", err.Error())
		return
	}

	invokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sourceID := config.SourceID.ValueString()

	// Passing an (empty) *os.File is a deliberate workaround for a bug in
	// the vendored golang-sdk/v3 client - see
	// source_load_entitlement_wait_v1's Create comment for the full
	// explanation (ImportEntitlements always sets a multipart/form-data
	// Content-Type but only builds a correctly-boundary'd body when a file
	// is supplied).
	emptyFile, err := emptyMultipartFile("aggregate-entitlements")
	if err != nil {
		resp.Diagnostics.AddError("Error triggering entitlement aggregation", fmt.Sprintf("Could not prepare request body: %s", err.Error()))
		return
	}
	defer func() {
		_ = emptyFile.Close()
		_ = os.Remove(emptyFile.Name())
	}()

	task, httpResp, err := a.client.SourcesAPI.ImportEntitlementsV1(invokeCtx, sourceID).File(emptyFile).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error triggering entitlement aggregation", util.SailpointErrorDetail(err, httpResp))
		return
	}
	if task == nil || !task.HasId() || task.GetId() == "" {
		resp.Diagnostics.AddError(
			"Error triggering entitlement aggregation",
			fmt.Sprintf("Source %q aggregation did not return a task id to poll.", sourceID),
		)
		return
	}

	taskID := task.GetId()
	tflog.Info(invokeCtx, "Triggered entitlement aggregation", map[string]interface{}{"source_id": sourceID, "task_id": taskID})
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Triggered entitlement aggregation task %s for source %s", taskID, sourceID)})

	if err := waitForTaskCompletion(invokeCtx, a.client, taskID, resp.SendProgress); err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for entitlement aggregation task",
			fmt.Sprintf("Task %q for source %q did not complete successfully: %s", taskID, sourceID, err.Error()),
		)
		return
	}
}
