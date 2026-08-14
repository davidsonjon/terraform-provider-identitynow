package source_actions_v1

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/util"
)

var (
	_ action.Action              = (*AggregateAccountsAction)(nil)
	_ action.ActionWithConfigure = (*AggregateAccountsAction)(nil)
)

func NewAggregateAccountsAction() action.Action {
	return &AggregateAccountsAction{}
}

// AggregateAccountsAction triggers SailPoint account aggregation
// (load accounts) for a source and waits for the background task to
// complete.
type AggregateAccountsAction struct {
	client *sailpoint.APIClient
}

type aggregateAccountsActionModel struct {
	SourceID            types.String `tfsdk:"source_id"`
	DisableOptimization types.Bool   `tfsdk:"disable_optimization"`
	Timeout             types.String `tfsdk:"timeout"`
}

func (a *AggregateAccountsAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aggregate_accounts"
}

func (a *AggregateAccountsAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Triggers SailPoint account aggregation (load accounts) for a source and waits for the background task to complete.",
		MarkdownDescription: "Triggers SailPoint account aggregation (`load accounts`) for a source and waits for the background " +
			"task to complete. Reports progress periodically while polling. Prefer invoking this via an `action_trigger` on the " +
			"resource that changes account-affecting configuration (for example an account-type `identitynow_source_schema_v1`), " +
			"or manually via `terraform apply -invoke`.",
		Attributes: map[string]actionschema.Attribute{
			"source_id": actionschema.StringAttribute{
				Required:            true,
				Description:         "Plain IdentityNow/ISC source id to aggregate.",
				MarkdownDescription: "Plain IdentityNow/ISC source id to aggregate.",
			},
			"disable_optimization": actionschema.BoolAttribute{
				Optional: true,
				Description: "When true, forces a full re-aggregation of all accounts, bypassing the API's incremental " +
					"optimization. Defaults to false when unset.",
				MarkdownDescription: "When `true`, forces a full re-aggregation of all accounts, bypassing the API's incremental " +
					"optimization. Defaults to `false` when unset.",
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

func (a *AggregateAccountsAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if client := configureClient(req, resp); client != nil {
		a.client = client
	}
}

func (a *AggregateAccountsAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config aggregateAccountsActionModel
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

	// See source_load_entitlement_wait_v1's Create comment / this package's
	// emptyMultipartFile doc for why an (empty) file is required to make the
	// golang-sdk/v3 request builder take its correct multipart-encoding path.
	emptyFile, err := emptyMultipartFile("aggregate-accounts")
	if err != nil {
		resp.Diagnostics.AddError("Error triggering account aggregation", fmt.Sprintf("Could not prepare request body: %s", err.Error()))
		return
	}
	defer func() {
		_ = emptyFile.Close()
		_ = os.Remove(emptyFile.Name())
	}()

	call := a.client.SourcesAPI.ImportAccountsV1(invokeCtx, sourceID).File(emptyFile)
	if !config.DisableOptimization.IsNull() && !config.DisableOptimization.IsUnknown() {
		call = call.DisableOptimization(strconv.FormatBool(config.DisableOptimization.ValueBool()))
	}

	task, httpResp, err := call.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error triggering account aggregation", util.SailpointErrorDetail(err, httpResp))
		return
	}
	if task == nil || task.Task == nil || !task.Task.HasId() || task.Task.GetId() == "" {
		resp.Diagnostics.AddError(
			"Error triggering account aggregation",
			fmt.Sprintf("Source %q aggregation did not return a task id to poll.", sourceID),
		)
		return
	}

	taskID := task.Task.GetId()
	tflog.Info(invokeCtx, "Triggered account aggregation", map[string]interface{}{"source_id": sourceID, "task_id": taskID})
	resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Triggered account aggregation task %s for source %s", taskID, sourceID)})

	if err := waitForTaskCompletion(invokeCtx, a.client, taskID, resp.SendProgress); err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for account aggregation task",
			fmt.Sprintf("Task %q for source %q did not complete successfully: %s", taskID, sourceID, err.Error()),
		)
		return
	}
}
