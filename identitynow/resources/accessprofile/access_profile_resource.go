package accessprofile

import (
	"context"
	"fmt"
	"net/http"

	sailpoint "github.com/davidsonjon/golang-sdk"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	v3 "github.com/davidsonjon/golang-sdk/v3"
	"github.com/wI2L/jsondiff"
)

var _ resource.Resource = &AccessProfileResource{}
var _ resource.ResourceWithImportState = &AccessProfileResource{}

func NewAccessProfileResource() resource.Resource {
	return &AccessProfileResource{}
}

type AccessProfileResource struct {
	client *sailpoint.APIClient
}

func (r *AccessProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_profile"
}

func (r *AccessProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attr := map[string]attr.Type{}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Access Profile resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the Access Profile",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the Access Profile",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Information about the Access Profile",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date the Access Profile was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// "modified": schema.StringAttribute{
			// 	Computed:            true,
			// 	MarkdownDescription: "Date the Access Profile was last modified.",
			// 	PlanModifiers: []planmodifier.String{
			// 		stringplanmodifier.UseStateForUnknown(),
			// 	},
			// },
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the Access Profile is enabled. If the Access Profile is enabled then you must include at least one Entitlement.",
			},
			"requestable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the Access Profile is requestable via access request. Currently, making an Access Profile non-requestable is only supported for customers enabled with the new Request Center. Otherwise, attempting to create an Access Profile with a value false in this field results in a 400 error.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Identity id",
					},
					"name": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
					},
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("IDENTITY"),
						},
						MarkdownDescription: "The type of the Source, will always be `IDENTITY`",
					},
				},
				Required: true,
			},
			"source": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The ID of the Source with with which the Access Profile is associated",
					},
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The display name of the associated Source",
					},
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("SOURCE"),
						},
						MarkdownDescription: "The type of the Source, will always be SOURCE",
					},
				},
				Required: true,
			},
			"entitlements": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The ID of the Entitlement",
						},
						"type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("ENTITLEMENT"),
							},
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT",
						},
					},
				},
				Required: true,
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
				Required: true,
			},
			"revocation_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"comments_required": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Whether the requester of the containing object must provide comments justifying the request",
					},
					"denial_comments_required": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Whether an approver must provide comments when denying the request",
					},
					"approval_schemes": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows. **APP_OWNER**: The owner of the Application  **OWNER**: Owner of the associated Access Profile or Role  **SOURCE_OWNER**: Owner of the Source associated with an Access Profile  **MANAGER**: Manager of the Identity making the request  **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field",
									Validators: []validator.String{
										stringvalidator.OneOf("APP_OWNER", "OWNER", "SOURCE_OWNER", "MANAGER", "GOVERNANCE_GROUP"),
									},
								},
								"approver_id": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP",
								},
							},
						},
						Optional:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request",
					},
				},
				Optional: true,
				Computed: true,
				Default: objectdefault.StaticValue(
					types.ObjectNull(attr),
				),
			},
		},
	}
}

func (r *AccessProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *AccessProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessProfile

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	accessProfile := convertAccessProfileV3(&data)

	ap, httpResp, err := r.client.V3.AccessProfilesApi.CreateAccessProfile(ctx).AccessProfile(*accessProfile).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.CreateAccessProfile",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.CreateAccessProfile",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.parseComputedAttributes(ap)
	data.parseConfiguredAttributes(ap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessProfile

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ap, httpResp, err := r.client.V3.AccessProfilesApi.GetAccessProfile(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			resp.Diagnostics.AddWarning(
				"Error when calling GetAccessProfile",
				fmt.Sprintf("AccessProfile with id:%s is not found. Removing from state.",
					data.Id.ValueString()))
			resp.State.RemoveResource(ctx)
		} else {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling V3.AccessProfilesApi.GetAccessProfile",
					fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling V3.AccessProfilesApi.GetAccessProfile",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}

			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		}
		return
	}

	data.parseComputedAttributes(ap)
	data.parseConfiguredAttributes(ap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update AccessProfile
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planAp := convertAccessProfileV3(&plan)
	stateAp := convertAccessProfileV3(&state)

	jsonPatchOperation := []v3.JsonPatchOperation{}

	patch, err := jsondiff.Compare(stateAp, planAp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)

		return
	}

	for _, p := range patch {
		patch := *v3.NewJsonPatchOperationWithDefaults()

		op, err := p.MarshalJSON()
		if err != nil {
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %T, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	ap, httpResp, err := r.client.V3.AccessProfilesApi.PatchAccessProfile(ctx, state.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.PatchAccessProfile",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.PatchAccessProfile",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	update.parseComputedAttributes(ap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *AccessProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccessProfile

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessProfileBulkDeleteRequest := *v3.NewAccessProfileBulkDeleteRequest() // AccessProfileBulkDeleteRequest |
	accessProfileBulkDeleteRequest.BestEffortOnly = v3.PtrBool(false)
	accessProfileBulkDeleteRequest.AccessProfileIds = append(accessProfileBulkDeleteRequest.AccessProfileIds, state.Id.ValueString())

	_, httpResp, err := r.client.V3.AccessProfilesApi.DeleteAccessProfilesInBulk(ctx).AccessProfileBulkDeleteRequest(accessProfileBulkDeleteRequest).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.DeleteAccessProfilesInBulk",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.DeleteAccessProfilesInBulk",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}
}

func (r *AccessProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
