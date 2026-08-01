package role_v1

// This file provides deeper read-back conversion for the pass-through-only
// blocks documented in resource_role.go's package doc: access_model_metadata,
// access_request_config, revocation_request_config, and membership. Unlike the
// rest of roleDtoToModel/roleDatasourceDtoToModel (which prefer the fallback
// plan/prior-state value for these blocks), the functions here build a real
// Terraform value from the API response DTO so that IdentityNow-side defaults
// and drift are visible in `terraform plan`/`refresh` instead of being hidden
// behind an always-pass-through value.
//
// Scope/limits:
//   - Only used when the practitioner has NOT configured the corresponding
//     block (fallback is Null/Unknown) - see rolePassThroughOrReadBack in
//     resource_role.go. If a block IS configured, the existing pass-through +
//     AddWarning behavior is preserved (this provider still does not send
//     these blocks to the API on Create/Update - see the roleModelToDto doc
//     comment - so overwriting a configured value with the API's response
//     would produce a permanent, non-convergent diff).
//   - access_request_config's "dimension_schema" attribute has no counterpart
//     in roles.RequestabilityForRole (the v1 API added it; the beta SDK
//     this pilot maps onto has not caught up yet - see the
//     tfplugingen-openapi-type-reviewer knowledge file's per-service-v1 SDK
//     lag note). It is always left null here.
//   - legacy_membership_info is intentionally NOT handled here: its generated
//     schema has zero attributes (the DTO field is an arbitrary
//     map[string]interface{} that never got any concrete attribute mapping),
//     so there is nothing for a read-back to populate - it remains fully
//     pass-through.
//   - The criteria tree only resolves 3 levels deep (criteria -> children ->
//     grandchildren), matching the depth tfplugingen-framework flattened the
//     recursive OpenAPI schema to. A membership criteria tree deeper than 3
//     levels is not expected from IdentityNow's role membership UI, but if
//     encountered, any 4th-level nested children are silently dropped (not
//     represented in the generated schema at all, so there's nowhere to put
//     them).

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sailpoint-oss/golang-sdk/v3/roles"

	"terraform-provider-identitynow/internal/provider/role_v1/resource_role"
)

