package applicationaccessassocation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	sailpoint "github.com/davidsonjon/golang-sdk"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/application"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
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
				Validators: []validator.String{
					stringvalidator.LengthBetween(5, 5),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9]+$`),
						"must contain only numeric characters",
					),
				},
			},
			"access_profile_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of Access Profile(s)",
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

func readAppAssociation(r *AccessProfileAssociationResource, id string) (*application.SailApplicationAccessProfiles, *http.Response, error) {
	_, httpResp, err := r.client.CC.ApplicationsApi.GetApplicationAccessProfiles(context.Background(), id).Execute()
	if err != nil {
		tflog.Info(context.Background(), fmt.Sprintf("Full HTTP response: %v", httpResp))
		return nil, httpResp, err
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, err
	}

	respAccessProfile := application.SailApplicationAccessProfiles{}
	err = json.Unmarshal(body, &respAccessProfile)
	if err != nil {
		return nil, nil, err
	}
	return &respAccessProfile, httpResp, nil
}

func updateAppAssociation(r *AccessProfileAssociationResource, plan application.ApplicationAccessAssociation, state application.ApplicationAccessAssociation) (*http.Response, error) {
	id := state.ApplicationId.ValueString()

	planProfiles := make(map[string]struct{})

	for _, v := range plan.AccessProfileIds.Elements() {
		planProfiles[v.(types.String).ValueString()] = struct{}{}
	}

	appAcessProfiles, httpResp, err := readAppAssociation(r, id)
	if err != nil {
		log.Printf("Error when calling `ApplicationsApi.UpdateApplication``: %v\n", err)
		return httpResp, err
	}

	var accessProfileIds []string
	for _, v := range appAcessProfiles.Items {
		accessProfileIds = append(accessProfileIds, v.Id)
	}
	for _, v := range plan.AccessProfileIds.Elements() {
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

	newItems := map[string]interface{}{
		"accessProfileIds": list,
	}

	_, httpResp, err = r.client.CC.ApplicationsApi.UpdateApplication(context.Background(), id).RequestBody(newItems).Execute()
	if err != nil {
		log.Printf("Error when calling `ApplicationsApi.UpdateApplication``: %v\n", err)
		return httpResp, err
	}

	return httpResp, nil
}

func (r *AccessProfileAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data application.ApplicationAccessAssociation

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = data.ApplicationId

	httpResp, err := updateAppAssociation(r, data, data)
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling updateAppAssociation",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling updateAppAssociation",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data application.ApplicationAccessAssociation

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ApplicationId.ValueString() // string |
	data.Id = data.ApplicationId

	appAccessProfiles, httpResp, err := readAppAssociation(r, id)
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling readAppAssociation",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling readAppAssociation",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	currentAssocProfiles := make(map[string]bool)

	for _, v := range appAccessProfiles.Items {
		currentAssocProfiles[v.Id] = true
	}

	resourceProfilesFound := true

	for _, v := range data.AccessProfileIds.Elements() {
		if !currentAssocProfiles[v.(types.String).ValueString()] {
			log.Printf("User roles differ updating user: %+v", v.(types.String).ValueString())
			resourceProfilesFound = false
		}
	}

	if !resourceProfilesFound {
		resp.Diagnostics.AddWarning(
			"AccessProfile associations not found",
			fmt.Sprintf("SQL User's parent cluster with clusterID %s is not found. Removing from state.",
				id))
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessProfileAssociationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update application.ApplicationAccessAssociation

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	update.Id = update.ApplicationId

	httpResp, err := updateAppAssociation(r, update, state)
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling updateAppAssociation",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling updateAppAssociation",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *AccessProfileAssociationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state application.ApplicationAccessAssociation

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ApplicationId.ValueString()

	appAccessProfiles, httpResp, err := readAppAssociation(r, id)
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling readAppAssociation",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling readAppAssociation",
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

	for i := len(appAccessProfiles.Items) - 1; i >= 0; i-- {
		_, ok := list[appAccessProfiles.Items[i].Id]
		if ok {
			appAccessProfiles.Items = append(appAccessProfiles.Items[:i], appAccessProfiles.Items[i+1:]...)
		}
	}

	remainingAccessProfiles := []string{}

	for _, v := range appAccessProfiles.Items {
		remainingAccessProfiles = append(remainingAccessProfiles, v.Id)
	}

	newItems := map[string]interface{}{
		"accessProfileIds": remainingAccessProfiles,
	}
	log.Printf("remainingAccessProfiles: %v\n", remainingAccessProfiles)

	_, httpResp, err = r.client.CC.ApplicationsApi.UpdateApplication(ctx, id).RequestBody(newItems).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.UpdateApplication",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling CC.ApplicationsApi.UpdateApplication",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
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
	listValue, ok := types.ListValue(types.StringType, elements)
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_profile_ids"), listValue)...)
}
