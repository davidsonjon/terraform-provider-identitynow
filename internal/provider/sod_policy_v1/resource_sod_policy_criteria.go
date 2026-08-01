package sod_policy_v1

// Hand-written conversion helpers for the two fields excluded from codegen
// via generator_config_sod_policy_v1.yml's `schema.ignores` (see the package
// doc in resource_sod_policy.go for why): "conflicting_access_criteria" and
// "violation_owner_assignment_config". Both are modeled as plain
// `types.Object` (not a generated CustomType), following the exact pattern
// segment_v1 established for its "visibility_criteria" field.

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/sailpoint-oss/golang-sdk/v3/sod_policies"
)

// --- conflicting_access_criteria ---

type conflictingAccessCriteriaModel struct {
	LeftCriteria  types.Object `tfsdk:"left_criteria"`
	RightCriteria types.Object `tfsdk:"right_criteria"`
}

type accessCriteriaModel struct {
	Name         types.String `tfsdk:"name"`
	CriteriaList types.List   `tfsdk:"criteria_list"`
}

type criteriaListItemModel struct {
	Type types.String `tfsdk:"type"`
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func criteriaListItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"id":   types.StringType,
		"name": types.StringType,
	}
}

func accessCriteriaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":          types.StringType,
		"criteria_list": types.ListType{ElemType: types.ObjectType{AttrTypes: criteriaListItemAttrTypes()}},
	}
}

func conflictingAccessCriteriaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"left_criteria":  types.ObjectType{AttrTypes: accessCriteriaAttrTypes()},
		"right_criteria": types.ObjectType{AttrTypes: accessCriteriaAttrTypes()},
	}
}

// applyResourceConflictingAccessCriteriaField adds the hand-written
// "conflicting_access_criteria" attribute to a resource schema's generated
// Attributes map (see generator_config_sod_policy_v1.yml's schema.ignores
// comment - this field could not be generated due to a Go symbol collision).
func applyResourceConflictingAccessCriteriaField(attrs *map[string]schema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	(*attrs)["conflicting_access_criteria"] = schema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		Description:         conflictingAccessCriteriaDescription,
		MarkdownDescription: conflictingAccessCriteriaDescription,
		Attributes: map[string]schema.Attribute{
			"left_criteria":  accessCriteriaResourceAttribute(),
			"right_criteria": accessCriteriaResourceAttribute(),
		},
	}
}

const conflictingAccessCriteriaDescription = "The two access-item lists (left_criteria/right_criteria) that define a " +
	"CONFLICTING_ACCESS_BASED policy - required (and only meaningful) when type = \"CONFLICTING_ACCESS_BASED\"; left as " +
	"null for a type = \"GENERAL\" policy. Each list holds 1-50 ENTITLEMENT references; the API derives policy_query " +
	"itself from this criteria."

func accessCriteriaResourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Business name for this access construct list.",
				MarkdownDescription: "Business name for this access construct list.",
			},
			"criteria_list": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "List of criteria (1-50 items). Each item references an ENTITLEMENT by id.",
				MarkdownDescription: "List of criteria (1-50 items). Each item references an ENTITLEMENT by id.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Description:         "Type of the referenced object. Only \"ENTITLEMENT\" is currently supported.",
							MarkdownDescription: "Type of the referenced object. Only `ENTITLEMENT` is currently supported.",
						},
						"id": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Description:         "ID of the referenced object.",
							MarkdownDescription: "ID of the referenced object.",
						},
						"name": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Description:         "Human-readable display name of the referenced object.",
							MarkdownDescription: "Human-readable display name of the referenced object.",
						},
					},
				},
			},
		},
	}
}

// applyDataSourceConflictingAccessCriteriaField is the data-source-schema
// equivalent of applyResourceConflictingAccessCriteriaField above (all
// attributes Computed-only, no Optional).
func applyDataSourceConflictingAccessCriteriaField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	(*attrs)["conflicting_access_criteria"] = datasourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         conflictingAccessCriteriaDescription,
		MarkdownDescription: conflictingAccessCriteriaDescription,
		Attributes: map[string]datasourceschema.Attribute{
			"left_criteria":  accessCriteriaDataSourceAttribute(),
			"right_criteria": accessCriteriaDataSourceAttribute(),
		},
	}
}

