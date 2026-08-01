// Package entitlement_v1 is a pilot implementation of the entitlement
// resource/data sources generated from SailPoint's per-service v1 OpenAPI spec
// (api-specs/idn/apis/entitlements), following the same hand-written CRUD
// pattern established by the other _v1 pilot targets.
//
// This target is the repo's first "adopt-existing" resource: the Entitlements
// API exposes GET/PATCH list/read/update behavior, but no create endpoint and
// no delete endpoint. Entitlements come into existence only when a source
// aggregation/import process creates them elsewhere, and this resource's
// lifecycle therefore models Terraform adoption of an already-existing object
// rather than API-side creation:
//   - Create never POSTs anything. It adopts either by `id`, or by the
//     reference-provider-compatible lookup key pair `source_id` + `value`.
//     The generated schema marked `id` as Required and `value` as
//     Computed-only, but this wrapper deliberately overrides them to
//     Optional+Computed and hand-adds Optional `source_id` so practitioners
//     can use either adoption mode.
//   - When `id` is set, Create calls GetEntitlement directly. When `source_id`
//   - `value` are set instead, Create performs a filtered ListEntitlements
//     lookup, requires exactly one match, then adopts that returned object.
//   - After the initial GET/list lookup, Create reads back all
//     live/computed fields into state and only then issues a follow-up PATCH
//     if the practitioner explicitly configured writable fields whose desired
//     values differ from the live object returned by that initial read.
//   - Read is a normal GetEntitlement refresh. A 404 means the adopted object
//     no longer exists, so state is removed.
//   - Update sends a minimal RFC 6902 JSON Patch over only the writable subset
//     this target actually manages: requestable, segments, owner, name, and
//     description. Read-only fields returned by the API (source, attribute,
//     access model metadata, direct permissions, tags, etc.) are never sent.
//   - Delete is intentionally a Terraform-only state removal. There is no
//     DELETE /entitlements/v1/{id} endpoint, so the provider must not invent
//     one or simulate destructive behavior.
//
// Codegen notes:
//   - `attributes` (the entitlement's free-form source attributes) is a true
//     dynamic JSON object, excluded from codegen and hand-added here as a
//     jsontypes.Normalized JSON string like sources_v1's connector_attributes.
//   - `manually_updated_fields` was also excluded from codegen even though the
//     real SDK type is not truly dynamic: the OpenAPI schema declared
//     additionalProperties, but the vendored golang-sdk exposes two fixed keys
//     (DISPLAY_NAME and DESCRIPTION). This package hand-adds it back as a
//     Computed nested object with `display_name` and `description` booleans.
//   - The code-generated schema/model also include `privilege_level` and
//     `tags`, but the current golang-sdk Entitlement struct does not publish
//     typed fields for them. These wrappers read them from the SDK model's
//     AdditionalProperties map when present.
package entitlement_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/entitlements"

	"terraform-provider-identitynow/internal/provider/entitlement_v1/resource_entitlement"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                     = (*entitlementResource)(nil)
	_ resource.ResourceWithConfigValidators = (*entitlementResource)(nil)
	_ resource.ResourceWithConfigure        = (*entitlementResource)(nil)
	_ resource.ResourceWithImportState      = (*entitlementResource)(nil)
)

func NewEntitlementResource() resource.Resource {
	return &entitlementResource{}
}

type entitlementResource struct {
	client *sailpoint.APIClient
}

