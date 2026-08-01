// See resource_sod_policy.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package sod_policy_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/sod_policy_v1/datasource_sod_policies"
)

var (
	_ datasource.DataSource              = (*sodPoliciesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sodPoliciesDataSource)(nil)
)

type sodPoliciesDataSourceModel struct {
	Filters     types.String `tfsdk:"filters"`
	Limit       types.Int64  `tfsdk:"limit"`
	Offset      types.Int64  `tfsdk:"offset"`
	SodPolicies types.Set    `tfsdk:"sod_policies"`
	Sorters     types.String `tfsdk:"sorters"`
}

// sodPoliciesListItemModel mirrors one element of the generated
// datasource_sod_policies.SodPoliciesValue plus the two hand-added fields -
// see resource_sod_policy.go's package doc for why they're hand-written.
// owner_ref here is a plain types.Object (not the generated OwnerRefValue
// CustomType): associated_external_type is not honored by
// tfplugingen-framework for an attribute nested inside a list_nested/
// set_nested collection (confirmed precedent: segment_v1's plural
// `segments` data source), so this field is hand-converted directly below.
type sodPoliciesListItemModel struct {
	CompensatingControls           types.String `tfsdk:"compensating_controls"`
	ConflictingAccessCriteria      types.Object `tfsdk:"conflicting_access_criteria"`
	CorrectionAdvice               types.String `tfsdk:"correction_advice"`
	Created                        types.String `tfsdk:"created"`
	CreatorId                      types.String `tfsdk:"creator_id"`
	Description                    types.String `tfsdk:"description"`
	ExternalPolicyReference        types.String `tfsdk:"external_policy_reference"`
	Id                             types.String `tfsdk:"id"`
	Modified                       types.String `tfsdk:"modified"`
	ModifierId                     types.String `tfsdk:"modifier_id"`
	Name                           types.String `tfsdk:"name"`
	OwnerRef                       types.Object `tfsdk:"owner_ref"`
	PolicyQuery                    types.String `tfsdk:"policy_query"`
	Scheduled                      types.Bool   `tfsdk:"scheduled"`
	State                          types.String `tfsdk:"state"`
	Tags                           types.List   `tfsdk:"tags"`
	Type                           types.String `tfsdk:"type"`
	ViolationOwnerAssignmentConfig types.Object `tfsdk:"violation_owner_assignment_config"`
}

func NewSodPoliciesDataSource() datasource.DataSource {
	return &sodPoliciesDataSource{}
}

type sodPoliciesDataSource struct {
	client *sailpoint.APIClient
}

func (d *sodPoliciesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sod_policies_v1"
}

func (d *sodPoliciesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = sodPoliciesDataSourceSchema(ctx)
	resp.Schema.Description = "Lists Separation of Duties (SOD) Policies from IdentityNow/ISC, optionally filtered/sorted/paginated."
	resp.Schema.MarkdownDescription = "Lists [Separation of Duties (SOD) Policies](https://documentation.sailpoint.com/saas/help/sod/manage-policies.html) " +
		"from IdentityNow/ISC via `GET /sod-policies/v1`, optionally filtered/sorted/paginated.\n\n" +
		"~> This is a `_v1` pilot data source. Per a live API check documented in the resource's \"Known Limitations & Live " +
		"Testing Notes\", invoking this data source with a fully-known `filters` value performs a live API call at `plan` " +
		"time, not just `apply` time."
}

func (d *sodPoliciesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = cp.GetClient()
}