func accessCriteriaDataSourceAttribute() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]datasourceschema.Attribute{
			"name": datasourceschema.StringAttribute{
				Computed:            true,
				Description:         "Business name for this access construct list.",
				MarkdownDescription: "Business name for this access construct list.",
			},
			"criteria_list": datasourceschema.ListNestedAttribute{
				Computed:            true,
				Description:         "List of criteria (1-50 items). Each item references an ENTITLEMENT by id.",
				MarkdownDescription: "List of criteria (1-50 items). Each item references an ENTITLEMENT by id.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: map[string]datasourceschema.Attribute{
						"type": datasourceschema.StringAttribute{
							Computed:            true,
							Description:         "Type of the referenced object. Only \"ENTITLEMENT\" is currently supported.",
							MarkdownDescription: "Type of the referenced object. Only `ENTITLEMENT` is currently supported.",
						},
						"id": datasourceschema.StringAttribute{
							Computed:            true,
							Description:         "ID of the referenced object.",
							MarkdownDescription: "ID of the referenced object.",
						},
						"name": datasourceschema.StringAttribute{
							Computed:            true,
							Description:         "Human-readable display name of the referenced object.",
							MarkdownDescription: "Human-readable display name of the referenced object.",
						},
					},
				},
			},
		},
	}
}

func conflictingAccessCriteriaObjectToAPI(ctx context.Context, obj types.Object) (*sod_policies.SodPolicyConflictingAccessCriteria, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("conflicting_access_criteria is unknown", "conflicting_access_criteria must be known before it can be sent to the API.")
		return nil, diags
	}

	var model conflictingAccessCriteriaModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	left, d := accessCriteriaObjectToAPI(ctx, model.LeftCriteria)
	diags.Append(d...)
	right, d := accessCriteriaObjectToAPI(ctx, model.RightCriteria)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	cac := sod_policies.NewSodPolicyConflictingAccessCriteriaWithDefaults()
	cac.LeftCriteria = left
	cac.RightCriteria = right
	return cac, diags
}

func accessCriteriaObjectToAPI(ctx context.Context, obj types.Object) (*sod_policies.AccessCriteria, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var model accessCriteriaModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	ac := sod_policies.NewAccessCriteriaWithDefaults()
	if !model.Name.IsNull() && !model.Name.IsUnknown() {
		ac.Name = model.Name.ValueStringPointer()
	}

	if !model.CriteriaList.IsNull() && !model.CriteriaList.IsUnknown() {
		var items []criteriaListItemModel
		diags.Append(model.CriteriaList.ElementsAs(ctx, &items, false)...)
		list := make([]sod_policies.AccessCriteriaCriteriaListInner, 0, len(items))
		for _, item := range items {
			list = append(list, sod_policies.AccessCriteriaCriteriaListInner{
				Type: item.Type.ValueStringPointer(),
				Id:   item.Id.ValueStringPointer(),
				Name: item.Name.ValueStringPointer(),
			})
		}
		ac.CriteriaList = list
	}

	return ac, diags
}

func conflictingAccessCriteriaObjectFromAPI(ctx context.Context, cac *sod_policies.SodPolicyConflictingAccessCriteria) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cac == nil {
		return types.ObjectNull(conflictingAccessCriteriaAttrTypes()), diags
	}

	left, d := accessCriteriaObjectFromAPI(ctx, cac.LeftCriteria)
	diags.Append(d...)
	right, d := accessCriteriaObjectFromAPI(ctx, cac.RightCriteria)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(conflictingAccessCriteriaAttrTypes()), diags
	}

	obj, d := types.ObjectValue(conflictingAccessCriteriaAttrTypes(), map[string]attr.Value{
		"left_criteria":  left,
		"right_criteria": right,
	})
	diags.Append(d...)
	return obj, diags
}