type entitlementResourceModel struct {
	AccessModelMetadata    resource_entitlement.AccessModelMetadataValue `tfsdk:"access_model_metadata"`
	Attribute              types.String                                  `tfsdk:"attribute"`
	Attributes             jsontypes.Normalized                          `tfsdk:"attributes"`
	CloudGoverned          types.Bool                                    `tfsdk:"cloud_governed"`
	Created                types.String                                  `tfsdk:"created"`
	Description            types.String                                  `tfsdk:"description"`
	DirectPermissions      types.List                                    `tfsdk:"direct_permissions"`
	Id                     types.String                                  `tfsdk:"id"`
	ManuallyUpdatedFields  types.Object                                  `tfsdk:"manually_updated_fields"`
	Modified               types.String                                  `tfsdk:"modified"`
	Name                   types.String                                  `tfsdk:"name"`
	Owner                  resource_entitlement.OwnerValue               `tfsdk:"owner"`
	PrivilegeLevel         resource_entitlement.PrivilegeLevelValue      `tfsdk:"privilege_level"`
	Requestable            types.Bool                                    `tfsdk:"requestable"`
	Segments               types.List                                    `tfsdk:"segments"`
	SourceId               types.String                                  `tfsdk:"source_id"`
	Source                 resource_entitlement.SourceValue              `tfsdk:"source"`
	SourceSchemaObjectType types.String                                  `tfsdk:"source_schema_object_type"`
	Tags                   types.List                                    `tfsdk:"tags"`
	Value                  types.String                                  `tfsdk:"value"`
}

func (r *entitlementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement_v1"
}

func (r *entitlementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_entitlement.EntitlementResourceSchema(ctx)
	resp.Schema.Description = "Adopts and manages an existing Entitlement in IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Adopts and manages an existing [Entitlement](https://documentation.sailpoint.com/) in " +
		"IdentityNow/ISC either by `id` or by the lookup key pair `source_id` + `value`. This target does **not** create or " +
		"delete entitlements in the upstream API: Create resolves an existing entitlement, then optionally PATCHes writable fields " +
		"(`requestable`, `segments`, `owner`, `name`, `description`) to match configuration; Delete removes only Terraform state " +
		"because the API has no delete endpoint."

	patchEntitlementResourceSchema(&resp.Schema)
	applyEntitlementUseStateForUnknown(&resp.Schema)
}

func patchEntitlementResourceSchema(s *resourceschema.Schema) {
	if a, ok := s.Attributes["id"].(resourceschema.StringAttribute); ok {
		a.Required = false
		a.Optional = true
		a.Computed = true
		a.Description = "ID of the entitlement. Optional when adopting by source_id + value lookup instead."
		a.MarkdownDescription = "ID of the entitlement. Optional when adopting by `source_id` + `value` lookup instead."
		a.Validators = append(a.Validators,
			stringvalidator.ConflictsWith(path.MatchRoot("source_id"), path.MatchRoot("value")),
		)
		s.Attributes["id"] = a
	}
	if a, ok := s.Attributes["requestable"].(resourceschema.BoolAttribute); ok {
		a.Default = nil
		s.Attributes["requestable"] = a
	}
	if a, ok := s.Attributes["value"].(resourceschema.StringAttribute); ok {
		a.Optional = true
		a.Computed = true
		a.Description = "The entitlement value. Also used as the lookup key, together with source_id, when adopting by value instead of by id."
		a.MarkdownDescription = "The entitlement value. Also used as the lookup key, together with `source_id`, when adopting by value instead of by id."
		a.Validators = append(a.Validators,
			stringvalidator.AlsoRequires(path.MatchRoot("source_id")),
			stringvalidator.ConflictsWith(path.MatchRoot("id")),
		)
		s.Attributes["value"] = a
	}
	s.Attributes["source_id"] = resourceschema.StringAttribute{
		Optional:            true,
		Description:         "Source ID used with value to locate an entitlement when id is not known.",
		MarkdownDescription: "Source ID used with `value` to locate an entitlement when `id` is not known.",
		Validators: []validator.String{
			stringvalidator.AlsoRequires(path.MatchRoot("value")),
			stringvalidator.ConflictsWith(path.MatchRoot("id")),
		},
	}
	applyEntitlementResourceAttributesField(&s.Attributes)
	applyEntitlementResourceManuallyUpdatedFieldsField(&s.Attributes)
}

func (r *entitlementResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("source_id"),
		),
	}
}

func applyEntitlementResourceAttributesField(attrs *map[string]resourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]resourceschema.Attribute{}
	}
	desc := "Raw source-system attributes for this entitlement, represented as a normalized JSON object because the shape is connector-specific and truly dynamic."
	(*attrs)["attributes"] = resourceschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

