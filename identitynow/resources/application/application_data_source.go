package application

import (
	"context"
	"encoding/json"

	"fmt"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/davidsonjon/golang-sdk"
)

var _ datasource.DataSource = &ApplicationDataSource{}

func NewApplicationDataSource() datasource.DataSource {
	return &ApplicationDataSource{}
}

type ApplicationDataSource struct {
	client *sailpoint.APIClient
}

func (d *ApplicationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *ApplicationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Application data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "id of the application",
			},
			"app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "app_id of the application",
			},
			"service_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "service_id of the application",
			},
			"service_app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "service_app_id of the application",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the application",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the application",
			},
			"app_center_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Determines if application is enabled in app center",
			},
			"provision_request_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Determines if application is requestable in app center",
			},
			"launchpad_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Launchpad enabled",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner name",
					},
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner id",
					},
				},
				Computed:            true,
				MarkdownDescription: "Owner information",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "date created",
			},
			// "last_updated": schema.StringAttribute{
			// 	Computed:            true,
			// 	MarkdownDescription: "",
			// },
			"access_profile_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of access profile id's",
			},
			"account_service_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "account_service_id of application",
			},
		},
	}
}

func (d *ApplicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *ApplicationDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *ApplicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ListApplications200ResponseInner

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.IsNull() {
		apps, httpResp, err := d.client.CC.ApplicationsApi.ListApplications(context.Background()).Filter("org").Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling GetApplication",
					fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling GetApplication",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		}

		for _, a := range apps {
			tflog.Info(ctx, fmt.Sprintf("a.Name: %v", *a.Name))

			if *a.Name == data.Name.ValueString() {
				tflog.Info(ctx, fmt.Sprintf("a.Name: %v", *a.Name))
				data.Id = types.StringPointerValue(a.Id)
			}
		}

	}

	app, httpResp, err := d.client.CC.ApplicationsApi.GetApplication(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling GetApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling GetApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	parseAttributesApplication(&data, app, &resp.Diagnostics)

	appAccessProfiles, httpResp, err := d.client.CC.ApplicationsApi.GetApplicationAccessProfiles(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling GetApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling GetApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}

	jsonAppAccessProfiles, err := json.Marshal(appAccessProfiles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling json.Marshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	respAccessProfile := SailApplicationAccessProfiles{}
	err = json.Unmarshal(jsonAppAccessProfiles, &respAccessProfile)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling json.Unmarshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	parseAccessProfiles(&data, respAccessProfile, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
