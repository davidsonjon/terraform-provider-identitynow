// Package access_profile_v1 is a pilot implementation of the access_profile
// resource/data source generated from SailPoint's per-service v1 OpenAPI spec
// (api-specs/idn/apis/access-profiles), following the same hand-written CRUD
// pattern established by role_v1 and service_desk_integration_v1 (the two
// schemas are nearly structurally identical: both have owner/additionalOwners/
// accessModelMetadata/accessRequestConfig/revocationRequestConfig/segments/
// entitlements; access_profile additionally has a required "source" reference
// and a "provisioning_criteria" recursive matching-criteria tree in place of
// role's dimension/membership fields).
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_access_profile and
// datasource_access_profile, backed by the golang-sdk v2
// access_profiles.AccessProfilesAPIService client (the SDK does not yet publish a
// per-service v1 package; v1 is the stabilization of what was beta).
//
// Known limitations (tracked for follow-up before promoting out of the _v1
// pilot package into internal/provider/access_profile):
//   - "access_model_metadata", "access_request_config", and
//     "revocation_request_config" get a deeper read-back from the API response
//     (see resource_access_profile_readback.go / datasource_access_profile_readback.go)
//     whenever the practitioner hasn't configured them, surfacing IdentityNow's
//     actual computed values/drift instead of a permanently-Null placeholder.
//     They remain write-side pass-through only, though (this provider does not
//     send them to the API on Create/Update - see accessProfileModelToDto/
//     accessProfilePassThroughWarning) - a configured value is preserved as-is
//     (with an AddWarning) rather than overwritten by the API's response, to
//     avoid a permanent non-convergent diff.
//   - "entitlements" and "additional_owners" are populated on Create/Update from
//     plan and read back from the API response, but converted by hand (not via a
//     generated ToApi_beta.../FromApi_beta... helper) because
//     access_profiles.EntitlementRef.Name and access_profiles.AdditionalOwnerRef.Name are
//     access_profiles.NullableString, a shape tfplugingen-framework's associated_external_type
//     converter templates cannot bridge to the schema's plain string attribute
//     (same limitation documented in role_v1's package doc).
//   - "provisioning_criteria" is read back from the API response (see
//     resource_access_profile_readback.go) and, like role_v1's "membership"
//     criteria tree, only resolves 3 levels deep (provisioning_criteria ->
//     children -> grandchildren), matching the depth tfplugingen-framework
//     flattened the recursive OpenAPI schema to.
package access_profile_v1

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
	"github.com/sailpoint-oss/golang-sdk/v3/access_profiles"

	"terraform-provider-identitynow/internal/provider/access_profile_v1/resource_access_profile"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*accessProfileResource)(nil)
	_ resource.ResourceWithConfigure   = (*accessProfileResource)(nil)
	_ resource.ResourceWithImportState = (*accessProfileResource)(nil)
)

func NewAccessProfileResource() resource.Resource {
	return &accessProfileResource{}
}

type accessProfileResource struct {
	client *sailpoint.APIClient
}

func (r *accessProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_profile_v1"
}

func (r *accessProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_access_profile.AccessProfileResourceSchema(ctx)
	resp.Schema.Description = "Manages an Access Profile in IdentityNow/ISC. Access profiles group entitlements, which " +
		"represent access rights on sources, into a broader set of access that can be requested or assigned together."
	resp.Schema.MarkdownDescription = "Manages an [Access Profile](https://documentation.sailpoint.com/saas/help/access/access-profiles.html) " +
		"in IdentityNow/ISC. Access profiles group entitlements, which represent access rights on sources, into a broader " +
		"set of access that can be requested or assigned together.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."
	applyAccessProfileUseStateForUnknown(&resp.Schema)
}

