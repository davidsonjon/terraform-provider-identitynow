package application

import (
	"context"

	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
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
		MarkdownDescription: "Source Application data source",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "id of the application",
			},
			"cloud_app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The deprecated source app id",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "The source app name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the source app was created",
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the source app was last modified",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the source app is enabled",
			},
			"provision_request_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the source app is provision request enabled",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the source app",
			},
			"match_all_accounts": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the source app match all accounts",
			},
			"appcenter_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the source app is shown in the app center",
			},
			"account_source_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Source ID of the source app",
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
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner type",
					},
				},
				Computed:            true,
				MarkdownDescription: "Owner information",
			},
			"access_profile_ids": schema.SetAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Set of access profile ids",
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
	var data SourceApp

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.IsNull() {
		apps, httpResp, err := d.client.Beta.AppsAPI.ListAllSourceApp(context.Background()).Filters("name eq \"" + data.Name.ValueString() + "\"").Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.AppsAPI.ListAllSourceApp",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.AppsAPI.ListAllSourceApp",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		}

		for _, a := range apps {
			if *a.Name == data.Name.ValueString() {
				data.Id = types.StringPointerValue(a.Id)
			}
		}

		if data.Id.IsNull() {
			resp.Diagnostics.AddError(
				"Error when calling Looking up SourceApp by name",
				fmt.Sprintf("Error finding SourceApp name: %s", data.Name.ValueString()),
			)
			return
		}

	}

	app, httpResp, err := d.client.Beta.AppsAPI.GetSourceApp(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.GetSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.GetSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Id = types.StringPointerValue(app.Id)
	data.CloudAppId = types.StringPointerValue(app.CloudAppId)
	data.Name = types.StringPointerValue(app.Name)
	data.Created = types.StringValue(app.Created.String())
	data.Modified = types.StringValue(app.Modified.String())
	data.Enabled = types.BoolPointerValue(app.Enabled)
	data.ProvisionRequestEnabled = types.BoolPointerValue(app.ProvisionRequestEnabled)
	data.Description = types.StringPointerValue(app.Description)
	data.MatchAllAccounts = types.BoolPointerValue(app.MatchAllAccounts)
	data.AppCenterEnabled = types.BoolPointerValue(app.AppCenterEnabled)

	owner, ok := types.ObjectValue(listApplications200ResponseInnerOwnerTypes, map[string]attr.Value{
		"name": types.StringPointerValue(app.Owner.Get().Name),
		"id":   types.StringPointerValue(app.Owner.Get().Id),
		"type": types.StringPointerValue((*string)(app.Owner.Get().Type)),
	})

	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	data.Owner = owner

	data.AccountSourceId = types.StringPointerValue(app.AccountSource.Get().Id)

	appAccessProfiles, httpResp, err := d.client.Beta.AppsAPI.ListAccessProfilesForSourceApp(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling eta.AppsAPI.ListAccessProfilesForSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling eta.AppsAPI.ListAccessProfilesForSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}

	elements := []attr.Value{}

	for _, v := range appAccessProfiles {
		elements = append(elements, types.StringPointerValue(v.Id))
	}

	setValue, diags := types.SetValueFrom(ctx, types.StringType, elements)
	resp.Diagnostics.Append(diags...)

	data.AccessProfileIds = setValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
