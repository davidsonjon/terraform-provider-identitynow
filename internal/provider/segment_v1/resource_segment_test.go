package segment_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/sailpoint-oss/golang-sdk/v3/segments"

	"terraform-provider-identitynow/internal/provider/segment_v1/resource_segment"
)

// minimalSegmentModel builds a segmentResourceModel with owner/visibility_criteria
// explicitly null, suitable as a base for tests that only care about
// top-level scalar fields.
func minimalSegmentModel() segmentResourceModel {
	return segmentResourceModel{
		Active:             types.BoolValue(true),
		Created:            types.StringNull(),
		Description:        types.StringValue("a test segment"),
		Id:                 types.StringNull(),
		Modified:           types.StringNull(),
		Name:               types.StringValue("test-segment"),
		Owner:              resource_segment.NewOwnerValueNull(),
		VisibilityCriteria: types.ObjectNull(visibilityCriteriaAttrTypes()),
	}
}

func ownerValue(t *testing.T, id, name, typ string) resource_segment.OwnerValue {
	t.Helper()
	attrs := map[string]attr.Value{
		"id":   types.StringValue(id),
		"name": types.StringValue(name),
		"type": types.StringValue(typ),
	}
	v, diags := resource_segment.NewOwnerValue(resource_segment.OwnerValue{}.AttributeTypes(context.Background()), attrs)
	if diags.HasError() {
		t.Fatalf("NewOwnerValue returned diagnostics: %v", diags)
	}
	return v
}

// leafVisibilityCriteria builds a single-level visibility_criteria object
// (no children), e.g. {expression: {operator: EQUALS, attribute: "uid",
// value: {type: "STRING", value: "N1"}}}.
func leafVisibilityCriteria(t *testing.T, ctx context.Context, operator, attribute, valueType, value string) types.Object {
	t.Helper()

	valueObj, diags := types.ObjectValue(visibilityValueAttrTypes(), map[string]attr.Value{
		"type":  types.StringValue(valueType),
		"value": types.StringValue(value),
	})
	if diags.HasError() {
		t.Fatalf("building value object: %v", diags)
	}

	exprObj, diags := types.ObjectValue(visibilityExpressionAttrTypes(), map[string]attr.Value{
		"operator":  types.StringValue(operator),
		"attribute": types.StringValue(attribute),
		"value":     valueObj,
		"children":  types.ListNull(types.ObjectType{AttrTypes: visibilityChildAttrTypes()}),
	})
	if diags.HasError() {
		t.Fatalf("building expression object: %v", diags)
	}

	vcObj, diags := types.ObjectValue(visibilityCriteriaAttrTypes(), map[string]attr.Value{
		"expression": exprObj,
	})
	if diags.HasError() {
		t.Fatalf("building visibility_criteria object: %v", diags)
	}
	return vcObj
}

func TestSegmentResourceModelToDTO(t *testing.T) {
	ctx := context.Background()
	model := minimalSegmentModel()
	model.Owner = ownerValue(t, "owner-id", "", "IDENTITY")
	model.VisibilityCriteria = leafVisibilityCriteria(t, ctx, "EQUALS", "uid", "STRING", "N1")

	dto, diags := segmentResourceModelToDTO(ctx, model)
	if diags.HasError() {
		t.Fatalf("segmentResourceModelToDTO returned diagnostics: %v", diags)
	}

	if dto.Name == nil || *dto.Name != "test-segment" {
		t.Errorf("Name = %v, want %q", dto.Name, "test-segment")
	}
	if dto.Description == nil || *dto.Description != "a test segment" {
		t.Errorf("Description = %v, want %q", dto.Description, "a test segment")
	}
	if dto.Active == nil || !*dto.Active {
		t.Errorf("Active = %v, want true", dto.Active)
	}
	owner := dto.Owner.Get()
	if owner == nil || owner.Id == nil || *owner.Id != "owner-id" {
		t.Errorf("Owner = %+v, want Id = %q", owner, "owner-id")
	}
	vc := dto.VisibilityCriteria
	if vc == nil || vc.Expression == nil {
		t.Fatalf("VisibilityCriteria = %v, want a populated expression", vc)
	}
	if vc.Expression.Operator == nil || *vc.Expression.Operator != "EQUALS" {
		t.Errorf("Expression.Operator = %v, want %q", vc.Expression.Operator, "EQUALS")
	}
	if vc.Expression.Attribute.Get() == nil || *vc.Expression.Attribute.Get() != "uid" {
		t.Errorf("Expression.Attribute = %v, want %q", vc.Expression.Attribute.Get(), "uid")
	}
	val := vc.Expression.Value.Get()
	if val == nil || val.Value == nil || *val.Value != "N1" {
		t.Errorf("Expression.Value = %v, want Value = %q", val, "N1")
	}
}

func TestSegmentResourceDTOToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()

	segmentID := "segment-id"
	name := "test-segment"
	description := "a test segment"
	active := true
	ownerID := "owner-id"
	ownerType := "IDENTITY"
	operator := "EQUALS"
	attribute := "uid"
	valueType := "STRING"
	value := "N1"

	dto := &segments.Segment{
		Id:          &segmentID,
		Name:        &name,
		Description: &description,
		Active:      &active,
		Owner: *segments.NewNullableOwnerReferenceSegments(&segments.OwnerReferenceSegments{
			Id:   &ownerID,
			Type: &ownerType,
		}),
		VisibilityCriteria: &segments.SegmentVisibilityCriteria{
			Expression: &segments.Expression{
				Operator:  &operator,
				Attribute: *segments.NewNullableString(&attribute),
				Value: *segments.NewNullableValue(&segments.Value{
					Type:  &valueType,
					Value: &value,
				}),
			},
		},
	}

	model, diags := segmentResourceDTOToModel(ctx, dto, types.StringNull())
	if diags.HasError() {
		t.Fatalf("segmentResourceDTOToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != segmentID {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), segmentID)
	}
	if model.Name.ValueString() != name {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), name)
	}
	if model.Description.ValueString() != description {
		t.Errorf("Description = %q, want %q", model.Description.ValueString(), description)
	}
	if !model.Active.ValueBool() {
		t.Errorf("Active = %v, want true", model.Active.ValueBool())
	}
	if model.Owner.Id.ValueString() != ownerID {
		t.Errorf("Owner.Id = %q, want %q", model.Owner.Id.ValueString(), ownerID)
	}

	var vcModel visibilityCriteriaModel
	diags = model.VisibilityCriteria.As(ctx, &vcModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("VisibilityCriteria.As returned diagnostics: %v", diags)
	}
	var exprModel visibilityExpressionModel
	diags = vcModel.Expression.As(ctx, &exprModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("Expression.As returned diagnostics: %v", diags)
	}
	if exprModel.Operator.ValueString() != operator {
		t.Errorf("Expression.Operator = %q, want %q", exprModel.Operator.ValueString(), operator)
	}
	if exprModel.Attribute.ValueString() != attribute {
		t.Errorf("Expression.Attribute = %q, want %q", exprModel.Attribute.ValueString(), attribute)
	}
	if !exprModel.Children.IsNull() {
		t.Errorf("Expression.Children = %v, want null (no children in this fixture)", exprModel.Children)
	}
}

func TestSegmentPatchOps_NoChanges(t *testing.T) {
	ctx := context.Background()
	model := minimalSegmentModel()

	ops, diags := segmentPatchOps(ctx, model, model)
	if diags.HasError() {
		t.Fatalf("segmentPatchOps returned diagnostics: %v", diags)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %+v, want empty (plan == state)", ops)
	}
}

func TestSegmentPatchOps_ScalarChanges(t *testing.T) {
	ctx := context.Background()
	state := minimalSegmentModel()
	plan := minimalSegmentModel()
	plan.Description = types.StringValue("an updated description")
	plan.Active = types.BoolValue(false)

	ops, diags := segmentPatchOps(ctx, plan, state)
	if diags.HasError() {
		t.Fatalf("segmentPatchOps returned diagnostics: %v", diags)
	}
	if len(ops) != 2 {
		t.Fatalf("len(ops) = %d, want 2: %+v", len(ops), ops)
	}

	byPath := map[string]segmentJSONPatchOp{}
	for _, op := range ops {
		byPath[op.Path] = op
	}

	descOp, ok := byPath["/description"]
	if !ok {
		t.Fatalf("no patch op for /description: %+v", ops)
	}
	if descOp.Op != "replace" {
		t.Errorf("/description Op = %q, want %q", descOp.Op, "replace")
	}

	activeOp, ok := byPath["/active"]
	if !ok {
		t.Fatalf("no patch op for /active: %+v", ops)
	}
	if activeOp.Op != "replace" {
		t.Errorf("/active Op = %q, want %q", activeOp.Op, "replace")
	}
}

