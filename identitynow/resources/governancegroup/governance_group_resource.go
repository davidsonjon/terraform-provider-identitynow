package governancegroup

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"
)

var _ resource.Resource = &GovernanceGroupResource{}
var _ resource.ResourceWithImportState = &GovernanceGroupResource{}

func NewGovernanceGroupResource() resource.Resource {
	return &GovernanceGroupResource{}
}

type GovernanceGroupResource struct {
	client *sailpoint.APIClient
}

func (r *GovernanceGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group"
}

func (r *GovernanceGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {

	resp.Schema = schema.Schema{
		MarkdownDescription: "Governance Group resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Governance group ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Governance group name.",
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Governance group description.",
			},
			"member_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the governance group.",
			},
			"connection_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of connections in the governance group.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Owner's DTO type.",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner's identity ID.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner's display name.",
					},
				},
				Required:            true,
				MarkdownDescription: "Owner",
			},
			"membership": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Identity's DTO type.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Identity ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Identity's display name.",
						},
					},
				},
				Required:            true,
				MarkdownDescription: "Membership",
			},
		},
	}
}

func (r *GovernanceGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *GovernanceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkgroupDto

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	workgroupDto := *api_beta.NewWorkgroupDto() // WorkgroupDto |
	workgroupDto.Name = data.Name.ValueStringPointer()
	workgroupDto.Description = data.Description.ValueStringPointer()

	var propsState *BaseReferenceDto1
	resp.Diagnostics.Append(data.Owner.As(ctx, &propsState, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupDto.Owner = &api_beta.WorkgroupDtoOwner{
		Name: propsState.Name.ValueStringPointer(),
		Id:   propsState.Id.ValueStringPointer(),
		Type: propsState.Type.ValueStringPointer(),
	}

	workgroup, httpResp, err := r.client.Beta.GovernanceGroupsAPI.CreateWorkgroup(ctx).WorkgroupDto(workgroupDto).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.CreateWorkgroup",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.CreateWorkgroup",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	bulkWorkgroupMembersRequestInner := []api_beta.BulkWorkgroupMembersRequestInner{}
	base := []BaseReferenceDto1{}
	data.Membership.ElementsAs(ctx, &base, false)

	for _, m := range base {
		member := api_beta.BulkWorkgroupMembersRequestInner{
			Id:   m.Id.ValueStringPointer(),
			Name: m.Name.ValueStringPointer(),
			Type: m.Id.ValueStringPointer(),
		}
		bulkWorkgroupMembersRequestInner = append(bulkWorkgroupMembersRequestInner, member)
	}

	_, httpResp, err = r.client.Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers(ctx, *workgroup.Id).BulkWorkgroupMembersRequestInner(bulkWorkgroupMembersRequestInner).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Id = types.StringPointerValue(workgroup.Id)
	data.Name = types.StringPointerValue(workgroup.Name)
	data.Description = types.StringPointerValue(workgroup.Description)
	data.MemberCount = types.Int64PointerValue(workgroup.MemberCount)
	data.ConnectionCount = types.Int64PointerValue(workgroup.ConnectionCount)

	workgroupMembers, httpResp, err := r.client.Beta.GovernanceGroupsAPI.ListWorkgroupMembers(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	elements := []attr.Value{}
	for _, v := range workgroupMembers {
		member, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
			"name": types.StringPointerValue(v.Name),
			"id":   types.StringPointerValue(v.Id),
			"type": types.StringPointerValue((*string)(v.Type)),
		})
		if ok.HasError() {
			resp.Diagnostics.Append(ok...)
		}

		elements = append(elements, member)
	}

	listValue := types.SetValueMust(types.ObjectType{AttrTypes: baseReferenceDto1Types}, elements)

	data.Membership = listValue

	owner, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
		"name": types.StringPointerValue(workgroup.Owner.Name),
		"id":   types.StringPointerValue(workgroup.Owner.Id),
		"type": types.StringPointerValue((*string)(workgroup.Owner.Type)),
	})
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	data.Owner = owner

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GovernanceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkgroupDto

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	workgroup, httpResp, err := r.client.Beta.GovernanceGroupsAPI.GetWorkgroup(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			resp.Diagnostics.AddWarning(
				"Error when calling Beta.GovernanceGroupsAPI.GetWorkgroup",
				fmt.Sprintf("GovernanceGroup with id:%s is not found. Removing from state.",
					data.Id.ValueString()))
			resp.State.RemoveResource(ctx)
		} else {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.GetWorkgroup",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.GetWorkgroup",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Id = types.StringPointerValue(workgroup.Id)
	data.Name = types.StringPointerValue(workgroup.Name)
	data.Description = types.StringPointerValue(workgroup.Description)
	data.MemberCount = types.Int64PointerValue(workgroup.MemberCount)
	data.ConnectionCount = types.Int64PointerValue(workgroup.ConnectionCount)

	workgroupMembers, httpResp, err := r.client.Beta.GovernanceGroupsAPI.ListWorkgroupMembers(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	elements := []attr.Value{}
	for _, v := range workgroupMembers {
		member, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
			"name": types.StringPointerValue(v.Name),
			"id":   types.StringPointerValue(v.Id),
			"type": types.StringPointerValue((*string)(v.Type)),
		})
		if ok.HasError() {
			resp.Diagnostics.Append(ok...)
		}

		elements = append(elements, member)
	}

	listValue := types.SetValueMust(types.ObjectType{AttrTypes: baseReferenceDto1Types}, elements)

	data.Membership = listValue
	if workgroup.Owner != nil {
		owner, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
			"name": types.StringPointerValue(workgroup.Owner.Name),
			"id":   types.StringPointerValue(workgroup.Owner.Id),
			"type": types.StringPointerValue((*string)(workgroup.Owner.Type)),
		})
		if ok.HasError() {
			resp.Diagnostics.Append(ok...)
		}
		data.Owner = owner
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GovernanceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update WorkgroupDto

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planEnt := convertWorkgroupBeta(ctx, &plan)
	stateEnt := convertWorkgroupBeta(ctx, &state)

	jsonPatchOperation := []api_beta.JsonPatchOperation{}

	patch, err := jsondiff.Compare(stateEnt, planEnt)
	if err != nil {
		// handle error
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	for _, p := range patch {
		patch := *api_beta.NewJsonPatchOperationWithDefaults()

		op, err := p.MarshalJSON()
		if err != nil {
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		tflog.Info(ctx, fmt.Sprintf("op: %v", string(op)))
		patch.UnmarshalJSON(op)
		tflog.Info(ctx, fmt.Sprintf("patch: %v", patch))
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	workgroup, httpResp, err := r.client.Beta.GovernanceGroupsAPI.PatchWorkgroup(ctx, state.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.PatchWorkgroup",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.PatchWorkgroup",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	bulkWorkgroupMembersRequestInner := []api_beta.BulkWorkgroupMembersRequestInner{}
	bulkWorkgroupMembersRequestInnerRemove := []api_beta.BulkWorkgroupMembersRequestInner{}
	stateMembership := []BaseReferenceDto1{}
	state.Membership.ElementsAs(ctx, &stateMembership, false)
	planMembership := []BaseReferenceDto1{}
	plan.Membership.ElementsAs(ctx, &planMembership, false)

	for _, m := range planMembership {
		member := api_beta.BulkWorkgroupMembersRequestInner{
			Id:   m.Id.ValueStringPointer(),
			Name: m.Name.ValueStringPointer(),
			Type: m.Id.ValueStringPointer(),
		}
		if !slices.Contains(stateMembership, m) {
			bulkWorkgroupMembersRequestInner = append(bulkWorkgroupMembersRequestInner, member)
		}
	}

	if len(bulkWorkgroupMembersRequestInner) > 0 {
		_, httpResp, err = r.client.Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers(ctx, *workgroup.Id).BulkWorkgroupMembersRequestInner(bulkWorkgroupMembersRequestInner).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.UpdateWorkgroupMembers",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}
	}

	for _, m := range stateMembership {
		member := api_beta.BulkWorkgroupMembersRequestInner{
			Id:   m.Id.ValueStringPointer(),
			Name: m.Name.ValueStringPointer(),
			Type: m.Id.ValueStringPointer(),
		}
		if !slices.Contains(planMembership, m) {
			bulkWorkgroupMembersRequestInnerRemove = append(bulkWorkgroupMembersRequestInnerRemove, member)
		}
	}

	if len(bulkWorkgroupMembersRequestInnerRemove) > 0 {
		_, httpResp, err = r.client.Beta.GovernanceGroupsAPI.DeleteWorkgroupMembers(ctx, *workgroup.Id).BulkWorkgroupMembersRequestInner(bulkWorkgroupMembersRequestInnerRemove).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.DeleteWorkgroupMembers",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.DeleteWorkgroupMembers",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}
	}

	update.Id = types.StringPointerValue(workgroup.Id)
	update.Name = types.StringPointerValue(workgroup.Name)
	update.Description = types.StringPointerValue(workgroup.Description)
	update.MemberCount = types.Int64PointerValue(workgroup.MemberCount)
	update.ConnectionCount = types.Int64PointerValue(workgroup.ConnectionCount)

	workgroupMembers, httpResp, err := r.client.Beta.GovernanceGroupsAPI.ListWorkgroupMembers(ctx, state.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	elements := []attr.Value{}
	for _, v := range workgroupMembers {
		member, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
			"name": types.StringPointerValue(v.Name),
			"id":   types.StringPointerValue(v.Id),
			"type": types.StringPointerValue((*string)(v.Type)),
		})
		if ok.HasError() {
			resp.Diagnostics.Append(ok...)
		}

		elements = append(elements, member)
	}

	listValue := types.SetValueMust(types.ObjectType{AttrTypes: baseReferenceDto1Types}, elements)

	update.Membership = listValue

	owner, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
		"name": types.StringPointerValue(workgroup.Owner.Name),
		"id":   types.StringPointerValue(workgroup.Owner.Id),
		"type": types.StringPointerValue((*string)(workgroup.Owner.Type)),
	})
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	update.Owner = owner

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *GovernanceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WorkgroupDto

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupBulkDeleteRequest := *api_beta.NewWorkgroupBulkDeleteRequest()
	workgroupBulkDeleteRequest.Ids = []string{state.Id.ValueString()}

	_, httpResp, err := r.client.Beta.GovernanceGroupsAPI.DeleteWorkgroupsInBulk(ctx).WorkgroupBulkDeleteRequest(workgroupBulkDeleteRequest).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.DeleteWorkgroupsInBulk",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.DeleteWorkgroupsInBulk",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

}

func (r *GovernanceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
