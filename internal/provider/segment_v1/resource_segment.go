// Package segment_v1 is a pilot implementation of the segment resource/data
// sources generated from SailPoint's new per-service v1 OpenAPI spec
// (api-specs/idn/apis/segments).
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model/value types in resource_segment,
// datasource_segment, and datasource_segments, backed by the golang-sdk v2
// api_beta.SegmentsAPI client (the SDK does not yet publish a per-service v1
// package; v1 is the stabilization of what was beta).
//
// Known limitations / design notes:
//   - visibility_criteria is intentionally hand-written here instead of
//     generated: tfplugingen-framework keys generated Go helper types only by
//     attribute name, so the spec's repeated nested "value" block
//     (expression.value and expression.children.value) caused duplicate
//     ValueType/ValueValue declarations and an unfixable build failure in the
//     generated package. The schema and SDK conversions for that subtree are
//     therefore implemented manually below.
//   - The API's visibility criteria tree is capped here at two levels exactly
//     as requested for this target: visibility_criteria.expression.children
//     exists, but each child's own children field is intentionally omitted from
//     Terraform and is always sent to the API as explicit null.
//   - owner is mapped via associated_external_type for the resource and
//     singular data source only, so those wrappers use the generated
//     ToApi_betaOwnerReferenceSegments/FromApi_betaOwnerReferenceSegments
//     helpers directly. The plural data source hand-converts owner instead.
package segment_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/segment_v1/resource_segment"
	"terraform-provider-identitynow/internal/provider/util"
)

