// Package access_model_metadata_attribute_v1 is a pilot implementation of the
// Access Model Metadata Attribute resource/data source, generated from
// SailPoint's per-service v1 OpenAPI spec (api-specs/idn/apis/access-model-metadata),
// following the same hand-written CRUD pattern established by
// role_v1/service_desk_integration_v1/transform_v1.
//
// Design decision: dedicated resource, not baked into role/entitlement/
// access-profile (2026-07-25). Access Model Metadata is its own distinct
// SailPoint service/concern with its own top-level OpenAPI spec and its own
// full CRUD-ish API surface (list/create/get/update attributes; list/create/
// get/update values under an attribute) - it is NOT merely a sub-field of any
// one access-item resource. It defines a shared, tenant-wide taxonomy of
// metadata attributes/values (e.g. "iscPrivacy" -> "public"/"internal"/
// "confidential") that can then be *referenced/assigned* from role,
// entitlement, and access profile resources via their own embedded
// "access_model_metadata" block (already implemented as a read-back-only,
// pass-through field on role_v1 - see resource_role.go). Managing the
// taxonomy itself (this package) and assigning existing taxonomy values to a
// specific access item (role_v1's embedded field) are different concerns
// with different lifecycles and correctly belong in different resources -
// exactly the same shape as, e.g., an AWS/GCP tag *key* catalog resource
// being distinct from the individual tag *assignments* on other resources.
//
// Spec/SDK gap - DELETE is real but undocumented and unwrapped by the SDK:
// SailPoint's published `api-specs` OpenAPI document for this service (both
// the legacy `beta` spec and the newer per-service `v1` spec) omits
// `DELETE /access-model-metadata/attributes/{key}` entirely, so
// `golang-sdk`'s generated `AccessModelMetadataAPIService` has no delete
// method - initial investigation (2026-07-25) concluded there was no delete
// capability at all. That was **wrong**: a captured browser network call
// from the IdentityNow Admin UI itself (2026-07-26) confirmed
// `DELETE /beta/access-model-metadata/attributes/{key}` is a real, working
// endpoint - it's a spec/SDK documentation gap, not a genuine API
// limitation. `Delete()` therefore hand-rolls a raw HTTP DELETE call (see
// `resource_access_model_metadata_attribute_delete.go`) using only the SDK's
// exported configuration/auth surface, rather than emitting a
// permanent-orphan warning. If SailPoint ever publishes this operation in
// the spec (and a future `golang-sdk` release generates a real method for
// it), replace this hand-rolled call with the generated one. See the
// 2026-07-26 knowledge entry for the full investigation and reversal.
//
// The attribute-level PATCH endpoint's own documentation still states only
// "name", "description", "multiselect", and "values" are patchable - "key",
// "type", "object_types", and "status" cannot be changed via PATCH, which is
// why those still get RequiresReplace()/UseStateForUnknown() treatment below
// even though Delete() itself is no longer a permanence concern.
//
// Immutable-after-create fields: "key" (the technical name), "type" ("custom"
// vs "governance"), and "object_types" are all accepted at Create time but
// are NOT in the PATCH-patchable field list, so a config change to any of
// them post-creation cannot be silently applied - resource_access_model_metadata_attribute_planmodifiers.go
// applies stringplanmodifier.RequiresReplace()/listplanmodifier.RequiresReplace()
// to force a destroy+recreate (with the same permanent-orphan caveat as
// above) rather than let Terraform believe a no-op Update succeeded while
// drift silently persists forever.
package access_model_metadata_attribute_v1

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/access_model_metadata"

	"terraform-provider-identitynow/internal/provider/access_model_metadata_attribute_v1/resource_access_model_metadata_attribute"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
	GetClientConfig() *sailpoint.Configuration
}

var (
	_ resource.Resource                = (*accessModelMetadataAttributeResource)(nil)
	_ resource.ResourceWithConfigure   = (*accessModelMetadataAttributeResource)(nil)
	_ resource.ResourceWithImportState = (*accessModelMetadataAttributeResource)(nil)
)

func NewAccessModelMetadataAttributeResource() resource.Resource {
	return &accessModelMetadataAttributeResource{}
}

type accessModelMetadataAttributeResource struct {
	client *sailpoint.APIClient
	config *sailpoint.Configuration
}

func (r *accessModelMetadataAttributeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_model_metadata_attribute_v1"
}

