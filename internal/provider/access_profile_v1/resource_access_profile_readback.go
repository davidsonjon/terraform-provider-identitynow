package access_profile_v1

// This file provides deeper read-back conversion for the pass-through-only
// blocks documented in resource_access_profile.go's package doc:
// access_model_metadata, access_request_config, and revocation_request_config.
// Unlike the rest of accessProfileDtoToModel (which prefers the fallback
// plan/prior-state value for these blocks), the functions here build a real
// Terraform value from the API response DTO so that IdentityNow-side defaults
// and drift are visible in `terraform plan`/`refresh` instead of being hidden
// behind an always-pass-through value.
//
// It also provides full read AND write conversion for provisioning_criteria,
// which - unlike the three blocks above - this provider does send to the API
// on Create/Update (see accessProfileModelToDto), so both directions are
// handled here.
//
// Scope/limits:
//   - The three pass-through-only blocks are only built here when the
//     practitioner has NOT configured the corresponding block (fallback is
//     Null/Unknown) - see accessProfilePassThroughOrReadBack semantics inlined
//     in accessProfileDtoToModel. If a block IS configured, the existing
//     pass-through + AddWarning behavior is preserved (this provider still
//     does not send these blocks to the API on Create/Update - see the
//     accessProfileModelToDto doc comment - so overwriting a configured value
//     with the API's response would produce a permanent, non-convergent
//     diff).
//   - The provisioning_criteria tree only resolves 3 levels deep
//     (provisioning_criteria -> children -> grandchildren), matching the depth
//     tfplugingen-framework flattened the recursive OpenAPI schema to. A
//     criteria tree deeper than 3 levels is not expected from IdentityNow's
//     access profile provisioning criteria UI, but if encountered, any 4th
//     level nested children are silently dropped (not represented in the
//     generated schema at all, so there's nowhere to put them).

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sailpoint-oss/golang-sdk/v3/access_profiles"

	"terraform-provider-identitynow/internal/provider/access_profile_v1/resource_access_profile"
)

