// Package entitlement_request_config_v1 is a pilot implementation of the
// entitlement request configuration resource generated from SailPoint's
// per-service v1 Entitlements OpenAPI spec, following the same hand-written
// adopt-existing CRUD pattern as entitlement_v1.
//
// This target manages the request/revocation approval configuration of an
// entitlement that already exists in IdentityNow/ISC. There is no distinct
// "entitlement request config" object with its own lifecycle:
//   - Create adopts an existing entitlement by `id`, reads its current request
//     configuration, and only then issues a PUT if the practitioner explicitly
//     configured changes.
//   - Read refreshes the current configuration from
//     GET /entitlements/v1/{id}/entitlement-request-config.
//   - Update also uses PUT /entitlements/v1/{id}/entitlement-request-config as
//     a full-document replace, but it first overlays the configured plan onto
//     the live API object so omitted Optional+Computed fields are preserved
//     rather than reset.
//   - Delete removes only Terraform state. It must not delete the underlying
//     entitlement or invent a "reset to defaults" API call.
package entitlement_request_config_v1

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/entitlements"

	"terraform-provider-identitynow/internal/provider/entitlement_request_config_v1/resource_entitlement_request_config"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*entitlementRequestConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*entitlementRequestConfigResource)(nil)
	_ resource.ResourceWithImportState = (*entitlementRequestConfigResource)(nil)
)

func NewEntitlementRequestConfigResource() resource.Resource {
	return &entitlementRequestConfigResource{}
}

type entitlementRequestConfigResource struct {
	client *sailpoint.APIClient
}

func (r *entitlementRequestConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement_request_config_v1"
}

func (r *entitlementRequestConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_entitlement_request_config.EntitlementRequestConfigResourceSchema(ctx)
	resp.Schema.Description = "Adopts and manages an existing entitlement's request/revocation approval configuration in IdentityNow/ISC by entitlement id."
	resp.Schema.MarkdownDescription = "Adopts and manages an existing entitlement's request/revocation approval configuration in " +
		"IdentityNow/ISC by entitlement `id`. This target does **not** create or delete the underlying entitlement: " +
		"Create and Update both call `PUT /entitlements/v1/{id}/entitlement-request-config`, while Delete removes only " +
		"Terraform state."
	patchEntitlementRequestConfigSchema(&resp.Schema)
	applyEntitlementRequestConfigUseStateForUnknown(&resp.Schema)
}

func patchEntitlementRequestConfigSchema(s *schema.Schema) {
	a, ok := s.Attributes["id"].(schema.StringAttribute)
	if !ok {
		return
	}
	a.Description = "ID of the existing entitlement whose request/revocation configuration should be adopted and managed."
	a.MarkdownDescription = "ID of the existing entitlement whose request/revocation configuration should be adopted and managed."
	s.Attributes["id"] = a
}

func (r *entitlementRequestConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = cp.GetClient()
}

