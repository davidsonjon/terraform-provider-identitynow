// Package source_provisioning_policy_v1 is a pilot implementation of the
// Source Provisioning Policy resource/data source generated from SailPoint's
// per-service v1 OpenAPI spec (api-specs/idn/apis/sources), covering the
// `/sources/v1/{sourceId}/provisioning-policies` family of endpoints - a
// sub-resource of the existing sources_v1.SourceResource
// (identitynow_source_v1) that resource's own "Known Limitations" section
// previously deferred to "a future, separate resource/data source".
//
// v1 vs. v2 shape decision
// ------------------------
// SailPoint's spec ships TWO generations of this API side by side:
//   - v1 (/sources/v1/{sourceId}/provisioning-policies[/{usageType}]):
//     identifies a provisioning policy by its (sourceId, usageType) composite
//     key. A source can only have one provisioning policy per usageType, so
//     this is a real, if unusual, natural key - not an arbitrary choice.
//   - v2 (/sources/v2/{sourceId}/provisioning-policies[/{id}]): identifies a
//     provisioning policy by a real server-generated `id`, explicitly
//     documented as letting practitioners have >1 policy per usageType
//     ("The V2 API allows you to use a unique identifier (id) for each
//     provisioning policy instead of usageType.").
//
// v2's by-id shape is the objectively better fit for a normal Terraform
// resource (a stable, single-attribute identifier, matching every other
// _v1 pilot in this repo) and was the task's stated preference - but
// github.com/sailpoint-oss/golang-sdk/v2@v2.7.106 (api_beta.SourcesAPIService)
// has NO generated bindings at all for the v2 endpoints (confirmed via
// `grep -n "ProvisioningPolic" api_sources.go`: only
// Create/Get/Put/Update/Delete/ListProvisioningPolicies, all v1); v2 also
// requires a mandatory `X-SailPoint-Experimental: true` header the SDK has no
// generated support for attaching. Since this pipeline's hand-written CRUD is
// built against the published golang-sdk client (see the IdentityNow agent's
// Non-Goals: never invent SDK behavior), v1 is used instead - the resource's
// Terraform-facing "id" is a synthesized `sourceId/usageType` composite (see
// idFromParts/idToParts below), analogous to segment_access_v1's/
// application_access_association_v1's composite-key patterns. Revisit this
// decision (and migrate to a real by-id resource) once/if the SDK publishes
// v2 bindings.
//
// "fields" dynamic-shape decision
// --------------------------------
// Each ProvisioningPolicyDto's "fields" is an array of FieldDetailsDto, and
// every element has its own "transform" (a discriminated union keyed by a
// sibling "type" - structurally identical to transform_v1's top-level
// "attributes") AND its own free-form "attributes" object (the transform's
// parameters). tfplugingen-openapi/-framework cannot generate a faithful
// static schema for this, so the entire "fields" array (not just the
// "transform"/"attributes" sub-fields) is hand-added as a single
// jsontypes.Normalized JSON-string CustomType - the same established pattern
// as transform_v1's "attributes" and source_v1's "connectorAttributes" (see
// the IdentityNow agent's Project Context "Dynamic/discriminated-union
// attributes-style fields" bullet), just applied to a whole array instead of
// a single object.
package source_provisioning_policy_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/source_provisioning_policy_v1/resource_source_provisioning_policy"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*sourceProvisioningPolicyResource)(nil)
	_ resource.ResourceWithConfigure   = (*sourceProvisioningPolicyResource)(nil)
	_ resource.ResourceWithImportState = (*sourceProvisioningPolicyResource)(nil)
)

func NewSourceProvisioningPolicyResource() resource.Resource {
	return &sourceProvisioningPolicyResource{}
}

type sourceProvisioningPolicyResource struct {
	client *sailpoint.APIClient
}

