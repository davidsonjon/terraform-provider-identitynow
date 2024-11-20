package segment

import (
	v3 "github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Segment struct {
	// The entitlement id
	Id types.String `tfsdk:"id"`
	// The segment's business name.
	Name types.String `tfsdk:"name"`
	// The time when the segment is created.
	Created types.String `tfsdk:"created"`
	// The time when the segment is modified.
	Modified types.String `tfsdk:"modified"`
	// The segment's optional description.
	Description        types.String        `tfsdk:"description"`
	VisibilityCriteria *VisibilityCriteria `tfsdk:"visibility_criteria"`
	// // This boolean indicates whether the segment is currently active. Inactive segments have no effect.
	Active types.Bool `tfsdk:"active"`
}

type VisibilityCriteria struct {
	// Operator for the expression
	Operator types.String `tfsdk:"operator"`
	// Name for the attribute
	Attribute types.String `tfsdk:"attribute"`
	Value     *Value       `tfsdk:"value"`
	// List of expressions
	Children []VisibilityCriteria `tfsdk:"children"`
}

type Value struct {
	// The type of attribute value
	Type types.String `tfsdk:"type"`
	// The attribute value
	Value types.String `tfsdk:"value"`
}

type SegmentAccess struct {
	// The id
	Id types.String `tfsdk:"id"`
	// The segment id
	SegmentId types.String `tfsdk:"segment_id"`
	// The access profiles to assign to the segment
	Assignments []SegmentAccessAssignments `tfsdk:"assignments"`
}

type SegmentAccessAssignments struct {
	// The id
	Id types.String `tfsdk:"id"`
	// The segment id
	Type types.String `tfsdk:"type"`
}

func convertSegmentV3(seg *Segment) *v3.Segment {

	segment := v3.Segment{}
	segment.Name = seg.Name.ValueStringPointer()
	segment.Description = seg.Description.ValueStringPointer()
	segment.Active = seg.Active.ValueBoolPointer()

	segment.VisibilityCriteria = v3.NewSegmentVisibilityCriteriaWithDefaults()
	segment.VisibilityCriteria.Expression = &v3.Expression{
		Operator: seg.VisibilityCriteria.Operator.ValueStringPointer(),
		Children: []v3.ExpressionChildrenInner{},
	}

	if !seg.VisibilityCriteria.Attribute.IsNull() {
		segment.VisibilityCriteria.Expression.Attribute = *v3.NewNullableString(seg.VisibilityCriteria.Attribute.ValueStringPointer())
	}

	if seg.VisibilityCriteria.Value != nil {
		segVisCritValue := v3.NewNullableValue(&v3.Value{Type: seg.VisibilityCriteria.Value.Type.ValueStringPointer(),
			Value: seg.VisibilityCriteria.Value.Value.ValueStringPointer()})
		segment.VisibilityCriteria.Expression.Value = *segVisCritValue
	}

	for _, c := range seg.VisibilityCriteria.Children {
		childValue := v3.NewNullableValue(&v3.Value{Type: c.Value.Type.ValueStringPointer(),
			Value: c.Value.Value.ValueStringPointer()})
		child := v3.ExpressionChildrenInner{
			Operator:  c.Operator.ValueStringPointer(),
			Attribute: *v3.NewNullableString(c.Attribute.ValueStringPointer()),
			Value:     *childValue,
			Children:  v3.NullableString{},
		}

		segment.VisibilityCriteria.Expression.Children = append(segment.VisibilityCriteria.Expression.Children, child)
	}

	return &segment
}
