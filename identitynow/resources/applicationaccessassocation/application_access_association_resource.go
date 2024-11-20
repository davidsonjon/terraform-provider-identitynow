package applicationaccessassocation

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &AccessProfileAssociationResource{}
var _ resource.ResourceWithImportState = &AccessProfileAssociationResource{}

func NewAccessProfileAssociationResource() resource.Resource {
	return &AccessProfileAssociationResource{}
}

type AccessProfileAssociationResource struct {
	client *sailpoint.APIClient
}

func (r *AccessProfileAssociationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_access_association"
}

func (r *AccessProfileAssociationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Application Access Profile Association resource. Used to manage Access Profile assignment to an application outside of the identitynow_application resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Id",
			},
			"application_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application ID",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(32, 32),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z0-9]+$`),
						"must contain only lowercase alphanumeric characters of length 32",
					),
				},
			},
			"access_profile_ids": schema.SetAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "Set of Access Profile(s)",
			},
		},
	}
}

func (r *AccessProfileAssociationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *AccessProfileAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ApplicationAccessAssociation

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = data.ApplicationId

	planProfiles := make(map[string]struct{})

	for _, v := range data.AccessProfileIds.Elements() {
		planProfiles[v.(types.String).ValueString()] = struct{}{}
	}

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

	var accessProfileIds []string
	for _, v := range appAccessProfiles {
		accessProfileIds = append(accessProfileIds, *v.Id)
	}
	for _, v := range data.AccessProfileIds.Elements() {
		accessProfileIds = append(accessProfileIds, v.(types.String).ValueString())
	}

	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range accessProfileIds {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	jsonPatchOperation := []beta.JsonPatchOperation{}

	accessProfiles := make([]types.String, 0, len(data.AccessProfileIds.Elements()))
	diags := data.AccessProfileIds.ElementsAs(ctx, &accessProfiles, false)
	resp.Diagnostics.Append(diags...)

	aps := []beta.ArrayInner{}
	for _, v := range list {
		aps = append(aps, beta.ArrayInner{String: &v})
	}
	patchInnerValue := beta.UpdateMultiHostSourcesRequestInnerValue{
		ArrayOfArrayInner: &aps,
	}
	patch := *beta.NewJsonPatchOperationWithDefaults()
	patch.Op = "add"
	patch.Path = "/accessProfiles"
	patch.Value = &patchInnerValue
	jsonPatchOperation = append(jsonPatchOperation, patch)

	_, httpResp, err = r.client.Beta.AppsAPI.PatchSourceApp(context.Background(), data.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling .Beta.AppsAPI.PatchSourceAp",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling .Beta.AppsAPI.PatchSourceAp",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ApplicationAccessAssociation

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ApplicationId.ValueString() // string |
	data.Id = data.ApplicationId

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

	currentAssocProfiles := make(map[string]bool)

	for _, v := range appAccessProfiles {
		currentAssocProfiles[*v.Id] = true
	}

	resourceProfilesFound := true

	for _, v := range data.AccessProfileIds.Elements() {
		if !currentAssocProfiles[v.(types.String).ValueString()] {
			log.Printf("AccessProfile: %+v not found in application", v.(types.String).ValueString())
			resourceProfilesFound = false
		}
	}

	if !resourceProfilesFound {
		resp.Diagnostics.AddWarning(
			"AccessProfile associations not found",
			fmt.Sprintf("AccessProfile associations in application %s is not found. Removing from state.",
				id))
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileAssociationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update ApplicationAccessAssociation

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	update.Id = update.ApplicationId

	planProfiles := make(map[string]struct{})

	for _, v := range update.AccessProfileIds.Elements() {
		planProfiles[v.(types.String).ValueString()] = struct{}{}
	}

	appAccessProfiles, httpResp, err := r.client.Beta.AppsAPI.ListAccessProfilesForSourceApp(context.Background(), update.Id.ValueString()).Execute()
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

	var accessProfileIds []string
	for _, v := range appAccessProfiles {
		accessProfileIds = append(accessProfileIds, *v.Id)
	}
	for _, v := range update.AccessProfileIds.Elements() {
		accessProfileIds = append(accessProfileIds, v.(types.String).ValueString())
	}
	for i, v := range state.AccessProfileIds.Elements() {
		_, ok := planProfiles[v.(types.String).ValueString()]
		if !ok {
			accessProfileIds = append(accessProfileIds[:i], accessProfileIds[i+1:]...)
		}
	}

	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range accessProfileIds {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	jsonPatchOperation := []beta.JsonPatchOperation{}

	accessProfiles := make([]types.String, 0, len(update.AccessProfileIds.Elements()))
	diags := update.AccessProfileIds.ElementsAs(ctx, &accessProfiles, false)
	resp.Diagnostics.Append(diags...)

	aps := []beta.ArrayInner{}
	for _, v := range list {
		aps = append(aps, beta.ArrayInner{String: &v})
	}
	patchInnerValue := beta.UpdateMultiHostSourcesRequestInnerValue{
		ArrayOfArrayInner: &aps,
	}
	patch := *beta.NewJsonPatchOperationWithDefaults()
	patch.Op = "add"
	patch.Path = "/accessProfiles"
	patch.Value = &patchInnerValue
	jsonPatchOperation = append(jsonPatchOperation, patch)

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

	elements := []attr.Value{}

	for _, v := range appAccessProfilesPatch.AccessProfiles {
		elements = append(elements, types.StringPointerValue(&v))
	}

	setValue, diags := types.SetValueFrom(ctx, types.StringType, elements)
	resp.Diagnostics.Append(diags...)

	update.AccessProfileIds = setValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *AccessProfileAssociationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ApplicationAccessAssociation

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appAccessProfiles, httpResp, err := r.client.Beta.AppsAPI.ListAccessProfilesForSourceApp(context.Background(), state.Id.ValueString()).Execute()
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

	list := make(map[string]struct{})

	for _, s := range state.AccessProfileIds.Elements() {
		list[s.(types.String).ValueString()] = struct{}{}
	}

	for i := len(appAccessProfiles) - 1; i >= 0; i-- {
		_, ok := list[*appAccessProfiles[i].Id]
		if ok {
			appAccessProfiles = append(appAccessProfiles[:i], appAccessProfiles[i+1:]...)
		}
	}

	remainingAccessProfiles := []string{}

	for _, v := range appAccessProfiles {
		remainingAccessProfiles = append(remainingAccessProfiles, *v.Id)
	}

	jsonPatchOperation := []beta.JsonPatchOperation{}

	aps := []beta.ArrayInner{}
	for _, v := range remainingAccessProfiles {
		aps = append(aps, beta.ArrayInner{String: &v})
	}
	patchInnerValue := beta.UpdateMultiHostSourcesRequestInnerValue{
		ArrayOfArrayInner: &aps,
	}
	patch := *beta.NewJsonPatchOperationWithDefaults()
	patch.Op = "add"
	patch.Path = "/accessProfiles"
	patch.Value = &patchInnerValue
	jsonPatchOperation = append(jsonPatchOperation, patch)

	_, httpResp, err = r.client.Beta.AppsAPI.PatchSourceApp(context.Background(), state.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
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
		return
	}

}

func (r *AccessProfileAssociationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ",")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		err := fmt.Errorf("unexpected format for ID (%[1]s), expected application_id,access_profile_id", req.ID)
		resp.Diagnostics.AddError(fmt.Sprintf("importing Access Profile Association (%s)", req.ID), err.Error())
		return
	}

	accessProfiles := strings.Split(parts[1], "/")

	log.Printf("accessProfiles: %v\n", accessProfiles)

	elements := []attr.Value{}
	for _, v := range accessProfiles {
		elements = append(elements, types.StringValue(v))
	}
	setValue, ok := types.SetValue(types.StringType, elements)
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_profile_ids"), setValue)...)
}
