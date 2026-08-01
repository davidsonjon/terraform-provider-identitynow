// Package role_v1 is a pilot implementation of the role resource/data source
// generated from SailPoint's new per-service v1 OpenAPI spec
// (api-specs/idn/apis/roles), following the same hand-written CRUD pattern
// established by service_desk_integration_v1.
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_role and datasource_role,
// backed by the golang-sdk v3 roles.RolesAPI client (the SDK does not yet
// publish a per-service v1 package; v1 is the stabilization of what was beta).
//
// Known limitations (tracked for follow-up before promoting out of the _v1 pilot
// package into internal/provider/role):
//   - "access_model_metadata", "access_request_config", "revocation_request_config",
//     and "membership" now get a deeper read-back from the API response (see
//     resource_role_readback.go / datasource_role_readback.go) whenever the
//     practitioner hasn't configured them, surfacing IdentityNow's actual
//     computed values/drift instead of a permanently-Null placeholder. They
//     remain write-side pass-through only, though (this provider does not send
//     them to the API on Create/Update - see roleModelToDto/rolePassThroughWarning) -
//     a configured value is preserved as-is (with an AddWarning) rather than
//     overwritten by the API's response, to avoid a permanent non-convergent diff.
//   - "legacy_membership_info" remains fully pass-through (state mirrors
//     plan/prior-state): its generated schema has zero attributes (the API's
//     arbitrary map[string]interface{} shape never got any concrete attribute
//     mapping), so there is nothing for a read-back to populate.
//   - "entitlements" and "additional_owners" are populated on Create/Update from
//     plan and read back from the API response, but converted by hand (not via a
//     generated ToApi_beta.../FromApi_beta... helper) because
//     roles.EntitlementRef.Name and roles.AdditionalOwnerRef.Name are
//     roles.NullableString, a shape tfplugingen-framework's associated_external_type
//     converter templates cannot bridge to the schema's plain string attribute.
package role_v1

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
	"github.com/sailpoint-oss/golang-sdk/v3/roles"

	"terraform-provider-identitynow/internal/provider/role_v1/resource_role"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*roleResource)(nil)
	_ resource.ResourceWithConfigure   = (*roleResource)(nil)
	_ resource.ResourceWithImportState = (*roleResource)(nil)
)

func NewRoleResource() resource.Resource {
	return &roleResource{}
}

type roleResource struct {
	client *sailpoint.APIClient
}

func (r *roleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_v1"
}

func (r *roleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_role.RoleResourceSchema(ctx)
	resp.Schema.Description = "Manages a Role in IdentityNow/ISC. Roles grant a set of access (access profiles, entitlements) " +
		"to identities that either request them or meet a set of membership criteria."
	resp.Schema.MarkdownDescription = "Manages a [Role](https://documentation.sailpoint.com/saas/help/access/roles.html) in " +
		"IdentityNow/ISC. Roles grant a set of access (access profiles, entitlements) to identities that either request them or " +
		"meet a set of membership criteria.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying on " +
		"it in production configurations."
	applyRoleUseStateForUnknown(&resp.Schema)
}

