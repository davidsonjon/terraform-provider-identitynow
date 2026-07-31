package entitlement_request_config_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/entitlement_request_config_v1/resource_entitlement_request_config"
)

func minimalEntitlementRequestConfigModel() resource_entitlement_request_config.EntitlementRequestConfigModel {
	return resource_entitlement_request_config.EntitlementRequestConfigModel{
		AccessRequestConfig:     resource_entitlement_request_config.NewAccessRequestConfigValueNull(),
		Id:                      types.StringNull(),
		RevocationRequestConfig: resource_entitlement_request_config.NewRevocationRequestConfigValueNull(),
	}
}

func accessApprovalSchemeModel(t *testing.T, approverType string, approverID *string) resource_entitlement_request_config.ApprovalSchemesValue {
	t.Helper()

	attrs := map[string]attr.Value{
		"approver_id":   types.StringNull(),
		"approver_type": types.StringValue(approverType),
	}
	if approverID != nil {
		attrs["approver_id"] = types.StringValue(*approverID)
	}

	v, diags := resource_entitlement_request_config.NewApprovalSchemesValue(
		resource_entitlement_request_config.ApprovalSchemesValue{}.AttributeTypes(context.Background()),
		attrs,
	)
	if diags.HasError() {
		t.Fatalf("NewApprovalSchemesValue returned diagnostics: %v", diags)
	}
	return v
}

func revocationApprovalSchemeModel(t *testing.T, approverType string, approverID *string) resource_entitlement_request_config.RevocationApprovalSchemesValue {
	t.Helper()

	attrs := map[string]attr.Value{
		"approver_id":   types.StringNull(),
		"approver_type": types.StringValue(approverType),
	}
	if approverID != nil {
		attrs["approver_id"] = types.StringValue(*approverID)
	}

	v, diags := resource_entitlement_request_config.NewRevocationApprovalSchemesValue(
		resource_entitlement_request_config.RevocationApprovalSchemesValue{}.AttributeTypes(context.Background()),
		attrs,
	)
	if diags.HasError() {
		t.Fatalf("NewRevocationApprovalSchemesValue returned diagnostics: %v", diags)
	}
	return v
}

func approvalSchemesList(t *testing.T, items ...resource_entitlement_request_config.ApprovalSchemesValue) basetypes.ListValue {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), resource_entitlement_request_config.ApprovalSchemesValue{}.Type(context.Background()), items)
	if diags.HasError() {
		t.Fatalf("ListValueFrom returned diagnostics: %v", diags)
	}
	return list
}

func revocationApprovalSchemesList(t *testing.T, items ...resource_entitlement_request_config.RevocationApprovalSchemesValue) basetypes.ListValue {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), resource_entitlement_request_config.RevocationApprovalSchemesValue{}.Type(context.Background()), items)
	if diags.HasError() {
		t.Fatalf("ListValueFrom returned diagnostics: %v", diags)
	}
	return list
}