func roleAccessModelMetadataFromApi(ctx context.Context, dto *roles.AttributeDTOList) (resource_role.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_role.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := roleAttributeDTOListFromApi(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := resource_role.NewAccessModelMetadataValue(
		resource_role.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func roleAttributeDTOListFromApi(ctx context.Context, items []roles.AttributeDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := roleAttributeValueDTOListFromApi(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := resource_role.NewAttributesValue(
			resource_role.AttributesValue{}.AttributeTypes(ctx),
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

func roleAttributeValueDTOListFromApi(ctx context.Context, items []roles.AttributeValueDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_role.NewValuesValue(
			resource_role.ValuesValue{}.AttributeTypes(ctx),
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

func roleApprovalSchemeForRoleListFromApi(ctx context.Context, items []roles.ApprovalSchemeForRole) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.ApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.ApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_role.NewApprovalSchemesValue(
			resource_role.ApprovalSchemesValue{}.AttributeTypes(ctx),
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

func roleRevocationApprovalSchemeForRoleListFromApi(ctx context.Context, items []roles.ApprovalSchemeForRole) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.RevocationApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.RevocationApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_role.NewRevocationApprovalSchemesValue(
			resource_role.RevocationApprovalSchemesValue{}.AttributeTypes(ctx),
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

func roleMaxPermittedAccessDurationFromApi(ctx context.Context, dto *roles.AccessDuration) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_role.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)
	if dto == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var valueAttr types.Int64
	if dto.Value != nil {
		valueAttr = types.Int64Value(int64(*dto.Value))
	} else {
		valueAttr = types.Int64Null()
	}

	v, d := resource_role.NewMaxPermittedAccessDurationValue(
		attrTypes,
		map[string]attr.Value{
			"time_unit": types.StringPointerValue(dto.TimeUnit),
			"value":     valueAttr,
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func roleAccessRequestConfigFromApi(ctx context.Context, dto *roles.RequestabilityForRole) (resource_role.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_role.NewAccessRequestConfigValueNull(), diags
	}

	approvalSchemes, d := roleApprovalSchemeForRoleListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	maxPermitted, d := roleMaxPermittedAccessDurationFromApi(ctx, dto.MaxPermittedAccessDuration.Get())
	diags.Append(d...)

	// dimension_schema has no counterpart in roles.RequestabilityForRole -
	// see the package-level doc comment at the top of this file.
	dimensionSchemaNull := types.ObjectNull(resource_role.DimensionSchemaValue{}.AttributeTypes(ctx))

	v, d := resource_role.NewAccessRequestConfigValue(
		resource_role.AccessRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"approval_schemes":              approvalSchemes,
			"comments_required":             types.BoolPointerValue(dto.CommentsRequired.Get()),
			"denial_comments_required":      types.BoolPointerValue(dto.DenialCommentsRequired.Get()),
			"dimension_schema":              dimensionSchemaNull,
			"form_definition_id":            types.StringPointerValue(dto.FormDefinitionId.Get()),
			"max_permitted_access_duration": maxPermitted,
			"reauthorization_required":      types.BoolPointerValue(dto.ReauthorizationRequired.Get()),
			"require_end_date":              types.BoolPointerValue(dto.RequireEndDate),
		},
	)
	diags.Append(d...)
	return v, diags
}

func roleRevocationRequestConfigFromApi(ctx context.Context, dto *roles.RevocabilityForRole) (resource_role.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_role.NewRevocationRequestConfigValueNull(), diags
	}

	revocationApprovalSchemes, d := roleRevocationApprovalSchemeForRoleListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	v, d := resource_role.NewRevocationRequestConfigValue(
		resource_role.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"comments_required":           types.BoolPointerValue(dto.CommentsRequired.Get()),
			"denial_comments_required":    types.BoolPointerValue(dto.DenialCommentsRequired.Get()),
			"revocation_approval_schemes": revocationApprovalSchemes,
		},
	)
	diags.Append(d...)
	return v, diags
}

func roleIdentitiesFromApi(ctx context.Context, items []roles.RoleMembershipIdentity) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.IdentitiesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.IdentitiesValue, 0, len(items))
	for _, item := range items {
		var identityType *string
		if item.Type != nil {
			s := string(*item.Type)
			identityType = &s
		}

		v, d := resource_role.NewIdentitiesValue(
			resource_role.IdentitiesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"alias_name": types.StringPointerValue(item.AliasName.Get()),
				"id":         types.StringPointerValue(item.Id),
				"name":       types.StringPointerValue(item.Name.Get()),
				"type":       types.StringPointerValue(identityType),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func roleCriteriaKeyFromApi(ctx context.Context, key *roles.RoleCriteriaKey) types.Object {
	attrTypes := resource_role.KeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := resource_role.NewKeyValueMust(
		attrTypes,
		map[string]attr.Value{
			"property":  types.StringValue(key.Property),
			"source_id": types.StringPointerValue(key.SourceId.Get()),
			"type":      types.StringValue(string(key.Type)),
		},
	)
	obj, _ := v.ToObjectValue(ctx)
	return obj
}

func roleChildKeyFromApi(ctx context.Context, key *roles.RoleCriteriaKey) types.Object {
	attrTypes := resource_role.ChildKeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := resource_role.NewChildKeyValueMust(
		attrTypes,
		map[string]attr.Value{
			"property":  types.StringValue(key.Property),
			"source_id": types.StringPointerValue(key.SourceId.Get()),
			"type":      types.StringValue(string(key.Type)),
		},
	)
	obj, _ := v.ToObjectValue(ctx)
	return obj
}

func roleGrandchildKeyFromApi(ctx context.Context, key *roles.RoleCriteriaKey) types.Object {
	attrTypes := resource_role.GrandchildKeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := resource_role.NewGrandchildKeyValueMust(
		attrTypes,
		map[string]attr.Value{
			"property":  types.StringValue(key.Property),
			"source_id": types.StringPointerValue(key.SourceId.Get()),
			"type":      types.StringValue(string(key.Type)),
		},
	)
	obj, _ := v.ToObjectValue(ctx)
	return obj
}

func roleGrandchildrenFromApi(ctx context.Context, items []roles.RoleCriteriaLevel3) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.GrandchildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.GrandchildrenValue, 0, len(items))
	for _, item := range items {
		var op *string
		if item.Operation != nil {
			s := string(*item.Operation)
			op = &s
		}

		v, d := resource_role.NewGrandchildrenValue(
			resource_role.GrandchildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"grandchild_key": roleGrandchildKeyFromApi(ctx, item.Key.Get()),
				"operation":      types.StringPointerValue(op),
				"string_value":   types.StringPointerValue(item.StringValue.Get()),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func roleChildrenFromApi(ctx context.Context, items []roles.RoleCriteriaLevel2) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_role.ChildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_role.ChildrenValue, 0, len(items))
	for _, item := range items {
		var op *string
		if item.Operation != nil {
			s := string(*item.Operation)
			op = &s
		}

		grandchildren, d := roleGrandchildrenFromApi(ctx, item.Children)
		diags.Append(d...)

		v, d := resource_role.NewChildrenValue(
			resource_role.ChildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"child_key":     roleChildKeyFromApi(ctx, item.Key.Get()),
				"grandchildren": grandchildren,
				"operation":     types.StringPointerValue(op),
				"string_value":  types.StringPointerValue(item.StringValue.Get()),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func roleCriteriaFromApi(ctx context.Context, level1 *roles.RoleCriteriaLevel1) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_role.CriteriaValue{}.AttributeTypes(ctx)
	if level1 == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var op *string
	if level1.Operation != nil {
		s := string(*level1.Operation)
		op = &s
	}

	children, d := roleChildrenFromApi(ctx, level1.Children)
	diags.Append(d...)

	v, d := resource_role.NewCriteriaValue(
		attrTypes,
		map[string]attr.Value{
			"children":     children,
			"key":          roleCriteriaKeyFromApi(ctx, level1.Key.Get()),
			"operation":    types.StringPointerValue(op),
			"string_value": types.StringPointerValue(level1.StringValue.Get()),
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func roleMembershipFromApi(ctx context.Context, dto *roles.RoleMembershipSelector) (resource_role.MembershipValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_role.NewMembershipValueNull(), diags
	}

	criteria, d := roleCriteriaFromApi(ctx, dto.Criteria.Get())
	diags.Append(d...)

	identities, d := roleIdentitiesFromApi(ctx, dto.Identities)
	diags.Append(d...)

	var membershipType *string
	if dto.Type != nil {
		s := string(*dto.Type)
		membershipType = &s
	}

	v, d := resource_role.NewMembershipValue(
		resource_role.MembershipValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"criteria":   criteria,
			"identities": identities,
			"type":       types.StringPointerValue(membershipType),
		},
	)
	diags.Append(d...)
	return v, diags
}
