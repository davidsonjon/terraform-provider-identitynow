package role_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v3/roles"

	"terraform-provider-identitynow/internal/provider/role_v1/resource_role"
)

// TestRoleDtoToModel_ReadBackWhenUnconfigured verifies that, when a
// pass-through-only block is unconfigured (fallback is Null), roleDtoToModel
// populates it from the API response DTO instead of leaving it Null - the
// "deeper read-back" behavior added on top of the original pass-through-only
// implementation.
func TestRoleDtoToModel_ReadBackWhenUnconfigured(t *testing.T) {
	ctx := context.Background()
	fallback := minimalRoleModel()
	fallback.Owner = ownerModel(t, "owner-id", "", "IDENTITY")

	roleId := "role-id"
	approverId := "approver-id"
	approverType := "GOVERNANCE_GROUP"
	commentsRequired := true
	formDefId := "form-def-id"
	maxDurationValue := int32(30)
	maxDurationUnit := "DAYS"

	criteriaOp := roles.RoleCriteriaOperation("AND")
	criteriaKeyType := roles.RoleCriteriaKeyType("IDENTITY")
	childOp := roles.RoleCriteriaOperation("EQUALS")
	childStringValue := "engineering"

	membershipType := roles.RoleMembershipSelectorType("STANDARD")

	dto := &roles.Role{
		Id:   &roleId,
		Name: "test-role",
		Owner: *roles.NewNullableOwnerReference(&roles.OwnerReference{
			Id:   func() *string { s := "owner-id"; return &s }(),
			Type: func() *string { s := "IDENTITY"; return &s }(),
		}),
		AccessModelMetadata: &roles.AttributeDTOList{
			Attributes: []roles.AttributeDTO{
				{
					Key:  func() *string { s := "risk"; return &s }(),
					Name: func() *string { s := "Risk"; return &s }(),
				},
			},
		},
		AccessRequestConfig: &roles.RequestabilityForRole{
			CommentsRequired: *roles.NewNullableBool(&commentsRequired),
			ApprovalSchemes: []roles.ApprovalSchemeForRole{
				{
					ApproverId:   *roles.NewNullableString(&approverId),
					ApproverType: &approverType,
				},
			},
			FormDefinitionId: *roles.NewNullableString(&formDefId),
			MaxPermittedAccessDuration: *roles.NewNullableAccessDuration(&roles.AccessDuration{
				Value:    &maxDurationValue,
				TimeUnit: &maxDurationUnit,
			}),
		},
		RevocationRequestConfig: &roles.RevocabilityForRole{
			CommentsRequired: *roles.NewNullableBool(&commentsRequired),
			ApprovalSchemes: []roles.ApprovalSchemeForRole{
				{
					ApproverId:   *roles.NewNullableString(&approverId),
					ApproverType: &approverType,
				},
			},
		},
		Membership: *roles.NewNullableRoleMembershipSelector(&roles.RoleMembershipSelector{
			Type: &membershipType,
			Criteria: *roles.NewNullableRoleCriteriaLevel1(&roles.RoleCriteriaLevel1{
				Operation: &criteriaOp,
				Key: *roles.NewNullableRoleCriteriaKey(&roles.RoleCriteriaKey{
					Type:     criteriaKeyType,
					Property: "department",
				}),
				Children: []roles.RoleCriteriaLevel2{
					{
						Operation:   &childOp,
						StringValue: *roles.NewNullableString(&childStringValue),
					},
				},
			}),
		}),
	}

	model, diags := roleDtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("roleDtoToModel returned diagnostics: %v", diags)
	}

	if model.AccessModelMetadata.IsNull() {
		t.Fatal("AccessModelMetadata is Null, want populated from API response")
	}
	attrsList := model.AccessModelMetadata.Attributes
	if attrsList.IsNull() || len(attrsList.Elements()) != 1 {
		t.Fatalf("AccessModelMetadata.Attributes = %v, want 1 element", attrsList)
	}

	if model.AccessRequestConfig.IsNull() {
		t.Fatal("AccessRequestConfig is Null, want populated from API response")
	}
	if !model.AccessRequestConfig.CommentsRequired.ValueBool() {
		t.Errorf("AccessRequestConfig.CommentsRequired = %v, want true", model.AccessRequestConfig.CommentsRequired)
	}
	if model.AccessRequestConfig.FormDefinitionId.ValueString() != formDefId {
		t.Errorf("AccessRequestConfig.FormDefinitionId = %q, want %q", model.AccessRequestConfig.FormDefinitionId.ValueString(), formDefId)
	}
	if model.AccessRequestConfig.DimensionSchema.IsNull() != true {
		t.Errorf("AccessRequestConfig.DimensionSchema should remain Null (no SDK counterpart), got %v", model.AccessRequestConfig.DimensionSchema)
	}
	if len(model.AccessRequestConfig.ApprovalSchemes.Elements()) != 1 {
		t.Errorf("AccessRequestConfig.ApprovalSchemes has %d elements, want 1", len(model.AccessRequestConfig.ApprovalSchemes.Elements()))
	}

	if model.RevocationRequestConfig.IsNull() {
		t.Fatal("RevocationRequestConfig is Null, want populated from API response")
	}
	if len(model.RevocationRequestConfig.RevocationApprovalSchemes.Elements()) != 1 {
		t.Errorf("RevocationRequestConfig.RevocationApprovalSchemes has %d elements, want 1", len(model.RevocationRequestConfig.RevocationApprovalSchemes.Elements()))
	}

	if model.Membership.IsNull() {
		t.Fatal("Membership is Null, want populated from API response")
	}
	if model.Membership.MembershipType.ValueString() != "STANDARD" {
		t.Errorf("Membership.Type = %q, want %q", model.Membership.MembershipType.ValueString(), "STANDARD")
	}
	if model.Membership.Criteria.IsNull() {
		t.Fatal("Membership.Criteria is Null, want populated from API response")
	}
	criteriaAttrs := model.Membership.Criteria.Attributes()
	criteriaOperationAttr, ok := criteriaAttrs["operation"].(types.String)
	if !ok || criteriaOperationAttr.ValueString() != "AND" {
		t.Errorf("Membership.Criteria.operation = %v, want %q", criteriaAttrs["operation"], "AND")
	}
	criteriaChildren, ok := criteriaAttrs["children"].(types.List)
	if !ok || len(criteriaChildren.Elements()) != 1 {
		t.Fatalf("Membership.Criteria.children = %v, want 1 element", criteriaAttrs["children"])
	}
}

