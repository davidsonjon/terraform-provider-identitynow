// Package sod_policy_v1 is a pilot implementation of the sod_policy
// resource/data sources generated from SailPoint's per-service v1 OpenAPI
// spec (api-specs/idn/apis/sod-policies), following the same hand-written
// CRUD pattern established by transform_v1/connector_rule_v1 (a full-replace
// PUT for Update, rather than a JSON-Patch-based Update like segment_v1/
// governance_group_v1 - the sod-policies v1 API exposes a real PUT endpoint,
// so there is no need to hand-build JSON-Patch operations here).
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model/value types in resource_sod_policy,
// datasource_sod_policy, and datasource_sod_policies, backed by the golang-sdk
// v2 sod_policies.SODPoliciesAPI client (the SDK does not yet publish a
// per-service v1 package; v1 is the stabilization of what was beta - see the
// "SDK path note" below).
//
// Known limitations / design notes:
//   - "conflicting_access_criteria" and "violation_owner_assignment_config"
//     are intentionally hand-written here instead of generated:
//     tfplugingen-framework keys generated Go helper types only by attribute
//     name, not full tree path, so
//     conflictingAccessCriteria.{leftCriteria,rightCriteria}.criteriaList
//     (the identical leaf name "criteriaList" reused by both sibling
//     branches) and violationOwnerAssignmentConfig.ownerRef (colliding with
//     the top-level ownerRef field) each caused duplicate
//     Type/Value declarations and an unfixable build failure in the
//     generated package - the same failure class as segment_v1's
//     visibility_criteria "value"/"children" collision. Both fields are
//     schema.ignores'd in generator_config_sod_policy_v1.yml and
//     hand-written/hand-converted below instead, preserving the exact
//     attribute names from the spec.
//   - "owner_ref" (top-level only, not the one nested inside
//     violation_owner_assignment_config) IS generated + associated_external_type
//     mapped to sod_policies.SodPolicyOwnerRef - see type_mappings_sod_policy_v1.yml.
//   - This is a General-or-Conflicting-Access-Based "either/or" resource per
//     SailPoint's own API design: a GENERAL policy only meaningfully uses
//     name/description/policy_query/state/tags/owner_ref/etc., while a
//     CONFLICTING_ACCESS_BASED policy additionally requires
//     conflicting_access_criteria (and the API derives policy_query itself
//     from it - policy_query is therefore always Computed-only in spirit,
//     though the generated schema leaves it Optional+Computed per the spec's
//     own lack of a readOnly marker; practitioners managing a
//     CONFLICTING_ACCESS_BASED policy should leave policy_query unconfigured
//     and let the API compute/return it).
//   - "schedule" (GET/PUT /sod-policies/v1/{id}/schedule) and the
//     evaluate/violation-report/violation-report-status endpoints are
//     intentionally out of scope for this CRUD resource - see the package's
//     "Known Limitations & Live Testing Notes" doc section.
//   - SDK path note: every sod_policies.SODPoliciesAPIService method is annotated
//     "Deprecated" in the SDK's generated doc comments - this is a blanket
//     annotation applied uniformly across the whole api_beta package as part
//     of SailPoint's versioning migration (see
//     https://developer.sailpoint.com/docs/api/api-versioning-migration/),
//     not a defect or a sod-policy-specific signal; every other _v1 pilot in
//     this repo (role_v1, transform_v1, service_desk_integration_v1, ...)
//     uses the same api_beta client for the identical reason (the SDK has
//     not yet published a dedicated per-service v1 Go package).
package sod_policy_v1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/sod_policies"

	"terraform-provider-identitynow/internal/provider/sod_policy_v1/resource_sod_policy"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*sodPolicyResource)(nil)
	_ resource.ResourceWithConfigure   = (*sodPolicyResource)(nil)
	_ resource.ResourceWithImportState = (*sodPolicyResource)(nil)
)

// sodPolicyResourceModel mirrors resource_sod_policy.SodPolicyModel plus the
// two hand-added fields the generator was told to ignore (see package doc).
// Kept as a distinct, hand-written struct (rather than embedding the
// generated model) since Go doesn't allow adding a field to an imported
// struct type, and req.Plan.Get/resp.State.Set match purely on `tfsdk` tags,
// not on which struct type declares them.
type sodPolicyResourceModel struct {
	CompensatingControls           types.String                      `tfsdk:"compensating_controls"`
	ConflictingAccessCriteria      types.Object                      `tfsdk:"conflicting_access_criteria"`
	CorrectionAdvice               types.String                      `tfsdk:"correction_advice"`
	Created                        types.String                      `tfsdk:"created"`
	CreatorId                      types.String                      `tfsdk:"creator_id"`
	Description                    types.String                      `tfsdk:"description"`
	ExternalPolicyReference        types.String                      `tfsdk:"external_policy_reference"`
	Id                             types.String                      `tfsdk:"id"`
	Modified                       types.String                      `tfsdk:"modified"`
	ModifierId                     types.String                      `tfsdk:"modifier_id"`
	Name                           types.String                      `tfsdk:"name"`
	OwnerRef                       resource_sod_policy.OwnerRefValue `tfsdk:"owner_ref"`
	PolicyQuery                    types.String                      `tfsdk:"policy_query"`
	Scheduled                      types.Bool                        `tfsdk:"scheduled"`
	State                          types.String                      `tfsdk:"state"`
	Tags                           types.List                        `tfsdk:"tags"`
	Type                           types.String                      `tfsdk:"type"`
	ViolationOwnerAssignmentConfig types.Object                      `tfsdk:"violation_owner_assignment_config"`
}

func NewSodPolicyResource() resource.Resource {
	return &sodPolicyResource{}
}

type sodPolicyResource struct {
	client *sailpoint.APIClient
}