func accessCriteriaObjectFromAPI(ctx context.Context, ac *sod_policies.AccessCriteria) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if ac == nil {
		return types.ObjectNull(accessCriteriaAttrTypes()), diags
	}

	elemType := types.ObjectType{AttrTypes: criteriaListItemAttrTypes()}
	values := make([]attr.Value, 0, len(ac.CriteriaList))
	for i := range ac.CriteriaList {
		item := ac.CriteriaList[i]
		itemObj, d := types.ObjectValue(criteriaListItemAttrTypes(), map[string]attr.Value{
			"type": types.StringPointerValue(item.Type),
			"id":   types.StringPointerValue(item.Id),
			"name": types.StringPointerValue(item.Name),
		})
		diags.Append(d...)
		values = append(values, itemObj)
	}
	criteriaList, d := types.ListValue(elemType, values)
	diags.Append(d...)

	obj, d := types.ObjectValue(accessCriteriaAttrTypes(), map[string]attr.Value{
		"name":          types.StringPointerValue(ac.Name),
		"criteria_list": criteriaList,
	})
	diags.Append(d...)
	return obj, diags
}

// --- violation_owner_assignment_config ---

type violationOwnerAssignmentConfigModel struct {
	AssignmentRule types.String `tfsdk:"assignment_rule"`
	OwnerRef       types.Object `tfsdk:"owner_ref"`
}

type violationOwnerRefModel struct {
	Type types.String `tfsdk:"type"`
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func violationOwnerRefAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"id":   types.StringType,
		"name": types.StringType,
	}
}

func violationOwnerAssignmentConfigAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"assignment_rule": types.StringType,
		"owner_ref":       types.ObjectType{AttrTypes: violationOwnerRefAttrTypes()},
	}
}

const violationOwnerAssignmentConfigDescription = "Configures who is assigned as the owner of violations this policy " +
	"generates. assignment_rule = \"MANAGER\" assigns the violating identity's manager; assignment_rule = \"STATIC\" " +
	"assigns a specific owner_ref (an IDENTITY or GOVERNANCE_GROUP)."

// applyResourceViolationOwnerAssignmentConfigField adds the hand-written
// "violation_owner_assignment_config" attribute to a resource schema's
// generated Attributes map (see generator_config_sod_policy_v1.yml's
// schema.ignores comment - this field could not be generated due to a Go
// symbol collision on the nested "owner_ref" name, and separately has
// NullableString-shaped SDK fields incompatible with generated To/From
// converters regardless).
func applyResourceViolationOwnerAssignmentConfigField(attrs *map[string]schema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	(*attrs)["violation_owner_assignment_config"] = schema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		Description:         violationOwnerAssignmentConfigDescription,
		MarkdownDescription: violationOwnerAssignmentConfigDescription,
		Attributes: map[string]schema.Attribute{
			"assignment_rule": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "MANAGER or STATIC.",
				MarkdownDescription: "`MANAGER` or `STATIC`.",
			},
			"owner_ref": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "The static violation owner (required when assignment_rule = \"STATIC\").",
				MarkdownDescription: "The static violation owner (required when `assignment_rule = \"STATIC\"`).",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Description:         "Owner type: IDENTITY, GOVERNANCE_GROUP, or MANAGER.",
						MarkdownDescription: "Owner type: `IDENTITY`, `GOVERNANCE_GROUP`, or `MANAGER`.",
					},
					"id": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Description:         "Owner's ID.",
						MarkdownDescription: "Owner's ID.",
					},
					"name": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Description:         "Owner's name.",
						MarkdownDescription: "Owner's name.",
					},
				},
			},
		},
	}
}

