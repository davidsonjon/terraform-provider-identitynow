package connector_rule_v1

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/connector_rule_v1/datasource_connector_rule"
)

// datasourceSourceCodeFromAPI mirrors sourceCodeFromAPI (resource_connector_rule.go)
// but against datasource_connector_rule's generated SourceCodeValue type - a
// distinct Go type from resource_connector_rule.SourceCodeValue even though
// structurally identical, since tfplugingen-framework generates a fresh set
// of Value types per schema (resource vs. data source), so the two can't
// share a single conversion function.
func datasourceSourceCodeFromAPI(ctx context.Context, dto api_beta.SourceCode) (datasource_connector_rule.SourceCodeValue, diag.Diagnostics) {
	return datasource_connector_rule.NewSourceCodeValue(
		datasource_connector_rule.SourceCodeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"script":  types.StringValue(dto.Script),
			"version": types.StringValue(dto.Version),
		},
	)
}

// datasourceSignatureFromAPI mirrors signatureFromAPI (resource_connector_rule.go)
// but against datasource_connector_rule's generated types.
func datasourceSignatureFromAPI(ctx context.Context, dto *api_beta.ConnectorRuleCreateRequestSignature) (datasource_connector_rule.SignatureValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := datasource_connector_rule.SignatureValue{}.AttributeTypes(ctx)
	if dto == nil {
		return datasource_connector_rule.NewSignatureValueNull(), diags
	}

	input, d := datasourceArgumentListFromAPI(ctx, dto.Input)
	diags.Append(d...)

	output, d := datasourceArgumentObjectFromAPI(ctx, dto.Output)
	diags.Append(d...)

	v, d := datasource_connector_rule.NewSignatureValue(
		attrTypes,
		map[string]attr.Value{
			"input":  input,
			"output": output,
		},
	)
	diags.Append(d...)
	return v, diags
}

func datasourceArgumentListFromAPI(ctx context.Context, items []api_beta.Argument) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_connector_rule.InputValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_connector_rule.InputValue, 0, len(items))
	for _, item := range items {
		description := types.StringNull()
		if item.Description != nil {
			description = types.StringValue(*item.Description)
		}
		inputType := types.StringNull()
		if item.Type.IsSet() && item.Type.Get() != nil {
			inputType = types.StringValue(*item.Type.Get())
		}

		v, d := datasource_connector_rule.NewInputValue(
			datasource_connector_rule.InputValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"name":        types.StringValue(item.Name),
				"description": description,
				"type":        inputType,
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func datasourceArgumentObjectFromAPI(ctx context.Context, dto api_beta.NullableArgument) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := datasource_connector_rule.OutputValue{}.AttributeTypes(ctx)
	if !dto.IsSet() || dto.Get() == nil {
		return types.ObjectNull(attrTypes), diags
	}
	item := dto.Get()

	description := types.StringNull()
	if item.Description != nil {
		description = types.StringValue(*item.Description)
	}
	outputType := types.StringNull()
	if item.Type.IsSet() && item.Type.Get() != nil {
		outputType = types.StringValue(*item.Type.Get())
	}

	v, d := datasource_connector_rule.NewOutputValue(
		attrTypes,
		map[string]attr.Value{
			"name":        types.StringValue(item.Name),
			"description": description,
			"type":        outputType,
		},
	)
	diags.Append(d...)
	obj, d := v.ToObjectValue(ctx)
	diags.Append(d...)
	return obj, diags
}
