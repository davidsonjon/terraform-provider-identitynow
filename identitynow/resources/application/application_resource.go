package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	sailpoint "github.com/davidsonjon/golang-sdk"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	cc "github.com/davidsonjon/golang-sdk/cc"
)

var _ resource.Resource = &ApplicationResource{}
var _ resource.ResourceWithImportState = &ApplicationResource{}

func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

type ApplicationResource struct {
	client *sailpoint.APIClient
}

func (r *ApplicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *ApplicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Application data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "id of the application",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "app_id of the application",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "service_id of the application",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "service_app_id of the application",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the application",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the application",
			},
			"app_center_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Determines if application is enabled in app center",
			},
			"provision_request_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Determines if application is requestable in app center",
			},
			"launchpad_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Launchpad enabled",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
						MarkdownDescription: "Owner name",
					},
					"id": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
						MarkdownDescription: "Owner id",
					},
				},
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Owner information",
			},
			"date_created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "date created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// "last_updated": schema.StringAttribute{
			// 	Computed:            true,
			// 	MarkdownDescription: "",
			// },
			"access_profile_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "List of access profile id's",
			},
			"account_service_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "account_service_id of application",
			},
		},
	}
}

func (r *ApplicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ListApplications200ResponseInner

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	createApplicationRequest := *cc.NewCreateApplicationRequest()
	createApplicationRequest.Name = data.Name.ValueStringPointer()
	createApplicationRequest.Description = data.Description.ValueStringPointer()

	app, httpResp, err := r.client.CC.ApplicationsApi.CreateApplication(ctx).CreateApplicationRequest(createApplicationRequest).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.CreateApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.CreateApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	jsonapp, err := json.Marshal(app)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling json.Marshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	respApplication := SailApplication{}
	err = json.Unmarshal(jsonapp, &respApplication)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling json.Unmarshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	data.Id = types.StringValue(respApplication.Id)

	updateApplication(ctx, r, &data, &resp.Diagnostics)

	readAppAssociationApp(r, &data, &resp.Diagnostics)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ListApplications200ResponseInner

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	readAppAssociationApp(r, &data, &resp.Diagnostics)

	if resp.Diagnostics.WarningsCount() > 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update ListApplications200ResponseInner

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	updateApplication(ctx, r, &update, &resp.Diagnostics)

	readAppAssociationApp(r, &update, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ListApplications200ResponseInner

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.CC.ApplicationsApi.DeleteApplication(ctx, state.AppId.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.DeleteApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.DeleteApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}
}

func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func readAppAssociationApp(r *ApplicationResource, data *ListApplications200ResponseInner, diags *diag.Diagnostics) {
	log.Print("readAppAssociationApp")

	app, httpResp, err := r.client.CC.ApplicationsApi.GetApplication(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			diags.AddWarning(
				"Error when calling CC.ApplicationsApi.GetApplication",
				fmt.Sprintf("Application with id:%s is not found. Removing from state.",
					data.Id.ValueString()))
		} else {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				diags.AddWarning(
					"Error when calling CC.ApplicationsApi.GetApplication",
					fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
				)
			} else {
				diags.AddWarning(
					"Error when calling CC.ApplicationsApi.GetApplication",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
		}
		return
	}

	parseAttributesApplication(data, app, diags)

	appAccessProfiles, httpResp, err := r.client.CC.ApplicationsApi.GetApplicationAccessProfiles(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			diags.AddWarning(
				"Error when calling CC.ApplicationsApi.GetApplicationAccessProfiles",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			diags.AddWarning(
				"Error when calling CC.ApplicationsApi.GetApplicationAccessProfiles",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		return
	}

	jsonAppAccessProfiles, err := json.Marshal(appAccessProfiles)
	if err != nil {
		diags.AddError(
			"Error when calling json.Marshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	respAccessProfile := SailApplicationAccessProfiles{}
	err = json.Unmarshal(jsonAppAccessProfiles, &respAccessProfile)
	if err != nil {
		diags.AddError(
			"Error when calling json.Unmarshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	parseAccessProfiles(data, respAccessProfile, diags)
}

func updateApplication(ctx context.Context, r *ApplicationResource, update *ListApplications200ResponseInner, diags *diag.Diagnostics) {
	newItems := map[string]interface{}{}

	if !update.Name.IsNull() || !update.Name.IsUnknown() {
		newItems["alias"] = update.Name.ValueString()
	}
	if !update.Description.IsNull() || !update.Description.IsUnknown() {
		newItems["description"] = update.Description.ValueString()
	}
	if !update.AccountServiceId.IsNull() || !update.AccountServiceId.IsUnknown() {
		newItems["accountServiceId"] = update.AccountServiceId.ValueString()
	}
	if !update.ProvisionRequestEnabled.IsNull() || !update.ProvisionRequestEnabled.IsUnknown() {
		newItems["provisionRequestEnabled"] = update.ProvisionRequestEnabled.ValueBool()
	}
	if !update.LaunchpadEnabled.IsNull() || !update.LaunchpadEnabled.IsUnknown() {
		newItems["launchpadEnabled"] = update.LaunchpadEnabled.ValueBool()
	}
	if !update.AppCenterEnabled.IsNull() || !update.AppCenterEnabled.IsUnknown() {
		newItems["appCenterEnabled"] = update.AppCenterEnabled.ValueBool()
	}

	var propsState *ListApplications200ResponseInnerOwner
	diags.Append(update.Owner.As(ctx, &propsState, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return
	}

	if !update.Owner.IsNull() || !update.Owner.IsUnknown() {
		newItems["ownerId"] = propsState.Id.ValueString()
	}

	accessProfileIds := []string{}
	for _, v := range update.AccessProfileIds.Elements() {
		accessProfileIds = append(accessProfileIds, v.(types.String).ValueString())
	}

	newItems["accessProfileIds"] = accessProfileIds

	app, httpResp, err := r.client.CC.ApplicationsApi.UpdateApplication(ctx, update.Id.ValueString()).RequestBody(newItems).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			diags.AddError(
				"Error when calling CC.ApplicationsApi.UpdateApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			diags.AddError(
				"Error when calling CC.ApplicationsApi.UpdateApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	jsonApp, err := json.Marshal(app)
	if err != nil {
		diags.AddError(
			"Error when calling json.Marshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	respApplication := SailApplication{}
	err = json.Unmarshal(jsonApp, &respApplication)
	if err != nil {
		diags.AddError(
			"Error when calling json.Unmarshal",
			fmt.Sprintf("Error: %T, see debug info for more information", err),
		)
		return
	}

	update.Id = types.StringValue(respApplication.Id)
}
