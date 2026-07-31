// Package identity_profile_v1 is a pilot implementation of the Identity
// Profiles resource/data sources generated from SailPoint's per-service v1
// OpenAPI spec (api-specs/idn/apis/identity-profiles), following the same
// hand-written CRUD pattern established by
// governance_group_v1/role_v1/service_desk_integration_v1/sources_v1.
//
// Identity vs. Identity Profile scope decision: SailPoint's `/identities` API
// is GET-only (list+get) - identities are onboarded exclusively via
// authoritative-source aggregation and can never be created/updated/deleted
// directly via API, so "identity" can only ever be a read-only data source
// (deferred, tracked as a follow-up: identities_v1). `/identity-profiles` is
// the real Terraform-manageable governance object: it binds an authoritative
// source, defines the identity attribute mapping/transforms, and assigns
// lifecycle states - this is the resource implemented here. Lifecycle States
// themselves live in a distinct `lifecycle-states` sub-resource service and
// are deliberately deferred, mirroring governance_group_v1's
// members/connections precedent.
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in
// resource_identity_profile/datasource_identity_profile, backed by the
// golang-sdk v2 api_beta.IdentityProfilesAPIService client. Update() calls
// api_beta.IdentityProfilesAPIService.UpdateIdentityProfile, which is PATCH
// /identity-profiles/v1/{id} (RFC 6902 JSON Patch) using the exact same
// api_beta.JsonPatchOperation / UpdateMultiHostSourcesRequestInnerValue
// wrapper type as sources_v1 (a shared api_beta model, reused across
// unrelated JSON-Patch-based endpoints) - the jsonPatchReplace/
// optionalStringPatch/optionalInt32Patch/structToMap helper pattern is
// copied near-verbatim from sources_v1's resource_source.go (each _v1
// package keeps its own copy; there is no shared helper package by
// convention).
//
// Codegen notes (see generator_config_identity_profile_v1.yml for the full
// detail):
//   - The bundled api-specs/dereferenced/deref-identity-profiles.v1.yaml had
//     7 whole-response-level `allOf` occurrences (each response schema
//     merging a shared `basecommondto` base with its own properties) that
//     tfplugingen-openapi cannot decompose. Flattened via the checked-in
//     scripts/flatten_openapi_allof.py (promoted this segment from the
//     ad hoc per-target script used for transform/governance_group/
//     sources_v1/identity_profile_v1 - see that script's module docstring).
//   - The path parameter is named "{identity-profile-id}", NOT the "{id}"
//     name tfplugingen-openapi expects to auto-correlate with the response
//     body's own "id" property. This has TWO distinct consequences, both
//     patched around here rather than in the generator config:
//     1. A spurious extra "identityprofileid" attribute is synthesized from
//     the path parameter - removed via
//     generator_config/schema_overrides_identity_profile_v1.yml's new
//     `drop_attributes` override (see
//     scripts/apply_codespec_schema_overrides.py).
//     2. The singular identitynow_identity_profile_v1 data source's
//     generated "id" attribute is NOT auto-marked Required (compare
//     sources_v1/role_v1/governance_group_v1, whose "{id}"-named path
//     parameters DO get this treatment for free) - hand-patched to
//     Required in datasource_identity_profile_planmodifiers.go.
//   - "identityAttributeConfig" is a dynamic/discriminated-union blob
//     (attributeTransforms[].transformDefinition.attributes' shape depends
//     entirely on the sibling transformDefinition.type) - same pattern as
//     transform_v1's "attributes"/sources_v1's "connectorAttributes". Hand-
//     added as a jsontypes.Normalized JSON-string CustomType wrapping the
//     *entire* identityAttributeConfig object (enabled + attributeTransforms
//     together), since the object as a whole - not just one nested field -
//     is what generator_config told tfplugingen-openapi to ignore.
//   - owner/authoritative_source/identity_exception_report_reference are
//     associated_external_type-mapped directly onto their SDK structs
//     (IdentityProfileAllOfOwner/IdentityProfileAllOfAuthoritativeSource/
//     IdentityExceptionReportReference) via
//     generator_config/type_mappings_identity_profile_v1.yml - all three are
//     clean, leaf, all-*string-field structs, safe to map (see that file's
//     inline notes).
//
// Known API constraint (enforced here, not just documented): "Authoritative
// Source and Identity Attribute Configuration cannot be modified at once" -
// UpdateIdentityProfile's own SDK doc comment states this explicitly. Update()
// returns an error diagnostic (rather than silently sending both in one PATCH
// and getting a confusing 400 back) if a practitioner's plan changes both in
// the same apply.
//
// Known async-delete behavior (first occurrence in this repo): unlike every
// other _v1 pilot's Delete() (a synchronous 204, or a fire-and-forget 202
// whose background job this repo doesn't wait on, e.g. sources_v1),
// DeleteIdentityProfile's 202 response includes a *TaskResultSimplified*
// whose "id" can be polled via the generic
// api_beta.TaskManagementAPIService.GetTaskStatus(ctx, id) endpoint for real
// completion status (TaskStatus.Completed/CompletionStatus) - unlike a
// bare "fire and forget" 202, actually reflecting delete failure back to
// Terraform matters here because a failed background bulk-delete job (e.g.
// still-attached identities) should surface as an apply-time error rather
// than silently leaving practitioners with a resource Terraform believes is
// gone. Delete() polls this task status with bounded retries/backoff before
// returning.
//
// Deliberately deferred (out of scope for this pilot): lifecycle-states
// (own sub-resource service, mirrors governance_group_v1's
// members/connections precedent), identity-preview
// (ShowGenerateIdentityPreview), export/import (ExportIdentityProfiles/
// ImportIdentityProfiles), process-identities/sync
// (SyncIdentityProfile - triggers an on-demand refresh, not a CRUD
// operation), the bulk DeleteIdentityProfiles endpoint, and
// GetDefaultIdentityAttributeConfig (a template-fetch helper, not part of
// this resource's own lifecycle).
package identity_profile_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/identity_profile_v1/resource_identity_profile"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*identityProfileResource)(nil)
	_ resource.ResourceWithConfigure   = (*identityProfileResource)(nil)
	_ resource.ResourceWithImportState = (*identityProfileResource)(nil)
)

