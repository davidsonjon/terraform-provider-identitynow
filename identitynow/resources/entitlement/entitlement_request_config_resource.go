package entitlement

import (
	"context"
	"fmt"
	"log"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/golang-sdk/v2/api_beta"
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

var _ resource.Resource = &EntitlementRequestConfigResource{}
var _ resource.ResourceWithImportState = &EntitlementRequestConfigResource{}

func NewEntitlementRequestConfigResource() resource.Resource {
	return &EntitlementRequestConfigResource{}
}

type EntitlementRequestConfigResource struct {
	client *sailpoint.APIClient
}

func (r *EntitlementRequestConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement_request_config"
}

func (r *EntitlementRequestConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlement resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The entitlement id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"comments_required": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether the requester of the containing object must provide comments justifying the request",
					},
					"denial_comments_required": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether an approver must provide comments when denying the request",
					},
					"approval_schemes": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf("APP_OWNER", "OWNER", "SOURCE_OWNER", "MANAGER", "GOVERNANCE_GROUP"),
									},
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows. **APP_OWNER**: The owner of the Application  **OWNER**: Owner of the associated Access Profile or Role  **SOURCE_OWNER**: Owner of the Source associated with an Access Profile  **MANAGER**: Manager of the Identity making the request  **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field",
								},
								"approver_id": schema.StringAttribute{
									Optional:            true,
									Computed:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP",
								},
							},
						},
						Required:            true,
						MarkdownDescription: "List describing the steps in approving the request",
					},
				},
				Optional: true,
			},
		},
	}
}