func applyEntitlementResourceManuallyUpdatedFieldsField(attrs *map[string]resourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]resourceschema.Attribute{}
	}
	desc := "Flags describing whether selected entitlement fields were manually updated after first aggregation."
	(*attrs)["manually_updated_fields"] = resourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
		Attributes: map[string]resourceschema.Attribute{
			"display_name": resourceschema.BoolAttribute{
				Computed:            true,
				Description:         "Whether the entitlement display name was manually updated.",
				MarkdownDescription: "Whether the entitlement display name was manually updated.",
			},
			"description": resourceschema.BoolAttribute{
				Computed:            true,
				Description:         "Whether the entitlement description was manually updated.",
				MarkdownDescription: "Whether the entitlement description was manually updated.",
			},
		},
	}
}

func applyEntitlementUseStateForUnknown(s *resourceschema.Schema) {
	patchEntitlementString(s, "id")
	patchEntitlementString(s, "created")

	if owner, ok := s.Attributes["owner"].(resourceschema.SingleNestedAttribute); ok {
		if name, ok := owner.Attributes["name"].(resourceschema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			owner.Attributes["name"] = name
		}
		s.Attributes["owner"] = owner
	}

	if source, ok := s.Attributes["source"].(resourceschema.SingleNestedAttribute); ok {
		if name, ok := source.Attributes["name"].(resourceschema.StringAttribute); ok {
			name.PlanModifiers = append(name.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			source.Attributes["name"] = name
		}
		s.Attributes["source"] = source
	}
}

func patchEntitlementString(s *resourceschema.Schema, name string) {
	a, ok := s.Attributes[name].(resourceschema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}

func (r *entitlementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *entitlementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *entitlementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan entitlementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := r.resolveEntitlementAdoptionID(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Adopting Entitlement", map[string]interface{}{"id": id})

	state, notFound, diags := r.readEntitlementState(ctx, id, plan)
	if notFound {
		resp.Diagnostics.AddError(
			"Error adopting Entitlement",
			fmt.Sprintf("Entitlement %q does not exist or is no longer readable.", id),
		)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchOps, diags := entitlementResourcePatchOps(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(patchOps) > 0 {
		tflog.Debug(ctx, "Patching adopted Entitlement after initial GET", map[string]interface{}{"id": id, "patch_ops": len(patchOps)})
		_, httpResp, err := r.client.EntitlementsAPI.
			PatchEntitlementV1(ctx, id).
			JsonPatchOperation(patchOps).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error finalizing adopted Entitlement", map[string]interface{}{"id": id, "error": err.Error()})
			resp.Diagnostics.AddError("Error finalizing Entitlement adoption", entitlementErrDetail(err, httpResp))
			return
		}
	}

	state, notFound, diags = r.readEntitlementState(ctx, id, plan)
	if notFound {
		resp.Diagnostics.AddError(
			"Error reading adopted Entitlement",
			fmt.Sprintf("Entitlement %q disappeared immediately after adoption.", id),
		)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *entitlementResource) resolveEntitlementAdoptionID(ctx context.Context, plan entitlementResourceModel) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if !plan.Id.IsNull() && !plan.Id.IsUnknown() && plan.Id.ValueString() != "" {
		return plan.Id.ValueString(), diags
	}

	if plan.SourceId.IsNull() || plan.SourceId.IsUnknown() || plan.Value.IsNull() || plan.Value.IsUnknown() {
		diags.AddError(
			"Missing Entitlement adoption key",
			"Configure either `id`, or both `source_id` and `value`, before creating identitynow_entitlement_v1.",
		)
		return "", diags
	}

	filter := fmt.Sprintf("source.id eq %q and value eq %q", plan.SourceId.ValueString(), plan.Value.ValueString())
	results, httpResp, err := r.client.EntitlementsAPI.
		ListEntitlementsV1(ctx).
		Filters(filter).
		Limit(2).
		Execute()
	if err != nil {
		diags.AddError("Error locating Entitlement by source_id + value", entitlementErrDetail(err, httpResp))
		return "", diags
	}

	switch len(results) {
	case 0:
		diags.AddError(
			"Entitlement not found",
			fmt.Sprintf("No entitlement matched source_id=%q and value=%q.", plan.SourceId.ValueString(), plan.Value.ValueString()),
		)
		return "", diags
	case 1:
		if results[0].Id == nil || *results[0].Id == "" {
			diags.AddError(
				"Entitlement lookup returned no id",
				"The matching entitlement did not include an id in the API response.",
			)
			return "", diags
		}
		return *results[0].Id, diags
	default:
		diags.AddError(
			"Ambiguous entitlement lookup",
			fmt.Sprintf("Expected exactly one entitlement for source_id=%q and value=%q, but found %d.", plan.SourceId.ValueString(), plan.Value.ValueString(), len(results)),
		)
		return "", diags
	}
}

func (r *entitlementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state entitlementResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, notFound, diags := r.readEntitlementState(ctx, state.Id.ValueString(), state)
	if notFound {
		tflog.Warn(ctx, "Entitlement not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *entitlementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan entitlementResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state entitlementResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchOps, diags := entitlementResourcePatchOps(ctx, plan, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(patchOps) > 0 {
		tflog.Debug(ctx, "Patching Entitlement", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patchOps)})
		_, httpResp, err := r.client.EntitlementsAPI.
			PatchEntitlementV1(ctx, state.Id.ValueString()).
			JsonPatchOperation(patchOps).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error updating Entitlement", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
			resp.Diagnostics.AddError("Error updating Entitlement", entitlementErrDetail(err, httpResp))
			return
		}
	} else {
		tflog.Debug(ctx, "Entitlement update required no patch operations", map[string]interface{}{"id": state.Id.ValueString()})
	}

	newState, notFound, diags := r.readEntitlementState(ctx, state.Id.ValueString(), plan)
	if notFound {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *entitlementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state entitlementResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Removing Entitlement from Terraform state only", map[string]interface{}{"id": state.Id.ValueString()})
	resp.State.RemoveResource(ctx)
}

func (r *entitlementResource) readEntitlementState(ctx context.Context, id string, fallback entitlementResourceModel) (entitlementResourceModel, bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	dto, httpResp, err := r.client.EntitlementsAPI.
		GetEntitlementV1(ctx, id).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return entitlementResourceModel{}, true, diags
		}
		diags.AddError("Error reading Entitlement", entitlementErrDetail(err, httpResp))
		return entitlementResourceModel{}, false, diags
	}

	model, d := entitlementResourceDtoToModel(ctx, dto, fallback)
	diags.Append(d...)
	return model, false, diags
}

func entitlementResourcePatchOps(ctx context.Context, plan, state entitlementResourceModel) ([]entitlements.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := make([]entitlements.JsonPatchOperation, 0, 5)

	patch = append(patch, entitlementStringPatchOps("/name", plan.Name, state.Name)...)
	patch = append(patch, entitlementStringPatchOps("/description", plan.Description, state.Description)...)
	patch = append(patch, entitlementBoolPatchOps("/requestable", plan.Requestable, state.Requestable)...)

	segmentsOps, d := entitlementListPatchOps(ctx, "/segments", plan.Segments, state.Segments)
	diags.Append(d...)
	patch = append(patch, segmentsOps...)

	ownerOps, d := entitlementOwnerPatchOps(plan.Owner, state.Owner)
	diags.Append(d...)
	patch = append(patch, ownerOps...)

	return patch, diags
}

func entitlementStringPatchOps(path string, plan, state types.String) []entitlements.JsonPatchOperation {
	if plan.IsUnknown() || plan.Equal(state) {
		return nil
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil
		}
		return []entitlements.JsonPatchOperation{entitlementJSONPatchRemove(path)}
	}
	v := plan.ValueString()
	if state.IsNull() || state.IsUnknown() {
		return []entitlements.JsonPatchOperation{entitlementJSONPatchAdd(path, entitlements.StringAsJsonPatchOperationValue(&v))}
	}
	return []entitlements.JsonPatchOperation{entitlementJSONPatchReplace(path, entitlements.StringAsJsonPatchOperationValue(&v))}
}

