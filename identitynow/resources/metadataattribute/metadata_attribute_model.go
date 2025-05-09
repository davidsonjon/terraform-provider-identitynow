package metadataattribute

import (
	"context"

	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Identity
type AttributeDTO struct {
	// Technical name of the Attribute. This is unique and cannot be changed after creation.
	Key types.String `tfsdk:"key"`
	// The display name of the key.
	Name types.String `tfsdk:"name"`
	// Indicates whether the attribute can have multiple values.
	Multiselect types.Bool `tfsdk:"multiselect"`
	// The status of the Attribute.
	Status types.String `tfsdk:"status"`
	// The type of the Attribute. This can be either \"custom\" or \"governance\".
	Type types.String `tfsdk:"type"`
	// An array of object types this attributes values can be applied to. Possible values are \"all\" or \"entitlement\". Value \"all\" means this attribute can be used with all object types that are supported.
	ObjectTypes types.List `tfsdk:"object_types"`
	// The description of the Attribute.
	Description types.String        `tfsdk:"description"`
	Values      []AttributeValueDTO `tfsdk:"values"`
}

type AttributeValueDTO struct {
	// Technical name of the Attribute value. This is unique and cannot be changed after creation.
	Value types.String `tfsdk:"value"`
	// The display name of the Attribute value.
	Name types.String `tfsdk:"name"`
	// The status of the Attribute value.
	Status types.String `tfsdk:"status"`
}

func convertAttribute(ap *AttributeDTO, diags *diag.Diagnostics) *beta.AttributeDTO {
	attribute := beta.AttributeDTO{}

	attribute.Key = ap.Key.ValueStringPointer()
	attribute.Description = ap.Description.ValueStringPointer()
	attribute.Name = ap.Name.ValueStringPointer()
	attribute.Multiselect = ap.Multiselect.ValueBoolPointer()
	attribute.Status = ap.Status.ValueStringPointer()
	attribute.Type = ap.Type.ValueStringPointer()

	elements := make([]string, 0, len(ap.ObjectTypes.Elements()))
	diag := ap.ObjectTypes.ElementsAs(context.Background(), &elements, false)
	diags.Append(diag...)

	attribute.ObjectTypes = elements

	values := make([]beta.AttributeValueDTO, 0, len(ap.Values))

	for _, v := range ap.Values {
		value := beta.AttributeValueDTO{}

		value.Name = v.Name.ValueStringPointer()
		value.Status = v.Status.ValueStringPointer()
		value.Value = v.Value.ValueStringPointer()

		values = append(values, value)
	}

	attribute.Values = values

	return &attribute
}
