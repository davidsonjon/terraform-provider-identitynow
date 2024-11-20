package application

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"
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
		MarkdownDescription: "Source Application Resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "id of the application",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_app_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The deprecated source app id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The source app name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the source app was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the source app was last modified",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "True if the source app is enabled",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"provision_request_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "True if the source app is provision request enabled",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The description of the source app",
			},
			"match_all_accounts": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the source app match all accounts",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"appcenter_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "True if the source app is shown in the app center",
				Default:             booldefault.StaticBool(true),
			},
			"account_source_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Source ID of the source app",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Owner name",
					},
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Owner id",
					},
					"type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Owner type",
					},
				},
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Owner information",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"access_profile_ids": schema.SetAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Set of access profile ids",
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
	var data SourceApp

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	time.Sleep(5 * time.Second)

	source := beta.SourceAppCreateDtoAccountSource{
		Id: data.AccountSourceId.ValueString(),
	}

	createApplicationRequest := *beta.NewSourceAppCreateDto(data.Name.ValueString(), data.Description.ValueString(), source)

	app, httpResp, err := r.client.Beta.AppsAPI.CreateSourceApp(ctx).SourceAppCreateDto(createApplicationRequest).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.CreateSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.CreateSourceApp",
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
	data.Description = types.StringPointerValue(app.Description)

	sourceApp := beta.SourceApp{}
	sourceApp.Name = data.Name.ValueStringPointer()
	sourceApp.Description = data.Description.ValueStringPointer()
	sourceApp.Enabled = data.Enabled.ValueBoolPointer()
	value := &beta.SourceAppAccountSource{Id: data.AccountSourceId.ValueStringPointer()}
	sourceApp.AccountSource = *beta.NewNullableSourceAppAccountSource(value)
	sourceApp.ProvisionRequestEnabled = data.ProvisionRequestEnabled.ValueBoolPointer()
	sourceApp.MatchAllAccounts = data.MatchAllAccounts.ValueBoolPointer()
	sourceApp.AppCenterEnabled = data.AppCenterEnabled.ValueBoolPointer()

	if !data.Owner.IsNull() && !data.Owner.IsUnknown() {

		var ownerItem *ListApplications200ResponseInnerOwner

		diags := data.Owner.As(ctx, &ownerItem, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)

		owner2 := beta.NullableBaseReferenceDto1{}
		owner2.Set(&beta.BaseReferenceDto1{Id: ownerItem.Id.ValueStringPointer()})

		sourceApp.Owner = owner2
	} else {
		sourceApp.Owner = app.Owner
	}

	patch, err := jsondiff.Compare(app, sourceApp, jsondiff.Ignores("/created",
		"/modified",
		"/id",
		"/cloudAppId",
		"/accountSource/name",
		"/accountSource/passwordPolicies",
		"/accountSource/type",
		"/accountSource/useForPasswordManagement"))
	if err != nil {
		// handle error
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	jsonPatchOperation := []beta.JsonPatchOperation{} // []JsonPatchOperation |

	for _, p := range patch {
		patch := *beta.NewJsonPatchOperationWithDefaults()

		op, err := p.MarshalJSON()
		if err != nil {
			// handle error
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	accessProfiles := make([]types.String, 0, len(data.AccessProfileIds.Elements()))
	diags := data.AccessProfileIds.ElementsAs(ctx, &accessProfiles, false)
	resp.Diagnostics.Append(diags...)

	aps := []beta.ArrayInner{}
	for _, v := range accessProfiles {
		aps = append(aps, beta.ArrayInner{String: v.ValueStringPointer()})
	}
	patchInnerValue := beta.UpdateMultiHostSourcesRequestInnerValue{
		ArrayOfArrayInner: &aps,
	}
	accessProfileOp := *beta.NewJsonPatchOperationWithDefaults()
	accessProfileOp.Op = "add"
	accessProfileOp.Path = "/accessProfiles"
	accessProfileOp.Value = &patchInnerValue
	jsonPatchOperation = append(jsonPatchOperation, accessProfileOp)

	sourceAppPatch, httpResp, err := r.client.Beta.AppsAPI.PatchSourceApp(context.Background(), *app.Id).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.PatchSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.PatchSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Enabled = types.BoolPointerValue(sourceAppPatch.Enabled)
	data.ProvisionRequestEnabled = types.BoolPointerValue(sourceAppPatch.ProvisionRequestEnabled)
	data.MatchAllAccounts = types.BoolPointerValue(sourceAppPatch.MatchAllAccounts)
	data.AppCenterEnabled = types.BoolPointerValue(sourceAppPatch.AppCenterEnabled)
	data.AccountSourceId = types.StringPointerValue(sourceAppPatch.AccountSource.Get().Id)

	ownerValue := ListApplications200ResponseInnerOwner{
		Id:   types.StringValue(*sourceAppPatch.Owner.Get().Id),
		Name: types.StringValue(*sourceAppPatch.Owner.Get().Name),
		Type: types.StringPointerValue((*string)(app.Owner.Get().Type)),
	}

	objectValue, diags := types.ObjectValueFrom(ctx, ownerValue.AttributeTypes(), ownerValue)
	resp.Diagnostics.Append(diags...)

	data.Owner = objectValue

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SourceApp

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	app, httpResp, err := r.client.Beta.AppsAPI.GetSourceApp(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			resp.Diagnostics.AddWarning(
				"Error when calling Beta.AppsAPI.GetSourceApp",
				fmt.Sprintf("AccessProfile with id:%s is not found. Removing from state.",
					data.Id.ValueString()))
			resp.State.RemoveResource(ctx)
		} else {
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

	appAccessProfiles, httpResp, err := r.client.Beta.AppsAPI.ListAccessProfilesForSourceApp(context.Background(), data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.ListAccessProfilesForSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.ListAccessProfilesForSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
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

func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, update SourceApp

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// state

	sourceAppState := beta.SourceApp{}
	sourceAppState.Name = state.Name.ValueStringPointer()
	sourceAppState.Description = state.Description.ValueStringPointer()
	sourceAppState.Enabled = state.Enabled.ValueBoolPointer()
	valueState := &beta.SourceAppAccountSource{Id: state.AccountSourceId.ValueStringPointer()}
	sourceAppState.AccountSource = *beta.NewNullableSourceAppAccountSource(valueState)
	sourceAppState.ProvisionRequestEnabled = state.ProvisionRequestEnabled.ValueBoolPointer()
	sourceAppState.MatchAllAccounts = state.MatchAllAccounts.ValueBoolPointer()
	sourceAppState.AppCenterEnabled = state.AppCenterEnabled.ValueBoolPointer()

	if !state.Owner.IsNull() && !state.Owner.IsUnknown() {

		var ownerItem *ListApplications200ResponseInnerOwner

		diags := state.Owner.As(ctx, &ownerItem, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)

		owner2 := beta.NullableBaseReferenceDto1{}
		owner2.Set(&beta.BaseReferenceDto1{Id: ownerItem.Id.ValueStringPointer()})

		sourceAppState.Owner = owner2
	}

	// update
	sourceAppUpdate := beta.SourceApp{}
	sourceAppUpdate.Name = update.Name.ValueStringPointer()
	sourceAppUpdate.Description = update.Description.ValueStringPointer()
	sourceAppUpdate.Enabled = update.Enabled.ValueBoolPointer()
	valueUpdate := &beta.SourceAppAccountSource{Id: update.AccountSourceId.ValueStringPointer()}
	sourceAppUpdate.AccountSource = *beta.NewNullableSourceAppAccountSource(valueUpdate)
	sourceAppUpdate.ProvisionRequestEnabled = update.ProvisionRequestEnabled.ValueBoolPointer()
	sourceAppUpdate.MatchAllAccounts = update.MatchAllAccounts.ValueBoolPointer()
	sourceAppUpdate.AppCenterEnabled = update.AppCenterEnabled.ValueBoolPointer()

	if !update.Owner.IsNull() && !update.Owner.IsUnknown() {

		var ownerItem *ListApplications200ResponseInnerOwner

		diags := update.Owner.As(ctx, &ownerItem, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)

		owner2 := beta.NullableBaseReferenceDto1{}
		owner2.Set(&beta.BaseReferenceDto1{Id: ownerItem.Id.ValueStringPointer()})

		sourceAppUpdate.Owner = owner2
	} else {
		sourceAppUpdate.Owner = sourceAppState.Owner
	}

	patch, err := jsondiff.Compare(sourceAppState, sourceAppUpdate, jsondiff.Ignores(
		"/created",
		"/modified",
		"/id",
		"/cloudAppId",
		"/accountSource/name",
		"/accountSource/passwordPolicies",
		"/accountSource/type",
		"/accountSource/useForPasswordManagement",
	))
	if err != nil {
		// handle error
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	tflog.Info(ctx, fmt.Sprintf("patch: %v", patch))

	jsonPatchOperation := []beta.JsonPatchOperation{} // []JsonPatchOperation |

	for _, p := range patch {
		patch := *beta.NewJsonPatchOperationWithDefaults()

		op, err := p.MarshalJSON()
		if err != nil {
			// handle error
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	accessProfiles := make([]types.String, 0, len(update.AccessProfileIds.Elements()))
	diags := update.AccessProfileIds.ElementsAs(ctx, &accessProfiles, false)
	resp.Diagnostics.Append(diags...)

	aps := []beta.ArrayInner{}
	for _, v := range accessProfiles {
		aps = append(aps, beta.ArrayInner{String: v.ValueStringPointer()})
	}
	patchInnerValue := beta.UpdateMultiHostSourcesRequestInnerValue{
		ArrayOfArrayInner: &aps,
	}

	accessProfileOp := *beta.NewJsonPatchOperationWithDefaults()
	accessProfileOp.Op = "add"
	accessProfileOp.Path = "/accessProfiles"
	accessProfileOp.Value = &patchInnerValue
	jsonPatchOperation = append(jsonPatchOperation, accessProfileOp)

	appAccessProfilesPatch, httpResp, err := r.client.Beta.AppsAPI.PatchSourceApp(context.Background(), update.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.PatchSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.PatchSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	update.Id = types.StringPointerValue(appAccessProfilesPatch.Id)
	update.CloudAppId = types.StringPointerValue(appAccessProfilesPatch.CloudAppId)
	update.Name = types.StringPointerValue(appAccessProfilesPatch.Name)
	update.Created = types.StringValue(appAccessProfilesPatch.Created.String())
	update.Modified = types.StringValue(appAccessProfilesPatch.Modified.String())
	update.Enabled = types.BoolPointerValue(appAccessProfilesPatch.Enabled)
	update.ProvisionRequestEnabled = types.BoolPointerValue(appAccessProfilesPatch.ProvisionRequestEnabled)
	update.Description = types.StringPointerValue(appAccessProfilesPatch.Description)
	update.MatchAllAccounts = types.BoolPointerValue(appAccessProfilesPatch.MatchAllAccounts)
	update.AppCenterEnabled = types.BoolPointerValue(appAccessProfilesPatch.AppCenterEnabled)

	owner, ok := types.ObjectValue(listApplications200ResponseInnerOwnerTypes, map[string]attr.Value{
		"name": types.StringPointerValue(appAccessProfilesPatch.Owner.Get().Name),
		"id":   types.StringPointerValue(appAccessProfilesPatch.Owner.Get().Id),
		"type": types.StringPointerValue((*string)(appAccessProfilesPatch.Owner.Get().Type)),
	})

	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	update.Owner = owner

	update.AccountSourceId = types.StringPointerValue(appAccessProfilesPatch.AccountSource.Get().Id)

	elements := []attr.Value{}

	for _, v := range appAccessProfilesPatch.AccessProfiles {
		elements = append(elements, types.StringPointerValue(&v))
	}

	setValue, diags := types.SetValueFrom(ctx, types.StringType, elements)
	resp.Diagnostics.Append(diags...)

	update.AccessProfileIds = setValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SourceApp

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.Beta.AppsAPI.DeleteSourceApp(ctx, state.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.DeleteSourceApp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AppsAPI.DeleteSourceApp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
	}
}

func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