// applyDataSourceViolationOwnerAssignmentConfigField is the
// data-source-schema equivalent of
// applyResourceViolationOwnerAssignmentConfigField above (all attributes
// Computed-only, no Optional).
func applyDataSourceViolationOwnerAssignmentConfigField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	(*attrs)["violation_owner_assignment_config"] = datasourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         violationOwnerAssignmentConfigDescription,
		MarkdownDescription: violationOwnerAssignmentConfigDescription,
		Attributes: map[string]datasourceschema.Attribute{
			"assignment_rule": datasourceschema.StringAttribute{
				Computed:            true,
				Description:         "MANAGER or STATIC.",
				MarkdownDescription: "`MANAGER` or `STATIC`.",
			},
			"owner_ref": datasourceschema.SingleNestedAttribute{
				Computed:            true,
				Description:         "The static violation owner (set when assignment_rule = \"STATIC\").",
				MarkdownDescription: "The static violation owner (set when `assignment_rule = \"STATIC\"`).",
				Attributes: map[string]datasourceschema.Attribute{
					"type": datasourceschema.StringAttribute{
						Computed:            true,
						Description:         "Owner type: IDENTITY, GOVERNANCE_GROUP, or MANAGER.",
						MarkdownDescription: "Owner type: `IDENTITY`, `GOVERNANCE_GROUP`, or `MANAGER`.",
					},
					"id": datasourceschema.StringAttribute{
						Computed:            true,
						Description:         "Owner's ID.",
						MarkdownDescription: "Owner's ID.",
					},
					"name": datasourceschema.StringAttribute{
						Computed:            true,
						Description:         "Owner's name.",
						MarkdownDescription: "Owner's name.",
					},
				},
			},
		},
	}
}

func violationOwnerAssignmentConfigObjectToAPI(ctx context.Context, obj types.Object) (*sod_policies.ViolationOwnerAssignmentConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("violation_owner_assignment_config is unknown", "violation_owner_assignment_config must be known before it can be sent to the API.")
		return nil, diags
	}

	var model violationOwnerAssignmentConfigModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	voac := sod_policies.NewViolationOwnerAssignmentConfigWithDefaults()
	voac.AssignmentRule = *sod_policies.NewNullableString(model.AssignmentRule.ValueStringPointer())

	if !model.OwnerRef.IsNull() && !model.OwnerRef.IsUnknown() {
		var ownerModel violationOwnerRefModel
		diags.Append(model.OwnerRef.As(ctx, &ownerModel, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		owner := sod_policies.NewViolationOwnerAssignmentConfigOwnerRefWithDefaults()
		owner.Type = *sod_policies.NewNullableString(ownerModel.Type.ValueStringPointer())
		owner.Id = ownerModel.Id.ValueStringPointer()
		owner.Name = ownerModel.Name.ValueStringPointer()
		voac.OwnerRef = *sod_policies.NewNullableViolationOwnerAssignmentConfigOwnerRef(owner)
	}

	return voac, diags
}

func violationOwnerAssignmentConfigObjectFromAPI(ctx context.Context, voac *sod_policies.ViolationOwnerAssignmentConfig) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if voac == nil {
		return types.ObjectNull(violationOwnerAssignmentConfigAttrTypes()), diags
	}

	ownerObj, d := violationOwnerRefObjectFromAPI(ctx, voac.OwnerRef.Get())
	diags.Append(d...)

	obj, d := types.ObjectValue(violationOwnerAssignmentConfigAttrTypes(), map[string]attr.Value{
		"assignment_rule": types.StringPointerValue(voac.AssignmentRule.Get()),
		"owner_ref":       ownerObj,
	})
	diags.Append(d...)
	return obj, diags
}

func violationOwnerRefObjectFromAPI(ctx context.Context, ref *sod_policies.ViolationOwnerAssignmentConfigOwnerRef) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	_ = ctx
	if ref == nil {
		return types.ObjectNull(violationOwnerRefAttrTypes()), diags
	}

	obj, d := types.ObjectValue(violationOwnerRefAttrTypes(), map[string]attr.Value{
		"type": types.StringPointerValue(ref.Type.Get()),
		"id":   types.StringPointerValue(ref.Id),
		"name": types.StringPointerValue(ref.Name),
	})
	diags.Append(d...)
	return obj, diags
}