// TestRoleDtoToModel_PassThroughWhenConfigured verifies that a
// practitioner-configured pass-through-only block is left untouched by the
// read-back logic even when the API response contains different data - this
// provider never sends these blocks to the API on Create/Update, so
// overwriting a configured value with the API's response would produce a
// permanent, non-convergent diff (see resource_role_readback.go's doc
// comment).
func TestRoleDtoToModel_PassThroughWhenConfigured(t *testing.T) {
	ctx := context.Background()
	fallback := minimalRoleModel()
	fallback.Owner = ownerModel(t, "owner-id", "", "IDENTITY")

	configured, diags := resource_role.NewRevocationRequestConfigValue(
		resource_role.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"comments_required":           types.BoolValue(false),
			"denial_comments_required":    types.BoolNull(),
			"revocation_approval_schemes": types.ListNull(resource_role.RevocationApprovalSchemesValue{}.Type(ctx)),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewRevocationRequestConfigValue returned diagnostics: %v", diags)
	}
	fallback.RevocationRequestConfig = configured

	roleId := "role-id"
	apiCommentsRequired := true
	apiApproverId := "approver-id"
	apiApproverType := "GOVERNANCE_GROUP"

	dto := &roles.Role{
		Id:   &roleId,
		Name: "test-role",
		Owner: *roles.NewNullableOwnerReference(&roles.OwnerReference{
			Id:   func() *string { s := "owner-id"; return &s }(),
			Type: func() *string { s := "IDENTITY"; return &s }(),
		}),
		RevocationRequestConfig: &roles.RevocabilityForRole{
			CommentsRequired: *roles.NewNullableBool(&apiCommentsRequired),
			ApprovalSchemes: []roles.ApprovalSchemeForRole{
				{
					ApproverId:   *roles.NewNullableString(&apiApproverId),
					ApproverType: &apiApproverType,
				},
			},
		},
	}

	model, diags := roleDtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("roleDtoToModel returned diagnostics: %v", diags)
	}

	// The configured value (comments_required = false, no approval schemes)
	// must survive unchanged, even though the API's response says
	// comments_required = true with one approval scheme.
	if model.RevocationRequestConfig.CommentsRequired.ValueBool() {
		t.Errorf("RevocationRequestConfig.CommentsRequired = %v, want false (configured value preserved)", model.RevocationRequestConfig.CommentsRequired)
	}
	if len(model.RevocationRequestConfig.RevocationApprovalSchemes.Elements()) != 0 {
		t.Errorf("RevocationRequestConfig.RevocationApprovalSchemes has %d elements, want 0 (configured value preserved)",
			len(model.RevocationRequestConfig.RevocationApprovalSchemes.Elements()))
	}
}
