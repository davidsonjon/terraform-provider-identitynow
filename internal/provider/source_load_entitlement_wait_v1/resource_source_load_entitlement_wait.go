// Package source_load_entitlement_wait_v1 implements a hand-written,
// trigger-style Terraform resource for SailPoint's entitlement aggregation
// action endpoint.
//
// This target intentionally does not use the repo's OpenAPI codegen pipeline:
// it has no natural 1:1 REST CRUD object to model. Instead, it behaves more
// like terraform_data/null_resource for replacement semantics:
//   - Create optionally waits for already-running entitlement import jobs on a
//     source to finish, then triggers a new import and waits for that specific
//     background task to complete.
//   - Read is a no-op because there is no persistent upstream object to GET.
//   - Update persists Terraform-only knobs (for example wait preferences)
//     without re-triggering aggregation.
//   - Delete removes Terraform state only; there is nothing to undo in the
//     upstream API once an aggregation has been launched.
//   - Import parses a composite string so existing Terraform state can adopt
//     the trigger metadata even though no server-side object exists.
package source_load_entitlement_wait_v1

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/task_management"

	"terraform-provider-identitynow/internal/provider/util"
)

const (
	defaultCreateTimeoutString         = "30m"
	initialPollInterval                = 2 * time.Second
	maxPollInterval                    = 15 * time.Second
	importStatePartCount               = 3
	waitForActiveJobsImportIDComponent = 2
	sourceIDImportIDComponent          = 0
	triggersImportIDComponent          = 1
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*SourceLoadEntitlementWaitResource)(nil)
	_ resource.ResourceWithConfigure   = (*SourceLoadEntitlementWaitResource)(nil)
	_ resource.ResourceWithImportState = (*SourceLoadEntitlementWaitResource)(nil)
)

func NewSourceLoadEntitlementWaitResource() resource.Resource {
	return &SourceLoadEntitlementWaitResource{}
}

type SourceLoadEntitlementWaitResource struct {
	client *sailpoint.APIClient
}

type sourceLoadEntitlementWaitResourceModel struct {
	CreateTimeout     types.String `tfsdk:"create_timeout"`
	Id                types.String `tfsdk:"id"`
	SourceID          types.String `tfsdk:"source_id"`
	Triggers          types.Map    `tfsdk:"triggers"`
	WaitForActiveJobs types.Bool   `tfsdk:"wait_for_active_jobs"`
}

func (r *SourceLoadEntitlementWaitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_load_entitlement_wait_v1"
}

func (r *SourceLoadEntitlementWaitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Triggers SailPoint entitlement aggregation for a source and optionally waits for background jobs to complete.",
		MarkdownDescription: "Triggers SailPoint entitlement aggregation (`load entitlements`) for a source and optionally waits " +
			"for related background jobs to complete. This is a hand-written action resource with `null_resource`-style replacement " +
			"behavior rather than a CRUD wrapper around a persistent upstream object.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				Description:         "Synthetic Terraform identifier. When Create triggers a task, this is set to that task id; imported state uses source_id because no historical task id is available.",
				MarkdownDescription: "Synthetic Terraform identifier. When Create triggers a task, this is set to that task id; imported state uses `source_id` because no historical task id is available.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"source_id": resourceschema.StringAttribute{
				Required:            true,
				Description:         "Plain IdentityNow/ISC source id to aggregate. This resource intentionally uses the API path parameter's source id semantics, not the legacy provider's connector external id example.",
				MarkdownDescription: "Plain IdentityNow/ISC source id to aggregate. This resource intentionally uses the API path parameter's `source_id` semantics, not the legacy provider's connector external id example.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"triggers": resourceschema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Description:         "Arbitrary key/value pairs that force replacement when changed, re-triggering entitlement aggregation.",
				MarkdownDescription: "Arbitrary key/value pairs that force replacement when changed, re-triggering entitlement aggregation.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"wait_for_active_jobs": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				Description:         "When true, Create waits for any already-running entitlement aggregation jobs for this source to finish before launching a new one.",
				MarkdownDescription: "When `true`, Create waits for any already-running entitlement aggregation jobs for this source to finish before launching a new one.",
			},
			"create_timeout": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultCreateTimeoutString),
				Description:         "Overall timeout for Create, covering any pre-wait for active jobs, the trigger request, and waiting for the launched task to finish.",
				MarkdownDescription: "Overall timeout for Create, covering any pre-wait for active jobs, the trigger request, and waiting for the launched task to finish.",
			},
		},
	}
}

func (r *SourceLoadEntitlementWaitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = cp.GetClient()
}

func (r *SourceLoadEntitlementWaitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sourceLoadEntitlementWaitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, err := parseCreateTimeout(plan.CreateTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Invalid create_timeout", err.Error())
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	sourceID := plan.SourceID.ValueString()

	if plan.WaitForActiveJobs.ValueBool() {
		if err := r.waitForNoActiveEntitlementImports(createCtx, sourceID); err != nil {
			resp.Diagnostics.AddError(
				"Error waiting for active entitlement aggregation jobs",
				fmt.Sprintf("Source %q could not be cleared for a new aggregation: %s", sourceID, err.Error()),
			)
			return
		}
	}

	// Phase-B live testing against a real tenant confirmed ImportEntitlements
	// expects the plain ISC source id (the same id used everywhere else in
	// this provider), not the reference provider's connector cloud_external_id
	// example - both the plain id and a delimited-file source's cloudExternalId
	// were tried directly against the API; only the plain id was accepted.
	emptyFile, err := emptyImportEntitlementsFile()
	if err != nil {
		resp.Diagnostics.AddError("Error triggering entitlement aggregation", fmt.Sprintf("Could not prepare request body: %s", err.Error()))
		return
	}
	defer func() {
		_ = emptyFile.Close()
		_ = os.Remove(emptyFile.Name())
	}()

	// Passing an (empty) *os.File is a deliberate workaround for a bug in the
	// vendored golang-sdk/v3 client: ApiImportEntitlementsRequest always sets
	// the request Content-Type header to "multipart/form-data", but
	// client.prepareRequest only actually builds a (correctly boundary'd)
	// multipart body when `len(formFiles) > 0` due to an operator-precedence
	// bug in its condition ((hasMultipartPrefix && len(formParams) > 0) ||
	// len(formFiles) > 0). With no file, the header claims multipart/form-data
	// but the body/boundary are never set, which the API intermittently (and,
	// in live testing, consistently) rejected with HTTP 500. Supplying a real
	// (empty) file forces the SDK down its correct multipart-encoding path.
	task, httpResp, err := r.client.SourcesAPI.ImportEntitlementsV1(createCtx, sourceID).File(emptyFile).Execute()
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
	tflog.Info(createCtx, "Triggered entitlement aggregation", map[string]interface{}{"source_id": sourceID, "task_id": taskID})

	if err := r.waitForTaskCompletion(createCtx, sourceID, taskID); err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for entitlement aggregation task",
			fmt.Sprintf("Task %q for source %q did not complete successfully: %s", taskID, sourceID, err.Error()),
		)
		return
	}

	state := plan
	state.Id = types.StringValue(taskID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SourceLoadEntitlementWaitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// No-op by design: this action resource represents "an aggregation was
	// triggered" plus Terraform-only settings, not a persistent upstream object
	// that can be re-read from SailPoint.
	var state sourceLoadEntitlementWaitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SourceLoadEntitlementWaitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sourceLoadEntitlementWaitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sourceLoadEntitlementWaitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := parseCreateTimeout(plan.CreateTimeout); err != nil {
		resp.Diagnostics.AddError("Invalid create_timeout", err.Error())
		return
	}

	// source_id and triggers already force replacement. If Update is reached, it
	// is only for Terraform-local knobs like wait_for_active_jobs/create_timeout,
	// which should not re-trigger a new aggregation run on their own.
	state.SourceID = plan.SourceID
	state.Triggers = plan.Triggers
	state.WaitForActiveJobs = plan.WaitForActiveJobs
	state.CreateTimeout = plan.CreateTimeout

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SourceLoadEntitlementWaitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No API call: there is no server-side object or "undo" operation for a
	// previously launched entitlement aggregation task.
	resp.State.RemoveResource(ctx)
}

func (r *SourceLoadEntitlementWaitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseImportStateID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import identifier", err.Error())
		return
	}

	state := sourceLoadEntitlementWaitResourceModel{
		CreateTimeout:     types.StringValue(defaultCreateTimeoutString),
		Id:                types.StringValue(parsed.SourceID),
		SourceID:          types.StringValue(parsed.SourceID),
		Triggers:          parsed.Triggers,
		WaitForActiveJobs: types.BoolValue(parsed.WaitForActiveJobs),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SourceLoadEntitlementWaitResource) waitForNoActiveEntitlementImports(ctx context.Context, sourceID string) error {
	for attempt := 0; ; attempt++ {
		tasks, httpResp, err := r.client.TaskManagementAPI.
			GetTaskStatusListV1(ctx).
			Filters(entitlementImportTaskStatusListFilter(sourceID)).
			Limit(250).
			Execute()
		if err != nil {
			return fmt.Errorf("listing active entitlement aggregation tasks: %s", util.SailpointErrorDetail(err, httpResp))
		}
		if len(tasks) == 0 {
			return nil
		}

		pollInterval := pollInterval(attempt)
		tflog.Info(ctx, "Waiting for active entitlement aggregation tasks to finish", map[string]interface{}{
			"source_id":      sourceID,
			"active_tasks":   len(tasks),
			"poll_interval":  pollInterval.String(),
			"filter_applied": entitlementImportTaskStatusListFilter(sourceID),
		})

		if err := sleepWithContext(ctx, pollInterval); err != nil {
			return fmt.Errorf("timed out while waiting for active entitlement aggregation tasks to finish: %w", err)
		}
	}
}

func (r *SourceLoadEntitlementWaitResource) waitForTaskCompletion(ctx context.Context, sourceID, taskID string) error {
	for attempt := 0; ; attempt++ {
		status, httpResp, err := r.client.TaskManagementAPI.GetTaskStatusV1(ctx, taskID).Execute()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("timed out while polling task status: %w", ctx.Err())
			}
			return fmt.Errorf("retrieving task status: %s", util.SailpointErrorDetail(err, httpResp))
		}
		if status == nil {
			return fmt.Errorf("task status response for task %q was empty", taskID)
		}

		// Live testing showed `completed` and `completionStatus` are not
		// always written atomically: a poll can observe `completed` already
		// set while `completionStatus` is still empty/null for a brief
		// window. Treat that combination as "not yet finished" and keep
		// polling, rather than misreporting it as a failed/unknown
		// completion status.
		if finished, completionStatus := taskCompletionResult(status); finished {
			if !isSuccessfulCompletionStatus(completionStatus) {
				return fmt.Errorf("completed with status %q", completionStatus)
			}
			tflog.Info(ctx, "Entitlement aggregation task completed", map[string]interface{}{
				"source_id":         sourceID,
				"task_id":           taskID,
				"completion_status": completionStatus,
			})
			return nil
		}

		pollInterval := pollInterval(attempt)
		tflog.Debug(ctx, "Waiting for entitlement aggregation task completion", map[string]interface{}{
			"source_id":     sourceID,
			"task_id":       taskID,
			"poll_interval": pollInterval.String(),
		})
		if err := sleepWithContext(ctx, pollInterval); err != nil {
			return fmt.Errorf("timed out while waiting for task completion: %w", err)
		}
	}
}

type parsedImportState struct {
	SourceID          string
	Triggers          types.Map
	WaitForActiveJobs bool
}

func parseImportStateID(id string) (parsedImportState, error) {
	parts := strings.Split(id, ",")
	if len(parts) != importStatePartCount {
		return parsedImportState{}, fmt.Errorf(
			"expected import id in the format <source_id>,<trigger_key1>:<trigger_value1>/<trigger_key2>:<trigger_value2>,<wait_for_active_jobs>; got %q",
			id,
		)
	}

	sourceID := strings.TrimSpace(parts[sourceIDImportIDComponent])
	if sourceID == "" {
		return parsedImportState{}, fmt.Errorf("source_id component must not be empty")
	}

	waitForActiveJobs, err := strconv.ParseBool(strings.TrimSpace(parts[waitForActiveJobsImportIDComponent]))
	if err != nil {
		return parsedImportState{}, fmt.Errorf("wait_for_active_jobs component must be true or false: %w", err)
	}

	triggers, err := parseImportTriggers(parts[triggersImportIDComponent])
	if err != nil {
		return parsedImportState{}, err
	}

	return parsedImportState{
		SourceID:          sourceID,
		Triggers:          triggers,
		WaitForActiveJobs: waitForActiveJobs,
	}, nil
}