var (
	_ resource.Resource                = (*segmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*segmentResource)(nil)
	_ resource.ResourceWithImportState = (*segmentResource)(nil)
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

const segmentVisibilityCriteriaDescription = "Visibility criteria controlling which identities the segment applies to. This hand-written schema intentionally supports exactly two levels: visibility_criteria.expression plus visibility_criteria.expression.children; each child element's own API children field is always sent as null and is therefore omitted from Terraform."

type segmentResourceModel struct {
	Active             types.Bool                  `tfsdk:"active"`
	Created            types.String                `tfsdk:"created"`
	Description        types.String                `tfsdk:"description"`
	Id                 types.String                `tfsdk:"id"`
	Modified           types.String                `tfsdk:"modified"`
	Name               types.String                `tfsdk:"name"`
	Owner              resource_segment.OwnerValue `tfsdk:"owner"`
	VisibilityCriteria types.Object                `tfsdk:"visibility_criteria"`
}

type visibilityCriteriaModel struct {
	Expression types.Object `tfsdk:"expression"`
}

type visibilityExpressionModel struct {
	Operator  types.String `tfsdk:"operator"`
	Attribute types.String `tfsdk:"attribute"`
	Value     types.Object `tfsdk:"value"`
	Children  types.List   `tfsdk:"children"`
}

type visibilityChildModel struct {
	Operator  types.String `tfsdk:"operator"`
	Attribute types.String `tfsdk:"attribute"`
	Value     types.Object `tfsdk:"value"`
}

type visibilityValueModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

func NewSegmentResource() resource.Resource {
	return &segmentResource{}
}

type segmentResource struct {
	client *sailpoint.APIClient
}

func (r *segmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment_v1"
}

func (r *segmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = segmentResourceSchema(ctx)
	resp.Schema.Description = "Manages a Segment in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Segment](https://developer.sailpoint.com/docs/api/v2025/create-segment) in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource. `visibility_criteria` is hand-written here because the generated pipeline cannot represent the spec's repeated nested `value` blocks without a Go symbol collision; see the package doc for details."
	applySegmentUseStateForUnknown(&resp.Schema)
}

func (r *segmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = cp.GetClient()
}

func (r *segmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *segmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan segmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Segment", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := segmentResourceModelToDTO(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.SegmentsAPI.
		CreateSegment(ctx).
		Segment(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Segment", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Segment", segmentErrDetail(err, httpResp))
		return
	}

	state, diags := segmentResourceDTOToModel(ctx, apiResp, plan.Id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *segmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state segmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Segment", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.Beta.SegmentsAPI.
		GetSegment(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Segment not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Segment", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Segment", segmentErrDetail(err, httpResp))
		return
	}

	newState, diags := segmentResourceDTOToModel(ctx, apiResp, state.Id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *segmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan segmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state segmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch, diags := segmentPatchOps(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(patch) == 0 {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	tflog.Debug(ctx, "Patching Segment", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.Beta.SegmentsAPI.
		PatchSegment(ctx, state.Id.ValueString()).
		RequestBody(segmentPatchRequestBody(patch)).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Segment", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Segment", segmentErrDetail(err, httpResp))
		return
	}

	newState, diags := segmentResourceDTOToModel(ctx, apiResp, state.Id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *segmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state segmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Segment", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.Beta.SegmentsAPI.
		DeleteSegment(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "Segment already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Segment", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Segment", segmentErrDetail(err, httpResp))
		return
	}
}

func segmentResourceSchema(ctx context.Context) resourceschema.Schema {
	s := resource_segment.SegmentResourceSchema(ctx)
	applyResourceVisibilityCriteriaField(&s.Attributes)
	return s
}

func segmentResourceModelToDTO(ctx context.Context, m segmentResourceModel) (*api_beta.Segment, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto := api_beta.NewSegmentWithDefaults()
	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		dto.Name = m.Name.ValueStringPointer()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		dto.Description = m.Description.ValueStringPointer()
	}
	if !m.Active.IsNull() && !m.Active.IsUnknown() {
		dto.Active = m.Active.ValueBoolPointer()
	}
	if !m.Owner.IsUnknown() {
		owner, d := m.Owner.ToApi_betaOwnerReferenceSegments(ctx)
		diags.Append(d...)
		if owner != nil {
			dto.Owner = *api_beta.NewNullableOwnerReferenceSegments(owner)
		}
	}

	vc, d := visibilityCriteriaObjectToAPI(ctx, m.VisibilityCriteria)
	diags.Append(d...)
	if vc != nil {
		dto.VisibilityCriteria = *api_beta.NewNullableVisibilityCriteria(vc)
	}

	return dto, diags
}

func segmentResourceDTOToModel(ctx context.Context, dto *api_beta.Segment, fallbackID types.String) (segmentResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := segmentResourceModel{
		Active:             types.BoolNull(),
		Created:            types.StringNull(),
		Description:        types.StringNull(),
		Id:                 fallbackID,
		Modified:           types.StringNull(),
		Name:               types.StringNull(),
		Owner:              resource_segment.NewOwnerValueNull(),
		VisibilityCriteria: types.ObjectNull(visibilityCriteriaAttrTypes()),
	}

	if dto == nil {
		return model, diags
	}
	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		model.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	}
	if dto.Active != nil {
		model.Active = types.BoolValue(*dto.Active)
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	owner, d := resource_segment.OwnerValue{}.FromApi_betaOwnerReferenceSegments(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	vc, d := visibilityCriteriaObjectFromAPI(ctx, dto.VisibilityCriteria.Get())
	diags.Append(d...)
	model.VisibilityCriteria = vc

	return model, diags
}

func segmentPatchOps(ctx context.Context, plan, state segmentResourceModel) ([]api_beta.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := make([]api_beta.JsonPatchOperation, 0, 5)

	patch = append(patch, segmentStringPatchOps("/name", plan.Name, state.Name)...)
	patch = append(patch, segmentStringPatchOps("/description", plan.Description, state.Description)...)
	patch = append(patch, segmentBoolPatchOps("/active", plan.Active, state.Active)...)

	ownerOps, d := segmentOwnerPatchOps(ctx, plan.Owner, state.Owner)
	diags.Append(d...)
	patch = append(patch, ownerOps...)

	visibilityOps, d := segmentVisibilityCriteriaPatchOps(ctx, plan.VisibilityCriteria, state.VisibilityCriteria)
	diags.Append(d...)
	patch = append(patch, visibilityOps...)

	return patch, diags
}

func segmentStringPatchOps(path string, plan, state types.String) []api_beta.JsonPatchOperation {
	if plan.IsUnknown() || plan.Equal(state) {
		return nil
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil
		}
		return []api_beta.JsonPatchOperation{segmentJSONPatchRemove(path)}
	}
	v := plan.ValueString()
	if state.IsNull() || state.IsUnknown() {
		return []api_beta.JsonPatchOperation{segmentJSONPatchAdd(path, api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&v))}
	}
	return []api_beta.JsonPatchOperation{segmentJSONPatchReplace(path, api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&v))}
}

