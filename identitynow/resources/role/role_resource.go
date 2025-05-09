package role

import (
	"context"
	"encoding/json"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *sailpoint.APIClient
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Role Data Resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The id of the Role. This field must be left null when creating a Role, otherwise a 400 Bad Request error will result.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The human-readable display name of the Role.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A human-readable description of the Role.",
				PlanModifiers: []planmodifier.String{
					NullStringValue(),
				},
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date the Role was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the Role is enabled or not.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Identity id.",
					},
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The type of the owner, will always be `IDENTITY`.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
				Required: true,
			},
			"entitlements": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The ID of the Entitlement.",
						},
						"type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("ENTITLEMENT"),
							},
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT.",
						},
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Set{
					EmptyEntitlement(),
				},
			},
			"access_profiles": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The ID of the Entitlement.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT.",
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Set{
					EmptyAccessProfiles(),
				},
			},
			"requestable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the Role can be the target of access requests.",
			},
			"access_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"comments_required": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether the requester of the containing object must provide comments justifying the request.",
					},
					"denial_comments_required": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Whether an approver must provide comments when denying the request.",
					},
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf("APP_OWNER", "OWNER", "SOURCE_OWNER", "MANAGER", "GOVERNANCE_GROUP"),
									},
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows.\n\n **APP_OWNER**: The owner of the Application\n\n **OWNER**: Owner of the associated Access Profile or Role\n\n **SOURCE_OWNER**: Owner of the Source associated with an Access Profile\n\n **MANAGER**: Manager of the Identity making the request\n\n **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field.",
								},
								"approver_id": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP.",
								},
							},
						},
						Required:            true,
						MarkdownDescription: "List describing the steps in approving the request.",
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					EmptyAccessRequest(),
				},
			},
			"segments": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of segments IDs, if any, to which this Role is assigned.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"dimensional": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Whether the Role is dimensional.",
			},
			"revocation_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows.\n\n **APP_OWNER**: The owner of the Application\n\n **OWNER**: Owner of the associated Access Profile or Role\n\n **SOURCE_OWNER**: Owner of the Source associated with an Access Profile\n\n **MANAGER**: Manager of the Identity making the request\n\n **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field.",
								},
								"approver_id": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP.",
								},
							},
						},
						Required:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request.",
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					EmptyRevocationRequest(),
				},
			},
			"access_model_metadata": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Technical name of the Attribute. This is unique and cannot be changed after creation.",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The display name of the key.",
						},
						"multiselect": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Indicates whether the attribute can have multiple values.",
						},
						"status": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The status of the Attribute.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The type of the Attribute. This can be either `custom` or `governance`.",
						},
						"object_types": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.",
						},
						"description": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The description of the Attribute.",
						},
						"values": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"value": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Technical name of the Attribute value. This is unique and cannot be changed after creation.",
									},
									"name": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The display name of the Attribute value.",
									},
									"status": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The status of the Attribute value.",
									},
								},
							},
							Required: true,
						},
					},
				},
			},
			"membership": schema.SingleNestedAttribute{
				MarkdownDescription: "When present, specifies that the Role is to be granted to Identities which either satisfy specific criteria or to a provided list of Identities.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "This enum characterizes the type of a Role's membership selector. Only the following two are fully supported:\n\n **STANDARD**: Indicates that Role membership is defined in terms of a criteria expression\n\n **IDENTITY_LIST**: Indicates that Role membership is conferred on the specific identities listed.",
						Validators: []validator.String{
							stringvalidator.OneOf("STANDARD", "IDENTITY_LIST"),
						},
					},
					"criteria": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Default: objectdefault.StaticValue(
							types.ObjectNull(MembershipLevel1Object),
						),
						MarkdownDescription: "Defines STANDARD type Role membership.",
						Attributes: map[string]schema.Attribute{
							"operation": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
								Validators: []validator.String{
									stringvalidator.OneOf("EQUALS", "NOT_EQUALS", "CONTAINS", "STARTS_WITH", "ENDS_WITH", "AND", "OR"),
								},
							},
							"string_value": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
							},
							"key": schema.SingleNestedAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
								Default: objectdefault.StaticValue(
									types.ObjectNull(MembershipKey),
								),
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
										Validators: []validator.String{
											stringvalidator.OneOf("IDENTITY", "ACCOUNT", "ENTITLEMENT"),
										},
									},
									"property": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
									},
									"source_id": schema.StringAttribute{
										Optional:            true,
										Computed:            true,
										MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
									},
								},
							},
							"children": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"operation": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
											Validators: []validator.String{
												stringvalidator.OneOf("EQUALS", "NOT_EQUALS", "CONTAINS", "STARTS_WITH", "ENDS_WITH", "AND", "OR"),
											},
										},
										"string_value": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
										},
										"key": schema.SingleNestedAttribute{
											Optional:            true,
											Computed:            true,
											MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
											Default: objectdefault.StaticValue(
												types.ObjectNull(MembershipKey),
											),
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Required:            true,
													MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
													Validators: []validator.String{
														stringvalidator.OneOf("IDENTITY", "ACCOUNT", "ENTITLEMENT"),
													},
												},
												"property": schema.StringAttribute{
													Required:            true,
													MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
												},
												"source_id": schema.StringAttribute{
													Optional:            true,
													Computed:            true,
													MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
												},
											},
										},
										"children": schema.ListNestedAttribute{
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"operation": schema.StringAttribute{
														Required:            true,
														MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
														Validators: []validator.String{
															stringvalidator.OneOf("EQUALS", "NOT_EQUALS", "CONTAINS", "STARTS_WITH", "ENDS_WITH", "AND", "OR"),
														},
													},
													"string_value": schema.StringAttribute{
														Optional:            true,
														Computed:            true,
														MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
													},
													"key": schema.SingleNestedAttribute{
														Optional:            true,
														Computed:            true,
														MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
														Default: objectdefault.StaticValue(
															types.ObjectNull(MembershipKey),
														),
														Attributes: map[string]schema.Attribute{
															"type": schema.StringAttribute{
																Required:            true,
																MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
																Validators: []validator.String{
																	stringvalidator.OneOf("IDENTITY", "ACCOUNT", "ENTITLEMENT"),
																},
															},
															"property": schema.StringAttribute{
																Required:            true,
																MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
															},
															"source_id": schema.StringAttribute{
																Optional:            true,
																MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
															},
														},
													},
												},
											},
											Optional:            true,
											MarkdownDescription: "List describing the steps in approving the revocation request.",
										},
									},
								},
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "List describing the steps in approving the revocation request.",
							},
						},
					},
					"identities": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "An enumeration of the types of DTOs supported within the IdentityNow infrastructure.",
									Validators: []validator.String{
										stringvalidator.OneOf("ACCOUNT_CORRELATION_CONFIG", "ACCESS_PROFILE", "ACCESS_REQUEST_APPROVAL", "ACCOUNT", "APPLICATION", "CAMPAIGN", "CAMPAIGN_FILTER", "CERTIFICATION", "CLUSTER", "CONNECTOR_SCHEMA", "ENTITLEMENT", "GOVERNANCE_GROUP", "IDENTITY", "IDENTITY_PROFILE", "IDENTITY_REQUEST", "MACHINE_IDENTITY", "LIFECYCLE_STATE", "PASSWORD_POLICY", "ROLE", "RULE", "SOD_POLICY", "SOURCE", "TAG", "TAG_CATEGORY", "TASK_RESULT", "REPORT_RESULT", "SOD_VIOLATION", "ACCOUNT_ACTIVITY", "WORKGROUP"),
									},
								},
								"id": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Identity id.",
								},
								"name": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Human-readable display name of the Identity.",
								},
								"alias_name": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "User name of the Identity.",
								},
							},
						},
						Optional:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request.",
					},
				},
				Optional: true,
				Computed: true,
				Default: objectdefault.StaticValue(
					types.ObjectNull(MembershipType),
				),
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Role

	tflog.Info(ctx, fmt.Sprintf("create data123`: %v", data))

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	roleRequest := convertRoleV3(&data)

	marshaled, err := json.MarshalIndent(roleRequest, "", "   ")
	if err != nil {
		tflog.Info(ctx, fmt.Sprintf("marshaling error: %s", err))
	}

	tflog.Info(ctx, fmt.Sprintf("create data marshaled`: %v", string(marshaled)))

	tflog.Info(ctx, fmt.Sprintf("roleRequest`: %v", roleRequest))
	role, httpResp, err := r.client.V3.RolesAPI.CreateRole(ctx).Role(roleRequest).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesApi.CreateRole",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesApi.CreateRole",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	tflog.Info(ctx, fmt.Sprintf("Response from `RolesApi.CreateRole`: %v", role))

	parseAttributes(&data, role, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Role

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	role, httpResp, err := r.client.V3.RolesAPI.GetRole(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesApi.GetRole",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesApi.GetRole",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	tflog.Info(ctx, fmt.Sprintf("Response from `RolesApi.GetRole`: %v", resp))

	parseAttributes(&data, role, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update Role
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planAp := convertRoleV3(&plan)
	stateAp := convertRoleV3(&state)

	jsonPatchOperation := []api_v3.JsonPatchOperation{}

	patch, err := jsondiff.Compare(stateAp, planAp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	for _, p := range patch {
		jsonPatch := *api_v3.NewJsonPatchOperationWithDefaults()

		// description uniqueness
		if p.Path == "/description" && p.Value == nil {
			p.Value = ""
		}

		op, err := p.MarshalJSON()
		if err != nil {
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		jsonPatch.UnmarshalJSON(op)

		jsonPatchOperation = append(jsonPatchOperation, jsonPatch)
	}

	marshaled, err := json.MarshalIndent(jsonPatchOperation, "", "  ")
	if err != nil {
		tflog.Info(ctx, fmt.Sprintf("marshaling error: %s", err))
	}

	tflog.Debug(ctx, fmt.Sprintf("role marshaled: %v", string(marshaled)))

	role, httpResp, err := r.client.V3.RolesAPI.PatchRole(ctx, state.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesAPI.PatchRole",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesAPI.PatchRole",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	parseAttributes(&update, role, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Role

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.V3.RolesAPI.DeleteRole(ctx, state.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesAPI.DeleteRole",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.RolesAPI.DeleteRole",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// emptyAccessRequest implements the plan modifier.
type emptyAccessRequest struct{}

// Description returns a human-readable description of the plan modifier.
func (m emptyAccessRequest) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m emptyAccessRequest) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifyObject implements the plan modification logic.
func (m emptyAccessRequest) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {

	if req.PlanValue.IsUnknown() {
		var targetValues []attr.Value
		resp.PlanValue = basetypes.NewObjectValueMust(resp.PlanValue.AttributeTypes(ctx), map[string]attr.Value{
			"approval_schemes":         basetypes.NewListValueMust(ApprovalSchemeObject, targetValues),
			"denial_comments_required": basetypes.NewBoolValue(false),
			"comments_required":        basetypes.NewBoolValue(false),
		})
	}

}

func EmptyAccessRequest() planmodifier.Object {
	return emptyAccessRequest{}
}

// emptyRevocationRequest implements the plan modifier.
type emptyRevocationRequest struct{}

// Description returns a human-readable description of the plan modifier.
func (m emptyRevocationRequest) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m emptyRevocationRequest) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifyObject implements the plan modification logic.
func (m emptyRevocationRequest) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {

	if req.PlanValue.IsUnknown() {
		var targetValues []attr.Value

		resp.PlanValue = basetypes.NewObjectValueMust(resp.PlanValue.AttributeTypes(ctx), map[string]attr.Value{
			"approval_schemes": basetypes.NewListValueMust(ApprovalSchemeObject, targetValues),
		})
	}

}

func EmptyRevocationRequest() planmodifier.Object {
	return emptyRevocationRequest{}
}

// emptyEntitlement implements the plan modifier.
type emptyEntitlement struct{}

// Description returns a human-readable description of the plan modifier.
func (m emptyEntitlement) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m emptyEntitlement) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifySet implements the plan modification logic.
func (m emptyEntitlement) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {

	if req.PlanValue.IsUnknown() {
		var targetValues []attr.Value

		resp.PlanValue = basetypes.NewSetValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"type": types.StringType,
				"id":   types.StringType,
			},
		}, targetValues)
	}
}

func EmptyEntitlement() planmodifier.Set {
	return emptyEntitlement{}
}

// nullStringValue implements the plan modifier.
type nullStringValue struct{}

// Description returns a human-readable description of the plan modifier.
func (m nullStringValue) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m nullStringValue) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifyString implements the plan modification logic.
func (m nullStringValue) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsUnknown() {
		resp.PlanValue = basetypes.NewStringNull()
	}
}

func NullStringValue() planmodifier.String {
	return nullStringValue{}
}

// emptyAccessProfiles implements the plan modifier.
type emptyAccessProfiles struct{}

// Description returns a human-readable description of the plan modifier.
func (m emptyAccessProfiles) Description(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m emptyAccessProfiles) MarkdownDescription(_ context.Context) string {
	return "Only trigger on new keys and/or value changes"
}

// PlanModifySet implements the plan modification logic.
func (m emptyAccessProfiles) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {

	if req.PlanValue.IsUnknown() {
		var targetValues []attr.Value

		resp.PlanValue = basetypes.NewSetValueMust(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"type": types.StringType,
				"id":   types.StringType,
				"name": types.StringType,
			},
		}, targetValues)
	}
}

func EmptyAccessProfiles() planmodifier.Set {
	return emptyAccessProfiles{}
}
