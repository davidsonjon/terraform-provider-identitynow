// Package application_v1 is a pilot implementation of the application
// resource/data sources generated from SailPoint's per-service v1 OpenAPI spec
// (api-specs/idn/apis/apps), following the same hand-written CRUD pattern
// established by role_v1, sources_v1, access_profile_v1, and the other _v1
// pilot targets.
//
// These hand-written wrappers implement resource.Resource /
// datasource.DataSource around the generated schema/model types in
// resource_application and datasource_application, backed by the golang-sdk v2
// api_beta.AppsAPI client (the SDK does not yet publish a per-service v1
// package; v1 is the stabilization of what was beta).
//
// Known limitations (tracked for follow-up before promoting out of the _v1
// pilot package into internal/provider/application):
//   - "access_profile_ids" is hand-added here (resource + data sources) rather
//     than generated from the spec, because GET /source-apps/v1/{id} does not
//     embed accessProfiles even though PATCH /source-apps/v1/{id} can replace
//     them. Read paths therefore perform an additional
//     ListAccessProfilesForSourceApp call and flatten the response to a set of
//     access profile ids.
//   - "owner" and "account_source" are converted by hand (rather than via
//     generated associated_external_type helpers) because the generated schema's
//     custom object value types do not line up cleanly with the SDK's
//     BaseReferenceDto / SourceAppAccountSource / SourceAppCreateDtoAccountSource
//     pointer-vs-value shapes.
//   - Create is necessarily two-phase whenever configuration manages fields the
//     create DTO does not support directly (owner, access_profile_ids,
//     enabled, provision_request_enabled, app_center_enabled): the provider
//     first POSTs the core create payload, then immediately issues a minimal
//     JSON Patch follow-up for only the configured fields that differ from the
//     API's create-time defaults.
package application_v1

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/application_v1/resource_application"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

const applicationListMaxLimit = 250

var (
	_ resource.Resource                = (*applicationResource)(nil)
	_ resource.ResourceWithConfigure   = (*applicationResource)(nil)
	_ resource.ResourceWithImportState = (*applicationResource)(nil)
)

func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationResource struct {
	client *sailpoint.APIClient
}

// applicationResourceModel mirrors resource_application.ApplicationModel plus
// the hand-added access_profile_ids attribute.
type applicationResourceModel struct {
	AccountSource           resource_application.AccountSourceValue `tfsdk:"account_source"`
	AccessProfileIds        types.Set                               `tfsdk:"access_profile_ids"`
	AppCenterEnabled        types.Bool                              `tfsdk:"app_center_enabled"`
	CloudAppId              types.String                            `tfsdk:"cloud_app_id"`
	Created                 types.String                            `tfsdk:"created"`
	Description             types.String                            `tfsdk:"description"`
	Enabled                 types.Bool                              `tfsdk:"enabled"`
	Id                      types.String                            `tfsdk:"id"`
	MatchAllAccounts        types.Bool                              `tfsdk:"match_all_accounts"`
	Modified                types.String                            `tfsdk:"modified"`
	Name                    types.String                            `tfsdk:"name"`
	Owner                   resource_application.OwnerValue         `tfsdk:"owner"`
	ProvisionRequestEnabled types.Bool                              `tfsdk:"provision_request_enabled"`
}

func (r *applicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_v1"
}

func (r *applicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_application.ApplicationResourceSchema(ctx)
	resp.Schema.Description = "Manages an Application (source app) in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages an [Application](https://documentation.sailpoint.com/) " +
		"(source app) in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."

	patchApplicationResourceSchema(&resp.Schema)
	applyApplicationUseStateForUnknown(&resp.Schema)
}