func segmentBoolPatchOps(path string, plan, state types.Bool) []api_beta.JsonPatchOperation {
	if plan.IsUnknown() || plan.IsNull() || plan.Equal(state) {
		return nil
	}
	v := plan.ValueBool()
	if state.IsNull() || state.IsUnknown() {
		return []api_beta.JsonPatchOperation{segmentJSONPatchAdd(path, api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v))}
	}
	return []api_beta.JsonPatchOperation{segmentJSONPatchReplace(path, api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(&v))}
}

func segmentOwnerPatchOps(ctx context.Context, plan, state resource_segment.OwnerValue) ([]api_beta.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || plan.Equal(state) {
		return nil, diags
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil, diags
		}
		return []api_beta.JsonPatchOperation{segmentJSONPatchRemove("/owner")}, diags
	}

	owner, d := plan.ToApi_betaOwnerReferenceSegments(ctx)
	diags.Append(d...)
	if diags.HasError() || owner == nil {
		return nil, diags
	}
	ownerMap, err := segmentStructToMap(owner)
	if err != nil {
		diags.AddError("Error preparing owner patch value", err.Error())
		return nil, diags
	}
	value := api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&ownerMap)
	if state.IsNull() || state.IsUnknown() {
		return []api_beta.JsonPatchOperation{segmentJSONPatchAdd("/owner", value)}, diags
	}
	return []api_beta.JsonPatchOperation{segmentJSONPatchReplace("/owner", value)}, diags
}

func segmentVisibilityCriteriaPatchOps(ctx context.Context, plan, state types.Object) ([]api_beta.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || plan.Equal(state) {
		return nil, diags
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil, diags
		}
		return []api_beta.JsonPatchOperation{segmentJSONPatchRemove("/visibilityCriteria")}, diags
	}

	vc, d := visibilityCriteriaObjectToAPI(ctx, plan)
	diags.Append(d...)
	if diags.HasError() || vc == nil {
		return nil, diags
	}
	vcMap, err := segmentStructToMap(vc)
	if err != nil {
		diags.AddError("Error preparing visibility_criteria patch value", err.Error())
		return nil, diags
	}
	value := api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&vcMap)
	if state.IsNull() || state.IsUnknown() {
		return []api_beta.JsonPatchOperation{segmentJSONPatchAdd("/visibilityCriteria", value)}, diags
	}
	return []api_beta.JsonPatchOperation{segmentJSONPatchReplace("/visibilityCriteria", value)}, diags
}

func applyResourceVisibilityCriteriaField(attrs *map[string]resourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]resourceschema.Attribute{}
	}
	(*attrs)["visibility_criteria"] = resourceschema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		Description:         segmentVisibilityCriteriaDescription,
		MarkdownDescription: segmentVisibilityCriteriaDescription,
		Attributes: map[string]resourceschema.Attribute{
			"expression": resourceschema.SingleNestedAttribute{
				Required:            true,
				Description:         "Root visibility criteria expression.",
				MarkdownDescription: "Root visibility criteria expression.",
				Attributes:          resourceVisibilityExpressionAttributes(),
			},
		},
	}
}

func applyDataSourceVisibilityCriteriaField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	(*attrs)["visibility_criteria"] = datasourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         segmentVisibilityCriteriaDescription,
		MarkdownDescription: segmentVisibilityCriteriaDescription,
		Attributes: map[string]datasourceschema.Attribute{
			"expression": datasourceschema.SingleNestedAttribute{
				Computed:            true,
				Description:         "Root visibility criteria expression.",
				MarkdownDescription: "Root visibility criteria expression.",
				Attributes:          dataSourceVisibilityExpressionAttributes(),
			},
		},
	}
}

func resourceVisibilityExpressionAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"operator": resourceschema.StringAttribute{
			Required:            true,
			Description:         "Operator for the expression.",
			MarkdownDescription: "Operator for the expression.",
			Validators:          segmentVisibilityOperatorValidators(),
		},
		"attribute": resourceschema.StringAttribute{
			Optional:            true,
			Description:         "Attribute name for leaf expressions.",
			MarkdownDescription: "Attribute name for leaf expressions.",
		},
		"value": resourceschema.SingleNestedAttribute{
			Optional:            true,
			Description:         "Attribute value for leaf expressions.",
			MarkdownDescription: "Attribute value for leaf expressions.",
			Attributes:          resourceVisibilityValueAttributes(),
		},
		"children": resourceschema.ListNestedAttribute{
			Optional:            true,
			Description:         "Second-level child expressions. A child element's own API children field is always null and is intentionally omitted from Terraform.",
			MarkdownDescription: "Second-level child expressions. A child element's own API `children` field is always `null` and is intentionally omitted from Terraform.",
			NestedObject:        resourceschema.NestedAttributeObject{Attributes: resourceVisibilityChildAttributes()},
		},
	}
}

func resourceVisibilityChildAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"operator": resourceschema.StringAttribute{
			Required:            true,
			Description:         "Operator for the child expression.",
			MarkdownDescription: "Operator for the child expression.",
			Validators:          segmentVisibilityOperatorValidators(),
		},
		"attribute": resourceschema.StringAttribute{
			Optional:            true,
			Description:         "Attribute name for leaf child expressions.",
			MarkdownDescription: "Attribute name for leaf child expressions.",
		},
		"value": resourceschema.SingleNestedAttribute{
			Optional:            true,
			Description:         "Attribute value for leaf child expressions.",
			MarkdownDescription: "Attribute value for leaf child expressions.",
			Attributes:          resourceVisibilityValueAttributes(),
		},
	}
}

func resourceVisibilityValueAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"type": resourceschema.StringAttribute{
			Optional:            true,
			Description:         "The type of attribute value.",
			MarkdownDescription: "The type of attribute value.",
		},
		"value": resourceschema.StringAttribute{
			Optional:            true,
			Description:         "The attribute value.",
			MarkdownDescription: "The attribute value.",
		},
	}
}

func dataSourceVisibilityExpressionAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"operator": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "Operator for the expression.",
			MarkdownDescription: "Operator for the expression.",
			Validators:          segmentVisibilityOperatorValidators(),
		},
		"attribute": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "Attribute name for leaf expressions.",
			MarkdownDescription: "Attribute name for leaf expressions.",
		},
		"value": datasourceschema.SingleNestedAttribute{
			Computed:            true,
			Description:         "Attribute value for leaf expressions.",
			MarkdownDescription: "Attribute value for leaf expressions.",
			Attributes:          dataSourceVisibilityValueAttributes(),
		},
		"children": datasourceschema.ListNestedAttribute{
			Computed:            true,
			Description:         "Second-level child expressions. A child element's own API children field is always null and is intentionally omitted from Terraform.",
			MarkdownDescription: "Second-level child expressions. A child element's own API `children` field is always `null` and is intentionally omitted from Terraform.",
			NestedObject:        datasourceschema.NestedAttributeObject{Attributes: dataSourceVisibilityChildAttributes()},
		},
	}
}

func dataSourceVisibilityChildAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"operator": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "Operator for the child expression.",
			MarkdownDescription: "Operator for the child expression.",
			Validators:          segmentVisibilityOperatorValidators(),
		},
		"attribute": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "Attribute name for leaf child expressions.",
			MarkdownDescription: "Attribute name for leaf child expressions.",
		},
		"value": datasourceschema.SingleNestedAttribute{
			Computed:            true,
			Description:         "Attribute value for leaf child expressions.",
			MarkdownDescription: "Attribute value for leaf child expressions.",
			Attributes:          dataSourceVisibilityValueAttributes(),
		},
	}
}

func dataSourceVisibilityValueAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"type": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "The type of attribute value.",
			MarkdownDescription: "The type of attribute value.",
		},
		"value": datasourceschema.StringAttribute{
			Computed:            true,
			Description:         "The attribute value.",
			MarkdownDescription: "The attribute value.",
		},
	}
}

func segmentVisibilityOperatorValidators() []validator.String {
	return []validator.String{stringvalidator.OneOf("AND", "EQUALS")}
}

func visibilityCriteriaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"expression": types.ObjectType{AttrTypes: visibilityExpressionAttrTypes()},
	}
}

func visibilityExpressionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"operator":  types.StringType,
		"attribute": types.StringType,
		"value":     types.ObjectType{AttrTypes: visibilityValueAttrTypes()},
		"children":  types.ListType{ElemType: types.ObjectType{AttrTypes: visibilityChildAttrTypes()}},
	}
}

func visibilityChildAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"operator":  types.StringType,
		"attribute": types.StringType,
		"value":     types.ObjectType{AttrTypes: visibilityValueAttrTypes()},
	}
}

func visibilityValueAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"value": types.StringType,
	}
}

func visibilityCriteriaObjectToAPI(ctx context.Context, obj types.Object) (*api_beta.VisibilityCriteria, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("Visibility criteria is unknown", "visibility_criteria must be known before it can be sent to the API.")
		return nil, diags
	}

	var model visibilityCriteriaModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	expr, d := visibilityExpressionObjectToAPI(ctx, model.Expression)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	vc := api_beta.NewVisibilityCriteria()
	if expr != nil {
		vc.Expression = expr
	}
	return vc, diags
}

func visibilityExpressionObjectToAPI(ctx context.Context, obj types.Object) (*api_beta.Expression, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("Visibility criteria expression is unknown", "visibility_criteria.expression must be known before it can be sent to the API.")
		return nil, diags
	}

	var model visibilityExpressionModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	expr := api_beta.NewExpression()
	if !model.Operator.IsNull() && !model.Operator.IsUnknown() {
		op := model.Operator.ValueString()
		expr.Operator = &op
	}
	if !model.Attribute.IsUnknown() {
		if model.Attribute.IsNull() {
			expr.SetAttributeNil()
		} else {
			expr.SetAttribute(model.Attribute.ValueString())
		}
	}

	value, d := visibilityValueObjectToAPI(ctx, model.Value)
	diags.Append(d...)
	if value != nil {
		expr.SetValue(*value)
	} else if !model.Value.IsUnknown() && !model.Value.IsNull() {
		expr.SetValueNil()
	}

	if !model.Children.IsNull() && !model.Children.IsUnknown() {
		var items []visibilityChildModel
		diags.Append(model.Children.ElementsAs(ctx, &items, false)...)
		children := make([]api_beta.Children, 0, len(items))
		for _, item := range items {
			child, d := visibilityChildModelToAPI(ctx, item)
			diags.Append(d...)
			if child != nil {
				children = append(children, *child)
			}
		}
		expr.Children = children
	}

	return expr, diags
}

func visibilityChildModelToAPI(ctx context.Context, model visibilityChildModel) (*api_beta.Children, diag.Diagnostics) {
	var diags diag.Diagnostics
	child := api_beta.NewChildren()

	if !model.Operator.IsNull() && !model.Operator.IsUnknown() {
		op := model.Operator.ValueString()
		child.Operator = &op
	}
	if !model.Attribute.IsNull() && !model.Attribute.IsUnknown() {
		child.SetAttribute(model.Attribute.ValueString())
	}

	value, d := visibilityValueObjectToAPI(ctx, model.Value)
	diags.Append(d...)
	if value != nil {
		child.SetValue(*value)
	} else if !model.Value.IsUnknown() && !model.Value.IsNull() {
		child.SetValueNil()
	}

	// The second level is the hard cap for this target. The API's child.children
	// field is always explicit null and intentionally omitted from Terraform.
	child.SetChildrenNil()

	return child, diags
}

