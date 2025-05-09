package role

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/metadataattribute"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &RoleDataSource{}

func NewRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

type RoleDataSource struct {
	client *sailpoint.APIClient
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Role Data Resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The id of the Role. This field must be left null when creating a Role, otherwise a 400 Bad Request error will result.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The human-readable display name of the Role.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A human-readable description of the Role.",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date the Role was created.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the Role is enabled or not.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Identity id.",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The type of the owner, will always be `IDENTITY`.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
					},
				},
				Computed: true,
			},
			"entitlements": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the Entitlement.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT.",
						},
						// "name": schema.StringAttribute{
						// 	Computed: true,
						// },
					},
				},
				Computed: true,
			},
			"access_profiles": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the Entitlement.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT.",
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				Computed: true,
			},
			"requestable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the Role can be the target of access requests.",
			},
			"access_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"comments_required": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether the requester of the containing object must provide comments justifying the request.",
					},
					"denial_comments_required": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether an approver must provide comments when denying the request.",
					},
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows.\n\n **APP_OWNER**: The owner of the Application\n\n **OWNER**: Owner of the associated Access Profile or Role\n\n **SOURCE_OWNER**: Owner of the Source associated with an Access Profile\n\n **MANAGER**: Manager of the Identity making the request\n\n **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field.",
								},
								"approver_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP.",
								},
							},
						},
						Computed:            true,
						MarkdownDescription: "List describing the steps in approving the request.",
					},
				},
				Computed: true,
			},
			"segments": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of segments IDs, if any, to which this Role is assigned.",
			},
			"dimensional": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the Role is dimensional.",
			},
			"revocation_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows.\n\n **APP_OWNER**: The owner of the Application\n\n **OWNER**: Owner of the associated Access Profile or Role\n\n **SOURCE_OWNER**: Owner of the Source associated with an Access Profile\n\n **MANAGER**: Manager of the Identity making the request\n\n **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field.",
								},
								"approver_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP.",
								},
							},
						},
						Computed:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request.",
					},
				},
				Computed: true,
			},
			"access_model_metadata": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Technical name of the Attribute. This is unique and cannot be changed after creation.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The display name of the key.",
						},
						"multiselect": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Indicates whether the attribute can have multiple values.",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The status of the Attribute.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the Attribute. This can be either `custom` or `governance`.",
						},
						"object_types": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The description of the Attribute.",
						},
						"values": schema.ListNestedAttribute{
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"value": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Technical name of the Attribute value. This is unique and cannot be changed after creation.",
									},
									"name": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The display name of the Attribute value.",
									},
									"status": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The status of the Attribute value.",
									},
								},
							},
							Computed: true,
						},
					},
				},
			},
			"membership": schema.SingleNestedAttribute{
				MarkdownDescription: "When present, specifies that the Role is to be granted to Identities which either satisfy specific criteria or to a provided list of Identities.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "This enum characterizes the type of a Role's membership selector. Only the following two are fully supported:\n\n **STANDARD**: Indicates that Role membership is defined in terms of a criteria expression\n\n **IDENTITY_LIST**: Indicates that Role membership is conferred on the specific identities listed.",
					},
					"criteria": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"operation": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Defines STANDARD type Role membership.",
							},
							"string_value": schema.StringAttribute{
								MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
								Computed:            true,
							},
							"key": schema.SingleNestedAttribute{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
									},
									"property": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
									},
									"source_id": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
									},
								},
								Computed:            true,
								MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
							},
							"children": schema.ListNestedAttribute{
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"operation": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
										},
										"string_value": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
										},
										"key": schema.SingleNestedAttribute{
											Attributes: map[string]schema.Attribute{
												"type": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
												},
												"property": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
												},
												"source_id": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
												},
											},
											Computed:            true,
											MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
										},
										"children": schema.ListNestedAttribute{
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"operation": schema.StringAttribute{
														Computed:            true,
														MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
													},
													"string_value": schema.StringAttribute{
														Computed:            true,
														MarkdownDescription: "An operation (EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, ENDS_WITH, AND, OR).",
													},
													"key": schema.SingleNestedAttribute{
														Attributes: map[string]schema.Attribute{
															"type": schema.StringAttribute{
																Computed:            true,
																MarkdownDescription: "String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.",
															},
															"property": schema.StringAttribute{
																Computed:            true,
																MarkdownDescription: "Refers to a specific Identity attribute, Account attribute, or Entitlement used in Role membership criteria.",
															},
															"source_id": schema.StringAttribute{
																Computed:            true,
																MarkdownDescription: "Indicates whether the associated criteria represents an expression on identity attributes, account attributes, or entitlements, respectively.",
															},
														},
														Computed:            true,
														MarkdownDescription: "The name of the attribute or entitlement to which the associated criteria applies.",
													},
												},
											},
											MarkdownDescription: "ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT.",
											Computed:            true,
										},
									},
								},
								Computed:            true,
								MarkdownDescription: "List describing the steps in approving the revocation request.",
							},
						},
						Computed:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request.",
					},
					"identities": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "An enumeration of the types of DTOs supported within the IdentityNow infrastructure.",
								},
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Identity id.",
								},
								"name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Human-readable display name of the Identity.",
								},
								"alias_name": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "User name of the Identity.",
								},
							},
						},
						MarkdownDescription: "List describing the steps in approving the revocation request.",
						Computed:            true,
					},
				},
				Computed: true,
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

	d.client = config.APIClient
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Role

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	role, httpResp, err := d.client.V3.RolesAPI.GetRole(ctx, data.Id.ValueString()).Execute()
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

	tflog.Info(ctx, fmt.Sprintf("Response from `RolesApi.GetRole`: %v", httpResp))

	parseAttributes(&data, role, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func parseAttributes(role *Role, v3Role *api_v3.Role, diags *diag.Diagnostics) {
	role.Id = types.StringValue(*v3Role.Id)
	role.Name = types.StringValue(v3Role.Name)
	if !(v3Role.Description.Get() == nil) {
		role.Description = types.StringPointerValue(v3Role.Description.Get())
	} else {
		role.Description = basetypes.NewStringNull()
	}
	role.Created = types.StringValue(v3Role.Created.String())
	role.Enabled = types.BoolValue(*v3Role.Enabled)
	role.Requestable = types.BoolPointerValue(v3Role.Requestable)
	role.Dimensional = types.BoolPointerValue(v3Role.Dimensional.Get())

	owner := OwnerReference{}
	if v3Role.Owner.HasId() {
		owner.Id = types.StringPointerValue(v3Role.Owner.Id)
		owner.Name = types.StringPointerValue(v3Role.Owner.Name)
		owner.Type = types.StringPointerValue(v3Role.Owner.Type)
	}
	role.Owner = &owner

	role.Entitlements = []EntitlementRef{}

	for _, e := range v3Role.Entitlements {
		entitlement := EntitlementRef{
			// Name: types.StringPointerValue(e.Name),
			Id:   types.StringPointerValue(e.Id),
			Type: types.StringPointerValue(e.Type),
		}
		role.Entitlements = append(role.Entitlements, entitlement)
	}

	role.AccessProfiles = []AccessProfileRef{}

	for _, e := range v3Role.AccessProfiles {
		accessProfile := AccessProfileRef{
			Name: types.StringPointerValue(e.Name),
			Id:   types.StringPointerValue(e.Id),
			Type: types.StringPointerValue(e.Type),
		}
		role.AccessProfiles = append(role.AccessProfiles, accessProfile)
	}

	if v3Role.AccessRequestConfig == nil {
		println("v3Role.AccessRequestConfig:nil", &v3Role.AccessRequestConfig)
		role.AccessRequestConfig = nil
	} else {
		role.AccessRequestConfig = &Requestability{ApprovalSchemes: []AccessProfileApprovalScheme{}}
		role.AccessRequestConfig.CommentsRequired = types.BoolPointerValue(v3Role.AccessRequestConfig.CommentsRequired.Get())
		role.AccessRequestConfig.DenialCommentsRequired = types.BoolPointerValue(v3Role.AccessRequestConfig.DenialCommentsRequired.Get())

		for _, a := range v3Role.AccessRequestConfig.ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			role.AccessRequestConfig.ApprovalSchemes = append(role.AccessRequestConfig.ApprovalSchemes, approval)
		}
	}

	if v3Role.RevocationRequestConfig == nil || len(v3Role.RevocationRequestConfig.ApprovalSchemes) == 0 {
		role.RevocationRequestConfig = &Revocability{ApprovalSchemes: []AccessProfileApprovalScheme{}}
	} else {
		role.RevocationRequestConfig = &Revocability{}

		for _, a := range v3Role.RevocationRequestConfig.ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			role.RevocationRequestConfig.ApprovalSchemes = append(role.RevocationRequestConfig.ApprovalSchemes, approval)
		}
	}

	segments, diags1 := types.ListValueFrom(context.Background(), types.StringType, role.Segments.Elements())
	role.Segments = segments
	diags.Append(diags1...)

	if len(v3Role.AccessModelMetadata.Attributes) > 0 {
		metadata := []metadataattribute.AttributeDTO{}

		for _, att := range v3Role.AccessModelMetadata.Attributes {
			metatdataAtts := metadataattribute.AttributeDTO{}
			metatdataAtts.Key = types.StringPointerValue(att.Key)
			metatdataAtts.Name = types.StringPointerValue(att.Name)
			metatdataAtts.Multiselect = types.BoolPointerValue(att.Multiselect)
			metatdataAtts.Status = types.StringPointerValue(att.Status)
			metatdataAtts.Type = types.StringPointerValue(att.Type)
			metatdataAtts.Description = types.StringPointerValue(att.Description)

			objectTypes, diags1 := types.ListValueFrom(context.Background(), types.StringType, att.ObjectTypes)
			metatdataAtts.ObjectTypes = objectTypes
			diags.Append(diags1...)

			for _, v := range att.Values {
				value := &metadataattribute.AttributeValueDTO{
					Value:  types.StringPointerValue(v.Value),
					Name:   types.StringPointerValue(v.Name),
					Status: types.StringPointerValue(v.Status),
				}
				metatdataAtts.Values = append(metatdataAtts.Values, *value)

			}
			metadata = append(metadata, metatdataAtts)

		}

		role.AccessModelMetadata = metadata
	}

	if v3Role.Membership.Get() != nil {
		println("v3Role.AccessRequestConfig:not")
		membership := &RoleMembershipSelector{}

		membership.Type = types.StringPointerValue((*string)(v3Role.Membership.Get().Type.Ptr()))

		for _, v := range v3Role.Membership.Get().Identities {
			identity := RoleMembershipIdentity{}
			identity.Type = types.StringPointerValue((*string)(v.Type.Ptr()))
			identity.Id = types.StringValue(v.GetId())
			identity.Name = types.StringPointerValue(v.Name.Get())
			identity.AliasName = types.StringPointerValue(v.AliasName.Get())

			membership.Identities = append(membership.Identities, identity)
		}

		if v3Role.Membership.Get().Criteria.Get() != nil {
			level1 := RoleCriteriaLevel1{}

			keyLevel1 := &RoleCriteriaKey{}
			if v3Role.Membership.Get().Criteria.Get().Key.Get() != nil {
				keyLevel1.Property = types.StringValue(v3Role.Membership.Get().Criteria.Get().Key.Get().Property)
				keyLevel1.Type = types.StringPointerValue((*string)(v3Role.Membership.Get().Criteria.Get().Key.Get().Type.Ptr()))
				keyLevel1.SourceId = types.StringValue(v3Role.Membership.Get().Criteria.Get().Key.Get().GetSourceId())
				level1.Key = keyLevel1
			}

			level1.Operation = types.StringPointerValue((*string)(v3Role.Membership.Get().Criteria.Get().Operation))
			level1.StringValue = types.StringPointerValue(v3Role.Membership.Get().Criteria.Get().StringValue.Get())

			level2 := []RoleCriteriaLevel2{}
			for _, l2 := range v3Role.Membership.Get().GetCriteria().Children {
				println("v3Role.Membership.Get().GetCriteria().Children :%v", v3Role.Membership.Get().GetCriteria().Children)
				c2 := RoleCriteriaLevel2{}

				keyLevel2 := &RoleCriteriaKey{}
				if l2.Key.Get() != nil {
					keyLevel2.Property = types.StringValue(l2.Key.Get().Property)
					keyLevel2.Type = types.StringPointerValue((*string)(l2.Key.Get().Type.Ptr()))
					keyLevel2.SourceId = types.StringValue(l2.Key.Get().GetSourceId())
					c2.Key = keyLevel2
				}

				c2.Operation = types.StringPointerValue((*string)(l2.Operation))
				// child string_values aren't actually nullable?
				if l2.StringValue.Get() != nil {
					c2.StringValue = types.StringPointerValue(l2.StringValue.Get())
				}

				level3 := []RoleCriteriaLevel3{}
				for _, l3 := range l2.Children {
					c3 := RoleCriteriaLevel3{}

					keyLevel3 := &RoleCriteriaKey{}
					if l3.Key.Get() != nil {
						keyLevel3.Property = types.StringValue(l3.Key.Get().Property)
						keyLevel3.Type = types.StringPointerValue((*string)(l3.Key.Get().Type.Ptr()))
						keyLevel3.SourceId = types.StringPointerValue(l3.Key.Get().SourceId.Get())
						c3.Key = keyLevel3
					}

					c3.Operation = types.StringPointerValue((*string)(l3.Operation))
					c3.StringValue = types.StringPointerValue(l3.StringValue)

					level3 = append(level3, c3)
				}
				c2.Children = level3

				level2 = append(level2, c2)
			}
			level1.Children = level2

			membership.Criteria = &level1
		}

		role.Membership = membership
	}

}