func (d *sodPoliciesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sodPoliciesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading SOD Policies data source")

	apiReq := d.client.Beta.SODPoliciesAPI.ListSodPolicies(ctx)
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		apiReq = apiReq.Limit(int32(config.Limit.ValueInt64()))
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}
	if !config.Filters.IsNull() && !config.Filters.IsUnknown() {
		apiReq = apiReq.Filters(config.Filters.ValueString())
	}
	if !config.Sorters.IsNull() && !config.Sorters.IsUnknown() {
		apiReq = apiReq.Sorters(config.Sorters.ValueString())
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading SOD Policies data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing SOD Policies", errDetail(err, httpResp))
		return
	}

	fullType := sodPoliciesDataSourceSchema(ctx).Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["sod_policies"].(basetypes.SetType).ElemType

	items := make([]sodPoliciesListItemModel, 0, len(dtos))
	for i := range dtos {
		item, diags := sodPoliciesListItemFromDTO(ctx, &dtos[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		items = append(items, item)
	}

	policiesSet, diags := types.SetValueFrom(ctx, elemType, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.SodPolicies = policiesSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func sodPoliciesDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	s := datasource_sod_policies.SodPoliciesDataSourceSchema(ctx)
	if policiesAttr, ok := s.Attributes["sod_policies"].(datasourceschema.SetNestedAttribute); ok {
		applyDataSourceConflictingAccessCriteriaField(&policiesAttr.NestedObject.Attributes)
		applyDataSourceViolationOwnerAssignmentConfigField(&policiesAttr.NestedObject.Attributes)
		// The generated NestedObject carries a fixed `CustomType`
		// (SodPoliciesType) derived at codegen time from the
		// (conflictingAccessCriteria/violationOwnerAssignmentConfig-ignored)
		// generated model, which takes precedence over the Attributes map
		// when the framework computes .Type() - so without clearing it here,
		// the object type would silently omit the two fields just added
		// above, causing a "Struct defines fields not found in object"
		// conversion error at Read time (same fix as segment_v1's
		// segments_data_source.go).
		policiesAttr.NestedObject.CustomType = nil
		s.Attributes["sod_policies"] = policiesAttr
	}
	return s
}

func sodPoliciesListItemFromDTO(ctx context.Context, dto *api_beta.SodPolicy) (sodPoliciesListItemModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	item := sodPoliciesListItemModel{
		CompensatingControls:           types.StringNull(),
		ConflictingAccessCriteria:      types.ObjectNull(conflictingAccessCriteriaAttrTypes()),
		CorrectionAdvice:               types.StringNull(),
		Created:                        types.StringNull(),
		CreatorId:                      types.StringNull(),
		Description:                    types.StringNull(),
		ExternalPolicyReference:        types.StringNull(),
		Id:                             types.StringNull(),
		Modified:                       types.StringNull(),
		ModifierId:                     types.StringNull(),
		Name:                           types.StringNull(),
		OwnerRef:                       types.ObjectNull(pluralOwnerRefAttrTypes()),
		PolicyQuery:                    types.StringNull(),
		Scheduled:                      types.BoolNull(),
		State:                          types.StringNull(),
		Tags:                           types.ListNull(types.StringType),
		Type:                           types.StringNull(),
		ViolationOwnerAssignmentConfig: types.ObjectNull(violationOwnerAssignmentConfigAttrTypes()),
	}

	if dto == nil {
		return item, diags
	}
	if dto.Id != nil {
		item.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		item.Name = types.StringValue(*dto.Name)
	}
	item.Created = timeToStringValue(dto.Created)
	item.Modified = timeToStringValue(dto.Modified)
	if dto.Description.IsSet() {
		item.Description = types.StringPointerValue(dto.Description.Get())
	}
	if dto.ExternalPolicyReference.IsSet() {
		item.ExternalPolicyReference = types.StringPointerValue(dto.ExternalPolicyReference.Get())
	}
	if dto.PolicyQuery != nil {
		item.PolicyQuery = types.StringValue(*dto.PolicyQuery)
	}
	if dto.CompensatingControls.IsSet() {
		item.CompensatingControls = types.StringPointerValue(dto.CompensatingControls.Get())
	}
	if dto.CorrectionAdvice.IsSet() {
		item.CorrectionAdvice = types.StringPointerValue(dto.CorrectionAdvice.Get())
	}
	if dto.State != nil {
		item.State = types.StringValue(*dto.State)
	}
	if dto.Tags != nil {
		tagsList, d := types.ListValueFrom(ctx, types.StringType, dto.Tags)
		diags.Append(d...)
		item.Tags = tagsList
	}
	if dto.CreatorId != nil {
		item.CreatorId = types.StringValue(*dto.CreatorId)
	}
	if dto.ModifierId.IsSet() {
		item.ModifierId = types.StringPointerValue(dto.ModifierId.Get())
	}
	if dto.Scheduled != nil {
		item.Scheduled = types.BoolValue(*dto.Scheduled)
	}
	if dto.Type != nil {
		item.Type = types.StringValue(*dto.Type)
	}

	ownerObj, d := pluralOwnerRefObjectFromAPI(dto.OwnerRef)
	diags.Append(d...)
	item.OwnerRef = ownerObj

	voacObj, d := violationOwnerAssignmentConfigObjectFromAPI(ctx, dto.ViolationOwnerAssignmentConfig)
	diags.Append(d...)
	item.ViolationOwnerAssignmentConfig = voacObj

	cacObj, d := conflictingAccessCriteriaObjectFromAPI(ctx, dto.ConflictingAccessCriteria)
	diags.Append(d...)
	item.ConflictingAccessCriteria = cacObj

	return item, diags
}

func pluralOwnerRefAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"name": types.StringType,
		"type": types.StringType,
	}
}

func pluralOwnerRefObjectFromAPI(owner *api_beta.SodPolicyOwnerRef) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if owner == nil {
		return types.ObjectNull(pluralOwnerRefAttrTypes()), diags
	}
	obj, d := types.ObjectValue(pluralOwnerRefAttrTypes(), map[string]attr.Value{
		"id":   types.StringPointerValue(owner.Id),
		"name": types.StringPointerValue(owner.Name),
		"type": types.StringPointerValue(owner.Type),
	})
	diags.Append(d...)
	return obj, diags
}