func accessProfileAccessModelMetadataFromApi(ctx context.Context, dto *access_profiles.AttributeDTOList) (resource_access_profile.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_access_profile.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := accessProfileAttributeDTOListFromApi(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := resource_access_profile.NewAccessModelMetadataValue(
		resource_access_profile.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func accessProfileAttributeDTOListFromApi(ctx context.Context, items []access_profiles.AttributeDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := accessProfileAttributeValueDTOListFromApi(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := resource_access_profile.NewAttributesValue(
			resource_access_profile.AttributesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"description":  types.StringPointerValue(item.Description),
				"key":          types.StringPointerValue(item.Key),
				"multiselect":  types.BoolPointerValue(item.Multiselect),
				"name":         types.StringPointerValue(item.Name),
				"object_types": objectTypes,
				"status":       types.StringPointerValue(item.Status),
				"type":         types.StringPointerValue(item.Type),
				"values":       valuesList,
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func accessProfileAttributeValueDTOListFromApi(ctx context.Context, items []access_profiles.AttributeValueDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_access_profile.NewValuesValue(
			resource_access_profile.ValuesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"name":   types.StringPointerValue(item.Name),
				"status": types.StringPointerValue(item.Status),
				"value":  types.StringPointerValue(item.Value),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func accessProfileApprovalSchemeListFromApi(ctx context.Context, items []access_profiles.AccessProfileApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.ApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.ApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_access_profile.NewApprovalSchemesValue(
			resource_access_profile.ApprovalSchemesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"approver_id":   types.StringPointerValue(item.ApproverId.Get()),
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

func accessProfileRevocationApprovalSchemeListFromApi(ctx context.Context, items []access_profiles.AccessProfileApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.RevocationApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.RevocationApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_access_profile.NewRevocationApprovalSchemesValue(
			resource_access_profile.RevocationApprovalSchemesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"approver_id":   types.StringPointerValue(item.ApproverId.Get()),
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

func accessProfileMaxPermittedAccessDurationFromApi(ctx context.Context, dto *access_profiles.AccessDuration) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_access_profile.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)
	if dto == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var valueAttr types.Int64
	if dto.Value != nil {
		valueAttr = types.Int64Value(int64(*dto.Value))
	} else {
		valueAttr = types.Int64Null()
	}

	v, d := resource_access_profile.NewMaxPermittedAccessDurationValue(
		attrTypes,
		map[string]attr.Value{
			"time_unit": types.StringPointerValue(dto.TimeUnit),
			"value":     valueAttr,
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func accessProfileAccessRequestConfigFromApi(ctx context.Context, dto *access_profiles.Requestability) (resource_access_profile.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_access_profile.NewAccessRequestConfigValueNull(), diags
	}

	approvalSchemes, d := accessProfileApprovalSchemeListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	maxPermitted, d := accessProfileMaxPermittedAccessDurationFromApi(ctx, dto.MaxPermittedAccessDuration.Get())
	diags.Append(d...)

	v, d := resource_access_profile.NewAccessRequestConfigValue(
		resource_access_profile.AccessRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"approval_schemes":              approvalSchemes,
			"comments_required":             types.BoolPointerValue(dto.CommentsRequired.Get()),
			"denial_comments_required":      types.BoolPointerValue(dto.DenialCommentsRequired.Get()),
			"max_permitted_access_duration": maxPermitted,
			"reauthorization_required":      types.BoolPointerValue(dto.ReauthorizationRequired.Get()),
			"require_end_date":              types.BoolPointerValue(dto.RequireEndDate.Get()),
		},
	)
	diags.Append(d...)
	return v, diags
}

func accessProfileRevocationRequestConfigFromApi(ctx context.Context, dto *access_profiles.Revocability) (resource_access_profile.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_access_profile.NewRevocationRequestConfigValueNull(), diags
	}

	revocationApprovalSchemes, d := accessProfileRevocationApprovalSchemeListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	v, d := resource_access_profile.NewRevocationRequestConfigValue(
		resource_access_profile.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"revocation_approval_schemes": revocationApprovalSchemes,
		},
	)
	diags.Append(d...)
	return v, diags
}

// accessProfileProvisioningCriteriaFromApi converts the API's 3-level
// recursive ProvisioningCriteriaLevel1/2/3 tree into the generated 3-level
// ProvisioningCriteriaValue/ChildrenValue/GrandchildrenValue tree (see the
// package doc for the depth limitation).
func accessProfileProvisioningCriteriaFromApi(ctx context.Context, dto *access_profiles.ProvisioningCriteriaLevel1) (resource_access_profile.ProvisioningCriteriaValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_access_profile.NewProvisioningCriteriaValueNull(), diags
	}

	children, d := accessProfileProvisioningCriteriaLevel2ListFromApi(ctx, dto.Children)
	diags.Append(d...)

	var operation types.String
	if dto.Operation != nil {
		operation = types.StringValue(string(*dto.Operation))
	} else {
		operation = types.StringNull()
	}

	v, d := resource_access_profile.NewProvisioningCriteriaValue(
		resource_access_profile.ProvisioningCriteriaValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"attribute": types.StringPointerValue(dto.Attribute.Get()),
			"children":  children,
			"operation": operation,
			"value":     types.StringPointerValue(dto.Value.Get()),
		},
	)
	diags.Append(d...)
	return v, diags
}

func accessProfileProvisioningCriteriaLevel2ListFromApi(ctx context.Context, items []access_profiles.ProvisioningCriteriaLevel2) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.ChildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.ChildrenValue, 0, len(items))
	for _, item := range items {
		grandchildren, d := accessProfileProvisioningCriteriaLevel3ListFromApi(ctx, item.Children)
		diags.Append(d...)

		var operation types.String
		if item.Operation != nil {
			operation = types.StringValue(string(*item.Operation))
		} else {
			operation = types.StringNull()
		}

		v, d := resource_access_profile.NewChildrenValue(
			resource_access_profile.ChildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"attribute":     types.StringPointerValue(item.Attribute.Get()),
				"grandchildren": grandchildren,
				"operation":     operation,
				"value":         types.StringPointerValue(item.Value.Get()),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

// accessProfileProvisioningCriteriaLevel3ListFromApi converts level 3 of the
// tree. Level 3's own "children" field in the SDK (access_profiles.NullableString) is
// a dead/unused leftover matching the generated schema's "children" string
// attribute on GrandchildrenValue - there is no level 4, so it is always left
// null.
func accessProfileProvisioningCriteriaLevel3ListFromApi(ctx context.Context, items []access_profiles.ProvisioningCriteriaLevel3) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_access_profile.GrandchildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_access_profile.GrandchildrenValue, 0, len(items))
	for _, item := range items {
		var operation types.String
		if item.Operation != nil {
			operation = types.StringValue(string(*item.Operation))
		} else {
			operation = types.StringNull()
		}

		v, d := resource_access_profile.NewGrandchildrenValue(
			resource_access_profile.GrandchildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"attribute": types.StringPointerValue(item.Attribute.Get()),
				"children":  types.StringNull(),
				"operation": operation,
				"value":     types.StringPointerValue(item.Value.Get()),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

// accessProfileProvisioningCriteriaToApi converts the Terraform
// ProvisioningCriteriaValue tree from plan/config into the API's 3-level
// ProvisioningCriteriaLevel1/2/3 tree for Create/Update requests.
func accessProfileProvisioningCriteriaToApi(ctx context.Context, v resource_access_profile.ProvisioningCriteriaValue) (*access_profiles.ProvisioningCriteriaLevel1, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}

	level1 := &access_profiles.ProvisioningCriteriaLevel1{}
	if !v.Attribute.IsNull() && !v.Attribute.IsUnknown() {
		level1.Attribute = *access_profiles.NewNullableString(v.Attribute.ValueStringPointer())
	}
	if !v.Value.IsNull() && !v.Value.IsUnknown() {
		level1.Value = *access_profiles.NewNullableString(v.Value.ValueStringPointer())
	}
	if !v.Operation.IsNull() && !v.Operation.IsUnknown() {
		op := access_profiles.ProvisioningCriteriaOperation(v.Operation.ValueString())
		level1.Operation = &op
	}

	if !v.Children.IsNull() && !v.Children.IsUnknown() {
		var childItems []resource_access_profile.ChildrenValue
		diags.Append(v.Children.ElementsAs(ctx, &childItems, false)...)
		children := make([]access_profiles.ProvisioningCriteriaLevel2, 0, len(childItems))
		for _, c := range childItems {
			level2 := access_profiles.ProvisioningCriteriaLevel2{}
			if !c.Attribute.IsNull() && !c.Attribute.IsUnknown() {
				level2.Attribute = *access_profiles.NewNullableString(c.Attribute.ValueStringPointer())
			}
			if !c.Value.IsNull() && !c.Value.IsUnknown() {
				level2.Value = *access_profiles.NewNullableString(c.Value.ValueStringPointer())
			}
			if !c.Operation.IsNull() && !c.Operation.IsUnknown() {
				op := access_profiles.ProvisioningCriteriaOperation(c.Operation.ValueString())
				level2.Operation = &op
			}
			if !c.Grandchildren.IsNull() && !c.Grandchildren.IsUnknown() {
				var grandchildItems []resource_access_profile.GrandchildrenValue
				diags.Append(c.Grandchildren.ElementsAs(ctx, &grandchildItems, false)...)
				grandchildren := make([]access_profiles.ProvisioningCriteriaLevel3, 0, len(grandchildItems))
				for _, g := range grandchildItems {
					level3 := access_profiles.ProvisioningCriteriaLevel3{}
					if !g.Attribute.IsNull() && !g.Attribute.IsUnknown() {
						level3.Attribute = *access_profiles.NewNullableString(g.Attribute.ValueStringPointer())
					}
					if !g.Value.IsNull() && !g.Value.IsUnknown() {
						level3.Value = *access_profiles.NewNullableString(g.Value.ValueStringPointer())
					}
					if !g.Operation.IsNull() && !g.Operation.IsUnknown() {
						op := access_profiles.ProvisioningCriteriaOperation(g.Operation.ValueString())
						level3.Operation = &op
					}
					grandchildren = append(grandchildren, level3)
				}
				level2.Children = grandchildren
			}
			children = append(children, level2)
		}
		level1.Children = children
	}

	return level1, diags
}