func entitlementBoolPatchOps(path string, plan, state types.Bool) []entitlements.JsonPatchOperation {
	if plan.IsUnknown() || plan.IsNull() || plan.Equal(state) {
		return nil
	}
	v := plan.ValueBool()
	if state.IsNull() || state.IsUnknown() {
		return []entitlements.JsonPatchOperation{entitlementJSONPatchAdd(path, entitlements.BoolAsJsonPatchOperationValue(&v))}
	}
	return []entitlements.JsonPatchOperation{entitlementJSONPatchReplace(path, entitlements.BoolAsJsonPatchOperationValue(&v))}
}

func entitlementListPatchOps(ctx context.Context, path string, plan, state types.List) ([]entitlements.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || plan.Equal(state) {
		return nil, diags
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil, diags
		}
		return []entitlements.JsonPatchOperation{entitlementJSONPatchRemove(path)}, diags
	}
	arr, d := entitlementStringListToArrayInner(ctx, plan)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	value := entitlements.ArrayOfArrayInnerAsJsonPatchOperationValue(&arr)
	if state.IsNull() || state.IsUnknown() {
		return []entitlements.JsonPatchOperation{entitlementJSONPatchAdd(path, value)}, diags
	}
	return []entitlements.JsonPatchOperation{entitlementJSONPatchReplace(path, value)}, diags
}