func (r *accessProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accessProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *accessProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Access Profile", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := accessProfileModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "access_model_metadata", plan.AccessModelMetadata.IsNull())
	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "access_request_config", plan.AccessRequestConfig.IsNull())
	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "revocation_request_config", plan.RevocationRequestConfig.IsNull())

	apiResp, httpResp, err := r.client.AccessProfilesAPI.
		CreateAccessProfileV1(ctx).
		AccessProfile(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Access Profile", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Access Profile", accessProfileErrDetail(err, httpResp))
		return
	}

	state, diags := accessProfileDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Access Profile", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accessProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Access Profile", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.AccessProfilesAPI.
		GetAccessProfileV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Access Profile not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Access Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Access Profile", accessProfileErrDetail(err, httpResp))
		return
	}

	newState, diags := accessProfileDtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Access Profile", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *accessProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Access Profile", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := accessProfileModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "access_model_metadata", plan.AccessModelMetadata.IsNull())
	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "access_request_config", plan.AccessRequestConfig.IsNull())
	accessProfilePassThroughWarning(ctx, &resp.Diagnostics, "revocation_request_config", plan.RevocationRequestConfig.IsNull())

	// The v1 API updates via RFC 6902 JSON Patch. A "replace whole document"
	// patch of every top-level writable field is the simplest correct
	// approach for a pilot resource; a follow-up can move to a minimal diff.
	// Note: per the API's documented PATCH semantics, if "source" changes,
	// "entitlements" must be replaced in the same call with entitlements from
	// the new source - this provider always sends both together (see below),
	// so that constraint is satisfied automatically whenever either changes.
	patch := []access_profiles.JsonPatchOperation{
		accessProfileJSONPatchReplace("/name", access_profiles.StringAsJsonPatchOperationValue(&dto.Name)),
	}
	if m, err := accessProfileStructToMap(dto.Owner); err == nil && m != nil {
		patch = append(patch, accessProfileJSONPatchReplace("/owner", access_profiles.MapmapOfStringAnyAsJsonPatchOperationValue(&m)))
	}
	if m, err := accessProfileStructToMap(dto.Source); err == nil && m != nil {
		patch = append(patch, accessProfileJSONPatchReplace("/source", access_profiles.MapmapOfStringAnyAsJsonPatchOperationValue(&m)))
	}
	if dto.Description.IsSet() {
		desc := dto.Description.Get()
		patch = append(patch, accessProfileJSONPatchReplace("/description", access_profiles.StringAsJsonPatchOperationValue(desc)))
	}
	if dto.Enabled != nil {
		patch = append(patch, accessProfileJSONPatchReplace("/enabled", access_profiles.BoolAsJsonPatchOperationValue(dto.Enabled)))
	}
	if dto.Requestable != nil {
		patch = append(patch, accessProfileJSONPatchReplace("/requestable", access_profiles.BoolAsJsonPatchOperationValue(dto.Requestable)))
	}
	if dto.Entitlements != nil {
		if arr, err := accessProfileSliceToArrayInner(dto.Entitlements); err == nil {
			patch = append(patch, accessProfileJSONPatchReplace("/entitlements", access_profiles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.AdditionalOwners != nil {
		if arr, err := accessProfileSliceToArrayInner(dto.AdditionalOwners); err == nil {
			patch = append(patch, accessProfileJSONPatchReplace("/additionalOwners", access_profiles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		}
	}
	if dto.Segments != nil {
		arr := make([]access_profiles.ArrayInner, 0, len(dto.Segments))
		for i := range dto.Segments {
			arr = append(arr, access_profiles.ArrayInner{String: &dto.Segments[i]})
		}
		patch = append(patch, accessProfileJSONPatchReplace("/segments", access_profiles.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
	}
	if dto.ProvisioningCriteria.IsSet() {
		if m, err := accessProfileStructToMap(dto.ProvisioningCriteria.Get()); err == nil {
			patch = append(patch, accessProfileJSONPatchReplace("/provisioningCriteria", access_profiles.MapmapOfStringAnyAsJsonPatchOperationValue(&m)))
		}
	}

	tflog.Debug(ctx, "Patching Access Profile", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.AccessProfilesAPI.
		PatchAccessProfileV1(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Access Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Access Profile", accessProfileErrDetail(err, httpResp))
		return
	}

	newState, diags := accessProfileDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Access Profile", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *accessProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_access_profile.AccessProfileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Access Profile", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.AccessProfilesAPI.
		DeleteAccessProfileV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Access Profile already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Access Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Access Profile", accessProfileErrDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Access Profile", map[string]interface{}{"id": state.Id.ValueString()})
}

// accessProfileModelToDto converts the Terraform plan/config model into the
// SDK create/update DTO shape. See package doc for the pass-through-only and
// hand-written list-conversion caveats.
//
// owner and source are both Required in the schema (matches the API's
// required: [name, owner, source]), so they are never Unknown by the time
// Create/Update run.
func accessProfileModelToDto(ctx context.Context, m resource_access_profile.AccessProfileModel) (*access_profiles.AccessProfile, diag.Diagnostics) {
	var diags diag.Diagnostics

	owner, d := m.Owner.ToApi_betaOwnerReference(ctx)
	diags.Append(d...)
	if diags.HasError() || owner == nil {
		return nil, diags
	}

	source, d := m.Source.ToApi_betaAccessProfileSourceRef(ctx)
	diags.Append(d...)
	if diags.HasError() || source == nil {
		return nil, diags
	}

	dto := access_profiles.NewAccessProfileWithDefaults()
	dto.Name = m.Name.ValueString()
	dto.Owner = *access_profiles.NewNullableOwnerReference(owner)
	dto.Source = *source

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		dto.Description = *access_profiles.NewNullableString(m.Description.ValueStringPointer())
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		dto.Enabled = m.Enabled.ValueBoolPointer()
	}
	if !m.Requestable.IsNull() && !m.Requestable.IsUnknown() {
		dto.Requestable = m.Requestable.ValueBoolPointer()
	}

	if !m.Segments.IsNull() && !m.Segments.IsUnknown() {
		var segments []string
		diags.Append(m.Segments.ElementsAs(ctx, &segments, false)...)
		dto.Segments = segments
	}

	if !m.Entitlements.IsNull() && !m.Entitlements.IsUnknown() {
		var items []resource_access_profile.EntitlementsValue
		diags.Append(m.Entitlements.ElementsAs(ctx, &items, false)...)
		refs := make([]access_profiles.EntitlementRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, access_profiles.EntitlementRef{
				Id:   item.Id.ValueStringPointer(),
				Name: *access_profiles.NewNullableString(item.Name.ValueStringPointer()),
				Type: item.EntitlementsType.ValueStringPointer(),
			})
		}
		dto.Entitlements = refs
	}

	if !m.AdditionalOwners.IsNull() && !m.AdditionalOwners.IsUnknown() {
		var items []resource_access_profile.AdditionalOwnersValue
		diags.Append(m.AdditionalOwners.ElementsAs(ctx, &items, false)...)
		refs := make([]access_profiles.AdditionalOwnerRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, access_profiles.AdditionalOwnerRef{
				Id:   item.Id.ValueStringPointer(),
				Name: *access_profiles.NewNullableString(item.Name.ValueStringPointer()),
				Type: item.AdditionalOwnersType.ValueStringPointer(),
			})
		}
		dto.AdditionalOwners = refs
	}

	if !m.ProvisioningCriteria.IsNull() && !m.ProvisioningCriteria.IsUnknown() {
		level1, d := accessProfileProvisioningCriteriaToApi(ctx, m.ProvisioningCriteria)
		diags.Append(d...)
		if level1 != nil {
			dto.ProvisioningCriteria = *access_profiles.NewNullableProvisioningCriteriaLevel1(level1)
		}
	}

	return dto, diags
}

// accessProfileDtoToModel converts an API response DTO into the Terraform
// state model, preferring fields carried over from fallback (plan/prior
// state) for the pass-through-only blocks documented in the package doc.
func accessProfileDtoToModel(ctx context.Context, dto *access_profiles.AccessProfile, fallback resource_access_profile.AccessProfileModel) (resource_access_profile.AccessProfileModel, diag.Diagnostics) {
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

	owner, d := resource_access_profile.OwnerValue{}.FromApi_betaOwnerReference(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	source, d := resource_access_profile.SourceValue{}.FromApi_betaAccessProfileSourceRef(ctx, &dto.Source)
	diags.Append(d...)
	model.Source = source

	if dto.Segments != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.Segments)
		diags.Append(d...)
		model.Segments = listVal
	}

	if dto.Entitlements != nil {
		values := make([]resource_access_profile.EntitlementsValue, 0, len(dto.Entitlements))
		for _, e := range dto.Entitlements {
			values = append(values, resource_access_profile.EntitlementsValue{
				Id:               types.StringPointerValue(e.Id),
				Name:             types.StringPointerValue(e.Name.Get()),
				EntitlementsType: types.StringPointerValue(e.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, resource_access_profile.EntitlementsValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Entitlements = listVal
	}

	if dto.AdditionalOwners != nil {
		values := make([]resource_access_profile.AdditionalOwnersValue, 0, len(dto.AdditionalOwners))
		for _, o := range dto.AdditionalOwners {
			values = append(values, resource_access_profile.AdditionalOwnersValue{
				Id:                   types.StringPointerValue(o.Id),
				Name:                 types.StringPointerValue(o.Name.Get()),
				AdditionalOwnersType: types.StringPointerValue(o.Type),
			})
		}
		listVal, d := types.ListValueFrom(ctx, resource_access_profile.AdditionalOwnersValue{}.Type(ctx), values)
		diags.Append(d...)
		model.AdditionalOwners = listVal
	}

	// access_model_metadata, access_request_config, and revocation_request_config
	// are still pass-through-only on the write path (this provider does not
	// send them to the API on Create/Update - see the accessProfileModelToDto
	// doc comment), but on the read path they now get a real deeper read-back
	// (see resource_access_profile_readback.go) whenever the practitioner
	// hasn't configured them: this surfaces IdentityNow's actual computed
	// defaults/drift for these blocks instead of only ever showing Null. If
	// the practitioner HAS configured one of these blocks, we keep
	// pass-through of their configured value (with
	// accessProfilePassThroughWarning already emitted above) rather than
	// overwriting it with the API's response, since overwriting would produce
	// a permanent, non-convergent diff for a value this provider never
	// actually writes.
	if model.AccessModelMetadata.IsNull() || model.AccessModelMetadata.IsUnknown() {
		v, d := accessProfileAccessModelMetadataFromApi(ctx, dto.AccessModelMetadata)
		diags.Append(d...)
		model.AccessModelMetadata = v
	}
	if model.AccessRequestConfig.IsNull() || model.AccessRequestConfig.IsUnknown() {
		v, d := accessProfileAccessRequestConfigFromApi(ctx, dto.AccessRequestConfig.Get())
		diags.Append(d...)
		model.AccessRequestConfig = v
	}
	if model.RevocationRequestConfig.IsNull() || model.RevocationRequestConfig.IsUnknown() {
		v, d := accessProfileRevocationRequestConfigFromApi(ctx, dto.RevocationRequestConfig.Get())
		diags.Append(d...)
		model.RevocationRequestConfig = v
	}

	// provisioning_criteria has real write support (see accessProfileModelToDto),
	// so - unlike the three pass-through-only blocks above - it is always read
	// back from the API response rather than only when unconfigured, mirroring
	// how entitlements/additional_owners/segments are always read back.
	pc, d := accessProfileProvisioningCriteriaFromApi(ctx, dto.ProvisioningCriteria.Get())
	diags.Append(d...)
	model.ProvisioningCriteria = pc

	return model, diags
}

func accessProfileErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

// accessProfilePassThroughWarning adds a warning diagnostic when the
// practitioner has configured one of the pass-through-only nested blocks (see
// the package doc) so it's clear in `terraform plan`/`apply` output - not just
// in code comments or DEBUG logs - that Terraform won't detect drift on that
// block: state will always mirror whatever was last planned/configured for
// it, never the live API value.
func accessProfilePassThroughWarning(ctx context.Context, diags *diag.Diagnostics, attrName string, isNull bool) {
	if isNull {
		return
	}
	tflog.Warn(ctx, "Access Profile pass-through-only attribute configured; drift will not be detected", map[string]interface{}{
		"attribute": attrName,
	})
	diags.AddWarning(
		fmt.Sprintf("%q is not read back from the API", attrName),
		fmt.Sprintf(
			"The identitynow_access_profile_v1 resource does not parse %q from the SailPoint API response (see the "+
				"access_profile_v1 package documentation for why). Terraform will not detect drift on this attribute; "+
				"its state will always mirror the last configured/planned value.", attrName),
	)
}

func accessProfileJSONPatchReplace(path string, value access_profiles.JsonPatchOperationValue) access_profiles.JsonPatchOperation {
	return access_profiles.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// accessProfileStructToMap round-trips an SDK model struct through JSON to get
// a map[string]interface{} suitable for
// access_profiles.MapmapOfStringAnyAsJsonPatchOperationValue, since the
// JSON Patch value wrapper type doesn't accept typed structs directly.
func accessProfileStructToMap(v interface{}) (map[string]interface{}, error) {
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

// accessProfileSliceToArrayInner round-trips a slice of SDK model structs
// (e.g. []access_profiles.EntitlementRef) through JSON to build a
// []access_profiles.ArrayInner suitable for
// access_profiles.ArrayOfArrayInnerAsJsonPatchOperationValue, since
// JsonPatchOperation.Value has no generic "array of objects" constructor.
func accessProfileSliceToArrayInner(v interface{}) ([]access_profiles.ArrayInner, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var maps []map[string]interface{}
	if err := json.Unmarshal(b, &maps); err != nil {
		return nil, err
	}
	arr := make([]access_profiles.ArrayInner, 0, len(maps))
	for i := range maps {
		m := maps[i]
		arr = append(arr, access_profiles.ArrayInner{MapmapOfStringAny: &m})
	}
	return arr, nil
}