func (r *EntitlementRequestConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EntitlementRequestConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EntitlementRequestConfig

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqConfig := api_beta.NewEntitlementRequestConfigWithDefaults()

	reqConfig.AccessRequestConfig = &api_beta.EntitlementAccessRequestConfig{}
	reqConfig.AccessRequestConfig.RequestCommentRequired = data.AccessRequestConfig.CommentsRequired.ValueBoolPointer()
	reqConfig.AccessRequestConfig.DenialCommentRequired = data.AccessRequestConfig.DenialCommentsRequired.ValueBoolPointer()

	for _, ar := range data.AccessRequestConfig.ApprovalSchemes {
		as := api_beta.EntitlementApprovalScheme{}
		arId := api_beta.NullableString{}
		arId.Set(ar.ApproverId.ValueStringPointer())

		as.ApproverType = ar.ApproverType.ValueStringPointer()
		as.ApproverId = arId

		reqConfig.AccessRequestConfig.ApprovalSchemes = append(reqConfig.AccessRequestConfig.ApprovalSchemes, as)
	}

	tflog.Info(ctx, fmt.Sprintf("*reqConfig.AccessRequestConfig.ApprovalSchemes: %v", reqConfig.AccessRequestConfig.ApprovalSchemes))

	requestConfig, httpResp, err := r.client.Beta.EntitlementsAPI.PutEntitlementRequestConfig(context.Background(), data.Id.ValueString()).EntitlementRequestConfig(*reqConfig).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	reqConfigData := &Requestability{}

	reqConfigData.CommentsRequired = types.BoolPointerValue(requestConfig.AccessRequestConfig.RequestCommentRequired)
	reqConfigData.DenialCommentsRequired = types.BoolPointerValue(requestConfig.AccessRequestConfig.DenialCommentRequired)

	for _, a := range requestConfig.AccessRequestConfig.ApprovalSchemes {
		approval := AccessProfileApprovalScheme{
			ApproverType: types.StringPointerValue(a.ApproverType),
		}
		appId, err := a.ApproverId.MarshalJSON()
		if err != nil {
			log.Printf("error ApproverId.MarshalJSON: %+v\n", a.ApproverId)
		}
		if appId != nil {
			approval.ApproverId = types.StringPointerValue(a.ApproverId.Get())
		} else {
			approval.ApproverId = types.StringNull()
		}

		reqConfigData.ApprovalSchemes = append(reqConfigData.ApprovalSchemes, approval)
	}

	data.AccessRequestConfig = reqConfigData

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EntitlementRequestConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EntitlementRequestConfig

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	reqConfigResp, httpResp, err := r.client.Beta.EntitlementsAPI.GetEntitlementRequestConfig(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {

		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlementRequestConfig",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlementRequestConfig",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	reqConfig := &Requestability{}

	reqConfig.CommentsRequired = types.BoolPointerValue(reqConfigResp.AccessRequestConfig.RequestCommentRequired)
	reqConfig.DenialCommentsRequired = types.BoolPointerValue(reqConfigResp.AccessRequestConfig.DenialCommentRequired)

	for _, a := range reqConfigResp.AccessRequestConfig.ApprovalSchemes {
		approval := AccessProfileApprovalScheme{
			ApproverType: types.StringPointerValue(a.ApproverType),
			// ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
		}
		appId, err := a.ApproverId.MarshalJSON()
		if err != nil {
			log.Printf("error ApproverId.MarshalJSON: %+v\n", a.ApproverId)
		}
		if appId != nil {
			approval.ApproverId = types.StringPointerValue(a.ApproverId.Get())
		} else {
			approval.ApproverId = types.StringNull()
		}

		reqConfig.ApprovalSchemes = append(reqConfig.ApprovalSchemes, approval)
	}

	data.AccessRequestConfig = reqConfig

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EntitlementRequestConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update EntitlementRequestConfig
	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	reqConfig := api_beta.NewEntitlementRequestConfigWithDefaults()

	reqConfig.AccessRequestConfig = &api_beta.EntitlementAccessRequestConfig{}
	reqConfig.AccessRequestConfig.RequestCommentRequired = plan.AccessRequestConfig.CommentsRequired.ValueBoolPointer()
	reqConfig.AccessRequestConfig.DenialCommentRequired = plan.AccessRequestConfig.DenialCommentsRequired.ValueBoolPointer()

	for _, ar := range plan.AccessRequestConfig.ApprovalSchemes {
		as := api_beta.EntitlementApprovalScheme{}
		arId := api_beta.NullableString{}
		arId.Set(ar.ApproverId.ValueStringPointer())

		as.ApproverType = ar.ApproverType.ValueStringPointer()
		as.ApproverId = arId

		reqConfig.AccessRequestConfig.ApprovalSchemes = append(reqConfig.AccessRequestConfig.ApprovalSchemes, as)
	}

	tflog.Info(ctx, fmt.Sprintf("*reqConfig.AccessRequestConfig.ApprovalSchemes: %v", reqConfig.AccessRequestConfig.ApprovalSchemes))

	requestConfig, httpResp, err := r.client.Beta.EntitlementsAPI.PutEntitlementRequestConfig(context.Background(), plan.Id.ValueString()).EntitlementRequestConfig(*reqConfig).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	reqConfigData := &Requestability{}

	reqConfigData.CommentsRequired = types.BoolPointerValue(requestConfig.AccessRequestConfig.RequestCommentRequired)
	reqConfigData.DenialCommentsRequired = types.BoolPointerValue(requestConfig.AccessRequestConfig.DenialCommentRequired)

	for _, a := range requestConfig.AccessRequestConfig.ApprovalSchemes {
		approval := AccessProfileApprovalScheme{
			ApproverType: types.StringPointerValue(a.ApproverType),
		}
		appId, err := a.ApproverId.MarshalJSON()
		if err != nil {
			log.Printf("error ApproverId.MarshalJSON: %+v\n", a.ApproverId)
		}
		if appId != nil {
			approval.ApproverId = types.StringPointerValue(a.ApproverId.Get())
		} else {
			approval.ApproverId = types.StringNull()
		}

		reqConfigData.ApprovalSchemes = append(reqConfigData.ApprovalSchemes, approval)
	}

	update.AccessRequestConfig = reqConfigData

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *EntitlementRequestConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EntitlementRequestConfig

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqConfig := api_beta.NewEntitlementRequestConfigWithDefaults()

	reqConfig.AccessRequestConfig = &api_beta.EntitlementAccessRequestConfig{}
	falseBool := false
	reqConfig.AccessRequestConfig.RequestCommentRequired = &falseBool
	reqConfig.AccessRequestConfig.DenialCommentRequired = &falseBool
	reqConfig.AccessRequestConfig.ApprovalSchemes = []api_beta.EntitlementApprovalScheme{}

	tflog.Info(ctx, fmt.Sprintf("*reqConfig.AccessRequestConfig.ApprovalSchemes: %v", reqConfig.AccessRequestConfig.ApprovalSchemes))

	_, httpResp, err := r.client.Beta.EntitlementsAPI.PutEntitlementRequestConfig(context.Background(), state.Id.ValueString()).EntitlementRequestConfig(*reqConfig).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *EntitlementRequestConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