func (r *accessModelMetadataAttributeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_access_model_metadata_attribute.AccessModelMetadataAttributeResourceSchema(ctx)
	resp.Schema.Description = "Manages an Access Model Metadata Attribute in IdentityNow/ISC - a shared, tenant-wide " +
		"metadata attribute (with its allowed values) that can be assigned to entitlements, access profiles, and roles " +
		"to add contextual information (risk, regulations, privacy levels, etc.)."
	resp.Schema.MarkdownDescription = "Manages an [Access Model Metadata](https://documentation.sailpoint.com/saas/help/access/metadata.html) " +
		"Attribute in IdentityNow/ISC - a shared, tenant-wide metadata attribute (with its allowed values) that can be " +
		"assigned to entitlements, access profiles, and roles to add contextual information (risk, regulations, privacy " +
		"levels, etc.).\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below. Notably, `Delete` " +
		"uses a hand-rolled HTTP call rather than the golang-sdk (whose generated client has no delete method for " +
		"this resource - a spec/SDK documentation gap, not a real API limitation)."
	applyAccessModelMetadataAttributeUseStateForUnknown(&resp.Schema)
}

func (r *accessModelMetadataAttributeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.config = cp.GetClientConfig()
}

func (r *accessModelMetadataAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}

func (r *accessModelMetadataAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Access Model Metadata Attribute", map[string]interface{}{"key": plan.Key.ValueString(), "name": plan.Name.ValueString()})

	dto, diags := ammModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.AccessModelMetadataAPI.
		CreateAccessModelMetadataAttributeV1(ctx).
		AttributeDTO(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Access Model Metadata Attribute", map[string]interface{}{"key": plan.Key.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Access Model Metadata Attribute", errDetail(err, httpResp))
		return
	}

	state, diags := ammDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accessModelMetadataAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString()})

	apiResp, httpResp, err := r.client.AccessModelMetadataAPI.
		GetAccessModelMetadataAttributeV1(ctx, state.Key.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Access Model Metadata Attribute not found, removing from state", map[string]interface{}{"key": state.Key.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Access Model Metadata Attribute", errDetail(err, httpResp))
		return
	}

	newState, diags := ammDtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Access Model Metadata Attribute", map[string]interface{}{"key": newState.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *accessModelMetadataAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString()})

	// Per the API's own documentation, only "name", "description",
	// "multiselect", and "values" are patchable - "key", "type", and
	// "object_types" are RequiresReplace() in the schema (see
	// resource_access_model_metadata_attribute_planmodifiers.go) precisely so
	// Update is never asked to (silently fail to) change them.
	dto, diags := ammModelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := []access_model_metadata.JsonPatchOperation{
		ammJSONPatchReplace("/name", access_model_metadata.StringAsJsonPatchOperationValue(dto.Name)),
		ammJSONPatchReplace("/multiselect", access_model_metadata.BoolAsJsonPatchOperationValue(dto.Multiselect)),
	}
	if dto.Description != nil {
		patch = append(patch, ammJSONPatchReplace("/description", access_model_metadata.StringAsJsonPatchOperationValue(dto.Description)))
	}
	if dto.Values != nil {
		if arr, err := ammSliceToArrayInner(dto.Values); err == nil {
			patch = append(patch, ammJSONPatchReplace("/values", access_model_metadata.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)))
		} else {
			resp.Diagnostics.AddError("Error encoding \"values\" for update", err.Error())
			return
		}
	}

	apiResp, httpResp, err := r.client.AccessModelMetadataAPI.
		UpdateAccessModelMetadataAttributeV1(ctx, state.Key.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Access Model Metadata Attribute", errDetail(err, httpResp))
		return
	}

	newState, diags := ammDtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Access Model Metadata Attribute", map[string]interface{}{"key": newState.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete calls the real DELETE /access-model-metadata/attributes/{key}
// endpoint. This is NOT exposed by the generated golang-sdk (SailPoint's
// published OpenAPI spec omits this operation even though it demonstrably
// works - confirmed 2026-07-26 via a captured browser network call from the
// IdentityNow Admin UI itself), so this hand-rolls the HTTP call via
// deleteAccessModelMetadataAttribute (see resource_access_model_metadata_attribute_delete.go)
// using only the SDK's exported configuration/auth surface. A 404 is treated
// as already-deleted (not an error), matching every other resource's Read()
// convention in this provider.
func (r *accessModelMetadataAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString()})

	httpResp, err := deleteAccessModelMetadataAttribute(ctx, r.config, state.Key.ValueString())
	if err != nil {
		tflog.Error(ctx, "Error deleting Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Access Model Metadata Attribute", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent && httpResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError(
			"Error deleting Access Model Metadata Attribute",
			fmt.Sprintf("unexpected status %s deleting attribute %q: %s", httpResp.Status, state.Key.ValueString(), string(body)),
		)
		return
	}

	tflog.Info(ctx, "Deleted Access Model Metadata Attribute", map[string]interface{}{"key": state.Key.ValueString()})
}