func TestSegmentStringPatchOps(t *testing.T) {
	t.Run("null to value is add", func(t *testing.T) {
		ops := segmentStringPatchOps("/description", types.StringValue("new"), types.StringNull())
		if len(ops) != 1 || ops[0].Op != "add" {
			t.Fatalf("ops = %+v, want a single add op", ops)
		}
	})

	t.Run("value to null is remove", func(t *testing.T) {
		ops := segmentStringPatchOps("/description", types.StringNull(), types.StringValue("old"))
		if len(ops) != 1 || ops[0].Op != "remove" {
			t.Fatalf("ops = %+v, want a single remove op", ops)
		}
	})

	t.Run("value to different value is replace", func(t *testing.T) {
		ops := segmentStringPatchOps("/description", types.StringValue("new"), types.StringValue("old"))
		if len(ops) != 1 || ops[0].Op != "replace" {
			t.Fatalf("ops = %+v, want a single replace op", ops)
		}
	})

	t.Run("unchanged value produces no ops", func(t *testing.T) {
		ops := segmentStringPatchOps("/description", types.StringValue("same"), types.StringValue("same"))
		if len(ops) != 0 {
			t.Fatalf("ops = %+v, want empty", ops)
		}
	})

	t.Run("unknown plan produces no ops", func(t *testing.T) {
		ops := segmentStringPatchOps("/description", types.StringUnknown(), types.StringValue("old"))
		if len(ops) != 0 {
			t.Fatalf("ops = %+v, want empty (unknown plan means not-yet-computed, not a real change)", ops)
		}
	})
}

func TestSegmentVisibilityCriteriaPatchOps(t *testing.T) {
	ctx := context.Background()

	t.Run("no change produces no ops", func(t *testing.T) {
		vc := leafVisibilityCriteria(t, ctx, "EQUALS", "uid", "STRING", "N1")
		ops, diags := segmentVisibilityCriteriaPatchOps(ctx, vc, vc)
		if diags.HasError() {
			t.Fatalf("segmentVisibilityCriteriaPatchOps returned diagnostics: %v", diags)
		}
		if len(ops) != 0 {
			t.Fatalf("ops = %+v, want empty", ops)
		}
	})

	t.Run("changed value produces a single replace op", func(t *testing.T) {
		state := leafVisibilityCriteria(t, ctx, "EQUALS", "uid", "STRING", "N1")
		plan := leafVisibilityCriteria(t, ctx, "EQUALS", "uid", "STRING", "N2")

		ops, diags := segmentVisibilityCriteriaPatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("segmentVisibilityCriteriaPatchOps returned diagnostics: %v", diags)
		}
		if len(ops) != 1 {
			t.Fatalf("len(ops) = %d, want 1: %+v", len(ops), ops)
		}
		if ops[0].Op != "replace" || ops[0].Path != "/visibilityCriteria" {
			t.Errorf("op = %+v, want replace /visibilityCriteria", ops[0])
		}
		if ops[0].Value == nil {
			t.Fatal("op.Value is nil, want a populated map")
		}
	})

	t.Run("null plan on non-null state produces a remove op", func(t *testing.T) {
		state := leafVisibilityCriteria(t, ctx, "EQUALS", "uid", "STRING", "N1")
		plan := types.ObjectNull(visibilityCriteriaAttrTypes())

		ops, diags := segmentVisibilityCriteriaPatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("segmentVisibilityCriteriaPatchOps returned diagnostics: %v", diags)
		}
		if len(ops) != 1 || ops[0].Op != "remove" {
			t.Fatalf("ops = %+v, want a single remove op", ops)
		}
	})
}

func TestSegmentPatchRequestBody(t *testing.T) {
	name := "new-name"
	ops := []segmentJSONPatchOp{
		segmentJSONPatchReplace("/name", name),
	}

	body := segmentPatchRequestBody(ops)
	if len(body) != 1 {
		t.Fatalf("len(body) = %d, want 1", len(body))
	}
	if body[0]["op"] != "replace" {
		t.Errorf(`body[0]["op"] = %v, want "replace"`, body[0]["op"])
	}
	if body[0]["path"] != "/name" {
		t.Errorf(`body[0]["path"] = %v, want "/name"`, body[0]["path"])
	}
	if body[0]["value"] != "new-name" {
		t.Errorf(`body[0]["value"] = %v, want "new-name"`, body[0]["value"])
	}
}

func TestVisibilityChildModelToAPI_AlwaysNilsChildren(t *testing.T) {
	ctx := context.Background()
	model := visibilityChildModel{
		Operator:  types.StringValue("EQUALS"),
		Attribute: types.StringValue("department"),
		Value:     types.ObjectNull(visibilityValueAttrTypes()),
	}

	child, diags := visibilityChildModelToAPI(ctx, model)
	if diags.HasError() {
		t.Fatalf("visibilityChildModelToAPI returned diagnostics: %v", diags)
	}
	if child.Children.Get() != nil {
		t.Errorf("child.Children = %v, want nil (this target caps the tree at 2 levels)", child.Children.Get())
	}
	if !child.Children.IsSet() {
		t.Error("child.Children.IsSet() = false, want true (explicit null must be sent to the API)")
	}
}