func (r *entitlementRequestConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *entitlementRequestConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_entitlement_request_config.EntitlementRequestConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := entitlementRequestConfigID(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Adopting Entitlement request config", map[string]interface{}{"id": id})

	liveState, notFound, diags := r.readEntitlementRequestConfigState(ctx, id, emptyEntitlementRequestConfigModel(id))
	if notFound {
		resp.Diagnostics.AddError(
			"Error adopting Entitlement request config",
			fmt.Sprintf("Entitlement %q does not exist or its request configuration is no longer readable.", id),
		)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired, diags := entitlementRequestConfigMergePlanWithFallback(ctx, plan, liveState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !entitlementRequestConfigModelsEqual(desired, liveState) {
		tflog.Debug(ctx, "Replacing adopted Entitlement request config after initial GET", map[string]interface{}{"id": id})
		dto, diags := entitlementRequestConfigModelToAPI(ctx, desired)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		_, httpResp, err := r.client.EntitlementsAPI.
			PutEntitlementRequestConfigV1(ctx, id).
			EntitlementRequestConfig(*dto).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error finalizing adopted Entitlement request config", map[string]interface{}{"id": id, "error": err.Error()})
			resp.Diagnostics.AddError("Error finalizing Entitlement request config adoption", entitlementRequestConfigErrDetail(err, httpResp))
			return
		}
	}

	newState, notFound, diags := r.readEntitlementRequestConfigState(ctx, id, desired)
	if notFound {
		resp.Diagnostics.AddError(
			"Error reading adopted Entitlement request config",
			fmt.Sprintf("Entitlement %q request configuration disappeared immediately after adoption.", id),
		)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *entitlementRequestConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_entitlement_request_config.EntitlementRequestConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, notFound, diags := r.readEntitlementRequestConfigState(ctx, state.Id.ValueString(), state)
	if notFound {
		tflog.Warn(ctx, "Entitlement request config not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *entitlementRequestConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_entitlement_request_config.EntitlementRequestConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_entitlement_request_config.EntitlementRequestConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.Id.ValueString()
	tflog.Debug(ctx, "Updating Entitlement request config", map[string]interface{}{"id": id})

	liveState, notFound, diags := r.readEntitlementRequestConfigState(ctx, id, state)
	if notFound {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired, diags := entitlementRequestConfigMergePlanWithFallback(ctx, plan, liveState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !entitlementRequestConfigModelsEqual(desired, liveState) {
		dto, diags := entitlementRequestConfigModelToAPI(ctx, desired)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		_, httpResp, err := r.client.EntitlementsAPI.
			PutEntitlementRequestConfigV1(ctx, id).
			EntitlementRequestConfig(*dto).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error updating Entitlement request config", map[string]interface{}{"id": id, "error": err.Error()})
			resp.Diagnostics.AddError("Error updating Entitlement request config", entitlementRequestConfigErrDetail(err, httpResp))
			return
		}
	} else {
		tflog.Debug(ctx, "Entitlement request config update required no PUT", map[string]interface{}{"id": id})
	}

	newState, notFound, diags := r.readEntitlementRequestConfigState(ctx, id, desired)
	if notFound {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *entitlementRequestConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_entitlement_request_config.EntitlementRequestConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Removing Entitlement request config from Terraform state only", map[string]interface{}{"id": state.Id.ValueString()})
	resp.State.RemoveResource(ctx)
}

func (r *entitlementRequestConfigResource) readEntitlementRequestConfigState(ctx context.Context, id string, fallback resource_entitlement_request_config.EntitlementRequestConfigModel) (resource_entitlement_request_config.EntitlementRequestConfigModel, bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto, httpResp, err := r.client.EntitlementsAPI.
		GetEntitlementRequestConfigV1(ctx, id).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return resource_entitlement_request_config.EntitlementRequestConfigModel{}, true, diags
		}
		diags.AddError("Error reading Entitlement request config", entitlementRequestConfigErrDetail(err, httpResp))
		return resource_entitlement_request_config.EntitlementRequestConfigModel{}, false, diags
	}

	model, d := entitlementRequestConfigDtoToModel(ctx, id, dto, fallback)
	diags.Append(d...)
	return model, false, diags
}

func emptyEntitlementRequestConfigModel(id string) resource_entitlement_request_config.EntitlementRequestConfigModel {
	return resource_entitlement_request_config.EntitlementRequestConfigModel{
		AccessRequestConfig:     resource_entitlement_request_config.NewAccessRequestConfigValueNull(),
		Id:                      types.StringValue(id),
		RevocationRequestConfig: resource_entitlement_request_config.NewRevocationRequestConfigValueNull(),
	}
}

func entitlementRequestConfigID(model resource_entitlement_request_config.EntitlementRequestConfigModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model.Id.IsNull() || model.Id.IsUnknown() || model.Id.ValueString() == "" {
		diags.AddError(
			"Missing Entitlement id",
			"Configure the existing entitlement `id` before creating identitynow_entitlement_request_config_v1.",
		)
		return "", diags
	}
	return model.Id.ValueString(), diags
}

func entitlementRequestConfigMergePlanWithFallback(ctx context.Context, plan, fallback resource_entitlement_request_config.EntitlementRequestConfigModel) (resource_entitlement_request_config.EntitlementRequestConfigModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	merged := fallback
	if !plan.Id.IsNull() && !plan.Id.IsUnknown() && plan.Id.ValueString() != "" {
		merged.Id = plan.Id
	}

	accessRequestConfig, d := mergeAccessRequestConfigValue(ctx, plan.AccessRequestConfig, fallback.AccessRequestConfig)
	diags.Append(d...)
	merged.AccessRequestConfig = accessRequestConfig

	revocationRequestConfig, d := mergeRevocationRequestConfigValue(ctx, plan.RevocationRequestConfig, fallback.RevocationRequestConfig)
	diags.Append(d...)
	merged.RevocationRequestConfig = revocationRequestConfig

	return merged, diags
}

func mergeAccessRequestConfigValue(ctx context.Context, plan, fallback resource_entitlement_request_config.AccessRequestConfigValue) (resource_entitlement_request_config.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || plan.IsNull() {
		if !fallback.IsUnknown() {
			return fallback, diags
		}
		return resource_entitlement_request_config.NewAccessRequestConfigValueNull(), diags
	}

	maxDuration, d := mergeMaxPermittedAccessDurationValue(ctx, plan.MaxPermittedAccessDuration, fallback.MaxPermittedAccessDuration)
	diags.Append(d...)

	v, d := resource_entitlement_request_config.NewAccessRequestConfigValue(
		resource_entitlement_request_config.AccessRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"approval_schemes":              mergeApprovalSchemesList(ctx, plan.ApprovalSchemes, fallback.ApprovalSchemes),
			"denial_comment_required":       mergeBoolValue(plan.DenialCommentRequired, fallback.DenialCommentRequired),
			"max_permitted_access_duration": maxDuration,
			"reauthorization_required":      mergeBoolValue(plan.ReauthorizationRequired, fallback.ReauthorizationRequired),
			"request_comment_required":      mergeBoolValue(plan.RequestCommentRequired, fallback.RequestCommentRequired),
			"require_end_date":              mergeBoolValue(plan.RequireEndDate, fallback.RequireEndDate),
		},
	)
	diags.Append(d...)
	return v, diags
}

func mergeRevocationRequestConfigValue(ctx context.Context, plan, fallback resource_entitlement_request_config.RevocationRequestConfigValue) (resource_entitlement_request_config.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || plan.IsNull() {
		if !fallback.IsUnknown() {
			return fallback, diags
		}
		return resource_entitlement_request_config.NewRevocationRequestConfigValueNull(), diags
	}

	v, d := resource_entitlement_request_config.NewRevocationRequestConfigValue(
		resource_entitlement_request_config.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"revocation_approval_schemes": mergeRevocationApprovalSchemesList(ctx, plan.RevocationApprovalSchemes, fallback.RevocationApprovalSchemes),
		},
	)
	diags.Append(d...)
	return v, diags
}

func mergeApprovalSchemesList(ctx context.Context, plan, fallback basetypes.ListValue) basetypes.ListValue {
	if !plan.IsUnknown() {
		return plan
	}
	if !fallback.IsNull() && !fallback.IsUnknown() {
		return fallback
	}
	return types.ListNull(resource_entitlement_request_config.ApprovalSchemesValue{}.Type(ctx))
}

func mergeRevocationApprovalSchemesList(ctx context.Context, plan, fallback basetypes.ListValue) basetypes.ListValue {
	if !plan.IsUnknown() {
		return plan
	}
	if !fallback.IsNull() && !fallback.IsUnknown() {
		return fallback
	}
	return types.ListNull(resource_entitlement_request_config.RevocationApprovalSchemesValue{}.Type(ctx))
}

func mergeBoolValue(plan, fallback basetypes.BoolValue) basetypes.BoolValue {
	if !plan.IsUnknown() {
		return plan
	}
	if !fallback.IsNull() && !fallback.IsUnknown() {
		return fallback
	}
	return types.BoolNull()
}

func mergeMaxPermittedAccessDurationValue(ctx context.Context, plan, fallback basetypes.ObjectValue) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_entitlement_request_config.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)

	if plan.IsUnknown() {
		if !fallback.IsNull() && !fallback.IsUnknown() {
			return fallback, diags
		}
		return types.ObjectNull(attrTypes), diags
	}
	if plan.IsNull() {
		return plan, diags
	}

	merged := map[string]attr.Value{
		"time_unit": types.StringNull(),
		"value":     types.Int64Null(),
	}
	for name, value := range plan.Attributes() {
		merged[name] = value
	}

	if !fallback.IsNull() && !fallback.IsUnknown() {
		fallbackAttrs := fallback.Attributes()
		for name, value := range merged {
			if !value.IsUnknown() {
				continue
			}
			if fallbackValue, ok := fallbackAttrs[name]; ok && !fallbackValue.IsUnknown() {
				merged[name] = fallbackValue
				continue
			}
			switch name {
			case "time_unit":
				merged[name] = types.StringNull()
			case "value":
				merged[name] = types.Int64Null()
			}
		}
	}

	obj, d := types.ObjectValue(attrTypes, merged)
	diags.Append(d...)
	return obj, diags
}

func entitlementRequestConfigModelsEqual(a, b resource_entitlement_request_config.EntitlementRequestConfigModel) bool {
	return a.Id.Equal(b.Id) &&
		a.AccessRequestConfig.Equal(b.AccessRequestConfig) &&
		a.RevocationRequestConfig.Equal(b.RevocationRequestConfig)
}

func entitlementRequestConfigDtoToModel(ctx context.Context, id string, dto *entitlements.EntitlementRequestConfig, fallback resource_entitlement_request_config.EntitlementRequestConfigModel) (resource_entitlement_request_config.EntitlementRequestConfigModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := fallback
	model.Id = types.StringValue(id)

	accessRequestConfigDTO, accessRequestConfigOK := dto.GetAccessRequestConfigOk()
	accessRequestConfig, d := accessRequestConfigFromAPI(ctx, accessRequestConfigDTO, accessRequestConfigOK)
	diags.Append(d...)
	model.AccessRequestConfig = accessRequestConfig

	revocationRequestConfigDTO, revocationRequestConfigOK := dto.GetRevocationRequestConfigOk()
	revocationRequestConfig, d := revocationRequestConfigFromAPI(ctx, revocationRequestConfigDTO, revocationRequestConfigOK)
	diags.Append(d...)
	model.RevocationRequestConfig = revocationRequestConfig

	return model, diags
}

func accessRequestConfigFromAPI(ctx context.Context, dto *entitlements.EntitlementAccessRequestConfig, ok bool) (resource_entitlement_request_config.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !ok || dto == nil {
		return resource_entitlement_request_config.NewAccessRequestConfigValueNull(), diags
	}

	approvalSchemes, d := accessApprovalSchemesFromAPI(ctx, dto.ApprovalSchemes)
	diags.Append(d...)
	maxPermittedAccessDurationDTO, maxPermittedAccessDurationOK := dto.GetMaxPermittedAccessDurationOk()
	maxPermittedAccessDuration, d := maxPermittedAccessDurationFromAPI(ctx, maxPermittedAccessDurationDTO, maxPermittedAccessDurationOK)
	diags.Append(d...)

	v, d := resource_entitlement_request_config.NewAccessRequestConfigValue(
		resource_entitlement_request_config.AccessRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"approval_schemes":              approvalSchemes,
			"denial_comment_required":       types.BoolPointerValue(dto.DenialCommentRequired),
			"max_permitted_access_duration": maxPermittedAccessDuration,
			"reauthorization_required":      types.BoolPointerValue(dto.ReauthorizationRequired),
			"request_comment_required":      types.BoolPointerValue(dto.RequestCommentRequired),
			"require_end_date":              types.BoolPointerValue(dto.RequireEndDate),
		},
	)
	diags.Append(d...)
	return v, diags
}