func NewIdentityProfileResource() resource.Resource {
	return &identityProfileResource{}
}

type identityProfileResource struct {
	client *sailpoint.APIClient
}

// identityProfileResourceModel mirrors
// resource_identity_profile.IdentityProfileModel plus the hand-added
// "identity_attribute_config" field the generator was told to ignore (see
// package doc). Kept as a distinct, hand-written struct (rather than
// embedding the generated model) since Go doesn't allow adding a field to an
// imported struct type, and req.Plan.Get/resp.State.Set match purely on
// `tfsdk` tags, not on which struct type declares them.
type identityProfileResourceModel struct {
	AuthoritativeSource              resource_identity_profile.AuthoritativeSourceValue              `tfsdk:"authoritative_source"`
	Created                          types.String                                                    `tfsdk:"created"`
	Description                      types.String                                                    `tfsdk:"description"`
	HasTimeBasedAttr                 types.Bool                                                      `tfsdk:"has_time_based_attr"`
	Id                               types.String                                                    `tfsdk:"id"`
	IdentityAttributeConfig          jsontypes.Normalized                                            `tfsdk:"identity_attribute_config"`
	IdentityCount                    types.Int64                                                     `tfsdk:"identity_count"`
	IdentityExceptionReportReference resource_identity_profile.IdentityExceptionReportReferenceValue `tfsdk:"identity_exception_report_reference"`
	IdentityRefreshRequired          types.Bool                                                      `tfsdk:"identity_refresh_required"`
	Modified                         types.String                                                    `tfsdk:"modified"`
	Name                             types.String                                                    `tfsdk:"name"`
	Owner                            resource_identity_profile.OwnerValue                            `tfsdk:"owner"`
	Priority                         types.Int64                                                     `tfsdk:"priority"`
}

func (r *identityProfileResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_profile_v1"
}

func (r *identityProfileResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_identity_profile.IdentityProfileResourceSchema(ctx)
	resp.Schema.Description = "Manages an Identity Profile in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages an [Identity Profile](https://documentation.sailpoint.com/saas/help/setup/identity_profiles.html) " +
		"in IdentityNow/ISC - the governance object that binds an authoritative source, defines the identity attribute " +
		"mapping, and assigns lifecycle states for the identities it aggregates.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before " +
		"relying on it in production configurations. Lifecycle states, identity preview, export/import, on-demand " +
		"sync, and several other sub-resource endpoints are deliberately deferred (see the package doc)."
	applyIdentityAttributeConfigField(&resp.Schema.Attributes, false)
	applyIdentityProfileUseStateForUnknown(&resp.Schema)
}