// ammModelToDto converts the resource's plan model into an access_model_metadata.AttributeDTO
// suitable for both Create and (partially) Update.
func ammModelToDto(ctx context.Context, m resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel) (*access_model_metadata.AttributeDTO, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto := access_model_metadata.NewAttributeDTOWithDefaults()
	dto.Key = m.Key.ValueStringPointer()
	dto.Name = m.Name.ValueStringPointer()
	dto.Multiselect = m.Multiselect.ValueBoolPointer()
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		dto.Description = m.Description.ValueStringPointer()
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		dto.Type = m.Type.ValueStringPointer()
	}
	if !m.Status.IsNull() && !m.Status.IsUnknown() {
		dto.Status = m.Status.ValueStringPointer()
	}

	if !m.ObjectTypes.IsNull() && !m.ObjectTypes.IsUnknown() {
		var objectTypes []string
		diags.Append(m.ObjectTypes.ElementsAs(ctx, &objectTypes, false)...)
		dto.ObjectTypes = objectTypes
	}

	if !m.Values.IsNull() && !m.Values.IsUnknown() {
		var items []resource_access_model_metadata_attribute.ValuesValue
		diags.Append(m.Values.ElementsAs(ctx, &items, false)...)
		values := make([]access_model_metadata.AttributeValueDTO, 0, len(items))
		for _, item := range items {
			v, d := item.ToApi_betaAttributeValueDTO(ctx)
			diags.Append(d...)
			if v != nil {
				values = append(values, *v)
			}
		}
		dto.Values = values
	}

	return dto, diags
}

// ammDtoToModel converts an API response DTO into the resource's state model.
func ammDtoToModel(ctx context.Context, dto *access_model_metadata.AttributeDTO, fallback resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel) (resource_access_model_metadata_attribute.AccessModelMetadataAttributeModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.Key = types.StringPointerValue(dto.Key)
	model.Name = types.StringPointerValue(dto.Name)
	model.Multiselect = types.BoolPointerValue(dto.Multiselect)
	model.Description = types.StringPointerValue(dto.Description)
	model.Status = types.StringPointerValue(dto.Status)
	model.Type = types.StringPointerValue(dto.Type)

	if dto.ObjectTypes != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.ObjectTypes)
		diags.Append(d...)
		model.ObjectTypes = listVal
	} else {
		model.ObjectTypes = types.ListNull(types.StringType)
	}

	values := make([]resource_access_model_metadata_attribute.ValuesValue, 0, len(dto.Values))
	for i := range dto.Values {
		v, d := resource_access_model_metadata_attribute.ValuesValue{}.FromApi_betaAttributeValueDTO(ctx, &dto.Values[i])
		diags.Append(d...)
		values = append(values, v)
	}
	listVal, d := types.ListValueFrom(ctx, resource_access_model_metadata_attribute.ValuesValue{}.Type(ctx), values)
	diags.Append(d...)
	model.Values = listVal

	return model, diags
}

// ammJSONPatchReplace builds a "replace" RFC 6902 JSON Patch operation - a
// small helper mirroring role_v1's roleJSONPatchReplace for the same shared
// access_model_metadata.JsonPatchOperationValue oneOf wrapper type used
// across the SDK's JSON Patch endpoints (not specific to any one resource
// despite the type's role-flavored name).
func ammJSONPatchReplace(path string, value access_model_metadata.JsonPatchOperationValue) access_model_metadata.JsonPatchOperation {
	return access_model_metadata.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// ammSliceToArrayInner round-trips a slice of access_model_metadata.AttributeValueDTO
// structs into []access_model_metadata.ArrayInner (each wrapping a map[string]interface{})
// for use in a JSON Patch "replace" operation's array value - mirrors role_v1's
// roleSliceToArrayInner for the same purpose.
func ammSliceToArrayInner(v []access_model_metadata.AttributeValueDTO) ([]access_model_metadata.ArrayInner, error) {
	arr := make([]access_model_metadata.ArrayInner, 0, len(v))
	for i := range v {
		m := map[string]interface{}{}
		if v[i].Value != nil {
			m["value"] = *v[i].Value
		}
		if v[i].Name != nil {
			m["name"] = *v[i].Name
		}
		if v[i].Status != nil {
			m["status"] = *v[i].Status
		}
		arr = append(arr, access_model_metadata.ArrayInner{MapmapOfStringAny: &m})
	}
	return arr, nil
}

// errDetail delegates to the shared util.SailpointErrorDetail helper, same as
// role_v1/service_desk_integration_v1/transform_v1.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
