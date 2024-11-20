package accessprofile

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &AccessProfileDataSource{}

func NewAccessProfileDataSource() datasource.DataSource {
	return &AccessProfileDataSource{}
}

type AccessProfileDataSource struct {
	client *sailpoint.APIClient
}

func (d *AccessProfileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_profile"
}

func (d *AccessProfileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Access Profile Data Source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The ID of the Access Profile",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the Access Profile",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Information about the Access Profile",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Date the Access Profile was created",
			},
			// "modified": schema.StringAttribute{
			// 	Computed:            true,
			// 	MarkdownDescription: "Date the Access Profile was last modified",
			// },
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the Access Profile is enabled. If the Access Profile is enabled then you must include at least one Entitlement.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Identity id",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "THe type of the owner, will always be `IDENTITY`",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
					},
				},
				Computed: true,
			},
			"source": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The ID of the Source with with which the Access Profile is associated",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The type of the Source, will always be SOURCE",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The display name of the associated Source",
					},
				},
				Computed: true,
			},
			"entitlements": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The ID of the Entitlement",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the Entitlement, will always be ENTITLEMENT",
						},
						// "name": schema.StringAttribute{
						// 	Computed: true,
						// },
					},
				},
				Computed: true,
			},
			"requestable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the Access Profile is requestable via access request. Currently, making an Access Profile non-requestable is only supported  for customers enabled with the new Request Center. Otherwise, attempting to create an Access Profile with a value  **false** in this field results in a 400 error.",
			},
			"access_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"comments_required": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether the requester of the containing object must provide comments justifying the request",
					},
					"denial_comments_required": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether an approver must provide comments when denying the request",
					},
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows. **APP_OWNER**: The owner of the Application  **OWNER**: Owner of the associated Access Profile or Role  **SOURCE_OWNER**: Owner of the Source associated with an Access Profile  **MANAGER**: Manager of the Identity making the request  **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field",
								},
								"approver_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP",
								},
							},
						},
						Computed:            true,
						MarkdownDescription: "List describing the steps in approving the request",
					},
				},
				Computed: true,
			},
			"revocation_request_config": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"approval_schemes": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"approver_type": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Describes the individual or group that is responsible for an approval step. Values are as follows. **APP_OWNER**: The owner of the Application  **OWNER**: Owner of the associated Access Profile or Role  **SOURCE_OWNER**: Owner of the Source associated with an Access Profile  **MANAGER**: Manager of the Identity making the request  **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field",
								},
								"approver_id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Id of the specific approver, used only when approverType is GOVERNANCE_GROUP",
								},
							},
						},
						Computed:            true,
						MarkdownDescription: "List describing the steps in approving the revocation request",
					},
				},
				Computed: true,
			},
		},
	}
}

func (d *AccessProfileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *AccessProfileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AccessProfile

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ap, httpResp, err := d.client.V3.AccessProfilesAPI.GetAccessProfile(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.GetAccessProfile",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesApi.GetAccessProfile",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	tflog.Info(ctx, fmt.Sprintf("Response from `AccessProfilesApi.GetAccessProfile`: %v", resp))

	data.parseComputedAttributes(ap)
	data.parseConfiguredAttributes(ap)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