func revocationRequestConfigFromAPI(ctx context.Context, dto *entitlements.EntitlementRevocationRequestConfig, ok bool) (resource_entitlement_request_config.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if !ok || dto == nil {
		return resource_entitlement_request_config.NewRevocationRequestConfigValueNull(), diags
	}

	approvalSchemes, d := revocationApprovalSchemesFromAPI(ctx, dto.ApprovalSchemes)
	diags.Append(d...)
	v, d := resource_entitlement_request_config.NewRevocationRequestConfigValue(
		resource_entitlement_request_config.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"revocation_approval_schemes": approvalSchemes,
		},
	)
	diags.Append(d...)
	return v, diags
}

func accessApprovalSchemesFromAPI(ctx context.Context, items []entitlements.EntitlementApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_entitlement_request_config.ApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_entitlement_request_config.ApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_entitlement_request_config.NewApprovalSchemesValue(
			resource_entitlement_request_config.ApprovalSchemesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"approver_id":   nullableStringValue(item.GetApproverIdOk()),
				"approver_type": types.StringPointerValue(item.ApproverType),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func revocationApprovalSchemesFromAPI(ctx context.Context, items []entitlements.EntitlementApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_entitlement_request_config.RevocationApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_entitlement_request_config.RevocationApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_entitlement_request_config.NewRevocationApprovalSchemesValue(
			resource_entitlement_request_config.RevocationApprovalSchemesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"approver_id":   nullableStringValue(item.GetApproverIdOk()),
				"approver_type": types.StringPointerValue(item.ApproverType),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func maxPermittedAccessDurationFromAPI(ctx context.Context, dto *entitlements.EntitlementAccessRequestConfigMaxPermittedAccessDuration, ok bool) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_entitlement_request_config.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)
	if !ok || dto == nil {
		return types.ObjectNull(attrTypes), diags
	}

	value := types.Int64Null()
	if dto.Value != nil {
		value = types.Int64Value(int64(*dto.Value))
	}

	v, d := resource_entitlement_request_config.NewMaxPermittedAccessDurationValue(
		attrTypes,
		map[string]attr.Value{
			"time_unit": types.StringPointerValue(dto.TimeUnit),
			"value":     value,
		},
	)
	diags.Append(d...)
	obj, d := v.ToObjectValue(ctx)
	diags.Append(d...)
	return obj, diags
}

func entitlementRequestConfigModelToAPI(ctx context.Context, model resource_entitlement_request_config.EntitlementRequestConfigModel) (*entitlements.EntitlementRequestConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto := entitlements.NewEntitlementRequestConfig()

	accessRequestConfig, d := accessRequestConfigModelToAPI(ctx, model.AccessRequestConfig)
	diags.Append(d...)
	if accessRequestConfig != nil {
		dto.SetAccessRequestConfig(*accessRequestConfig)
	}

	revocationRequestConfig, d := revocationRequestConfigModelToAPI(ctx, model.RevocationRequestConfig)
	diags.Append(d...)
	if revocationRequestConfig != nil {
		dto.SetRevocationRequestConfig(*revocationRequestConfig)
	}

	return dto, diags
}

func accessRequestConfigModelToAPI(ctx context.Context, model resource_entitlement_request_config.AccessRequestConfigValue) (*entitlements.EntitlementAccessRequestConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model.IsNull() {
		return nil, diags
	}
	if model.IsUnknown() {
		diags.AddError("Unknown access_request_config value", "access_request_config must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}

	dto := &entitlements.EntitlementAccessRequestConfig{}

	approvalSchemes, d := accessApprovalSchemesModelToAPI(ctx, model.ApprovalSchemes)
	diags.Append(d...)
	if !model.ApprovalSchemes.IsNull() {
		dto.SetApprovalSchemes(approvalSchemes)
	}

	if !model.DenialCommentRequired.IsNull() {
		dto.SetDenialCommentRequired(model.DenialCommentRequired.ValueBool())
	}
	if !model.ReauthorizationRequired.IsNull() {
		dto.SetReauthorizationRequired(model.ReauthorizationRequired.ValueBool())
	}
	if !model.RequestCommentRequired.IsNull() {
		dto.SetRequestCommentRequired(model.RequestCommentRequired.ValueBool())
	}
	if !model.RequireEndDate.IsNull() {
		dto.SetRequireEndDate(model.RequireEndDate.ValueBool())
	}

	maxPermittedAccessDuration, d := maxPermittedAccessDurationModelToAPI(model.MaxPermittedAccessDuration)
	diags.Append(d...)
	if maxPermittedAccessDuration != nil {
		dto.SetMaxPermittedAccessDuration(*maxPermittedAccessDuration)
	}

	return dto, diags
}

func revocationRequestConfigModelToAPI(ctx context.Context, model resource_entitlement_request_config.RevocationRequestConfigValue) (*entitlements.EntitlementRevocationRequestConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if model.IsNull() {
		return nil, diags
	}
	if model.IsUnknown() {
		diags.AddError("Unknown revocation_request_config value", "revocation_request_config must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}

	dto := &entitlements.EntitlementRevocationRequestConfig{}
	approvalSchemes, d := revocationApprovalSchemesModelToAPI(ctx, model.RevocationApprovalSchemes)
	diags.Append(d...)
	if !model.RevocationApprovalSchemes.IsNull() {
		// The generated Terraform schema/model had to rename this nested
		// attribute to `revocation_approval_schemes` to avoid a duplicate Go
		// type declaration; the upstream API field name remains approvalSchemes.
		dto.SetApprovalSchemes(approvalSchemes)
	}
	return dto, diags
}

func accessApprovalSchemesModelToAPI(ctx context.Context, list basetypes.ListValue) ([]entitlements.EntitlementApprovalScheme, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() {
		return nil, diags
	}
	if list.IsUnknown() {
		diags.AddError("Unknown approval_schemes value", "access_request_config.approval_schemes must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}

	var items []resource_entitlement_request_config.ApprovalSchemesValue
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	out := make([]entitlements.EntitlementApprovalScheme, 0, len(items))
	for _, item := range items {
		scheme := entitlements.EntitlementApprovalScheme{}
		if !item.ApproverType.IsNull() && !item.ApproverType.IsUnknown() {
			scheme.SetApproverType(item.ApproverType.ValueString())
		}
		// approver_id is Optional+Computed with no PlanModifier resolving it,
		// so Terraform reports it as unknown whenever a practitioner omits it
		// from config (it's only ever practitioner-supplied - the API never
		// computes a server-side default for it). Treat unknown the same as
		// null here (simply don't send it) rather than erroring; the
		// subsequent GET/PUT-response-driven Read repopulates real state
		// afterward, so this can never produce a stale/incorrect value.
		if !item.ApproverId.IsNull() && !item.ApproverId.IsUnknown() {
			scheme.SetApproverId(item.ApproverId.ValueString())
		}
		out = append(out, scheme)
	}
	return out, diags
}

func revocationApprovalSchemesModelToAPI(ctx context.Context, list basetypes.ListValue) ([]entitlements.EntitlementApprovalScheme, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() {
		return nil, diags
	}
	if list.IsUnknown() {
		diags.AddError("Unknown revocation_approval_schemes value", "revocation_request_config.revocation_approval_schemes must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}

	var items []resource_entitlement_request_config.RevocationApprovalSchemesValue
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	out := make([]entitlements.EntitlementApprovalScheme, 0, len(items))
	for _, item := range items {
		scheme := entitlements.EntitlementApprovalScheme{}
		if !item.ApproverType.IsNull() && !item.ApproverType.IsUnknown() {
			scheme.SetApproverType(item.ApproverType.ValueString())
		}
		// See the matching comment in accessApprovalSchemesModelToAPI above -
		// approver_id being unknown here just means the practitioner didn't
		// set it; treat it as null (omit) rather than erroring.
		if !item.ApproverId.IsNull() && !item.ApproverId.IsUnknown() {
			scheme.SetApproverId(item.ApproverId.ValueString())
		}
		out = append(out, scheme)
	}
	return out, diags
}

func maxPermittedAccessDurationModelToAPI(obj basetypes.ObjectValue) (*entitlements.EntitlementAccessRequestConfigMaxPermittedAccessDuration, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("Unknown max_permitted_access_duration value", "access_request_config.max_permitted_access_duration must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}

	attrs := obj.Attributes()
	timeUnit, _ := attrs["time_unit"].(basetypes.StringValue)
	value, _ := attrs["value"].(basetypes.Int64Value)

	if timeUnit.IsUnknown() || value.IsUnknown() {
		diags.AddError("Unknown max_permitted_access_duration child value", "access_request_config.max_permitted_access_duration.time_unit and value must be fully known before the entitlement request configuration can be sent to the API.")
		return nil, diags
	}
	if timeUnit.IsNull() && value.IsNull() {
		return nil, diags
	}

	dto := &entitlements.EntitlementAccessRequestConfigMaxPermittedAccessDuration{}
	if !timeUnit.IsNull() {
		dto.SetTimeUnit(timeUnit.ValueString())
	}
	if !value.IsNull() {
		if value.ValueInt64() > math.MaxInt32 || value.ValueInt64() < math.MinInt32 {
			diags.AddError(
				"Invalid max_permitted_access_duration.value",
				fmt.Sprintf("access_request_config.max_permitted_access_duration.value=%d is outside the API's supported int32 range.", value.ValueInt64()),
			)
			return nil, diags
		}
		dto.SetValue(int32(value.ValueInt64()))
	}
	return dto, diags
}

func nullableStringValue(v *string, ok bool) types.String {
	if !ok || v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func entitlementRequestConfigErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