func patchApplicationResourceSchema(s *resourceschema.Schema) {
	if a, ok := s.Attributes["enabled"].(resourceschema.BoolAttribute); ok {
		a.Optional = true
		s.Attributes["enabled"] = a
	}
	if a, ok := s.Attributes["provision_request_enabled"].(resourceschema.BoolAttribute); ok {
		a.Optional = true
		s.Attributes["provision_request_enabled"] = a
	}
	if a, ok := s.Attributes["app_center_enabled"].(resourceschema.BoolAttribute); ok {
		a.Optional = true
		s.Attributes["app_center_enabled"] = a
	}

	s.Attributes["access_profile_ids"] = resourceschema.SetAttribute{
		Optional:    true,
		Computed:    true,
		ElementType: types.StringType,
		Description: "Set of access profile IDs assigned to the application.",
		MarkdownDescription: "Set of access profile IDs assigned to the application. This attribute is read via a " +
			"separate `GET /source-apps/v1/{id}/access-profiles` call because the main source-app GET response does not embed them.",
		PlanModifiers: []planmodifier.Set{
			setplanmodifier.UseStateForUnknown(),
		},
	}
}

func (r *applicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	if cp.GetClient() == nil {
		resp.Diagnostics.AddError("Missing API client", "Provider configured without an API client.")
		return
	}
	r.client = cp.GetClient()
}

func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Application", map[string]interface{}{"name": plan.Name.ValueString()})

	createDto, diags := applicationModelToCreateDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, httpResp, err := r.client.Beta.AppsAPI.
		CreateSourceApp(ctx).
		SourceAppCreateDto(*createDto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Application", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Application", applicationErrDetail(err, httpResp))
		return
	}
	if created == nil || created.Id == nil {
		resp.Diagnostics.AddError("Error creating Application", "API returned a successful create response without an application id.")
		return
	}

	patchOps, diags := applicationCreatePatchOps(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(patchOps) > 0 {
		tflog.Debug(ctx, "Patching newly created Application", map[string]interface{}{"id": *created.Id, "patch_ops": len(patchOps)})
		_, httpResp, err = r.client.Beta.AppsAPI.
			PatchSourceApp(ctx, *created.Id).
			JsonPatchOperation(patchOps).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error patching newly created Application", map[string]interface{}{"id": *created.Id, "error": err.Error()})
			resp.Diagnostics.AddError("Error finalizing Application create", applicationErrDetail(err, httpResp))
			return
		}
	}

	state, _, diags := r.readApplicationState(ctx, *created.Id, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Application", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Application", map[string]interface{}{"id": state.Id.ValueString()})

	newState, notFound, diags := r.readApplicationState(ctx, state.Id.ValueString(), state)
	if notFound {
		tflog.Warn(ctx, "Application not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state applicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchOps, diags := applicationUpdatePatchOps(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(patchOps) > 0 {
		tflog.Debug(ctx, "Patching Application", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patchOps)})
		_, httpResp, err := r.client.Beta.AppsAPI.
			PatchSourceApp(ctx, state.Id.ValueString()).
			JsonPatchOperation(patchOps).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error updating Application", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
			resp.Diagnostics.AddError("Error updating Application", applicationErrDetail(err, httpResp))
			return
		}
	} else {
		tflog.Debug(ctx, "Application update required no patch operations", map[string]interface{}{"id": state.Id.ValueString()})
	}

	newState, _, diags := r.readApplicationState(ctx, state.Id.ValueString(), plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Application", map[string]interface{}{"id": newState.Id.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Application", map[string]interface{}{"id": state.Id.ValueString()})

	_, httpResp, err := r.client.Beta.AppsAPI.
		DeleteSourceApp(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Application already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Application", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Application", applicationErrDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Application", map[string]interface{}{"id": state.Id.ValueString()})
}

func (r *applicationResource) readApplicationState(ctx context.Context, id string, fallback applicationResourceModel) (applicationResourceModel, bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto, httpResp, err := r.client.Beta.AppsAPI.
		GetSourceApp(ctx, id).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return applicationResourceModel{}, true, diags
		}
		diags.AddError("Error reading Application", applicationErrDetail(err, httpResp))
		return applicationResourceModel{}, false, diags
	}

	accessProfileIDs, d := listApplicationAccessProfileIDs(ctx, r.client, id)
	diags.Append(d...)
	if diags.HasError() {
		return applicationResourceModel{}, false, diags
	}

	model, d := applicationDtoToModel(ctx, dto, accessProfileIDs, fallback)
	diags.Append(d...)
	return model, false, diags
}

