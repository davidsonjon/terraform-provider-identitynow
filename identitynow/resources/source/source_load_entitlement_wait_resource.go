package source

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	sailpoint "github.com/davidsonjon/golang-sdk"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/davidsonjon/golang-sdk/cc"
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
					mapplanmodifier.RequiresReplace(),
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

func NullSourceCheck(r *SourceLoadWaitResource, e *SourceLoadWait, oldSourceId string) error {
	var sort []cc.QuerySort

	sort = append(sort, cc.QuerySort{
		Property:  "timestamp",
		Direction: "DESC",
	})

	var filter []cc.QueryFilter

	filter = append(filter, cc.QueryFilter{
		Property: "type",
		Value:    "Source_AGGREGATION",
	})
	filter = append(filter, cc.QueryFilter{
		Property: "objectType",
		Value:    "source",
	})
	filter = append(filter, cc.QueryFilter{
		Property: "objectId",
		Value:    oldSourceId,
	})
	filter = append(filter, cc.QueryFilter{
		Property: "status",
		Value:    "PENDING",
	})

	if e.Wait.ValueBool() {
		eventList, httpResp, err := r.client.CC.ListEventsApi.ListEvents(context.TODO()).Limit(1).Filters(filter).Sorters(sort).Execute()
		if err != nil {
			log.Printf("Error when calling `ListEventsApi.ListEvents`: %v\n", err)
			log.Printf("Full HTTP response: %v\n", httpResp)
			return fmt.Errorf("error when calling `ListEventsApi.ListEvents`: %v", err)
		}
		for _, v := range *eventList.Items {
			task, httpResp, err := r.client.CC.TaskResultsApi.TaskResults(context.TODO(), v.Details.Id).Execute()
			if err != nil {
				log.Printf("Error when calling `TaskResultsApi.TaskResults`: %v\n", err)
				log.Printf("Full HTTP response: %v\n", httpResp)
				return fmt.Errorf("error when calling `TaskResultsApi.TaskResults`: %v", err)
			}
			_, _, err = task.WaitForTaskCompletion(*r.client.CC)
			if err != nil {
				log.Printf("Error when calling `task.WaitForTaskCompletion`: %v\n", err)
				return fmt.Errorf("error when calling `task.WaitForTaskCompletion`: %v", err)
			}
		}

	}

	load, httpResp, err := r.client.CC.SourcesAggregationApi.LoadEntitlements(context.Background(), oldSourceId).Execute()
	if err != nil {
		log.Printf("Full HTTP response: %v\n", httpResp)

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			return fmt.Errorf("Error:%v", sailpointError.FormattedMessage)
		} else {
			return fmt.Errorf("task status is ERROR no Sources loaded")
		}
	}

	task, httpResp, err := r.client.CC.TaskResultsApi.TaskResults(context.TODO(), *load.Task.Id).Execute()
	if err != nil {
		log.Printf("Full HTTP response: %v\n", httpResp)

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			return fmt.Errorf("Error:%v", sailpointError.FormattedMessage)
		} else {
			return fmt.Errorf("error when calling `TaskResultsApi.TaskResults``: %e", err)
		}
	}
	_, status, err := task.WaitForTaskCompletion(*r.client.CC)
	if err != nil {
		log.Printf("Full HTTP response: %v\n", httpResp)

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			return fmt.Errorf("Error:%v", sailpointError.FormattedMessage)
		} else {
			return fmt.Errorf("error when calling `task.WaitForTaskCompletion``: %e", err)
		}
	}
	if *status == "ERROR" {
		log.Printf("Full HTTP response: %v\n", httpResp)

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			return fmt.Errorf("Error:%v", sailpointError.FormattedMessage)
		} else {
			return fmt.Errorf("task status is ERROR no Sources loaded")
		}
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