func entitlementOwnerPatchOps(plan, state resource_entitlement.OwnerValue) ([]entitlements.JsonPatchOperation, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.IsUnknown() || entitlementResourceOwnersEqual(plan, state) {
		return nil, diags
	}
	if plan.IsNull() {
		if state.IsNull() || state.IsUnknown() {
			return nil, diags
		}
		return []entitlements.JsonPatchOperation{entitlementJSONPatchRemove("/owner")}, diags
	}
	ownerMap, d := entitlementResourceOwnerToPatchMap(plan)
	diags.Append(d...)
	if diags.HasError() || ownerMap == nil {
		return nil, diags
	}
	value := entitlements.MapmapOfStringAnyAsJsonPatchOperationValue(&ownerMap)
	if state.IsNull() || state.IsUnknown() {
		return []entitlements.JsonPatchOperation{entitlementJSONPatchAdd("/owner", value)}, diags
	}
	return []entitlements.JsonPatchOperation{entitlementJSONPatchReplace("/owner", value)}, diags
}

func entitlementResourceOwnersEqual(a, b resource_entitlement.OwnerValue) bool {
	if a.IsNull() || b.IsNull() {
		return a.IsNull() && b.IsNull()
	}
	if a.IsUnknown() || b.IsUnknown() {
		return false
	}
	return a.Id.Equal(b.Id) && a.OwnerType.Equal(b.OwnerType)
}

func entitlementResourceOwnerToPatchMap(v resource_entitlement.OwnerValue) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	if v.Id.IsNull() || v.Id.IsUnknown() || v.OwnerType.IsNull() || v.OwnerType.IsUnknown() {
		diags.AddError("Invalid owner value", "owner.id and owner.type must be known before the entitlement can be patched.")
		return nil, diags
	}
	return map[string]interface{}{
		"id":   v.Id.ValueString(),
		"type": v.OwnerType.ValueString(),
	}, diags
}

