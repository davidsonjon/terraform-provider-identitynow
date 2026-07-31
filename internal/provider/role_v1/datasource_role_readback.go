package role_v1

// This file mirrors resource_role_readback.go's deeper read-back conversion
// for the pass-through-only blocks documented in datasource_role.go's package
// doc: access_model_metadata, access_request_config, revocation_request_config,
// and membership. Unlike the resource, every one of the data source's
// attributes (including these) is Computed-only (no Optional counterpart), so
// there is no "practitioner configured it" case to special-case here - this
// conversion always runs in roleDatasourceDtoToModel.
//
// Function names are prefixed roleDatasource* (vs. resource_role_readback.go's
// role* names) even though both files are in the same role_v1 package and
// operate on structurally-identical-but-distinct generated types
// (resource_role.X vs. datasource_role.X, emitted as separate Go packages by
// tfplugingen-framework) - see the package doc in resource_role.go.
//
// Scope/limits:
//   - access_request_config's "dimension_schema" attribute has no counterpart
//     in api_beta.RequestabilityForRole (the v1 API added it; the beta SDK
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

	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/role_v1/datasource_role"
)

func roleDatasourceAccessModelMetadataFromApi(ctx context.Context, dto *api_beta.AttributeDTOList) (datasource_role.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_role.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := roleDatasourceAttributeDTOListFromApi(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := datasource_role.NewAccessModelMetadataValue(
		datasource_role.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func roleDatasourceAttributeDTOListFromApi(ctx context.Context, items []api_beta.AttributeDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := roleDatasourceAttributeValueDTOListFromApi(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := datasource_role.NewAttributesValue(
			datasource_role.AttributesValue{}.AttributeTypes(ctx),
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

func roleDatasourceAttributeValueDTOListFromApi(ctx context.Context, items []api_beta.AttributeValueDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_role.NewValuesValue(
			datasource_role.ValuesValue{}.AttributeTypes(ctx),
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

func roleDatasourceApprovalSchemeForRoleListFromApi(ctx context.Context, items []api_beta.ApprovalSchemeForRole) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.ApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.ApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_role.NewApprovalSchemesValue(
			datasource_role.ApprovalSchemesValue{}.AttributeTypes(ctx),
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

func roleDatasourceRevocationApprovalSchemeForRoleListFromApi(ctx context.Context, items []api_beta.ApprovalSchemeForRole) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.RevocationApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.RevocationApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_role.NewRevocationApprovalSchemesValue(
			datasource_role.RevocationApprovalSchemesValue{}.AttributeTypes(ctx),
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

func roleDatasourceMaxPermittedAccessDurationFromApi(ctx context.Context, dto *api_beta.AccessDuration) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := datasource_role.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)
	if dto == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var valueAttr types.Int64
	if dto.Value != nil {
		valueAttr = types.Int64Value(int64(*dto.Value))
	} else {
		valueAttr = types.Int64Null()
	}

	v, d := datasource_role.NewMaxPermittedAccessDurationValue(
		attrTypes,
		map[string]attr.Value{
			"time_unit": types.StringPointerValue(dto.TimeUnit),
			"value":     valueAttr,
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func roleDatasourceAccessRequestConfigFromApi(ctx context.Context, dto *api_beta.RequestabilityForRole) (datasource_role.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_role.NewAccessRequestConfigValueNull(), diags
	}

	approvalSchemes, d := roleDatasourceApprovalSchemeForRoleListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	maxPermitted, d := roleDatasourceMaxPermittedAccessDurationFromApi(ctx, dto.MaxPermittedAccessDuration.Get())
	diags.Append(d...)

	// dimension_schema has no counterpart in api_beta.RequestabilityForRole -
	// see the package-level doc comment at the top of this file.
	dimensionSchemaNull := types.ObjectNull(datasource_role.DimensionSchemaValue{}.AttributeTypes(ctx))

	v, d := datasource_role.NewAccessRequestConfigValue(
		datasource_role.AccessRequestConfigValue{}.AttributeTypes(ctx),
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

func roleDatasourceRevocationRequestConfigFromApi(ctx context.Context, dto *api_beta.RevocabilityForRole) (datasource_role.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_role.NewRevocationRequestConfigValueNull(), diags
	}

	revocationApprovalSchemes, d := roleDatasourceRevocationApprovalSchemeForRoleListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	v, d := datasource_role.NewRevocationRequestConfigValue(
		datasource_role.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"comments_required":           types.BoolPointerValue(dto.CommentsRequired.Get()),
			"denial_comments_required":    types.BoolPointerValue(dto.DenialCommentsRequired.Get()),
			"revocation_approval_schemes": revocationApprovalSchemes,
		},
	)
	diags.Append(d...)
	return v, diags
}

func roleDatasourceIdentitiesFromApi(ctx context.Context, items []api_beta.RoleMembershipIdentity) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.IdentitiesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.IdentitiesValue, 0, len(items))
	for _, item := range items {
		var identityType *string
		if item.Type != nil {
			s := string(*item.Type)
			identityType = &s
		}

		v, d := datasource_role.NewIdentitiesValue(
			datasource_role.IdentitiesValue{}.AttributeTypes(ctx),
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

func roleDatasourceCriteriaKeyFromApi(ctx context.Context, key *api_beta.RoleCriteriaKey) types.Object {
	attrTypes := datasource_role.KeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := datasource_role.NewKeyValueMust(
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

func roleDatasourceChildKeyFromApi(ctx context.Context, key *api_beta.RoleCriteriaKey) types.Object {
	attrTypes := datasource_role.ChildKeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := datasource_role.NewChildKeyValueMust(
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

func roleDatasourceGrandchildKeyFromApi(ctx context.Context, key *api_beta.RoleCriteriaKey) types.Object {
	attrTypes := datasource_role.GrandchildKeyValue{}.AttributeTypes(ctx)
	if key == nil {
		return types.ObjectNull(attrTypes)
	}

	v := datasource_role.NewGrandchildKeyValueMust(
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

func roleDatasourceGrandchildrenFromApi(ctx context.Context, items []api_beta.RoleCriteriaLevel3) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.GrandchildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.GrandchildrenValue, 0, len(items))
	for _, item := range items {
		var op *string
		if item.Operation != nil {
			s := string(*item.Operation)
			op = &s
		}

		v, d := datasource_role.NewGrandchildrenValue(
			datasource_role.GrandchildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"grandchild_key": roleDatasourceGrandchildKeyFromApi(ctx, item.Key.Get()),
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

func roleDatasourceChildrenFromApi(ctx context.Context, items []api_beta.RoleCriteriaLevel2) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_role.ChildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_role.ChildrenValue, 0, len(items))
	for _, item := range items {
		var op *string
		if item.Operation != nil {
			s := string(*item.Operation)
			op = &s
		}

		grandchildren, d := roleDatasourceGrandchildrenFromApi(ctx, item.Children)
		diags.Append(d...)

		v, d := datasource_role.NewChildrenValue(
			datasource_role.ChildrenValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"child_key":     roleDatasourceChildKeyFromApi(ctx, item.Key.Get()),
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

func roleDatasourceCriteriaFromApi(ctx context.Context, level1 *api_beta.RoleCriteriaLevel1) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := datasource_role.CriteriaValue{}.AttributeTypes(ctx)
	if level1 == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var op *string
	if level1.Operation != nil {
		s := string(*level1.Operation)
		op = &s
	}

	children, d := roleDatasourceChildrenFromApi(ctx, level1.Children)
	diags.Append(d...)

	v, d := datasource_role.NewCriteriaValue(
		attrTypes,
		map[string]attr.Value{
			"children":     children,
			"key":          roleDatasourceCriteriaKeyFromApi(ctx, level1.Key.Get()),
			"operation":    types.StringPointerValue(op),
			"string_value": types.StringPointerValue(level1.StringValue.Get()),
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func roleDatasourceMembershipFromApi(ctx context.Context, dto *api_beta.RoleMembershipSelector) (datasource_role.MembershipValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_role.NewMembershipValueNull(), diags
	}

	criteria, d := roleDatasourceCriteriaFromApi(ctx, dto.Criteria.Get())
	diags.Append(d...)

	identities, d := roleDatasourceIdentitiesFromApi(ctx, dto.Identities)
	diags.Append(d...)

	var membershipType *string
	if dto.Type != nil {
		s := string(*dto.Type)
		membershipType = &s
	}

	v, d := datasource_role.NewMembershipValue(
		datasource_role.MembershipValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"criteria":   criteria,
			"identities": identities,
			"type":       types.StringPointerValue(membershipType),
		},
	)
	diags.Append(d...)
	return v, diags
}