// sourceProvisioningPolicyResourceModel mirrors
// resource_source_provisioning_policy.SourceProvisioningPolicyModel plus the
// hand-added "id" (synthesized composite key) and "fields" fields - kept as a
// distinct, hand-written struct since Go doesn't allow adding fields to an
// imported generated struct type (see transform_v1's identical rationale).
type sourceProvisioningPolicyResourceModel struct {
	Id          types.String         `tfsdk:"id"`
	SourceId    types.String         `tfsdk:"source_id"`
	UsageType   types.String         `tfsdk:"usage_type"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Fields      jsontypes.Normalized `tfsdk:"fields"`
}

func (r *sourceProvisioningPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_provisioning_policy_v1"
}

func (r *sourceProvisioningPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_source_provisioning_policy.SourceProvisioningPolicyResourceSchema(ctx)
	resp.Schema.Description = "Manages a Provisioning Policy on an existing Source in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Provisioning Policy](https://developer.sailpoint.com/docs/extensibility/transforms/guides/transforms-in-provisioning-policies) " +
		"on an existing [Source](https://documentation.sailpoint.com/saas/help/sources/index.html) in IdentityNow/ISC. " +
		"A provisioning policy is a create/update/enable/disable/etc. template (keyed by `usage_type`) used when " +
		"provisioning accounts on the source.\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below before relying on it in " +
		"production configurations. In particular, it uses SailPoint's v1 provisioning-policies API (keyed by " +
		"`source_id`+`usage_type`), not the newer v2 by-id API, because the published golang-sdk has no v2 bindings yet - " +
		"see this package's doc comment for the full rationale."
	applySourceProvisioningPolicyFieldsField(&resp.Schema.Attributes, true)
	applySourceProvisioningPolicyPlanModifiers(&resp.Schema)
}

func (r *sourceProvisioningPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sourceProvisioningPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	sourceID, usageType, err := idToParts(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_id"), sourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("usage_type"), usageType)...)
}

func (r *sourceProvisioningPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sourceProvisioningPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := plan.SourceId.ValueString()
	tflog.Debug(ctx, "Creating Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": plan.UsageType.ValueString()})

	dto, diags := modelToDto(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		CreateProvisioningPolicy(ctx, sourceID).
		ProvisioningPolicyDto(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Source Provisioning Policy", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(apiResp, sourceID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Source Provisioning Policy", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sourceProvisioningPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sourceProvisioningPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	usageType := state.UsageType.ValueString()
	tflog.Debug(ctx, "Reading Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		GetProvisioningPolicy(ctx, sourceID, api_beta.UsageType(usageType)).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source Provisioning Policy not found, removing from state", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType, "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source Provisioning Policy", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(apiResp, sourceID, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceProvisioningPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sourceProvisioningPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sourceProvisioningPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	usageType := state.UsageType.ValueString()
	tflog.Debug(ctx, "Updating Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})

	// source_id/usage_type are RequiresReplace (see
	// resource_source_provisioning_policy_planmodifiers.go), so Update is
	// only ever reached for changes to name/description/fields. A full PUT
	// (putProvisioningPolicyV1) replacing the whole document is simplest and
	// correct here (mirrors transform_v1's Update, which also full-replaces
	// rather than JSON-Patching, even though a PATCH endpoint exists).
	dto, diags := modelToDto(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		PutProvisioningPolicy(ctx, sourceID, api_beta.UsageType(usageType)).
		ProvisioningPolicyDto(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType, "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Source Provisioning Policy", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(apiResp, sourceID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Source Provisioning Policy", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceProvisioningPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sourceProvisioningPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	usageType := state.UsageType.ValueString()
	tflog.Debug(ctx, "Deleting Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})

	httpResp, err := r.client.Beta.SourcesAPI.
		DeleteProvisioningPolicy(ctx, sourceID, api_beta.UsageType(usageType)).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source Provisioning Policy already absent on delete", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})
			return
		}
		tflog.Error(ctx, "Error deleting Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType, "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Source Provisioning Policy", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Source Provisioning Policy", map[string]interface{}{"source_id": sourceID, "usage_type": usageType})
}

// idFromParts / idToParts implement this resource's synthesized
// "sourceId/usageType" composite import id (see the package doc's v1-vs-v2
// rationale). "/" is used as the separator, matching the delimiter
// convention already established by application_access_association_v1's
// access-profile-id list encoding.
func idFromParts(sourceID, usageType string) string {
	return sourceID + "/" + usageType
}

func idToParts(id string) (sourceID, usageType string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import id in the form \"source_id/usage_type\", got: %q", id)
	}
	return parts[0], parts[1], nil
}

// fieldsToSlice decodes the practitioner-supplied "fields" JSON string (a
// JSON array of FieldDetailsDto-shaped objects) into
// []api_beta.FieldDetailsDto. A null/empty jsontypes.Normalized decodes to a
// nil slice (omitted "fields" on the wire), which the API accepts.
func fieldsToSlice(v jsontypes.Normalized) ([]api_beta.FieldDetailsDto, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, diags
	}
	var fields []api_beta.FieldDetailsDto
	if err := json.Unmarshal([]byte(v.ValueString()), &fields); err != nil {
		diags.AddError(
			"Invalid \"fields\" JSON",
			fmt.Sprintf("Could not decode \"fields\" as a JSON array of provisioning policy field objects: %s", err.Error()),
		)
		return nil, diags
	}
	return fields, diags
}

// normalizedFieldsFromAPI re-encodes an API-returned "fields" slice as a
// jsontypes.Normalized JSON string, matching transform_v1's
// normalizedAttributesFromAPI pattern. A nil slice (no fields configured on
// the policy) is normalized to an empty JSON array "[]", not the JSON
// literal "null", for predictable diffing.
func normalizedFieldsFromAPI(fields []api_beta.FieldDetailsDto) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if fields == nil {
		fields = []api_beta.FieldDetailsDto{}
	}
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		diags.AddError(
			"Error encoding \"fields\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"fields\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(fieldsJSON)), diags
}

// modelToDto builds an api_beta.ProvisioningPolicyDto request body from the
// plan model.
func modelToDto(plan sourceProvisioningPolicyResourceModel) (*api_beta.ProvisioningPolicyDto, diag.Diagnostics) {
	fields, diags := fieldsToSlice(plan.Fields)
	if diags.HasError() {
		return nil, diags
	}

	dto := api_beta.NewProvisioningPolicyDto(*api_beta.NewNullableString(plan.Name.ValueStringPointer()))
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		dto.SetDescription(plan.Description.ValueString())
	}
	usageType := api_beta.UsageType(plan.UsageType.ValueString())
	dto.UsageType = &usageType
	dto.Fields = fields
	return dto, diags
}

// dtoToModel converts an api_beta.ProvisioningPolicyDto API response into
// the resource's state model, synthesizing "id" from sourceID+usageType.
func dtoToModel(dto *api_beta.ProvisioningPolicyDto, sourceID string, fallback sourceProvisioningPolicyResourceModel) (sourceProvisioningPolicyResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	usageType := ""
	if dto.UsageType != nil {
		usageType = string(*dto.UsageType)
	}

	model.Id = types.StringValue(idFromParts(sourceID, usageType))
	model.SourceId = types.StringValue(sourceID)
	model.UsageType = types.StringValue(usageType)
	model.Name = types.StringValue(dto.GetName())
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	} else {
		model.Description = types.StringNull()
	}

	model.Fields, diags = normalizedFieldsFromAPI(dto.Fields)

	return model, diags
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (see
// transform_v1/role_v1/service_desk_integration_v1 for the same pattern).
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