func (r *identityProfileResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *identityProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *identityProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan identityProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Identity Profile", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.IdentityProfilesAPI.
		CreateIdentityProfile(ctx).
		IdentityProfile(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Identity Profile", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Identity Profile", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *identityProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state identityProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Identity Profile", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.Beta.IdentityProfilesAPI.
		GetIdentityProfile(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Identity Profile not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Identity Profile", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Identity Profile", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *identityProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan identityProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state identityProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Identity Profile", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Compare only "id"/"type" (not the whole AuthoritativeSourceValue via
	// .Equal()) - "name" is Computed-only (server-populated), so it's
	// Unknown on every plan unless UseStateForUnknown is applied to it (not
	// currently done for nested object sub-attributes in this pipeline -
	// see the package doc's tfplugingen-framework PlanModifiers gap note).
	// A whole-struct .Equal() comparison would spuriously report a "change"
	// on every Update just because "name" is Unknown-vs-known, even when
	// the practitioner only edited an unrelated attribute like "priority" -
	// confirmed live.
	authoritativeSourceChanged := plan.AuthoritativeSource.Id.ValueString() != state.AuthoritativeSource.Id.ValueString() ||
		plan.AuthoritativeSource.AuthoritativeSourceType.ValueString() != state.AuthoritativeSource.AuthoritativeSourceType.ValueString()
	identityAttributeConfigChanged := plan.IdentityAttributeConfig.ValueString() != state.IdentityAttributeConfig.ValueString()
	if authoritativeSourceChanged && identityAttributeConfigChanged {
		resp.Diagnostics.AddError(
			"Cannot update \"authoritative_source\" and \"identity_attribute_config\" together",
			"IdentityNow's Identity Profile PATCH API does not allow the Authoritative Source and Identity Attribute "+
				"Configuration to be modified in the same request. Split this change across two separate "+
				"`terraform apply` runs (update one, apply, then update the other).",
		)
		return
	}

	// updateIdentityProfileV1 (PATCH) explicitly documents these fields as
	// immutable: id, created, modified, identityCount,
	// identityRefreshRequired. Authoritative Source and Identity Attribute
	// Configuration cannot both change in a single PATCH (enforced above).
	// Every other field is patched here unconditionally as a "replace" op
	// (same simple, unconditional-replace convention as
	// governance_group_v1/role_v1/sources_v1).
	name := dto.Name.Get()
	patch := []api_beta.JsonPatchOperation{
		jsonPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(name)),
	}
	patch = append(patch, optionalStringPatch("/description", dto.Description.Get())...)
	patch = append(patch, optionalInt64Patch("/priority", dto.Priority)...)

	if m, err := structToMap(dto.Owner.Get()); err == nil && m != nil {
		patch = append(patch, jsonPatchReplace("/owner", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
	}

	if authoritativeSourceChanged {
		if m, err := structToMap(dto.AuthoritativeSource); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/authoritativeSource", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if identityAttributeConfigChanged && dto.IdentityAttributeConfig != nil {
		if m, err := structToMap(dto.IdentityAttributeConfig); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/identityAttributeConfig", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}

	tflog.Debug(ctx, "Patching Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.Beta.IdentityProfilesAPI.
		UpdateIdentityProfile(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Identity Profile", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Identity Profile", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// identityProfileDeletePollAttempts/-Interval bound how long Delete() waits
// for the background bulk-delete task (see the package doc's "Known
// async-delete behavior" note) to finish before giving up and returning
// anyway (with a warning, not a hard error - Terraform will already have
// dropped the resource from state at that point, matching every other _v1
// pilot's "don't block apply forever" convention for eventual-consistency
// waits, e.g. the sources_v1 acceptance test's CheckDestroy retry).
const (
	identityProfileDeletePollAttempts = 15
	identityProfileDeletePollInterval = 2 * time.Second
)

func (r *identityProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state identityProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Identity Profile", map[string]interface{}{"id": state.Id.ValueString()})

	taskResult, httpResp, err := r.client.Beta.IdentityProfilesAPI.
		DeleteIdentityProfile(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Identity Profile already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Identity Profile", errDetail(err, httpResp))
		return
	}

	if taskResult == nil || taskResult.Id == nil {
		tflog.Warn(ctx, "Delete Identity Profile returned no task id to poll; assuming success", map[string]interface{}{"id": state.Id.ValueString()})
		return
	}

	taskId := *taskResult.Id
	for attempt := 0; attempt < identityProfileDeletePollAttempts; attempt++ {
		status, _, statusErr := r.client.Beta.TaskManagementAPI.GetTaskStatus(ctx, taskId).Execute()
		if statusErr != nil {
			// The task status endpoint itself 404ing/erroring isn't
			// necessarily fatal - the delete may have already fully
			// completed and the task record pruned. Stop polling and treat
			// as success rather than failing the whole apply on a
			// secondary read.
			tflog.Warn(ctx, "Could not fetch Identity Profile delete task status; assuming success", map[string]interface{}{"id": state.Id.ValueString(), "task_id": taskId, "error": statusErr.Error()})
			return
		}
		if status != nil && status.Completed.IsSet() && status.Completed.Get() != nil {
			completionStatus := status.CompletionStatus.Get()
			if completionStatus != nil && (*completionStatus == "Error" || *completionStatus == "TerminatedWithErrors" || *completionStatus == "Cancelled") {
				resp.Diagnostics.AddError(
					"Error deleting Identity Profile",
					fmt.Sprintf("Background delete task %q for Identity Profile %q finished with completion status %q.", taskId, state.Id.ValueString(), *completionStatus),
				)
			}
			tflog.Info(ctx, "Deleted Identity Profile", map[string]interface{}{"id": state.Id.ValueString(), "task_id": taskId})
			return
		}
		time.Sleep(identityProfileDeletePollInterval)
	}

	tflog.Warn(ctx, "Timed out waiting for Identity Profile delete task to complete; it may still be processing in the background", map[string]interface{}{"id": state.Id.ValueString(), "task_id": taskId})
}

// modelToDto converts the Terraform plan/config model into the SDK
// create/update DTO shape. Only fields this resource actually manages are
// set - server-computed-only fields (id, created, modified, identity_count,
// has_time_based_attr) are left at their zero value and always re-populated
// from the live API response by dtoToModel instead.
func modelToDto(ctx context.Context, m identityProfileResourceModel) (*api_beta.IdentityProfile, diag.Diagnostics) {
	var diags diag.Diagnostics

	authoritativeSource, d := m.AuthoritativeSource.ToApi_betaIdentityProfileAllOfAuthoritativeSource(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	dto := api_beta.NewIdentityProfileWithDefaults()
	dto.Name = *api_beta.NewNullableString(m.Name.ValueStringPointer())
	if authoritativeSource != nil {
		dto.AuthoritativeSource = *authoritativeSource
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		v := m.Description.ValueString()
		dto.Description = *api_beta.NewNullableString(&v)
	}
	if !m.Priority.IsNull() && !m.Priority.IsUnknown() {
		v := m.Priority.ValueInt64()
		dto.Priority = &v
	}

	owner, d := m.Owner.ToApi_betaIdentityProfileAllOfOwner(ctx)
	diags.Append(d...)
	if owner != nil {
		dto.Owner = *api_beta.NewNullableIdentityProfileAllOfOwner(owner)
	}

	cfg, d := identityAttributeConfigToApi(m.IdentityAttributeConfig)
	diags.Append(d...)
	if cfg != nil {
		dto.IdentityAttributeConfig = cfg
	}

	return dto, diags
}

// dtoToModel converts an API response DTO into the Terraform state model.
// "fallback" supplies any attribute this converter doesn't itself populate,
// matching the established governance_group_v1/role_v1/sources_v1
// convention of always taking a fallback model.
func dtoToModel(ctx context.Context, dto *api_beta.IdentityProfile, fallback identityProfileResourceModel) (identityProfileResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringPointerValue(dto.Name.Get())
	model.Description = types.StringPointerValue(dto.Description.Get())
	if dto.Priority != nil {
		model.Priority = types.Int64Value(*dto.Priority)
	} else {
		model.Priority = types.Int64Null()
	}
	model.IdentityRefreshRequired = types.BoolPointerValue(dto.IdentityRefreshRequired)
	model.HasTimeBasedAttr = types.BoolPointerValue(dto.HasTimeBasedAttr)
	if dto.IdentityCount != nil {
		model.IdentityCount = types.Int64Value(int64(*dto.IdentityCount))
	} else {
		model.IdentityCount = types.Int64Null()
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	authoritativeSource, d := resource_identity_profile.AuthoritativeSourceValue{}.FromApi_betaIdentityProfileAllOfAuthoritativeSource(ctx, &dto.AuthoritativeSource)
	diags.Append(d...)
	model.AuthoritativeSource = authoritativeSource

	owner, d := resource_identity_profile.OwnerValue{}.FromApi_betaIdentityProfileAllOfOwner(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	exceptionRef, d := resource_identity_profile.IdentityExceptionReportReferenceValue{}.FromApi_betaIdentityExceptionReportReference(ctx, dto.IdentityExceptionReportReference.Get())
	diags.Append(d...)
	model.IdentityExceptionReportReference = exceptionRef

	// The live API auto-populates identity_attribute_config with a default
	// mapping (e.g. derived from the authoritative source's schema) even
	// when the practitioner never configures this Optional+Computed
	// attribute at all, and mutates/enriches it independently of what was
	// last configured. Echoing the live API value back into state
	// unconditionally causes a "Provider produced inconsistent result
	// after apply" error whenever the practitioner *did* configure a known
	// value (their configured subset can never equal the enriched value),
	// and also causes this attribute to spuriously show as "changed" on
	// every subsequent Update to unrelated attributes (confirmed live -
	// this previously misfired the authoritative_source/
	// identity_attribute_config mutual-exclusivity guard below). Mirrors
	// the same fallback-preservation pattern used by sources_v1's
	// connector_attributes: only fall back to the live API's value when
	// nothing was configured (e.g. after `terraform import`).
	if fallback.IdentityAttributeConfig.IsNull() || fallback.IdentityAttributeConfig.IsUnknown() {
		cfg, d := identityAttributeConfigFromAPI(dto.IdentityAttributeConfig)
		diags.Append(d...)
		model.IdentityAttributeConfig = cfg
	}

	return model, diags
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (adopted
// from every other _v1 pilot) so this target surfaces the same richer detail
// (HTTP status, detailCode, trackingId, and message text) in
// resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

func jsonPatchReplace(path string, value api_beta.UpdateMultiHostSourcesRequestInnerValue) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

func optionalStringPatch(path string, v *string) []api_beta.JsonPatchOperation {
	if v == nil {
		return nil
	}
	return []api_beta.JsonPatchOperation{jsonPatchReplace(path, api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(v))}
}

// optionalInt64Patch converts an *int64 (IdentityProfile.Priority's Go type)
// to an api_beta.JsonPatchOperation via
// Int32AsUpdateMultiHostSourcesRequestInnerValue - the shared
// UpdateMultiHostSourcesRequestInnerValue wrapper type only has an int32
// constructor (it originates from sources_v1's PATCH schema, which has no
// int64 fields); priority values fit comfortably within int32 range.
func optionalInt64Patch(path string, v *int64) []api_beta.JsonPatchOperation {
	if v == nil {
		return nil
	}
	v32 := int32(*v)
	return []api_beta.JsonPatchOperation{jsonPatchReplace(path, api_beta.Int32AsUpdateMultiHostSourcesRequestInnerValue(&v32))}
}

// structToMap round-trips an SDK model struct through JSON to get a
// map[string]interface{} suitable for
// api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue, since
// the JSON Patch value wrapper type doesn't accept typed structs directly
// (see sources_v1/resource_source.go's identical helper).
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

// identityAttributeConfigToApi decodes the practitioner-supplied
// "identity_attribute_config" JSON string into an
// *api_beta.IdentityAttributeConfig. A null/unknown/empty jsontypes.Normalized
// decodes to nil (identityAttributeConfig is Optional, not Required).
func identityAttributeConfigToApi(v jsontypes.Normalized) (*api_beta.IdentityAttributeConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, diags
	}
	var cfg api_beta.IdentityAttributeConfig
	if err := json.Unmarshal([]byte(v.ValueString()), &cfg); err != nil {
		diags.AddError(
			"Invalid \"identity_attribute_config\" JSON",
			fmt.Sprintf("Could not decode \"identity_attribute_config\" as a JSON object: %s", err.Error()),
		)
		return nil, diags
	}
	return &cfg, diags
}

// identityAttributeConfigFromAPI re-encodes an API-returned
// IdentityAttributeConfig as a jsontypes.Normalized JSON string. A nil value
// (identityAttributeConfig omitted entirely from the response) becomes a
// null jsontypes.Normalized rather than "{}".
func identityAttributeConfigFromAPI(cfg *api_beta.IdentityAttributeConfig) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cfg == nil {
		return jsontypes.NewNormalizedNull(), diags
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		diags.AddError(
			"Error encoding \"identity_attribute_config\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"identityAttributeConfig\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

// timeToStringValue formats an *api_beta.SailPointTime as RFC3339, or
// returns a null types.String if nil.
func timeToStringValue(t *api_beta.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
