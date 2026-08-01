// See resource_sod_policy.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package sod_policy_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/sod_policies"

	"terraform-provider-identitynow/internal/provider/sod_policy_v1/datasource_sod_policy"
)

var (
	_ datasource.DataSource              = (*sodPolicyDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sodPolicyDataSource)(nil)
)

// sodPolicyDataSourceModel mirrors datasource_sod_policy.SodPolicyModel plus
// the two hand-added fields the generator was told to ignore - see the
// resource's package doc for why.
type sodPolicyDataSourceModel struct {
	CompensatingControls           types.String                        `tfsdk:"compensating_controls"`
	ConflictingAccessCriteria      types.Object                        `tfsdk:"conflicting_access_criteria"`
	CorrectionAdvice               types.String                        `tfsdk:"correction_advice"`
	Created                        types.String                        `tfsdk:"created"`
	CreatorId                      types.String                        `tfsdk:"creator_id"`
	Description                    types.String                        `tfsdk:"description"`
	ExternalPolicyReference        types.String                        `tfsdk:"external_policy_reference"`
	Id                             types.String                        `tfsdk:"id"`
	Modified                       types.String                        `tfsdk:"modified"`
	ModifierId                     types.String                        `tfsdk:"modifier_id"`
	Name                           types.String                        `tfsdk:"name"`
	OwnerRef                       datasource_sod_policy.OwnerRefValue `tfsdk:"owner_ref"`
	PolicyQuery                    types.String                        `tfsdk:"policy_query"`
	Scheduled                      types.Bool                          `tfsdk:"scheduled"`
	State                          types.String                        `tfsdk:"state"`
	Tags                           types.List                          `tfsdk:"tags"`
	Type                           types.String                        `tfsdk:"type"`
	ViolationOwnerAssignmentConfig types.Object                        `tfsdk:"violation_owner_assignment_config"`
}

func NewSodPolicyDataSource() datasource.DataSource {
	return &sodPolicyDataSource{}
}

type sodPolicyDataSource struct {
	client *sailpoint.APIClient
}

func (d *sodPolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sod_policy_v1"
}

func (d *sodPolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = sodPolicyDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Separation of Duties (SOD) Policy from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Separation of Duties (SOD) Policy](https://documentation.sailpoint.com/saas/help/sod/manage-policies.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see the \"Known Limitations & Live Testing Notes\" section below before " +
		"relying on it in production configurations."
}

func sodPolicyDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	s := datasource_sod_policy.SodPolicyDataSourceSchema(ctx)
	applyDataSourceConflictingAccessCriteriaField(&s.Attributes)
	applyDataSourceViolationOwnerAssignmentConfigField(&s.Attributes)
	return s
}

func (d *sodPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sodPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sodPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading SOD Policy data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.SODPoliciesAPI.
		GetSodPolicyV1(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading SOD Policy data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading SOD Policy", errDetail(err, httpResp))
		return
	}

	state, diags := sodPolicyDataSourceDTOToModel(ctx, dto)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func sodPolicyDataSourceDTOToModel(ctx context.Context, dto *sod_policies.SodPolicy) (sodPolicyDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := sodPolicyDataSourceModel{
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
		OwnerRef:                       datasource_sod_policy.NewOwnerRefValueNull(),
		PolicyQuery:                    types.StringNull(),
		Scheduled:                      types.BoolNull(),
		State:                          types.StringNull(),
		Tags:                           types.ListNull(types.StringType),
		Type:                           types.StringNull(),
		ViolationOwnerAssignmentConfig: types.ObjectNull(violationOwnerAssignmentConfigAttrTypes()),
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
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)
	if dto.Description.IsSet() {
		model.Description = types.StringPointerValue(dto.Description.Get())
	}
	if dto.ExternalPolicyReference.IsSet() {
		model.ExternalPolicyReference = types.StringPointerValue(dto.ExternalPolicyReference.Get())
	}
	if dto.PolicyQuery != nil {
		model.PolicyQuery = types.StringValue(*dto.PolicyQuery)
	}
	if dto.CompensatingControls.IsSet() {
		model.CompensatingControls = types.StringPointerValue(dto.CompensatingControls.Get())
	}
	if dto.CorrectionAdvice.IsSet() {
		model.CorrectionAdvice = types.StringPointerValue(dto.CorrectionAdvice.Get())
	}
	if dto.State != nil {
		model.State = types.StringValue(*dto.State)
	}
	if dto.Tags != nil {
		tagsList, d := types.ListValueFrom(ctx, types.StringType, dto.Tags)
		diags.Append(d...)
		model.Tags = tagsList
	}
	if dto.CreatorId != nil {
		model.CreatorId = types.StringValue(*dto.CreatorId)
	}
	if dto.ModifierId.IsSet() {
		model.ModifierId = types.StringPointerValue(dto.ModifierId.Get())
	}
	if dto.Scheduled != nil {
		model.Scheduled = types.BoolValue(*dto.Scheduled)
	}
	if dto.Type != nil {
		model.Type = types.StringValue(*dto.Type)
	}

	owner, d := datasource_sod_policy.OwnerRefValue{}.FromApi_betaSodPolicyOwnerRef(ctx, dto.OwnerRef)
	diags.Append(d...)
	model.OwnerRef = owner

	voacObj, d := violationOwnerAssignmentConfigObjectFromAPI(ctx, dto.ViolationOwnerAssignmentConfig)
	diags.Append(d...)
	model.ViolationOwnerAssignmentConfig = voacObj

	cacObj, d := conflictingAccessCriteriaObjectFromAPI(ctx, dto.ConflictingAccessCriteria)
	diags.Append(d...)
	model.ConflictingAccessCriteria = cacObj

	return model, diags
}
