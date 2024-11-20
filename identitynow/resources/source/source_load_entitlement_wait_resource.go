package source

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SourceLoadWaitResource{}
var _ resource.ResourceWithImportState = &SourceLoadWaitResource{}

func NewSourceLoadWaitResource() resource.Resource {
	return &SourceLoadWaitResource{}
}

type SourceLoadWaitResource struct {
	client *sailpoint.APIClient
}

func (r *SourceLoadWaitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_load_entitlement_wait"
}

func (r *SourceLoadWaitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Source Load Entitlement Wait resource. On create will call /loadentitlement endpoint for a Source",

		Attributes: map[string]schema.Attribute{
			"source_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "Source ID",
			},
			"wait_for_active_jobs": schema.BoolAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Wait for any active jobs to finish before starting",
			},
			"triggers": schema.MapAttribute{
				MarkdownDescription: "(Optional) Arbitrary map of values that, when changed, will run any creation or destroy delays again. ",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers: []planmodifier.Map{
					TriggerOnAddOrValueChange(),
				},
			},
		},
	}
}

func (r *SourceLoadWaitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(config.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected sailpoint.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = config.APIClient
}

func NullSourceCheck(r *SourceLoadWaitResource, e *SourceLoadWait, sourceId string) error {

	if e.Wait.ValueBool() {
		limit := int32(5)  // int32 | Max number of results to return. See [V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters) for more information. (optional) (default to 250)
		offset := int32(0) // int32 | Offset into the full result set. Usually specified with *limit* to paginate through the results. See [V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters) for more information. (optional) (default to 0)
		count := false     // bool | If *true* it will populate the *X-Total-Count* response header with the number of results that would be returned if *limit* and *offset* were ignored.  Since requesting a total count can have a performance impact, it is recommended not to send **count=true** if that value will not be used.  See [V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters) for more information. (optional) (default to false)
		filters := fmt.Sprintf(`sourceId eq "%v"`, sourceId)
		sorters := "-created" // string | Sort results using the standard syntax described in [V3 API Standard Collection Parameters](https://developer.sailpoint.com/idn/api/standard-collection-parameters#sorting-results)  Sorting is supported for the following fields: **created** (optional)

		tasks, httpResp, err := r.client.Beta.TaskManagementAPI.GetTaskStatusList(context.Background()).Limit(limit).Offset(offset).Count(count).Filters(filters).Sorters(sorters).Execute()
		if err != nil {
			log.Printf("httpResp:%v", httpResp)
			return fmt.Errorf("task status is ERROR no Sources loaded")
		}
		log.Printf("[WaitForTaskCompletion] httpResp:%v", httpResp)

		if len(tasks) > 0 {

			if *tasks[0].CompletionStatus.Get() == "" {
				for {
					log.Printf("[WaitForTaskCompletion] polling taskId:%s", tasks[0].Id)
					time.Sleep(200 * time.Millisecond)

					task, httpResp, err := r.client.Beta.TaskManagementAPI.GetTaskStatus(context.TODO(), tasks[0].Id).Execute()
					if err != nil {
						log.Printf("httpResp:%v", httpResp)
						return fmt.Errorf("task status is ERROR no Sources loaded")
					}

					log.Printf("[WaitForTaskCompletion] task completion status:%v progress:%v", task.CompletionStatus, task.Progress)

					if *task.CompletionStatus.Get() != "" {
						time.Sleep(20 * time.Second)
						break
					}

					// This sleep is required for the API objects to become available post aggregation assuming the aggregation completed successfully
					time.Sleep(2 * time.Second)
				}
			}
		}

	}

	load, httpResp, err := r.client.Beta.SourcesAPI.ImportEntitlements(context.TODO(), sourceId).Execute()
	if err != nil {
		log.Printf("Full HTTP response: %v\n", httpResp)

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			return fmt.Errorf("Error:%v", *sailpointError.GetMessages()[0].Text)
		} else {
			return fmt.Errorf("task status is ERROR no Sources loaded")
		}
	}

	for {
		log.Printf("[WaitForTaskCompletion] polling taskId:%s", *load.Id)
		time.Sleep(200 * time.Millisecond)

		task, httpResp, err := r.client.Beta.TaskManagementAPI.GetTaskStatus(context.TODO(), *load.Id).Execute()
		if err != nil {
			log.Printf("httpResp:%v", httpResp)
			return fmt.Errorf("task status is ERROR no Sources loaded")
		}

		log.Printf("[WaitForTaskCompletion] task completion status:%v progress:%v", task.CompletionStatus, task.Progress)

		if task.CompletionStatus.Get() != nil {
			time.Sleep(20 * time.Second)
			break
		}

		// This sleep is required for the API objects to become available post aggregation assuming the aggregation completed successfully
		time.Sleep(2 * time.Second)
	}

	return nil
}