func visibilityValueObjectToAPI(ctx context.Context, obj types.Object) (*api_beta.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("Visibility criteria value is unknown", "visibility_criteria expression values must be known before they can be sent to the API.")
		return nil, diags
	}

	var model visibilityValueModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	value := api_beta.NewValue()
	if !model.Type.IsUnknown() {
		if model.Type.IsNull() {
			value.SetTypeNil()
		} else {
			value.SetType(model.Type.ValueString())
		}
	}
	if !model.Value.IsNull() && !model.Value.IsUnknown() {
		value.Value = model.Value.ValueStringPointer()
	}
	return value, diags
}

func visibilityCriteriaObjectFromAPI(ctx context.Context, vc *api_beta.VisibilityCriteria) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if vc == nil {
		return types.ObjectNull(visibilityCriteriaAttrTypes()), diags
	}

	exprObj, d := visibilityExpressionObjectFromAPI(ctx, vc.Expression)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(visibilityCriteriaAttrTypes()), diags
	}

	obj, d := types.ObjectValue(visibilityCriteriaAttrTypes(), map[string]attr.Value{
		"expression": exprObj,
	})
	diags.Append(d...)
	return obj, diags
}

func visibilityExpressionObjectFromAPI(ctx context.Context, expr *api_beta.Expression) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if expr == nil {
		return types.ObjectNull(visibilityExpressionAttrTypes()), diags
	}

	valueObj, d := visibilityValueObjectFromAPI(ctx, expr.Value.Get())
	diags.Append(d...)

	childrenList, d := visibilityChildrenListFromAPI(ctx, expr.Children)
	diags.Append(d...)

	obj, d := types.ObjectValue(visibilityExpressionAttrTypes(), map[string]attr.Value{
		"operator":  types.StringPointerValue(expr.Operator),
		"attribute": types.StringPointerValue(expr.Attribute.Get()),
		"value":     valueObj,
		"children":  childrenList,
	})
	diags.Append(d...)
	return obj, diags
}

func visibilityChildrenListFromAPI(ctx context.Context, items []api_beta.Children) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := types.ObjectType{AttrTypes: visibilityChildAttrTypes()}
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]attr.Value, 0, len(items))
	for i := range items {
		valueObj, d := visibilityValueObjectFromAPI(ctx, items[i].Value.Get())
		diags.Append(d...)
		childObj, d := types.ObjectValue(visibilityChildAttrTypes(), map[string]attr.Value{
			"operator":  types.StringPointerValue(items[i].Operator),
			"attribute": types.StringPointerValue(items[i].Attribute),
			"value":     valueObj,
		})
		diags.Append(d...)
		values = append(values, childObj)
	}

	listVal, d := types.ListValue(elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func visibilityValueObjectFromAPI(ctx context.Context, value *api_beta.Value) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	_ = ctx
	if value == nil {
		return types.ObjectNull(visibilityValueAttrTypes()), diags
	}

	obj, d := types.ObjectValue(visibilityValueAttrTypes(), map[string]attr.Value{
		"type":  types.StringPointerValue(value.Type.Get()),
		"value": types.StringPointerValue(value.Value),
	})
	diags.Append(d...)
	return obj, diags
}

func segmentErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func segmentJSONPatchAdd(path string, value api_beta.UpdateMultiHostSourcesRequestInnerValue) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{Op: "add", Path: path, Value: &value}
}

func segmentJSONPatchReplace(path string, value api_beta.UpdateMultiHostSourcesRequestInnerValue) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{Op: "replace", Path: path, Value: &value}
}

func segmentJSONPatchRemove(path string) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{Op: "remove", Path: path}
}

func segmentPatchRequestBody(ops []api_beta.JsonPatchOperation) []map[string]interface{} {
	body := make([]map[string]interface{}, 0, len(ops))
	for _, op := range ops {
		m, err := segmentStructToMap(op)
		if err != nil || m == nil {
			continue
		}
		body = append(body, m)
	}
	return body
}

func segmentStructToMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func timeToStringValue(t *api_beta.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
