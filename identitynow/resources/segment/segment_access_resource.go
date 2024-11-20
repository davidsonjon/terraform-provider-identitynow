package segment

import (
	"context"
	"fmt"
	"log"
	"slices"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &SegmentAccessResource{}
var _ resource.ResourceWithImportState = &SegmentAccessResource{}

func NewSegmentAccessResource() resource.Resource {
	return &SegmentAccessResource{}
}

type SegmentAccessResource struct {
	client *sailpoint.APIClient
}

func (r *SegmentAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment_access"
}

func (r *SegmentAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "SegmentAccess resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The segment access id - same segment id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"segment_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The segment id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"assignments": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The ID of the segment",
						},
						"type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("segment", "ACCESS_PROFILE", "ROLE"),
							},
							MarkdownDescription: "The type of the segment, will always be segment",
						},
					},
				},
				Required: true,
			},
		},
	}
}

func (r *SegmentAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(config.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = config.APIClient
}

func createSegmentAccess(r *SegmentAccessResource, plan SegmentAccess) error {

	assignments := []api_v3.SegmentAccessItem{}
	removals := []api_v3.SegmentAccessItem{}

	access := api_v3.SegmentAccess{
		Assignments: &assignments,
		Removals:    &removals,
	}

	// planAssignments := []SegmentAccessAssignments{}

	for _, m := range plan.Assignments {
		member := api_v3.SegmentAccessItem{
			Id:   m.Id.ValueStringPointer(),
			Type: m.Type.ValueStringPointer(),
		}
		assignments = append(assignments, member)
	}

	err := r.client.V3.SegmentsAPI.PatchSegmentAccess(context.TODO(), plan.SegmentId.ValueString()).SegmentAccess(access).Execute()
	if err != nil {
		tflog.Info(context.TODO(), fmt.Sprintf("Full HTTP response: %v", err))
		return err
	}

	return nil
}

func updateSegmentAccess(r *SegmentAccessResource, plan SegmentAccess, state SegmentAccess) error {

	assignments := []api_v3.SegmentAccessItem{}
	removals := []api_v3.SegmentAccessItem{}

	access := api_v3.SegmentAccess{
		Assignments: &assignments,
		Removals:    &removals,
	}

	// planAssignments := []SegmentAccessAssignments{}

	for _, m := range plan.Assignments {
		member := api_v3.SegmentAccessItem{
			Id:   m.Id.ValueStringPointer(),
			Type: m.Type.ValueStringPointer(),
		}
		assignments = append(assignments, member)
	}

	log.Printf("assignments: %v", assignments)

	for _, m := range state.Assignments {
		member := api_v3.SegmentAccessItem{
			Id:   m.Id.ValueStringPointer(),
			Type: m.Type.ValueStringPointer(),
		}
		if !slices.Contains(plan.Assignments, m) {
			log.Printf("member not in assignments: %v", m)
			removals = append(removals, member)
		}
		if slices.Contains(plan.Assignments, m) {
			log.Printf("member in assignments: %v", m)
		}
	}

	err := r.client.V3.SegmentsAPI.PatchSegmentAccess(context.TODO(), state.Id.ValueString()).SegmentAccess(access).Execute()
	if err != nil {
		log.Printf("Full HTTP response: %v", err)
		return err
	}

	return nil
}

func (r *SegmentAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SegmentAccess

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = data.SegmentId

	err := createSegmentAccess(r, data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling updateSegmentAccess",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SegmentAccess

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	stateAssignments := []SegmentAccessAssignments{}

	// ACCESS_PROFILES

	forSegmentIds := data.Id.ValueString() // string | If present and not empty, additionally filters Access Profiles to those which are assigned to the Segment(s) with the specified IDs.  If segmentation is currently unavailable, specifying this parameter results in an error. (optional)
	includeUnsegmented := false            // bool | Whether or not the response list should contain unsegmented Access Profiles. If *for-segment-ids* is absent or empty, specifying *include-unsegmented* as false results in an error. (optional) (default to true)

	tflog.Info(ctx, fmt.Sprintf("forSegmentIds: %v", forSegmentIds))
	segmentAccessProfiles, httpResp, err := r.client.V3.AccessProfilesAPI.ListAccessProfiles(context.Background()).ForSegmentIds(forSegmentIds).IncludeUnsegmented(includeUnsegmented).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.ListAccessProfiles",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.ListAccessProfiles",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	for _, v := range segmentAccessProfiles {
		stateAssignments = append(stateAssignments, SegmentAccessAssignments{
			Type: types.StringValue("ACCESS_PROFILE"),
			Id:   types.StringPointerValue(v.Id),
		})
	}

	// ENTITLEMENTS
	segmentEntitlements, httpResp, err := r.client.Beta.EntitlementsAPI.ListEntitlements(context.Background()).ForSegmentIds(forSegmentIds).IncludeUnsegmented(includeUnsegmented).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.ListEntitlements",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.ListEntitlements",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	for _, v := range segmentEntitlements {
		stateAssignments = append(stateAssignments, SegmentAccessAssignments{
			Type: types.StringValue("ENTITLEMENT"),
			Id:   types.StringPointerValue(v.Id),
		})
	}

	// ROLES
	segmentRoles, httpResp, err := r.client.V3.RolesAPI.ListRoles(context.Background()).ForSegmentIds(forSegmentIds).IncludeUnsegmented(includeUnsegmented).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SV3.RolesAPI.ListRoles",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SV3.RolesAPI.ListRoles",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	for _, v := range segmentRoles {
		stateAssignments = append(stateAssignments, SegmentAccessAssignments{
			Type: types.StringValue("ROLE"),
			Id:   types.StringPointerValue(v.Id),
		})
	}

	data.Assignments = stateAssignments

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update SegmentAccess
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := updateSegmentAccess(r, plan, state)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *SegmentAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SegmentAccess

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	removals := []api_v3.SegmentAccessItem{}

	access := api_v3.SegmentAccess{
		Assignments: nil,
		Removals:    &removals,
	}

	for _, m := range state.Assignments {
		member := api_v3.SegmentAccessItem{
			Id:   m.Id.ValueStringPointer(),
			Type: m.Type.ValueStringPointer(),
		}
		removals = append(removals, member)
	}

	err := r.client.V3.SegmentsAPI.PatchSegmentAccess(context.TODO(), state.SegmentId.ValueString()).SegmentAccess(access).Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

}

func (r *SegmentAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