func applicationModelToCreateDto(ctx context.Context, m applicationResourceModel) (*api_beta.SourceAppCreateDto, diag.Diagnostics) {
	var diags diag.Diagnostics

	accountSource, d := accountSourceToCreateAPI(m.AccountSource)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	dto := api_beta.NewSourceAppCreateDtoWithDefaults()
	dto.Name = m.Name.ValueString()
	dto.Description = m.Description.ValueString()
	dto.AccountSource = *accountSource

	if !m.MatchAllAccounts.IsNull() && !m.MatchAllAccounts.IsUnknown() {
		v := m.MatchAllAccounts.ValueBool()
		dto.MatchAllAccounts = &v
	}

	return dto, diags
}

func applicationCreatePatchOps(ctx context.Context, plan applicationResourceModel) ([]api_beta.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := make([]api_beta.JsonPatchOperation, 0, 5)

	if !plan.Owner.IsNull() && !plan.Owner.IsUnknown() {
		ownerMap, d := ownerToPatchMap(plan.Owner)
		diags.Append(d...)
		if !diags.HasError() && ownerMap != nil {
			patch = append(patch, applicationJSONPatchReplace("/owner", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&ownerMap)))
		}
	}

	if !plan.AccessProfileIds.IsNull() && !plan.AccessProfileIds.IsUnknown() {
		arr, d := stringSetToArrayInner(ctx, plan.AccessProfileIds)
		diags.Append(d...)
		if !diags.HasError() {
			patch = append(patch, applicationJSONPatchReplace("/accessProfiles", api_beta.ArrayOfArrayInnerAsUpdateMultiHostSourcesRequestInnerValue(&arr)))
		}
	}

	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool() {
		v := plan.Enabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/enabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.ProvisionRequestEnabled.IsNull() && !plan.ProvisionRequestEnabled.IsUnknown() && plan.ProvisionRequestEnabled.ValueBool() {
		v := plan.ProvisionRequestEnabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/provisionRequestEnabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.AppCenterEnabled.IsNull() && !plan.AppCenterEnabled.IsUnknown() && !plan.AppCenterEnabled.ValueBool() {
		v := plan.AppCenterEnabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/appCenterEnabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}

	return patch, diags
}

func applicationUpdatePatchOps(ctx context.Context, plan, state applicationResourceModel) ([]api_beta.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := make([]api_beta.JsonPatchOperation, 0, 9)

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		patch = append(patch, applicationJSONPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.Description.Equal(state.Description) {
		v := plan.Description.ValueString()
		patch = append(patch, applicationJSONPatchReplace("/description", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.Enabled.Equal(state.Enabled) {
		v := plan.Enabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/enabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.ProvisionRequestEnabled.Equal(state.ProvisionRequestEnabled) {
		v := plan.ProvisionRequestEnabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/provisionRequestEnabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.AppCenterEnabled.Equal(state.AppCenterEnabled) {
		v := plan.AppCenterEnabled.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/appCenterEnabled", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.MatchAllAccounts.Equal(state.MatchAllAccounts) {
		v := plan.MatchAllAccounts.ValueBool()
		patch = append(patch, applicationJSONPatchReplace("/matchAllAccounts", api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v)))
	}
	if !plan.AccountSource.Equal(state.AccountSource) {
		accountSourceMap, d := accountSourceToPatchMap(plan.AccountSource)
		diags.Append(d...)
		if !diags.HasError() && accountSourceMap != nil {
			patch = append(patch, applicationJSONPatchReplace("/accountSource", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&accountSourceMap)))
		}
	}
	if !plan.Owner.IsNull() && !plan.Owner.IsUnknown() && !plan.Owner.Equal(state.Owner) {
		ownerMap, d := ownerToPatchMap(plan.Owner)
		diags.Append(d...)
		if !diags.HasError() && ownerMap != nil {
			patch = append(patch, applicationJSONPatchReplace("/owner", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&ownerMap)))
		}
	}
	if !plan.AccessProfileIds.IsNull() && !plan.AccessProfileIds.IsUnknown() && !plan.AccessProfileIds.Equal(state.AccessProfileIds) {
		arr, d := stringSetToArrayInner(ctx, plan.AccessProfileIds)
		diags.Append(d...)
		if !diags.HasError() {
			patch = append(patch, applicationJSONPatchReplace("/accessProfiles", api_beta.ArrayOfArrayInnerAsUpdateMultiHostSourcesRequestInnerValue(&arr)))
		}
	}

	return patch, diags
}

func applicationDtoToModel(ctx context.Context, dto *api_beta.SourceApp, accessProfileIDs []string, fallback applicationResourceModel) (applicationResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	} else {
		model.Id = types.StringNull()
	}
	model.Name = types.StringPointerValue(dto.Name)
	model.Description = types.StringPointerValue(dto.Description)
	model.CloudAppId = types.StringPointerValue(dto.CloudAppId)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)
	model.Enabled = types.BoolPointerValue(dto.Enabled)
	model.ProvisionRequestEnabled = types.BoolPointerValue(dto.ProvisionRequestEnabled)
	model.AppCenterEnabled = types.BoolPointerValue(dto.AppCenterEnabled)
	model.MatchAllAccounts = types.BoolPointerValue(dto.MatchAllAccounts)

	owner, d := ownerFromAPI(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	accountSource, d := accountSourceFromAPI(ctx, dto.AccountSource.Get())
	diags.Append(d...)
	model.AccountSource = accountSource

	accessProfileSet, d := types.SetValueFrom(ctx, types.StringType, accessProfileIDs)
	diags.Append(d...)
	model.AccessProfileIds = accessProfileSet

	return model, diags
}

func ownerToPatchMap(v resource_application.OwnerValue) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	if v.Id.IsNull() || v.Id.IsUnknown() || v.OwnerType.IsNull() || v.OwnerType.IsUnknown() {
		diags.AddError("Invalid owner value", "owner.id and owner.type must be known before the application can be patched.")
		return nil, diags
	}
	return map[string]interface{}{
		"id":   v.Id.ValueString(),
		"type": v.OwnerType.ValueString(),
	}, diags
}

func ownerFromAPI(ctx context.Context, dto *api_beta.BaseReferenceDto) (resource_application.OwnerValue, diag.Diagnostics) {
	if dto == nil {
		return resource_application.NewOwnerValueNull(), nil
	}
	attrs := map[string]attr.Value{
		"id":   types.StringPointerValue(dto.Id),
		"name": types.StringPointerValue(dto.Name),
		"type": types.StringNull(),
	}
	if dto.Type != nil {
		attrs["type"] = types.StringValue(string(*dto.Type))
	}
	return resource_application.NewOwnerValue(resource_application.OwnerValue{}.AttributeTypes(ctx), attrs)
}

func accountSourceToCreateAPI(v resource_application.AccountSourceValue) (*api_beta.SourceAppCreateDtoAccountSource, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.Id.IsNull() || v.Id.IsUnknown() {
		diags.AddError("Invalid account_source value", "account_source.id must be known before the application can be created.")
		return nil, diags
	}
	dto := api_beta.NewSourceAppCreateDtoAccountSource(v.Id.ValueString())
	if !v.AccountSourceType.IsNull() && !v.AccountSourceType.IsUnknown() {
		t := v.AccountSourceType.ValueString()
		dto.Type = &t
	}
	if !v.Name.IsNull() && !v.Name.IsUnknown() {
		n := v.Name.ValueString()
		dto.Name = &n
	}
	return dto, diags
}

func accountSourceToPatchMap(v resource_application.AccountSourceValue) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	if v.Id.IsNull() || v.Id.IsUnknown() {
		diags.AddError("Invalid account_source value", "account_source.id must be known before the application can be patched.")
		return nil, diags
	}
	return map[string]interface{}{"id": v.Id.ValueString()}, diags
}

func accountSourceFromAPI(ctx context.Context, dto *api_beta.SourceAppAccountSource) (resource_application.AccountSourceValue, diag.Diagnostics) {
	if dto == nil {
		return resource_application.NewAccountSourceValueNull(), nil
	}

	passwordPolicies, diags := passwordPoliciesFromAPI(ctx, dto.PasswordPolicies)
	if diags.HasError() {
		return resource_application.NewAccountSourceValueUnknown(), diags
	}

	attrs := map[string]attr.Value{
		"id":                          types.StringPointerValue(dto.Id),
		"name":                        types.StringPointerValue(dto.Name),
		"password_policies":           passwordPolicies,
		"type":                        types.StringPointerValue(dto.Type),
		"use_for_password_management": types.BoolPointerValue(dto.UseForPasswordManagement),
	}
	return resource_application.NewAccountSourceValue(resource_application.AccountSourceValue{}.AttributeTypes(ctx), attrs)
}

func passwordPoliciesFromAPI(ctx context.Context, policies []api_beta.BaseReferenceDto) (types.List, diag.Diagnostics) {
	if len(policies) == 0 {
		return types.ListNull(resource_application.PasswordPoliciesValue{}.Type(ctx)), nil
	}

	var diags diag.Diagnostics
	values := make([]resource_application.PasswordPoliciesValue, 0, len(policies))
	for i := range policies {
		attrs := map[string]attr.Value{
			"id":   types.StringPointerValue(policies[i].Id),
			"name": types.StringPointerValue(policies[i].Name),
			"type": types.StringNull(),
		}
		if policies[i].Type != nil {
			attrs["type"] = types.StringValue(string(*policies[i].Type))
		}
		v, d := resource_application.NewPasswordPoliciesValue(resource_application.PasswordPoliciesValue{}.AttributeTypes(ctx), attrs)
		diags.Append(d...)
		values = append(values, v)
	}
	if diags.HasError() {
		return types.ListNull(resource_application.PasswordPoliciesValue{}.Type(ctx)), diags
	}
	listVal, d := types.ListValueFrom(ctx, resource_application.PasswordPoliciesValue{}.Type(ctx), values)
	diags.Append(d...)
	return listVal, diags
}

func listApplicationAccessProfileIDs(ctx context.Context, client *sailpoint.APIClient, applicationID string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if applicationID == "" {
		return []string{}, diags
	}
	ids := make([]string, 0)

	var offset int32
	for {
		page, httpResp, err := client.Beta.AppsAPI.
			ListAccessProfilesForSourceApp(ctx, applicationID).
			Limit(applicationListMaxLimit).
			Offset(offset).
			Execute()
		if err != nil {
			diags.AddError("Error listing Application access profiles", applicationErrDetail(err, httpResp))
			return nil, diags
		}
		for i := range page {
			if page[i].Id != nil {
				ids = append(ids, *page[i].Id)
			}
		}
		if len(page) < applicationListMaxLimit {
			break
		}
		offset += applicationListMaxLimit
	}

	return ids, diags
}

func stringSetToArrayInner(ctx context.Context, s types.Set) ([]api_beta.ArrayInner, diag.Diagnostics) {
	var ids []string
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	diags := s.ElementsAs(ctx, &ids, false)
	arr := make([]api_beta.ArrayInner, 0, len(ids))
	for i := range ids {
		id := ids[i]
		arr = append(arr, api_beta.ArrayInner{String: &id})
	}
	return arr, diags
}

func timeToStringValue(t *api_beta.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

func applicationErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func applicationJSONPatchReplace(path string, value api_beta.UpdateMultiHostSourcesRequestInnerValue) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}