func maxPermittedAccessDurationObject(t *testing.T, value int64, timeUnit string) basetypes.ObjectValue {
	t.Helper()
	v, diags := resource_entitlement_request_config.NewMaxPermittedAccessDurationValue(
		resource_entitlement_request_config.MaxPermittedAccessDurationValue{}.AttributeTypes(context.Background()),
		map[string]attr.Value{
			"time_unit": types.StringValue(timeUnit),
			"value":     types.Int64Value(value),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewMaxPermittedAccessDurationValue returned diagnostics: %v", diags)
	}
	obj, diags := v.ToObjectValue(context.Background())
	if diags.HasError() {
		t.Fatalf("ToObjectValue returned diagnostics: %v", diags)
	}
	return obj
}

func accessRequestConfigModel(t *testing.T) resource_entitlement_request_config.AccessRequestConfigValue {
	t.Helper()
	approvalID := "gov-group-id"
	v, diags := resource_entitlement_request_config.NewAccessRequestConfigValue(
		resource_entitlement_request_config.AccessRequestConfigValue{}.AttributeTypes(context.Background()),
		map[string]attr.Value{
			"approval_schemes":              approvalSchemesList(t, accessApprovalSchemeModel(t, "MANAGER", nil), accessApprovalSchemeModel(t, "GOVERNANCE_GROUP", &approvalID)),
			"denial_comment_required":       types.BoolValue(true),
			"max_permitted_access_duration": maxPermittedAccessDurationObject(t, 30, "DAYS"),
			"reauthorization_required":      types.BoolValue(true),
			"request_comment_required":      types.BoolValue(false),
			"require_end_date":              types.BoolValue(true),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewAccessRequestConfigValue returned diagnostics: %v", diags)
	}
	return v
}

func revocationRequestConfigModel(t *testing.T) resource_entitlement_request_config.RevocationRequestConfigValue {
	t.Helper()
	workflowID := "workflow-id"
	v, diags := resource_entitlement_request_config.NewRevocationRequestConfigValue(
		resource_entitlement_request_config.RevocationRequestConfigValue{}.AttributeTypes(context.Background()),
		map[string]attr.Value{
			"revocation_approval_schemes": revocationApprovalSchemesList(t, revocationApprovalSchemeModel(t, "WORKFLOW", &workflowID)),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewRevocationRequestConfigValue returned diagnostics: %v", diags)
	}
	return v
}

func TestEntitlementRequestConfigDtoToModel(t *testing.T) {
	ctx := context.Background()

	t.Run("populated dto", func(t *testing.T) {
		fallback := minimalEntitlementRequestConfigModel()

		dto := api_beta.NewEntitlementRequestConfig()

		access := api_beta.NewEntitlementAccessRequestConfig()
		access.SetApprovalSchemes([]api_beta.EntitlementApprovalScheme{
			{ApproverType: strPtr("MANAGER")},
			{ApproverType: strPtr("GOVERNANCE_GROUP"), ApproverId: *api_beta.NewNullableString(strPtr("gov-group-id"))},
		})
		access.SetRequestCommentRequired(false)
		access.SetDenialCommentRequired(true)
		access.SetReauthorizationRequired(true)
		access.SetRequireEndDate(true)
		duration := api_beta.NewPendingApprovalMaxPermittedAccessDuration()
		duration.SetValue(30)
		duration.SetTimeUnit("DAYS")
		access.SetMaxPermittedAccessDuration(*duration)
		dto.SetAccessRequestConfig(*access)

		revocation := api_beta.NewEntitlementRevocationRequestConfig()
		revocation.SetApprovalSchemes([]api_beta.EntitlementApprovalScheme{
			{ApproverType: strPtr("WORKFLOW"), ApproverId: *api_beta.NewNullableString(strPtr("workflow-id"))},
		})
		dto.SetRevocationRequestConfig(*revocation)

		model, diags := entitlementRequestConfigDtoToModel(ctx, "entitlement-id", dto, fallback)
		if diags.HasError() {
			t.Fatalf("entitlementRequestConfigDtoToModel returned diagnostics: %v", diags)
		}

		if model.Id.ValueString() != "entitlement-id" {
			t.Fatalf("Id = %q, want entitlement-id", model.Id.ValueString())
		}
		if model.AccessRequestConfig.IsNull() {
			t.Fatalf("AccessRequestConfig is null, want populated")
		}
		if model.RevocationRequestConfig.IsNull() {
			t.Fatalf("RevocationRequestConfig is null, want populated")
		}
		if !model.AccessRequestConfig.DenialCommentRequired.ValueBool() {
			t.Errorf("DenialCommentRequired = false, want true")
		}
		if !model.AccessRequestConfig.ReauthorizationRequired.ValueBool() {
			t.Errorf("ReauthorizationRequired = false, want true")
		}
		if model.AccessRequestConfig.RequestCommentRequired.ValueBool() {
			t.Errorf("RequestCommentRequired = true, want false")
		}
		if !model.AccessRequestConfig.RequireEndDate.ValueBool() {
			t.Errorf("RequireEndDate = false, want true")
		}

		var accessSchemes []resource_entitlement_request_config.ApprovalSchemesValue
		if diags := model.AccessRequestConfig.ApprovalSchemes.ElementsAs(ctx, &accessSchemes, false); diags.HasError() {
			t.Fatalf("ApprovalSchemes.ElementsAs returned diagnostics: %v", diags)
		}
		if len(accessSchemes) != 2 {
			t.Fatalf("len(accessSchemes) = %d, want 2", len(accessSchemes))
		}
		if accessSchemes[1].ApproverId.ValueString() != "gov-group-id" {
			t.Errorf("ApprovalSchemes[1].ApproverId = %q, want gov-group-id", accessSchemes[1].ApproverId.ValueString())
		}

		durationAttrs := model.AccessRequestConfig.MaxPermittedAccessDuration.Attributes()
		if got := durationAttrs["time_unit"].(basetypes.StringValue).ValueString(); got != "DAYS" {
			t.Errorf("MaxPermittedAccessDuration.time_unit = %q, want DAYS", got)
		}
		if got := durationAttrs["value"].(basetypes.Int64Value).ValueInt64(); got != 30 {
			t.Errorf("MaxPermittedAccessDuration.value = %d, want 30", got)
		}

		var revocationSchemes []resource_entitlement_request_config.RevocationApprovalSchemesValue
		if diags := model.RevocationRequestConfig.RevocationApprovalSchemes.ElementsAs(ctx, &revocationSchemes, false); diags.HasError() {
			t.Fatalf("RevocationApprovalSchemes.ElementsAs returned diagnostics: %v", diags)
		}
		if len(revocationSchemes) != 1 {
			t.Fatalf("len(revocationSchemes) = %d, want 1", len(revocationSchemes))
		}
		if revocationSchemes[0].ApproverType.ValueString() != "WORKFLOW" {
			t.Errorf("RevocationApprovalSchemes[0].ApproverType = %q, want WORKFLOW", revocationSchemes[0].ApproverType.ValueString())
		}
	})

	t.Run("missing nested configs", func(t *testing.T) {
		fallback := minimalEntitlementRequestConfigModel()
		model, diags := entitlementRequestConfigDtoToModel(ctx, "entitlement-id", api_beta.NewEntitlementRequestConfig(), fallback)
		if diags.HasError() {
			t.Fatalf("entitlementRequestConfigDtoToModel returned diagnostics: %v", diags)
		}
		if !model.AccessRequestConfig.IsNull() {
			t.Errorf("AccessRequestConfig.IsNull() = false, want true")
		}
		if !model.RevocationRequestConfig.IsNull() {
			t.Errorf("RevocationRequestConfig.IsNull() = false, want true")
		}
	})
}

func TestEntitlementRequestConfigModelToAPI(t *testing.T) {
	ctx := context.Background()

	model := resource_entitlement_request_config.EntitlementRequestConfigModel{
		AccessRequestConfig:     accessRequestConfigModel(t),
		Id:                      types.StringValue("entitlement-id"),
		RevocationRequestConfig: revocationRequestConfigModel(t),
	}

	dto, diags := entitlementRequestConfigModelToAPI(ctx, model)
	if diags.HasError() {
		t.Fatalf("entitlementRequestConfigModelToAPI returned diagnostics: %v", diags)
	}

	access, ok := dto.GetAccessRequestConfigOk()
	if !ok || access == nil {
		t.Fatalf("GetAccessRequestConfigOk() = nil,false, want populated config")
	}
	if got := access.GetDenialCommentRequired(); !got {
		t.Errorf("GetDenialCommentRequired() = false, want true")
	}
	if got := access.GetReauthorizationRequired(); !got {
		t.Errorf("GetReauthorizationRequired() = false, want true")
	}
	if got := access.GetRequestCommentRequired(); got {
		t.Errorf("GetRequestCommentRequired() = true, want false")
	}
	if got := access.GetRequireEndDate(); !got {
		t.Errorf("GetRequireEndDate() = false, want true")
	}

	accessSchemes := access.GetApprovalSchemes()
	if len(accessSchemes) != 2 {
		t.Fatalf("len(access.GetApprovalSchemes()) = %d, want 2", len(accessSchemes))
	}
	if accessSchemes[0].GetApproverType() != "MANAGER" {
		t.Errorf("ApprovalSchemes[0].ApproverType = %q, want MANAGER", accessSchemes[0].GetApproverType())
	}
	if accessSchemes[1].GetApproverId() != "gov-group-id" {
		t.Errorf("ApprovalSchemes[1].ApproverId = %q, want gov-group-id", accessSchemes[1].GetApproverId())
	}

	duration, ok := access.GetMaxPermittedAccessDurationOk()
	if !ok || duration == nil {
		t.Fatalf("GetMaxPermittedAccessDurationOk() = nil,false, want populated duration")
	}
	if got := duration.GetValue(); got != 30 {
		t.Errorf("Duration.GetValue() = %d, want 30", got)
	}
	if got := duration.GetTimeUnit(); got != "DAYS" {
		t.Errorf("Duration.GetTimeUnit() = %q, want DAYS", got)
	}

	revocation, ok := dto.GetRevocationRequestConfigOk()
	if !ok || revocation == nil {
		t.Fatalf("GetRevocationRequestConfigOk() = nil,false, want populated config")
	}
	revocationSchemes := revocation.GetApprovalSchemes()
	if len(revocationSchemes) != 1 {
		t.Fatalf("len(revocation.GetApprovalSchemes()) = %d, want 1", len(revocationSchemes))
	}
	if revocationSchemes[0].GetApproverType() != "WORKFLOW" {
		t.Errorf("Revocation ApprovalSchemes[0].ApproverType = %q, want WORKFLOW", revocationSchemes[0].GetApproverType())
	}
	if revocationSchemes[0].GetApproverId() != "workflow-id" {
		t.Errorf("Revocation ApprovalSchemes[0].ApproverId = %q, want workflow-id", revocationSchemes[0].GetApproverId())
	}
}

func strPtr(v string) *string { return &v }