func (r *roleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_role.RoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Role", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := roleModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rolePassThroughWarning(ctx, &resp.Diagnostics, "access_model_metadata", plan.AccessModelMetadata.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "access_request_config", plan.AccessRequestConfig.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "revocation_request_config", plan.RevocationRequestConfig.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "membership", plan.Membership.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "legacy_membership_info", plan.LegacyMembershipInfo.IsNull())

	apiResp, httpResp, err := r.client.RolesAPI.
		CreateRoleV1(ctx).
		Role(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Role", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Role", roleErrDetail(err, httpResp))
		return
	}

	state, diags := roleDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Role", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_role.RoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Role", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.RolesAPI.
		GetRoleV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Role not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Role", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Role", roleErrDetail(err, httpResp))
		return
	}

	newState, diags := roleDtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Role", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_role.RoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_role.RoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Role", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := roleModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rolePassThroughWarning(ctx, &resp.Diagnostics, "access_model_metadata", plan.AccessModelMetadata.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "access_request_config", plan.AccessRequestConfig.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "revocation_request_config", plan.RevocationRequestConfig.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "membership", plan.Membership.IsNull())
	rolePassThroughWarning(ctx, &resp.Diagnostics, "legacy_membership_info", plan.LegacyMembershipInfo.IsNull())

	// The v1 API updates via RFC 6902 JSON Patch. A "replace whole document"
	// patch of every top-level writable field is the simplest correct
	// approach for a pilot resource; a follow-up can move to a minimal diff.
	patch := []roles.JsonPatchOperation{
		roleJSONPatchReplace("/name", roles.StringAsJsonPatchOperationValue(&dto.Name)),
	}
	if m, err := roleStructToMap(dto.Owner.Get()); err == nil && m != nil {
		patch = append(patch, roleJSONPatchReplace("/owner", roles.MapmapOfStringAnyAsJsonPatchOperationValue(&m)))
	}
	if dto.Description.IsSet() {
		desc := dto.Description.Get()
		patch = append(patch, roleJSONPatchReplace("/description", roles.StringAsJsonPatchOperationValue(desc)))
	}
	if dto.Enabled != nil {
		patch = append(patch, roleJSONPatchReplace("/enabled", roles.BoolAsJsonPatchOperationValue(dto.Enabled)))
	}
	if dto.Requestable != nil {
		patch = append(patch, roleJSONPatchReplace("/requestable", roles.BoolAsJsonPatchOperationValue(dto.Requestable)))
	}
	if dto.AccessProfiles != nil {
		if arr, err := roleSliceToArrayInner(dto.AccessProfiles); err == nil {
			patch = append(patch, roleJSONPatchReplace("/accessProfiles", roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.Entitlements != nil {
		if arr, err := roleSliceToArrayInner(dto.Entitlements); err == nil {
			patch = append(patch, roleJSONPatchReplace("/entitlements", roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.AdditionalOwners != nil {
		if arr, err := roleSliceToArrayInner(dto.AdditionalOwners); err == nil {
			patch = append(patch, roleJSONPatchReplace("/additionalOwners", roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.DimensionRefs != nil {
		if arr, err := roleSliceToArrayInner(dto.DimensionRefs); err == nil {
			patch = append(patch, roleJSONPatchReplace("/dimensionRefs", roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.Segments != nil {
		arr := make([]roles.ArrayInner, 0, len(dto.Segments))
		for i := range dto.Segments {
			arr = append(arr, roles.ArrayInner{String: &dto.Segments[i]})
		}
		patch = append(patch, roleJSONPatchReplace("/segments", roles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
	}
	if dto.PrivilegeLevel.IsSet() {
		patch = append(patch, roleJSONPatchReplace("/privilegeLevel", roles.StringAsJsonPatchOperationValue(dto.PrivilegeLevel.Get())))
	}

	tflog.Debug(ctx, "Patching Role", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.RolesAPI.
		PatchRoleV1(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Role", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Role", roleErrDetail(err, httpResp))
		return
	}

	newState, diags := roleDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Role", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_role.RoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Role", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.RolesAPI.
		DeleteRoleV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Role already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Role", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Role", roleErrDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Role", map[string]interface{}{"id": state.Id.ValueString()})
}

// roleModelToDto converts the Terraform plan/config model into the SDK create/
// update DTO shape. See package doc for the pass-through-only and hand-written
// list-conversion caveats.
//
// owner is Required in the schema (matches the API's required: [name, owner]),
// so it is never Unknown by the time Create/Update run - Optional+Computed
// nested refs elsewhere (access_profiles, dimension_refs, etc.) are still
// guarded the same way service_desk_integration_v1's modelToDto guards
// before_provisioning_rule/cluster_ref/owner_ref, per that package's documented
// Unknown-vs-Null pattern.
func roleModelToDto(ctx context.Context, m resource_role.RoleModel) (*roles.Role, diag.Diagnostics) {
	var diags diag.Diagnostics

	owner, d := m.Owner.ToApi_betaOwnerReference(ctx)
	diags.Append(d...)
	if diags.HasError() || owner == nil {
		return nil, diags
	}

	dto := roles.NewRoleWithDefaults()
	dto.Name = m.Name.ValueString()
	dto.Owner = *roles.NewNullableOwnerReference(owner)

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		dto.Description = *roles.NewNullableString(m.Description.ValueStringPointer())
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		dto.Enabled = m.Enabled.ValueBoolPointer()
	}
	if !m.Requestable.IsNull() && !m.Requestable.IsUnknown() {
		dto.Requestable = m.Requestable.ValueBoolPointer()
	}
	if !m.PrivilegeLevel.IsNull() && !m.PrivilegeLevel.IsUnknown() {
		dto.PrivilegeLevel = *roles.NewNullableString(m.PrivilegeLevel.ValueStringPointer())
	}

	if !m.Segments.IsNull() && !m.Segments.IsUnknown() {
		var segments []string
		diags.Append(m.Segments.ElementsAs(ctx, &segments, false)...)
		dto.Segments = segments
	}

	if !m.AccessProfiles.IsNull() && !m.AccessProfiles.IsUnknown() {
		var items []resource_role.AccessProfilesValue
		diags.Append(m.AccessProfiles.ElementsAs(ctx, &items, false)...)
		refs := make([]roles.AccessProfileRef, 0, len(items))
		for _, item := range items {
			ref, d := item.ToApi_betaAccessProfileRef(ctx)
			diags.Append(d...)
			if ref != nil {
				refs = append(refs, *ref)
			}
		}
		dto.AccessProfiles = refs
	}

	if !m.DimensionRefs.IsNull() && !m.DimensionRefs.IsUnknown() {
		var items []resource_role.DimensionRefsValue
		diags.Append(m.DimensionRefs.ElementsAs(ctx, &items, false)...)
		refs := make([]roles.DimensionRef, 0, len(items))
		for _, item := range items {
			ref, d := item.ToApi_betaDimensionRef(ctx)
			diags.Append(d...)
			if ref != nil {
				refs = append(refs, *ref)
			}
		}
		dto.DimensionRefs = refs
	}

	if !m.Entitlements.IsNull() && !m.Entitlements.IsUnknown() {
		var items []resource_role.EntitlementsValue
		diags.Append(m.Entitlements.ElementsAs(ctx, &items, false)...)
		refs := make([]roles.EntitlementRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, roles.EntitlementRef{
				Id:   item.Id.ValueStringPointer(),
				Name: *roles.NewNullableString(item.Name.ValueStringPointer()),
				Type: item.EntitlementsType.ValueStringPointer(),
			})
		}
		dto.Entitlements = refs
	}

	if !m.AdditionalOwners.IsNull() && !m.AdditionalOwners.IsUnknown() {
		var items []resource_role.AdditionalOwnersValue
		diags.Append(m.AdditionalOwners.ElementsAs(ctx, &items, false)...)
		refs := make([]roles.AdditionalOwnerRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, roles.AdditionalOwnerRef{
				Id:   item.Id.ValueStringPointer(),
				Name: *roles.NewNullableString(item.Name.ValueStringPointer()),
				Type: item.AdditionalOwnersType.ValueStringPointer(),
			})
		}
		dto.AdditionalOwners = refs
	}

	return dto, diags
}

// roleDtoToModel converts an API response DTO into the Terraform state model,
// preferring fields carried over from fallback (plan/prior state) for the
// pass-through-only blocks documented in the package doc.
func roleDtoToModel(ctx context.Context, dto *roles.Role, fallback resource_role.RoleModel) (resource_role.RoleModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringValue(dto.Name)
	if dto.Description.IsSet() {
		model.Description = types.StringPointerValue(dto.Description.Get())
	}
	if dto.Created != nil {
		model.Created = types.StringValue(dto.Created.Format(time.RFC3339))
	}
	if dto.Modified != nil {
		model.Modified = types.StringValue(dto.Modified.Format(time.RFC3339))
	}
	if dto.Enabled != nil {
		model.Enabled = types.BoolPointerValue(dto.Enabled)
	}
	if dto.Requestable != nil {
		model.Requestable = types.BoolPointerValue(dto.Requestable)
	}
	if dto.PrivilegeLevel.IsSet() {
		model.PrivilegeLevel = types.StringPointerValue(dto.PrivilegeLevel.Get())
	}
	if dto.Dimensional.IsSet() {
		model.Dimensional = types.BoolPointerValue(dto.Dimensional.Get())
	}

	owner, d := resource_role.OwnerValue{}.FromApi_betaOwnerReference(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	if dto.Segments != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.Segments)
		diags.Append(d...)
		model.Segments = listVal
	}

	if dto.AccessProfiles != nil {
		values := make([]resource_role.AccessProfilesValue, 0, len(dto.AccessProfiles))
		for i := range dto.AccessProfiles {
			v, d := resource_role.AccessProfilesValue{}.FromApi_betaAccessProfileRef(ctx, &dto.AccessProfiles[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, resource_role.AccessProfilesValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AccessProfiles = listVal
	}

	if dto.DimensionRefs != nil {
		values := make([]resource_role.DimensionRefsValue, 0, len(dto.DimensionRefs))
		for i := range dto.DimensionRefs {
			v, d := resource_role.DimensionRefsValue{}.FromApi_betaDimensionRef(ctx, &dto.DimensionRefs[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, resource_role.DimensionRefsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.DimensionRefs = listVal
	}

	if dto.Entitlements != nil {
		values := make([]resource_role.EntitlementsValue, 0, len(dto.Entitlements))
		for _, e := range dto.Entitlements {
			values = append(values, resource_role.EntitlementsValue{
				Id:               types.StringPointerValue(e.Id),
				Name:             types.StringPointerValue(e.Name.Get()),
				EntitlementsType: types.StringPointerValue(e.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, resource_role.EntitlementsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Entitlements = listVal
	}

	if dto.AdditionalOwners != nil {
		values := make([]resource_role.AdditionalOwnersValue, 0, len(dto.AdditionalOwners))
		for _, o := range dto.AdditionalOwners {
			values = append(values, resource_role.AdditionalOwnersValue{
				Id:                   types.StringPointerValue(o.Id),
				Name:                 types.StringPointerValue(o.Name.Get()),
				AdditionalOwnersType: types.StringPointerValue(o.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, resource_role.AdditionalOwnersValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AdditionalOwners = listVal
	}

	// access_model_metadata, access_request_config, revocation_request_config,
	// and membership are still pass-through-only on the write path (this
	// provider does not send them to the API on Create/Update - see the
	// roleModelToDto doc comment), but on the read path they now get a real
	// deeper read-back (see resource_role_readback.go) whenever the
	// practitioner hasn't configured them: this surfaces IdentityNow's actual
	// computed defaults/drift for these blocks instead of only ever showing
	// Null. If the practitioner HAS configured one of these blocks, we keep
	// pass-through of their configured value (with rolePassThroughWarning
	// already emitted above) rather than overwriting it with the API's
	// response, since overwriting would produce a permanent, non-convergent
	// diff for a value this provider never actually writes.
	if model.AccessModelMetadata.IsNull() || model.AccessModelMetadata.IsUnknown() {
		v, d := roleAccessModelMetadataFromApi(ctx, dto.AccessModelMetadata)
		diags.Append(d...)
		model.AccessModelMetadata = v
	}
	if model.AccessRequestConfig.IsNull() || model.AccessRequestConfig.IsUnknown() {
		v, d := roleAccessRequestConfigFromApi(ctx, dto.AccessRequestConfig)
		diags.Append(d...)
		model.AccessRequestConfig = v
	}
	if model.RevocationRequestConfig.IsNull() || model.RevocationRequestConfig.IsUnknown() {
		v, d := roleRevocationRequestConfigFromApi(ctx, dto.RevocationRequestConfig)
		diags.Append(d...)
		model.RevocationRequestConfig = v
	}
	if model.Membership.IsNull() || model.Membership.IsUnknown() {
		v, d := roleMembershipFromApi(ctx, dto.Membership.Get())
		diags.Append(d...)
		model.Membership = v
	}
	// legacy_membership_info's generated schema has zero attributes (nothing
	// to populate from the API's arbitrary map[string]interface{} shape - see
	// resource_role_readback.go's doc comment), so it remains fully
	// pass-through: just resolve Unknown to Null so Create/Update always
	// return a known value.
	if model.LegacyMembershipInfo.IsUnknown() {
		model.LegacyMembershipInfo = resource_role.NewLegacyMembershipInfoValueNull()
	}

	return model, diags
}

func roleErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

// rolePassThroughWarning adds a warning diagnostic when the practitioner has
// configured one of the pass-through-only nested blocks (see the package doc)
// so it's clear in `terraform plan`/`apply` output - not just in code comments
// or DEBUG logs - that Terraform won't detect drift on that block: state will
// always mirror whatever was last planned/configured for it, never the live
// API value.
func rolePassThroughWarning(ctx context.Context, diags *diag.Diagnostics, attrName string, isNull bool) {
	if isNull {
		return
	}
	tflog.Warn(ctx, "Role pass-through-only attribute configured; drift will not be detected", map[string]interface{}{
		"attribute": attrName,
	})
	diags.AddWarning(
		fmt.Sprintf("%q is not read back from the API", attrName),
		fmt.Sprintf(
			"The identitynow_role_v1 resource does not parse %q from the SailPoint API response (see the role_v1 package documentation for why). "+
				"Terraform will keep whatever value you configure in state and will not detect changes made to it outside of Terraform, "+
				"nor will it detect drift if the value is rejected or altered by the API. If you need drift detection or read-back for this "+
				"attribute, please raise it as a follow-up before relying on it in production configurations.",
			attrName,
		),
	)
}

func roleJSONPatchReplace(path string, value roles.JsonPatchOperationValue) roles.JsonPatchOperation {
	return roles.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// roleStructToMap round-trips an SDK model struct through JSON to get a
// map[string]interface{} suitable for
// roles.MapmapOfStringAnyAsJsonPatchOperationValue, since the
// JSON Patch value wrapper type doesn't accept typed structs directly.
func roleStructToMap(v interface{}) (map[string]interface{}, error) {
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

// roleSliceToArrayInner round-trips a slice of SDK model structs (e.g.
// []roles.AccessProfileRef) through JSON to build a []roles.ArrayInner
// suitable for roles.ArrayOfArrayInnerAsJsonPatchOperationValue,
// since JsonPatchOperation.Value has no generic "array of objects" constructor.
func roleSliceToArrayInner(v interface{}) ([]roles.ArrayInner, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var maps []map[string]interface{}
	if err := json.Unmarshal(b, &maps); err != nil {
		return nil, err
	}
	arr := make([]roles.ArrayInner, 0, len(maps))
	for i := range maps {
		m := maps[i]
		arr = append(arr, roles.ArrayInner{MapmapOfStringAny: &m})
	}
	return arr, nil
}