func parseImportTriggers(raw string) (types.Map, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.MapNull(types.StringType), nil
	}

	values := map[string]attr.Value{}
	for _, pair := range strings.Split(raw, "/") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return types.Map{}, fmt.Errorf("trigger %q must be in key:value format", pair)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return types.Map{}, fmt.Errorf("trigger %q has an empty key", pair)
		}
		values[key] = types.StringValue(parts[1])
	}

	mapValue, diags := types.MapValue(types.StringType, values)
	if diags.HasError() {
		return types.Map{}, fmt.Errorf("building triggers map value: %v", diags)
	}
	return mapValue, nil
}

func parseCreateTimeout(v types.String) (time.Duration, error) {
	if v.IsNull() || v.IsUnknown() {
		return time.ParseDuration(defaultCreateTimeoutString)
	}

	d, err := time.ParseDuration(v.ValueString())
	if err != nil {
		return 0, fmt.Errorf("create_timeout must be a valid Go duration such as %q: %w", defaultCreateTimeoutString, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("create_timeout must be greater than zero")
	}
	return d, nil
}

// entitlementImportTaskStatusListFilter intentionally does not filter on
// `type`. The task-status-v1 spec documents a `type` filter enum that
// includes CLOUD_ENTITLEMENT_IMPORT, but live testing against a real tenant
// showed the API rejects that value outright ("Unsupported Task Definition
// type") and that the aggregation task's actual `type` field is always the
// generic scheduler value "QUARTZ" - the task's nature is only distinguished
// by `uniqueName`/`taskDefinitionSummary`, not `type`. Filtering on
// `sourceId` + `completionStatus isnull` alone is sufficient to find any
// in-progress task for this source (aggregation or otherwise) and avoids
// depending on an unusable/incorrect enum value.
// emptyImportEntitlementsFile creates a throwaway empty temp file purely so
// ImportEntitlements' generated request builder takes its correct
// multipart-encoding code path (see the Create comment above for why this is
// required). The file's content is irrelevant: this resource only supports
// non-file (directly connected) sources today, which ignore the request body
// entirely.
func emptyImportEntitlementsFile() (*os.File, error) {
	f, err := os.CreateTemp("", "source-load-entitlement-wait-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("creating empty temp file: %w", err)
	}
	return f, nil
}

func entitlementImportTaskStatusListFilter(sourceID string) string {
	return fmt.Sprintf("sourceId eq %q and completionStatus isnull", sourceID)
}

func normalizedCompletionStatus(status task_management.NullableString) string {
	if !status.IsSet() || status.Get() == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*status.Get()))
}

func isSuccessfulCompletionStatus(status string) bool {
	return status == "SUCCESS" || status == "WARNING"
}

// taskCompletionResult decides whether a polled task status should be
// treated as finished. `completed` and `completionStatus` are not always
// written atomically by the API: a poll can observe `completed` already set
// while `completionStatus` is still empty/null for a brief window. That
// combination is treated as "not yet finished" (finished=false) so the
// caller keeps polling instead of misreporting a false completion status.
func taskCompletionResult(status *task_management.TaskStatus) (finished bool, completionStatus string) {
	if status == nil {
		return false, ""
	}
	completionStatus = normalizedCompletionStatus(status.CompletionStatus)
	if status.Completed.IsSet() && status.Completed.Get() != nil && completionStatus != "" {
		return true, completionStatus
	}
	return false, ""
}

func pollInterval(attempt int) time.Duration {
	if attempt <= 0 {
		return initialPollInterval
	}

	interval := initialPollInterval
	for i := 0; i < attempt && interval < maxPollInterval; i++ {
		interval *= 2
		if interval >= maxPollInterval {
			return maxPollInterval
		}
	}
	return interval
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