func (r *sodPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sod_policy_v1"
}

func (r *sodPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = sodPolicyResourceSchema(ctx)
	resp.Schema.Description = "Manages a Separation of Duties (SOD) Policy in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Separation of Duties (SOD) Policy](https://documentation.sailpoint.com/saas/help/sod/manage-policies.html) " +
		"in IdentityNow/ISC. SOD policies let administrators prevent identities from gaining conflicting or excessive " +
		"access, either via a free-form search `policy_query` (`type = \"GENERAL\"`) or via two structured " +
		"`conflicting_access_criteria` entitlement lists (`type = \"CONFLICTING_ACCESS_BASED\"`).\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before " +
		"relying on it in production configurations. Note also that the read-only `identitynow_governance_group_connections_v1` " +
		"data source surfaces a `SOD_POLICY` connection type for governance groups referenced as a policy/violation owner - " +
		"see that data source's docs for the reverse-lookup relationship."
	applySodPolicyUseStateForUnknown(&resp.Schema)
}

func (r *sodPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sodPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *sodPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sodPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating SOD Policy", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := sodPolicyModelToDTO(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.SODPoliciesAPI.
		CreateSodPolicyV1(ctx).
		SodPolicy(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating SOD Policy", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating SOD Policy", errDetail(err, httpResp))
		return
	}

	state, diags := sodPolicyDTOToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created SOD Policy", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sodPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sodPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading SOD Policy", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.SODPoliciesAPI.
		GetSodPolicyV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "SOD Policy not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading SOD Policy", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading SOD Policy", errDetail(err, httpResp))
		return
	}

	newState, diags := sodPolicyDTOToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sodPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sodPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sodPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating SOD Policy", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := sodPolicyModelToDTO(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.SODPoliciesAPI.
		PutSodPolicyV1(ctx, state.Id.ValueString()).
		SodPolicy(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating SOD Policy", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating SOD Policy", errDetail(err, httpResp))
		return
	}

	newState, diags := sodPolicyDTOToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated SOD Policy", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sodPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sodPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting SOD Policy", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.SODPoliciesAPI.
		DeleteSodPolicyV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			tflog.Warn(ctx, "SOD Policy already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting SOD Policy", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting SOD Policy", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted SOD Policy", map[string]interface{}{"id": state.Id.ValueString()})
}

func sodPolicyResourceSchema(ctx context.Context) resourceschema.Schema {
	s := resource_sod_policy.SodPolicyResourceSchema(ctx)
	applyResourceConflictingAccessCriteriaField(&s.Attributes)
	applyResourceViolationOwnerAssignmentConfigField(&s.Attributes)
	return s
}

// sodPolicyModelToDTO builds the full sod_policies.SodPolicy request body sent to
// both Create (POST) and Update (PUT) - the sod-policies v1 API's PUT is a
// full-replace, not a partial patch, so both operations share this helper.
func sodPolicyModelToDTO(ctx context.Context, m sodPolicyResourceModel) (*sod_policies.SodPolicy, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto := sod_policies.NewSodPolicyWithDefaults()
	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		dto.Name = m.Name.ValueStringPointer()
	}
	if !m.Description.IsUnknown() {
		dto.Description = *sod_policies.NewNullableString(m.Description.ValueStringPointer())
	}
	if !m.ExternalPolicyReference.IsUnknown() {
		dto.ExternalPolicyReference = *sod_policies.NewNullableString(m.ExternalPolicyReference.ValueStringPointer())
	}
	if !m.CompensatingControls.IsUnknown() {
		dto.CompensatingControls = *sod_policies.NewNullableString(m.CompensatingControls.ValueStringPointer())
	}
	if !m.CorrectionAdvice.IsUnknown() {
		dto.CorrectionAdvice = *sod_policies.NewNullableString(m.CorrectionAdvice.ValueStringPointer())
	}
	if !m.State.IsNull() && !m.State.IsUnknown() {
		dto.State = m.State.ValueStringPointer()
	}
	if !m.Scheduled.IsNull() && !m.Scheduled.IsUnknown() {
		dto.Scheduled = m.Scheduled.ValueBoolPointer()
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		dto.Type = m.Type.ValueStringPointer()
	}
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		var tags []string
		diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
		dto.Tags = tags
	}
	if !m.OwnerRef.IsUnknown() {
		owner, d := m.OwnerRef.ToApi_betaSodPolicyOwnerRef(ctx)
		diags.Append(d...)
		if owner != nil {
			dto.OwnerRef = owner
		}
	}

	voac, d := violationOwnerAssignmentConfigObjectToAPI(ctx, m.ViolationOwnerAssignmentConfig)
	diags.Append(d...)
	dto.ViolationOwnerAssignmentConfig = voac

	cac, d := conflictingAccessCriteriaObjectToAPI(ctx, m.ConflictingAccessCriteria)
	diags.Append(d...)
	dto.ConflictingAccessCriteria = cac

	return dto, diags
}

func sodPolicyDTOToModel(ctx context.Context, dto *sod_policies.SodPolicy) (sodPolicyResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := sodPolicyResourceModel{
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
		OwnerRef:                       resource_sod_policy.NewOwnerRefValueNull(),
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

	owner, d := resource_sod_policy.OwnerRefValue{}.FromApi_betaSodPolicyOwnerRef(ctx, dto.OwnerRef)
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

// errDetail delegates to the shared util.SailpointErrorDetail helper (adopted
// from role_v1/service_desk_integration_v1/transform_v1) so all _v1 targets
// surface the same richer detail (HTTP status, detailCode, trackingId, and
// message text) in resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func timeToStringValue(t *sod_policies.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(timeRFC3339))
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
