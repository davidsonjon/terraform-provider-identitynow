// Package governance_group_v1 is a pilot implementation of the governance
// group ("workgroup") resource/data source generated from SailPoint's new
// per-service v1 OpenAPI spec (api-specs/idn/apis/governance-groups).
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_governance_group and
// datasource_governance_group, backed by the golang-sdk v3
// governance_groups.GovernanceGroupsAPIService client (the SDK does not yet publish a
// per-service v1 package; v1 is the stabilization of what was beta). Unlike
// service_desk_integration_v1/role_v1's *Dto types, governance_groups.WorkgroupDto
// declares a real typed Id field (no AdditionalProperties workaround needed).
//
// Known limitations (tracked for follow-up before promoting out of the _v1
// pilot package into internal/provider/governance_group):
//   - The API's members/connections sub-resource endpoints
//     (/workgroups/v1/{workgroupId}/members[/bulk-add|/bulk-delete],
//     /workgroups/v1/{workgroupId}/connections) are separate
//     collection-management endpoints (not part of the WorkgroupDto CRUD
//     lifecycle), so they are modeled as their own follow-up
//     resource/data-source rather than folded into this one - mirroring how
//     access_profile_v1/role_v1 do not attempt to fold membership/access
//     assignment into their own parent resource either. See
//     resource_governance_group_members.go (identitynow_governance_group_members_v1,
//     a full read/write "join" resource reconciling the API's list+bulk-add+
//     bulk-delete-only shape) and datasource_governance_group_connections.go
//     (identitynow_governance_group_connections_v1, read-only - connections
//     have no write endpoint at all).
//   - "owner" is the only nested object in this schema and is mapped to
//     governance_groups.WorkgroupDtoOwner via associated_external_type (all fields on
//     that SDK struct are plain *string - confirmed via direct source
//     inspection, no NullableString outliers).
package governance_group_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/governance_groups"

	"terraform-provider-identitynow/internal/provider/governance_group_v1/resource_governance_group"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*governanceGroupResource)(nil)
	_ resource.ResourceWithConfigure   = (*governanceGroupResource)(nil)
	_ resource.ResourceWithImportState = (*governanceGroupResource)(nil)
)

func NewGovernanceGroupResource() resource.Resource {
	return &governanceGroupResource{}
}

type governanceGroupResource struct {
	client *sailpoint.APIClient
}

func (r *governanceGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group_v1"
}

func (r *governanceGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_governance_group.GovernanceGroupResourceSchema(ctx)
	resp.Schema.Description = "Manages a Governance Group in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Governance Group](https://documentation.sailpoint.com/saas/help/common/governance_groups.html) " +
		"in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying on " +
		"it in production configurations. Membership/connections sub-resources are not yet modeled here - see the package doc."
	applyGovernanceGroupUseStateForUnknown(&resp.Schema)
}

func (r *governanceGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *governanceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *governanceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Governance Group", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.GovernanceGroupsAPI.
		CreateWorkgroupV1(ctx).
		WorkgroupDto(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Governance Group", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Governance Group", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Governance Group", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *governanceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Governance Group", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.GovernanceGroupsAPI.
		GetWorkgroupV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Governance Group not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Governance Group", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Governance Group", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Governance Group", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *governanceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Governance Group", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The v1 API updates via RFC 6902 JSON Patch. Per the spec's own
	// description, only name/description/owner are patchable - a "replace
	// whole document" patch of exactly those three fields is the simplest
	// correct approach for a pilot resource.
	patch := []governance_groups.JsonPatchOperation{
		jsonPatchReplace("/name", governance_groups.StringAsJsonPatchOperationValue(dto.Name)),
		jsonPatchReplace("/description", governance_groups.StringAsJsonPatchOperationValue(dto.Description)),
	}
	if m, err := structToMap(dto.Owner); err == nil && m != nil {
		patch = append(patch, jsonPatchReplace("/owner", governance_groups.MapmapOfStringAnyAsJsonPatchOperationValue(&m)))
	}

	tflog.Debug(ctx, "Patching Governance Group", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.GovernanceGroupsAPI.
		PatchWorkgroupV1(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Governance Group", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Governance Group", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Governance Group", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *governanceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Governance Group", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.GovernanceGroupsAPI.
		DeleteWorkgroupV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Governance Group already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Governance Group", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Governance Group", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Governance Group", map[string]interface{}{"id": state.Id.ValueString()})
}

// modelToDto converts the Terraform plan/config model into the SDK create/update
// DTO shape.
func modelToDto(ctx context.Context, m resource_governance_group.GovernanceGroupModel) (*governance_groups.WorkgroupDto, diag.Diagnostics) {
	var diags diag.Diagnostics

	var owner *governance_groups.WorkgroupDtoOwner
	if !m.Owner.IsUnknown() {
		var d diag.Diagnostics
		owner, d = m.Owner.ToApi_betaWorkgroupDtoOwner(ctx)
		diags.Append(d...)
	}
	if diags.HasError() {
		return nil, diags
	}

	dto := governance_groups.NewWorkgroupDtoWithDefaults()
	name := m.Name.ValueString()
	dto.Name = &name
	description := m.Description.ValueString()
	dto.Description = &description
	dto.Owner = owner

	return dto, diags
}

// dtoToModel converts an API response DTO into the Terraform state model.
func dtoToModel(ctx context.Context, dto *governance_groups.WorkgroupDto, fallback resource_governance_group.GovernanceGroupModel) (resource_governance_group.GovernanceGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		model.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	}
	if dto.MemberCount != nil {
		model.MemberCount = types.Int64Value(*dto.MemberCount)
	}
	if dto.ConnectionCount != nil {
		model.ConnectionCount = types.Int64Value(*dto.ConnectionCount)
	}
	if dto.Created != nil {
		model.Created = types.StringValue(dto.Created.Format(time.RFC3339))
	}
	if dto.Modified != nil {
		model.Modified = types.StringValue(dto.Modified.Format(time.RFC3339))
	}

	owner, d := resource_governance_group.OwnerValue{}.FromApi_betaWorkgroupDtoOwner(ctx, dto.Owner)
	diags.Append(d...)
	model.Owner = owner

	return model, diags
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (adopted
// from the role_v1/service_desk_integration_v1 pilots) so this target surfaces
// the same richer detail (HTTP status, detailCode, trackingId, and message
// text) in resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func jsonPatchReplace(path string, value governance_groups.JsonPatchOperationValue) governance_groups.JsonPatchOperation {
	return governance_groups.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// structToMap round-trips an SDK model struct through JSON to get a
// map[string]interface{} suitable for
// governance_groups.MapmapOfStringAnyAsJsonPatchOperationValue, since
// the JSON Patch value wrapper type doesn't accept typed structs directly
// (see the sdk-type-reference catalog's JsonPatchOperation entry).
func structToMap(v interface{}) (map[string]interface{}, error) {
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
