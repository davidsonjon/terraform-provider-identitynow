package access_profile_v1

// This file mirrors resource_access_profile_readback.go's deeper read-back
// conversion for the pass-through-only blocks documented in
// access_profile_data_source.go's package doc: access_model_metadata,
// access_request_config, revocation_request_config, and provisioning_criteria.
// Unlike the resource, every one of the data source's attributes (including
// these) is Computed-only (no Optional counterpart), so there is no
// "practitioner configured it" case to special-case here - this conversion
// always runs in accessProfileDatasourceDtoToModel.
//
// Function names are prefixed accessProfileDatasource* (vs.
// resource_access_profile_readback.go's accessProfile* names) even though
// both files are in the same access_profile_v1 package and operate on
// structurally-identical-but-distinct generated types
// (resource_access_profile.X vs. datasource_access_profile.X, emitted as
// separate Go packages by tfplugingen-framework) - see the package doc in
// resource_access_profile.go.
//
// Scope/limits: the provisioning_criteria tree only resolves 3 levels deep
// (provisioning_criteria -> children -> grandchildren), matching the depth
// tfplugingen-framework flattened the recursive OpenAPI schema to.

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sailpoint-oss/golang-sdk/v3/access_profiles"

	"terraform-provider-identitynow/internal/provider/access_profile_v1/datasource_access_profile"
)

func accessProfileDatasourceAccessModelMetadataFromApi(ctx context.Context, dto *access_profiles.AttributeDTOList) (datasource_access_profile.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_access_profile.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := accessProfileDatasourceAttributeDTOListFromApi(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := datasource_access_profile.NewAccessModelMetadataValue(
		datasource_access_profile.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func accessProfileDatasourceAttributeDTOListFromApi(ctx context.Context, items []access_profiles.AttributeDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := accessProfileDatasourceAttributeValueDTOListFromApi(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := datasource_access_profile.NewAttributesValue(
			datasource_access_profile.AttributesValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceAttributeValueDTOListFromApi(ctx context.Context, items []access_profiles.AttributeValueDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_access_profile.NewValuesValue(
			datasource_access_profile.ValuesValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceApprovalSchemeListFromApi(ctx context.Context, items []access_profiles.AccessProfileApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.ApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.ApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_access_profile.NewApprovalSchemesValue(
			datasource_access_profile.ApprovalSchemesValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceRevocationApprovalSchemeListFromApi(ctx context.Context, items []access_profiles.AccessProfileApprovalScheme) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.RevocationApprovalSchemesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.RevocationApprovalSchemesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_access_profile.NewRevocationApprovalSchemesValue(
			datasource_access_profile.RevocationApprovalSchemesValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceMaxPermittedAccessDurationFromApi(ctx context.Context, dto *access_profiles.AccessDuration) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := datasource_access_profile.MaxPermittedAccessDurationValue{}.AttributeTypes(ctx)
	if dto == nil {
		return types.ObjectNull(attrTypes), diags
	}

	var valueAttr types.Int64
	if dto.Value != nil {
		valueAttr = types.Int64Value(int64(*dto.Value))
	} else {
		valueAttr = types.Int64Null()
	}

	v, d := datasource_access_profile.NewMaxPermittedAccessDurationValue(
		attrTypes,
		map[string]attr.Value{
			"time_unit": types.StringPointerValue(dto.TimeUnit),
			"value":     valueAttr,
		},
	)
	diags.Append(d...)
	return v.ToObjectValue(ctx)
}

func accessProfileDatasourceAccessRequestConfigFromApi(ctx context.Context, dto *access_profiles.Requestability) (datasource_access_profile.AccessRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_access_profile.NewAccessRequestConfigValueNull(), diags
	}

	approvalSchemes, d := accessProfileDatasourceApprovalSchemeListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	maxPermitted, d := accessProfileDatasourceMaxPermittedAccessDurationFromApi(ctx, dto.MaxPermittedAccessDuration.Get())
	diags.Append(d...)

	v, d := datasource_access_profile.NewAccessRequestConfigValue(
		datasource_access_profile.AccessRequestConfigValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceRevocationRequestConfigFromApi(ctx context.Context, dto *access_profiles.Revocability) (datasource_access_profile.RevocationRequestConfigValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_access_profile.NewRevocationRequestConfigValueNull(), diags
	}

	revocationApprovalSchemes, d := accessProfileDatasourceRevocationApprovalSchemeListFromApi(ctx, dto.ApprovalSchemes)
	diags.Append(d...)

	v, d := datasource_access_profile.NewRevocationRequestConfigValue(
		datasource_access_profile.RevocationRequestConfigValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"revocation_approval_schemes": revocationApprovalSchemes,
		},
	)
	diags.Append(d...)
	return v, diags
}

func accessProfileDatasourceProvisioningCriteriaFromApi(ctx context.Context, dto *access_profiles.ProvisioningCriteriaLevel1) (datasource_access_profile.ProvisioningCriteriaValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_access_profile.NewProvisioningCriteriaValueNull(), diags
	}

	children, d := accessProfileDatasourceProvisioningCriteriaLevel2ListFromApi(ctx, dto.Children)
	diags.Append(d...)

	var operation types.String
	if dto.Operation != nil {
		operation = types.StringValue(string(*dto.Operation))
	} else {
		operation = types.StringNull()
	}

	v, d := datasource_access_profile.NewProvisioningCriteriaValue(
		datasource_access_profile.ProvisioningCriteriaValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceProvisioningCriteriaLevel2ListFromApi(ctx context.Context, items []access_profiles.ProvisioningCriteriaLevel2) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.ChildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.ChildrenValue, 0, len(items))
	for _, item := range items {
		grandchildren, d := accessProfileDatasourceProvisioningCriteriaLevel3ListFromApi(ctx, item.Children)
		diags.Append(d...)

		var operation types.String
		if item.Operation != nil {
			operation = types.StringValue(string(*item.Operation))
		} else {
			operation = types.StringNull()
		}

		v, d := datasource_access_profile.NewChildrenValue(
			datasource_access_profile.ChildrenValue{}.AttributeTypes(ctx),
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

func accessProfileDatasourceProvisioningCriteriaLevel3ListFromApi(ctx context.Context, items []access_profiles.ProvisioningCriteriaLevel3) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_access_profile.GrandchildrenValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_access_profile.GrandchildrenValue, 0, len(items))
	for _, item := range items {
		var operation types.String
		if item.Operation != nil {
			operation = types.StringValue(string(*item.Operation))
		} else {
			operation = types.StringNull()
		}

		v, d := datasource_access_profile.NewGrandchildrenValue(
			datasource_access_profile.GrandchildrenValue{}.AttributeTypes(ctx),
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