func (r *SourceLoadWaitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SourceLoadWait

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := NullSourceCheck(r, &data, data.SourceId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Couldn't NullSourceCheck Source",
			fmt.Sprintf("Could not find value: %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SourceLoadWaitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {

}

func (r *SourceLoadWaitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SourceLoadWait

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SourceLoadWaitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SourceLoadWait

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *SourceLoadWaitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ",")
	sourceId := parts[0]

	wait, err := strconv.ParseBool(parts[2])
	if err != nil {
		log.Fatal(err)
	}

	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		err := fmt.Errorf("unexpected format for ID (%[1]s), expected source_id,mapkey1:mapvalue1/mapkey2:mapvalue2,wait_for_active_jobs", req.ID)
		resp.Diagnostics.AddError(fmt.Sprintf("error importing entitlement wait (%s)", req.ID), err.Error())
		return
	}

	triggers := strings.Split(parts[1], "/")
	if len(parts) == 0 {
		err := fmt.Errorf("unexpected triggers (%[1]s), expected source_id,mapkey1:mapvalue1/mapkey2:mapvalue2,wait_for_active_jobs", req.ID)
		resp.Diagnostics.AddError(fmt.Sprintf("error importing entitlement wait (%s)", req.ID), err.Error())
		return
	}

	elements := map[string]attr.Value{}

	for _, t := range triggers {
		trigger := strings.Split(t, ":")
		if len(trigger) != 2 || trigger[0] == "" {
			err := fmt.Errorf("unexpected format for trigger (%[1]s), expected source_id,mapkey1:mapvalue1/mapkey2:mapvalue2,wait_for_active_jobs", req.ID)
			resp.Diagnostics.AddError(fmt.Sprintf("error importing entitlement wait (%s)", req.ID), err.Error())
			return
		}
		elements[trigger[0]] = types.StringValue(trigger[1])
	}

	mapValue, diags := types.MapValue(types.StringType, elements)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_id"), sourceId)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("triggers"), mapValue)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("wait_for_active_jobs"), wait)...)

}

// triggerOnAddOrValueChange implements the plan modifier.
type triggerOnAddOrValueChange struct{}

// Description returns a human-readable description of the plan modifier.
func (m triggerOnAddOrValueChange) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m triggerOnAddOrValueChange) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifyMap implements the plan modification logic.
func (m triggerOnAddOrValueChange) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {

	// Do nothing if there is no state value.
	if req.StateValue.IsNull() {
		return
	}

	if req.PlanValue.IsNull() {
		return
	}

	if req.PlanValue.IsUnknown() {
		resp.RequiresReplace = true
		return
	}

	var state, plan SourceLoadWait

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	stateTriggerMap := make(map[string]types.String, len(state.Triggers.Elements()))
	diags := state.Triggers.ElementsAs(ctx, &stateTriggerMap, false)
	resp.Diagnostics.Append(diags...)

	planTriggerMap := make(map[string]types.String, len(plan.Triggers.Elements()))
	diags = plan.Triggers.ElementsAs(ctx, &planTriggerMap, false)
	resp.Diagnostics.Append(diags...)

	for k, v := range planTriggerMap {
		if _, ok := stateTriggerMap[k]; !ok {
			resp.RequiresReplace = true
			return
		}
		if stateTriggerMap[k].ValueString() != v.ValueString() {
			resp.RequiresReplace = true
			return
		}
	}

	return
}

func TriggerOnAddOrValueChange() planmodifier.Map {
	return triggerOnAddOrValueChange{}
}
