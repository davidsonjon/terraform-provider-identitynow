package source_actions_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"

	"terraform-provider-identitynow/internal/provider/util"
)

var (
	_ action.Action              = (*SyncSourceAttributesAction)(nil)
	_ action.ActionWithConfigure = (*SyncSourceAttributesAction)(nil)
)

func NewSyncSourceAttributesAction() action.Action {
	return &SyncSourceAttributesAction{}
}

// SyncSourceAttributesAction triggers SailPoint source attribute
// synchronization for a source.
//
// Unlike aggregate accounts/entitlements, the synchronize-attributes API
// (SourcesAPI.SyncAttributesForSourceV1) does not return a task_management
// task id, and golang-sdk/v3 exposes no separate "get sync job status by id"
// endpoint for the SourceSyncJob it returns. This action therefore only
// invokes the call and reports the job id/status the API returns
// immediately - it does not poll for completion. If a live tenant confirms a
// polling endpoint exists after all, this should be revisited to match the
// wait-for-completion behavior of the other two actions.
type SyncSourceAttributesAction struct {
	client *sailpoint.APIClient
}

type syncSourceAttributesActionModel struct {
	SourceID types.String `tfsdk:"source_id"`
	Timeout  types.String `tfsdk:"timeout"`
}

func (a *SyncSourceAttributesAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sync_source_attributes"
}

func (a *SyncSourceAttributesAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Triggers SailPoint source attribute synchronization for a source.",
		MarkdownDescription: "Triggers SailPoint source attribute synchronization (`synchronize-attributes`) for a source. " +
			"This call does not expose a task id to poll, so this action reports the job id/status returned immediately and " +
			"does not wait for completion. Prefer invoking this via an `action_trigger` on `identitynow_source_v1`'s " +
			"`after_update` for connector attribute changes, or manually via `terraform apply -invoke`.",
		Attributes: map[string]actionschema.Attribute{
			"source_id": actionschema.StringAttribute{
				Required:            true,
				Description:         "Plain IdentityNow/ISC source id to synchronize attributes for.",
				MarkdownDescription: "Plain IdentityNow/ISC source id to synchronize attributes for.",
			},
			"timeout": actionschema.StringAttribute{
				Optional: true,
				Description: "Maximum time to wait for the synchronize-attributes API call itself, as a Go duration string " +
					"(for example \"30m\"). Defaults to 30m when unset.",
				MarkdownDescription: "Maximum time to wait for the synchronize-attributes API call itself, as a Go duration " +
					"string (for example `\"30m\"`). Defaults to `30m` when unset.",
			},
		},
	}
}

func (a *SyncSourceAttributesAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if client := configureClient(req, resp); client != nil {
		a.client = client
	}
}

func (a *SyncSourceAttributesAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config syncSourceAttributesActionModel
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

	job, httpResp, err := a.client.SourcesAPI.SyncAttributesForSourceV1(invokeCtx, sourceID).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error triggering source attribute synchronization", util.SailpointErrorDetail(err, httpResp))
		return
	}
	if job == nil {
		resp.Diagnostics.AddError(
			"Error triggering source attribute synchronization",
			fmt.Sprintf("Source %q attribute synchronization did not return a job.", sourceID),
		)
		return
	}

	tflog.Info(invokeCtx, "Triggered source attribute synchronization", map[string]interface{}{
		"source_id": sourceID,
		"job_id":    job.Id,
		"status":    job.Status,
	})
	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Triggered source attribute synchronization job %s for source %s (status: %s)", job.Id, sourceID, job.Status),
	})
}