func entitlementResourceDtoToModel(ctx context.Context, dto *entitlements.EntitlementV2, fallback entitlementResourceModel) (entitlementResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	} else {
		model.Id = types.StringNull()
	}
	model.Name = types.StringPointerValue(dto.Name)
	model.Attribute = types.StringPointerValue(dto.Attribute)
	model.Description = types.StringPointerValue(dto.Description.Get())
	model.Value = types.StringPointerValue(dto.Value)
	model.SourceSchemaObjectType = types.StringPointerValue(dto.SourceSchemaObjectType)
	model.CloudGoverned = types.BoolPointerValue(dto.CloudGoverned)
	model.Requestable = types.BoolPointerValue(dto.Requestable)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	v, d := entitlementResourceAccessModelMetadataFromAPI(ctx, dto.AccessModelMetadata)
	diags.Append(d...)
	model.AccessModelMetadata = v

	directPermissions, d := entitlementResourceDirectPermissionsFromAPI(ctx, dto.DirectPermissions)
	diags.Append(d...)
	model.DirectPermissions = directPermissions

	owner, d := entitlementResourceOwnerFromAPI(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	privilegeLevel, d := entitlementResourcePrivilegeLevelFromAPI(ctx, dto.PrivilegeLevel)
	diags.Append(d...)
	model.PrivilegeLevel = privilegeLevel

	segments, d := stringListValueFromAPI(ctx, dto.Segments)
	diags.Append(d...)
	model.Segments = segments

	source, d := entitlementResourceSourceFromAPI(ctx, dto.Source)
	diags.Append(d...)
	model.Source = source

	tags, d := entitlementTagsFromAPI(ctx, dto.Tags)
	diags.Append(d...)
	model.Tags = tags

	attributes, d := normalizedJSONFromMap(dto.Attributes)
	diags.Append(d...)
	model.Attributes = attributes

	manuallyUpdatedFields, d := entitlementManuallyUpdatedFieldsFromAPI(dto.ManuallyUpdatedFields)
	diags.Append(d...)
	model.ManuallyUpdatedFields = manuallyUpdatedFields

	return model, diags
}

func entitlementResourceAccessModelMetadataFromAPI(ctx context.Context, dto *entitlements.EntitlementV2AccessModelMetadata) (resource_entitlement.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return resource_entitlement.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := entitlementResourceAttributeDTOListFromAPI(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := resource_entitlement.NewAccessModelMetadataValue(
		resource_entitlement.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func entitlementResourceAttributeDTOListFromAPI(ctx context.Context, items []entitlements.AccessModelMetadata) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_entitlement.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_entitlement.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := entitlementResourceAttributeValueDTOListFromAPI(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := resource_entitlement.NewAttributesValue(
			resource_entitlement.AttributesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"description":  types.StringPointerValue(item.Description),
				"key":          types.StringPointerValue(item.Key),
				"multiselect":  types.BoolPointerValue(item.Multiselect),
				"name":         types.StringPointerValue(item.Name),
				"object_types": objectTypes,
				"status":       types.StringPointerValue(item.Status),
				"type":         types.StringPointerValue(item.Type),
				"values":       valuesList,
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementResourceAttributeValueDTOListFromAPI(ctx context.Context, items []entitlements.AccessModelMetadataValuesInner) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_entitlement.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_entitlement.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := resource_entitlement.NewValuesValue(
			resource_entitlement.ValuesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"name":   types.StringPointerValue(item.Name),
				"status": types.StringPointerValue(item.Status),
				"value":  types.StringPointerValue(item.Value),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementResourceDirectPermissionsFromAPI(ctx context.Context, items []entitlements.PermissionDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_entitlement.DirectPermissionsValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_entitlement.DirectPermissionsValue, 0, len(items))
	for _, item := range items {
		rights, d := types.ListValueFrom(ctx, types.StringType, item.Rights)
		diags.Append(d...)
		v, d := resource_entitlement.NewDirectPermissionsValue(
			resource_entitlement.DirectPermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"rights": rights,
				"target": types.StringPointerValue(item.Target),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementResourceOwnerFromAPI(ctx context.Context, dto *entitlements.EntitlementV2Owner) (resource_entitlement.OwnerValue, diag.Diagnostics) {
	if dto == nil {
		return resource_entitlement.NewOwnerValueNull(), nil
	}
	return resource_entitlement.NewOwnerValue(
		resource_entitlement.OwnerValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(dto.Id),
			"name": types.StringPointerValue(dto.Name),
			"type": types.StringPointerValue(dto.Type),
		},
	)
}

func entitlementResourceSourceFromAPI(ctx context.Context, dto *entitlements.EntitlementV2Source) (resource_entitlement.SourceValue, diag.Diagnostics) {
	if dto == nil {
		return resource_entitlement.NewSourceValueNull(), nil
	}
	return resource_entitlement.NewSourceValue(
		resource_entitlement.SourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(dto.Id),
			"name": types.StringPointerValue(dto.Name),
			"type": types.StringPointerValue(dto.Type),
		},
	)
}

func entitlementResourcePrivilegeLevelFromAPI(ctx context.Context, pl *entitlements.EntitlementV2PrivilegeLevel) (resource_entitlement.PrivilegeLevelValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if pl == nil {
		return resource_entitlement.NewPrivilegeLevelValueNull(), diags
	}
	v, d := resource_entitlement.NewPrivilegeLevelValue(
		resource_entitlement.PrivilegeLevelValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"direct":      types.StringPointerValue(pl.Direct),
			"effective":   types.StringPointerValue(pl.Effective),
			"inherited":   types.StringPointerValue(pl.Inherited.Get()),
			"set_by":      types.StringPointerValue(pl.SetBy),
			"set_by_type": types.StringPointerValue(pl.SetByType.Get()),
		},
	)
	diags.Append(d...)
	return v, diags
}

func entitlementManuallyUpdatedFieldsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"display_name": types.BoolType,
		"description":  types.BoolType,
	}
}

func entitlementManuallyUpdatedFieldsFromAPI(m map[string]interface{}) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := entitlementManuallyUpdatedFieldsAttrTypes()
	if m == nil {
		return types.ObjectNull(attrTypes), diags
	}
	// golang-sdk v3's EntitlementV2.ManuallyUpdatedFields is an untyped
	// map[string]interface{} (v2 exposed a typed EntitlementManuallyUpdatedFields
	// struct), so extract the two known boolean keys defensively.
	boolPtr := func(k string) *bool {
		if v, ok := m[k].(bool); ok {
			return &v
		}
		return nil
	}
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"display_name": types.BoolPointerValue(boolPtr("DISPLAY_NAME")),
		"description":  types.BoolPointerValue(boolPtr("DESCRIPTION")),
	})
	diags.Append(d...)
	return obj, diags
}

func entitlementStringListToArrayInner(ctx context.Context, v types.List) ([]entitlements.ArrayInner, diag.Diagnostics) {
	var values []string
	if v.IsNull() || v.IsUnknown() {
		return nil, nil
	}
	diags := v.ElementsAs(ctx, &values, false)
	arr := make([]entitlements.ArrayInner, 0, len(values))
	for i := range values {
		value := values[i]
		arr = append(arr, entitlements.ArrayInner{String: &value})
	}
	return arr, diags
}

func stringListValueFromAPI(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

func entitlementTagsFromAPI(ctx context.Context, tags []string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if tags == nil {
		return types.ListNull(types.StringType), diags
	}
	listVal, d := types.ListValueFrom(ctx, types.StringType, tags)
	diags.Append(d...)
	return listVal, diags
}

func additionalPropertiesObject(additional map[string]interface{}, key string) (map[string]interface{}, bool, error) {
	raw, ok := additional[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func stringAttrValue(m map[string]interface{}, key string) types.String {
	v, ok := m[key]
	if !ok || v == nil {
		return types.StringNull()
	}
	s, ok := v.(string)
	if !ok {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func entitlementErrDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func entitlementJSONPatchReplace(path string, value entitlements.JsonPatchOperationValue) entitlements.JsonPatchOperation {
	return entitlements.JsonPatchOperation{Op: "replace", Path: path, Value: &value}
}

func entitlementJSONPatchAdd(path string, value entitlements.JsonPatchOperationValue) entitlements.JsonPatchOperation {
	return entitlements.JsonPatchOperation{Op: "add", Path: path, Value: &value}
}

func entitlementJSONPatchRemove(path string) entitlements.JsonPatchOperation {
	return entitlements.JsonPatchOperation{Op: "remove", Path: path}
}

func normalizedJSONFromMap(v map[string]interface{}) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v == nil {
		return jsontypes.NewNormalizedNull(), diags
	}
	b, err := json.Marshal(v)
	if err != nil {
		diags.AddError("Error encoding JSON attribute from API response", err.Error())
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

func timeToStringValue(t *entitlements.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
