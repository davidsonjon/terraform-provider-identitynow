// Package sources_v1 is a pilot implementation of the Sources resource/data
// sources generated from SailPoint's per-service v1 OpenAPI spec
// (api-specs/idn/apis/sources), following the same hand-written CRUD pattern
// established by governance_group_v1/role_v1/service_desk_integration_v1.
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_source/datasource_source,
// backed by the golang-sdk v2 api_beta.SourcesAPIService client (the SDK does
// not yet publish a per-service v1 package; v1 is the stabilization of what
// was beta). Update() calls api_beta.SourcesAPIService.UpdateSource, which is
// PATCH /sources/v1/{id} (RFC 6902 JSON Patch) - deliberately NOT
// PutSource/PUT /sources/v1/{id} (the full-replace variant, whose own spec
// description explicitly calls out MORE fields as immutable than the PATCH
// variant does).
//
// Codegen note: the bundled api-specs/dereferenced/deref-sources.v1.yaml had
// one narrow, single-field `allOf` (managerCorrelationMapping, merging a
// `{accountAttributeName, identityAttributeName}` object with a sibling
// `{nullable: true, description: ...}` modifier-only member) that
// tfplugingen-openapi cannot decompose ("schema composition is currently not
// supported"). Flattened via a one-time scripted edit (a generic
// "merge any allOf whose members are either the sole `type: object` base or a
// modifier-only sibling" pass, following Playbook B's narrow-single-field
// case - see the 2026-07-28 governance_group_v1 knowledge entry for the
// precedent) before running `make gen-api-v1`; if this spec is ever
// re-bundled from a newer upstream revision, that flattening must be
// re-applied first.
//
// "connectorAttributes" dynamic-shape decision: like transform_v1's
// "attributes", connectorAttributes' real shape depends entirely on the
// sibling "type" field (Active Directory vs Workday vs a delimited-file
// source all have completely different connectorAttributes shapes), and the
// SDK itself types it as a plain map[string]interface{} rather than any
// generated struct. Hand-added (via schema.ignores in
// generator_config_sources_v1.yml) as a jsontypes.Normalized JSON-string
// CustomType, exactly like transform_v1's "attributes" field - see the
// IdentityNow agent's Project Context "Dynamic/discriminated-union
// attributes-style fields" bullet for the full rationale.
//
// Deliberately deferred (out of scope for this pilot - see the top-level
// pipeline task that created this package): provisioning-policies, schemas,
// schedules, connections, source-health, correlation-config,
// password-policies, connector/* endpoints (check-connection,
// peek-resource-objects, ping-cluster, test-configuration),
// native-change-detection-config, remove-accounts/load-accounts/
// load-uncorrelated-accounts/synchronize-attributes/load-entitlements,
// attribute-sync-config, entitlement-request-config, approval-config/*,
// upload-connector-file, connectors/source-config, and the bulk
// provisioning-policies update endpoint. "schemas" and "password_policies"
// ARE present as Computed-only read-back attributes on this resource (they
// are part of the Source object's own GET/POST/PATCH response body, and the
// PATCH endpoint's own docs list "passwordPolicies" as immutable - "schemas"
// is not explicitly called out either way, but is managed via its own
// sub-resource endpoints per SailPoint's docs, so this pilot treats it as
// read-only here too) - only the separate schema/schedule/password-policy
// *management* endpoints are deferred, not the reference lists themselves.
// The "provisionAsCsv" create-time query parameter (still part of the
// in-scope createSourceV1 operation) is also deliberately not yet exposed as
// a resource attribute - tracked as a follow-up, not implemented here.
package sources_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/sources_v1/resource_source"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*sourceResource)(nil)
	_ resource.ResourceWithConfigure   = (*sourceResource)(nil)
	_ resource.ResourceWithImportState = (*sourceResource)(nil)
)

func NewSourceResource() resource.Resource {
	return &sourceResource{}
}

type sourceResource struct {
	client *sailpoint.APIClient
}

// sourceResourceModel mirrors resource_source.SourceModel plus the hand-added
// "connector_attributes" field the generator was told to ignore (see package
// doc). Kept as a distinct, hand-written struct (rather than embedding the
// generated model) since Go doesn't allow adding a field to an imported
// struct type, and req.Plan.Get/resp.State.Set match purely on `tfsdk` tags,
// not on which struct type declares them.
type sourceResourceModel struct {
	AccountCorrelationConfig  resource_source.AccountCorrelationConfigValue  `tfsdk:"account_correlation_config"`
	AccountCorrelationRule    resource_source.AccountCorrelationRuleValue    `tfsdk:"account_correlation_rule"`
	Authoritative             types.Bool                                     `tfsdk:"authoritative"`
	BeforeProvisioningRule    resource_source.BeforeProvisioningRuleValue    `tfsdk:"before_provisioning_rule"`
	Category                  types.String                                   `tfsdk:"category"`
	Cluster                   resource_source.ClusterValue                   `tfsdk:"cluster"`
	ConnectionType            types.String                                   `tfsdk:"connection_type"`
	Connector                 types.String                                   `tfsdk:"connector"`
	ConnectorAttributes       jsontypes.Normalized                           `tfsdk:"connector_attributes"`
	ConnectorClass            types.String                                   `tfsdk:"connector_class"`
	ConnectorId               types.String                                   `tfsdk:"connector_id"`
	ConnectorImplementationId types.String                                   `tfsdk:"connector_implementation_id"`
	ConnectorName             types.String                                   `tfsdk:"connector_name"`
	Created                   types.String                                   `tfsdk:"created"`
	CredentialProviderEnabled types.Bool                                     `tfsdk:"credential_provider_enabled"`
	DeleteThreshold           types.Int64                                    `tfsdk:"delete_threshold"`
	Description               types.String                                   `tfsdk:"description"`
	Features                  types.List                                     `tfsdk:"features"`
	Healthy                   types.Bool                                     `tfsdk:"healthy"`
	Id                        types.String                                   `tfsdk:"id"`
	ManagementWorkgroup       resource_source.ManagementWorkgroupValue       `tfsdk:"management_workgroup"`
	ManagerCorrelationMapping resource_source.ManagerCorrelationMappingValue `tfsdk:"manager_correlation_mapping"`
	ManagerCorrelationRule    resource_source.ManagerCorrelationRuleValue    `tfsdk:"manager_correlation_rule"`
	Modified                  types.String                                   `tfsdk:"modified"`
	Name                      types.String                                   `tfsdk:"name"`
	Owner                     resource_source.OwnerValue                     `tfsdk:"owner"`
	PasswordPolicies          types.List                                     `tfsdk:"password_policies"`
	Schemas                   types.List                                     `tfsdk:"schemas"`
	Since                     types.String                                   `tfsdk:"since"`
	Status                    types.String                                   `tfsdk:"status"`
	Type                      types.String                                   `tfsdk:"type"`
}

func (r *sourceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_v1"
}

func (r *sourceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_source.SourceResourceSchema(ctx)
	resp.Schema.Description = "Manages a Source in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Source](https://documentation.sailpoint.com/saas/help/sources/index.html) " +
		"in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying on " +
		"it in production configurations. Only core create/read/update/delete lifecycle attributes are modeled here - " +
		"provisioning policies, schemas, schedules, connections, source health, correlation config, password policies, " +
		"connector test/discovery operations, and several other sub-resource endpoints are deliberately deferred (see " +
		"the package doc)."
	applySourceConnectorAttributesField(&resp.Schema.Attributes, false)
	applySourceUseStateForUnknown(&resp.Schema)
}

func (r *sourceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *sourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Source", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		CreateSource(ctx).
		Source(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Source", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Source", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Source", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Source", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		GetSource(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Source", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Source", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Source", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// updateSourceV1 (PATCH) explicitly documents these fields as immutable:
	// id, type, authoritative, created, modified, connector, connectorClass,
	// passwordPolicies. Every other field this resource writes on Create is
	// patched here unconditionally as a "replace" op (same simple,
	// unconditional-replace convention as governance_group_v1/role_v1,
	// rather than diffing state vs. plan per-field).
	patch := []api_beta.JsonPatchOperation{
		jsonPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&dto.Name)),
	}
	patch = append(patch, optionalStringPatch("/description", dto.Description)...)
	patch = append(patch, optionalBoolPatch("/credentialProviderEnabled", dto.CredentialProviderEnabled)...)
	patch = append(patch, optionalInt32Patch("/deleteThreshold", dto.DeleteThreshold)...)

	if len(dto.Features) > 0 {
		arr := make([]api_beta.ArrayInner, 0, len(dto.Features))
		for i := range dto.Features {
			arr = append(arr, api_beta.ArrayInner{String: &dto.Features[i]})
		}
		patch = append(patch, jsonPatchReplace("/features", api_beta.ArrayOfArrayInnerAsUpdateMultiHostSourcesRequestInnerValue(&arr)))
	}
	if m, err := structToMap(dto.ConnectorAttributes); err == nil && m != nil && plan.ConnectorAttributes.ValueString() != state.ConnectorAttributes.ValueString() {
		// connector_attributes genuinely changed - merge the practitioner's
		// newly configured keys on top of the source's current *live*
		// connectorAttributes (fetched fresh here, since state only ever
		// retains the practitioner's configured subset, not the API's
		// enriched superset - see dtoToModel's ConnectorAttributes handling
		// below). This preserves any server/connector-injected keys the API
		// needs, avoiding the same "Illegal attempt to modify \"healthy\"
		// field" false conflict that an unconditional, configured-subset-only
		// replace triggered (see the skip-when-unchanged comment above).
		// If the live re-fetch itself fails, fall back to sending just the
		// practitioner's configured subset rather than blocking the whole
		// update on a secondary read failure.
		var liveAttrs map[string]interface{}
		if liveResp, _, err := r.client.Beta.SourcesAPI.GetSource(ctx, state.Id.ValueString()).Execute(); err == nil && liveResp != nil {
			liveAttrs = liveResp.ConnectorAttributes
		} else {
			tflog.Warn(ctx, "Could not re-fetch live Source to merge connector_attributes before Update; sending only the configured subset", map[string]interface{}{"id": state.Id.ValueString()})
		}
		merged := mergeConnectorAttributes(liveAttrs, m)
		if merged != nil {
			patch = append(patch, jsonPatchReplace("/connectorAttributes", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&merged)))
		}
	}
	if owner := dto.Owner.Get(); owner != nil {
		if m, err := structToMap(owner); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/owner", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if cluster := dto.Cluster.Get(); cluster != nil {
		if m, err := structToMap(cluster); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/cluster", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.AccountCorrelationConfig.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/accountCorrelationConfig", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.AccountCorrelationRule.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/accountCorrelationRule", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.ManagerCorrelationMapping.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/managerCorrelationMapping", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.ManagerCorrelationRule.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/managerCorrelationRule", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.BeforeProvisioningRule.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/beforeProvisioningRule", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}
	if v := dto.ManagementWorkgroup.Get(); v != nil {
		if m, err := structToMap(v); err == nil && m != nil {
			patch = append(patch, jsonPatchReplace("/managementWorkgroup", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
		}
	}

	tflog.Debug(ctx, "Patching Source", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})
	if b, mErr := json.Marshal(patch); mErr == nil {
		tflog.Debug(ctx, "DEBUG_PATCH_BODY", map[string]interface{}{"body": string(b)})
	}

	apiResp, httpResp, err := r.client.Beta.SourcesAPI.
		UpdateSource(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Source", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Source", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Source", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Source", map[string]interface{}{"id": state.Id.ValueString()})

	// DELETE /sources/v1/{id} returns 202 Accepted with a TASK_RESULT
	// reference (the API removes accounts asynchronously before deleting the
	// source itself) - this resource does not poll that task to completion,
	// matching the "fire and forget" behavior of every other _v1 pilot
	// resource's Delete().
	_, httpResp, err := r.client.Beta.SourcesAPI.
		Delete(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Source", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Source", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Source", map[string]interface{}{"id": state.Id.ValueString()})
}

// modelToDto converts the Terraform plan/config model into the SDK
// create/update DTO shape. Only fields this resource actually manages are
// set - server-computed-only fields (id, created, modified, connector_id,
// connector_name, connection_type, connector_implementation_id, healthy,
// status, since, schemas, password_policies) are left at their zero value
// and always re-populated from the live API response by dtoToModel instead.
func modelToDto(ctx context.Context, m sourceResourceModel) (*api_beta.Source, diag.Diagnostics) {
	var diags diag.Diagnostics

	owner, d := m.Owner.ToApi_betaSourceOwner(ctx)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	dto := api_beta.NewSourceWithDefaults()
	dto.Name = m.Name.ValueString()
	dto.Connector = m.Connector.ValueString()
	dto.Owner = *api_beta.NewNullableSourceOwner(owner)

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		v := m.Description.ValueString()
		dto.Description = &v
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		v := m.Type.ValueString()
		dto.Type = &v
	}
	if !m.ConnectorClass.IsNull() && !m.ConnectorClass.IsUnknown() {
		v := m.ConnectorClass.ValueString()
		dto.ConnectorClass = &v
	}
	if !m.Authoritative.IsNull() && !m.Authoritative.IsUnknown() {
		v := m.Authoritative.ValueBool()
		dto.Authoritative = &v
	}
	if !m.CredentialProviderEnabled.IsNull() && !m.CredentialProviderEnabled.IsUnknown() {
		v := m.CredentialProviderEnabled.ValueBool()
		dto.CredentialProviderEnabled = &v
	}
	if !m.DeleteThreshold.IsNull() && !m.DeleteThreshold.IsUnknown() {
		v := int32(m.DeleteThreshold.ValueInt64())
		dto.DeleteThreshold = &v
	}
	// Category is intentionally never sent to the API: it's Computed-only
	// in the schema (see generator_config/schema_overrides_sources_v1.yml)
	// because live testing confirmed the server silently ignores any
	// configured value and always returns null/the server's own value.
	if !m.Features.IsNull() && !m.Features.IsUnknown() {
		var features []string
		diags.Append(m.Features.ElementsAs(ctx, &features, false)...)
		dto.Features = features
	}

	attrs, d := connectorAttributesToMap(m.ConnectorAttributes)
	diags.Append(d...)
	if attrs != nil {
		dto.ConnectorAttributes = attrs
	}

	if !m.Cluster.IsNull() && !m.Cluster.IsUnknown() {
		v := api_beta.MultiHostIntegrationsCluster{
			Id:   m.Cluster.Id.ValueString(),
			Name: m.Cluster.Name.ValueString(),
			Type: m.Cluster.ClusterType.ValueString(),
		}
		dto.Cluster = *api_beta.NewNullableMultiHostIntegrationsCluster(&v)
	}

	if !m.AccountCorrelationConfig.IsNull() && !m.AccountCorrelationConfig.IsUnknown() {
		v, d := m.AccountCorrelationConfig.ToApi_betaMultiHostSourcesAccountCorrelationConfig(ctx)
		diags.Append(d...)
		dto.AccountCorrelationConfig = *api_beta.NewNullableMultiHostSourcesAccountCorrelationConfig(v)
	}
	if !m.AccountCorrelationRule.IsNull() && !m.AccountCorrelationRule.IsUnknown() {
		v, d := m.AccountCorrelationRule.ToApi_betaMultiHostSourcesAccountCorrelationRule(ctx)
		diags.Append(d...)
		dto.AccountCorrelationRule = *api_beta.NewNullableMultiHostSourcesAccountCorrelationRule(v)
	}
	if !m.ManagerCorrelationMapping.IsNull() && !m.ManagerCorrelationMapping.IsUnknown() {
		v, d := m.ManagerCorrelationMapping.ToApi_betaManagerCorrelationMapping(ctx)
		diags.Append(d...)
		dto.ManagerCorrelationMapping = *api_beta.NewNullableManagerCorrelationMapping(v)
	}
	if !m.ManagerCorrelationRule.IsNull() && !m.ManagerCorrelationRule.IsUnknown() {
		v, d := m.ManagerCorrelationRule.ToApi_betaMultiHostSourcesManagerCorrelationRule(ctx)
		diags.Append(d...)
		dto.ManagerCorrelationRule = *api_beta.NewNullableMultiHostSourcesManagerCorrelationRule(v)
	}
	if !m.BeforeProvisioningRule.IsNull() && !m.BeforeProvisioningRule.IsUnknown() {
		v, d := m.BeforeProvisioningRule.ToApi_betaMultiHostSourcesBeforeProvisioningRule(ctx)
		diags.Append(d...)
		dto.BeforeProvisioningRule = *api_beta.NewNullableMultiHostSourcesBeforeProvisioningRule(v)
	}
	if !m.ManagementWorkgroup.IsNull() && !m.ManagementWorkgroup.IsUnknown() {
		v, d := m.ManagementWorkgroup.ToApi_betaMultiHostIntegrationsManagementWorkgroup(ctx)
		diags.Append(d...)
		dto.ManagementWorkgroup = *api_beta.NewNullableMultiHostIntegrationsManagementWorkgroup(v)
	}

	return dto, diags
}

// dtoToModel converts an API response DTO into the Terraform state model.
// "fallback" supplies any attribute this converter doesn't itself populate
// (there are none currently, but this matches the established
// governance_group_v1/role_v1 convention of always taking a fallback model).
func dtoToModel(ctx context.Context, dto *api_beta.Source, fallback sourceResourceModel) (sourceResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	model.Name = types.StringValue(dto.Name)
	model.Connector = types.StringValue(dto.Connector)
	model.Description = types.StringPointerValue(dto.Description)
	model.Type = types.StringPointerValue(dto.Type)
	model.ConnectorClass = types.StringPointerValue(dto.ConnectorClass)
	model.ConnectorId = types.StringPointerValue(dto.ConnectorId)
	model.ConnectorName = types.StringPointerValue(dto.ConnectorName)
	model.ConnectionType = types.StringPointerValue(dto.ConnectionType)
	model.ConnectorImplementationId = types.StringPointerValue(dto.ConnectorImplementationId)
	model.Authoritative = types.BoolPointerValue(dto.Authoritative)
	model.Healthy = types.BoolPointerValue(dto.Healthy)
	model.Status = types.StringPointerValue(dto.Status)
	model.Since = types.StringPointerValue(dto.Since)
	model.CredentialProviderEnabled = types.BoolPointerValue(dto.CredentialProviderEnabled)
	if dto.DeleteThreshold != nil {
		model.DeleteThreshold = types.Int64Value(int64(*dto.DeleteThreshold))
	} else {
		model.DeleteThreshold = types.Int64Null()
	}
	model.Category = types.StringPointerValue(dto.Category.Get())
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	// The live API merges additional server-computed status keys (e.g.
	// "healthy", "since", "status", "connectionType", "connectorName" -
	// duplicates of the top-level attributes already set above) into the
	// same connectorAttributes object the practitioner configured, rather
	// than returning back exactly what was sent. Since this attribute is
	// Optional+Computed, echoing that enriched superset back into state
	// whenever the practitioner *did* configure a known value causes a
	// "Provider produced inconsistent result after apply" error (confirmed
	// live) because the returned value can never semantically equal the
	// practitioner's configured subset. Preserve the practitioner's own
	// configured value as the source of truth in that case; only fall back
	// to the live API's value when nothing was configured (e.g. after
	// `terraform import`, or for a connector type where the schema doesn't
	// require this attribute at all), so unmanaged/imported sources still
	// get a populated value instead of Null.
	if fallback.ConnectorAttributes.IsNull() || fallback.ConnectorAttributes.IsUnknown() {
		connAttrs, d := normalizedConnectorAttributesFromAPI(dto.ConnectorAttributes)
		diags.Append(d...)
		model.ConnectorAttributes = connAttrs
	}

	if dto.Features != nil {
		featuresList, d := types.ListValueFrom(ctx, types.StringType, dto.Features)
		diags.Append(d...)
		model.Features = featuresList
	} else {
		model.Features = types.ListNull(types.StringType)
	}

	owner, d := resource_source.OwnerValue{}.FromApi_betaSourceOwner(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	clusterVal, d := clusterFromAPI(ctx, dto.Cluster.Get())
	diags.Append(d...)
	model.Cluster = clusterVal

	accountCorrelationConfig, d := resource_source.AccountCorrelationConfigValue{}.FromApi_betaMultiHostSourcesAccountCorrelationConfig(ctx, dto.AccountCorrelationConfig.Get())
	diags.Append(d...)
	model.AccountCorrelationConfig = accountCorrelationConfig

	accountCorrelationRule, d := resource_source.AccountCorrelationRuleValue{}.FromApi_betaMultiHostSourcesAccountCorrelationRule(ctx, dto.AccountCorrelationRule.Get())
	diags.Append(d...)
	model.AccountCorrelationRule = accountCorrelationRule

	managerCorrelationMapping, d := resource_source.ManagerCorrelationMappingValue{}.FromApi_betaManagerCorrelationMapping(ctx, dto.ManagerCorrelationMapping.Get())
	diags.Append(d...)
	model.ManagerCorrelationMapping = managerCorrelationMapping

	managerCorrelationRule, d := resource_source.ManagerCorrelationRuleValue{}.FromApi_betaMultiHostSourcesManagerCorrelationRule(ctx, dto.ManagerCorrelationRule.Get())
	diags.Append(d...)
	model.ManagerCorrelationRule = managerCorrelationRule

	beforeProvisioningRule, d := resource_source.BeforeProvisioningRuleValue{}.FromApi_betaMultiHostSourcesBeforeProvisioningRule(ctx, dto.BeforeProvisioningRule.Get())
	diags.Append(d...)
	model.BeforeProvisioningRule = beforeProvisioningRule

	managementWorkgroup, d := resource_source.ManagementWorkgroupValue{}.FromApi_betaMultiHostIntegrationsManagementWorkgroup(ctx, dto.ManagementWorkgroup.Get())
	diags.Append(d...)
	model.ManagementWorkgroup = managementWorkgroup

	if len(dto.Schemas) > 0 {
		values := make([]resource_source.SchemasValue, 0, len(dto.Schemas))
		for i := range dto.Schemas {
			v, d := resource_source.SchemasValue{}.FromApi_betaMultiHostSourcesSchemasInner(ctx, &dto.Schemas[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, resource_source.SchemasValue{}.Type(ctx), values)
		diags.Append(d...)
		model.Schemas = listVal
	} else {
		model.Schemas = types.ListNull(resource_source.SchemasValue{}.Type(ctx))
	}

	if len(dto.PasswordPolicies) > 0 {
		values := make([]resource_source.PasswordPoliciesValue, 0, len(dto.PasswordPolicies))
		for i := range dto.PasswordPolicies {
			v, d := resource_source.PasswordPoliciesValue{}.FromApi_betaMultiHostSourcesPasswordPoliciesInner(ctx, &dto.PasswordPolicies[i])
			diags.Append(d...)
			values = append(values, v)
		}
		listVal, d := types.ListValueFrom(ctx, resource_source.PasswordPoliciesValue{}.Type(ctx), values)
		diags.Append(d...)
		model.PasswordPolicies = listVal
	} else {
		model.PasswordPolicies = types.ListNull(resource_source.PasswordPoliciesValue{}.Type(ctx))
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

func optionalBoolPatch(path string, v *bool) []api_beta.JsonPatchOperation {
	if v == nil {
		return nil
	}
	return []api_beta.JsonPatchOperation{jsonPatchReplace(path, api_beta.BoolAsUpdateMultiHostSourcesRequestInnerValue(v))}
}

func optionalInt32Patch(path string, v *int32) []api_beta.JsonPatchOperation {
	if v == nil {
		return nil
	}
	return []api_beta.JsonPatchOperation{jsonPatchReplace(path, api_beta.Int32AsUpdateMultiHostSourcesRequestInnerValue(v))}
}

// structToMap round-trips an SDK model struct through JSON to get a
// map[string]interface{} suitable for
// api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue, since
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

// connectorAttributesToMap decodes the practitioner-supplied
// "connector_attributes" JSON string into a map[string]interface{} suitable
// for api_beta.Source.ConnectorAttributes. A null/unknown/empty
// jsontypes.Normalized decodes to nil (connectorAttributes is Optional, not
// Required, unlike transform_v1's "attributes").
func connectorAttributesToMap(v jsontypes.Normalized) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, diags
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError(
			"Invalid \"connector_attributes\" JSON",
			fmt.Sprintf("Could not decode \"connector_attributes\" as a JSON object: %s", err.Error()),
		)
		return nil, diags
	}
	return m, diags
}

// mergeConnectorAttributes overlays the practitioner's newly configured
// connector_attributes keys on top of the source's current live
// connectorAttributes value, preserving any server/connector-injected keys
// the practitioner never configured (see the "read-back enrichment"
// discussion in dtoToModel below). Used by Update when connector_attributes
// genuinely changed, so a real intentional edit doesn't strip the API's own
// managed keys and re-trigger the "Illegal attempt to modify \"healthy\"
// field" false conflict the same way an unconditional, configured-subset-only
// replace previously did. Returns nil if both inputs are empty (nothing to
// send). Practitioner-configured keys always win over the live value for any
// overlapping key.
func mergeConnectorAttributes(live map[string]interface{}, configured map[string]interface{}) map[string]interface{} {
	if len(live) == 0 && len(configured) == 0 {
		return nil
	}
	merged := make(map[string]interface{}, len(live)+len(configured))
	for k, v := range live {
		merged[k] = v
	}
	for k, v := range configured {
		merged[k] = v
	}
	return merged
}

// normalizedConnectorAttributesFromAPI re-encodes an API-returned
// connectorAttributes map as a jsontypes.Normalized JSON string. A nil map
// (connectorAttributes omitted entirely from the response) becomes a null
// jsontypes.Normalized rather than "{}", since this attribute is genuinely
// Optional (unlike transform_v1's Required "attributes").
func normalizedConnectorAttributesFromAPI(attrs map[string]interface{}) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if attrs == nil {
		return jsontypes.NewNormalizedNull(), diags
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		diags.AddError(
			"Error encoding \"connector_attributes\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"connectorAttributes\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(attrsJSON)), diags
}

// clusterFromAPI hand-converts an *api_beta.MultiHostIntegrationsCluster into
// the schema-native resource_source.ClusterValue. Not associated_external_type
// mapped - see the "NOT mapped" note in
// generator_config/type_mappings_sources_v1.yml (MultiHostIntegrationsCluster's
// id/name/type fields are plain, non-pointer `string`, since Source.yaml's
// cluster schema marks them all `required` within the nested object - this
// is incompatible with the generated converter template's
// `.ValueStringPointer()`/`types.StringPointerValue(...)` calls, which
// assume a plain `*string` field).
func clusterFromAPI(ctx context.Context, cluster *api_beta.MultiHostIntegrationsCluster) (resource_source.ClusterValue, diag.Diagnostics) {
	if cluster == nil {
		return resource_source.NewClusterValueNull(), nil
	}
	return resource_source.NewClusterValue(
		resource_source.ClusterValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(cluster.Id),
			"name": types.StringValue(cluster.Name),
			"type": types.StringValue(cluster.Type),
		},
	)
}

// timeToString formats an *api_beta.SailPointTime as RFC3339, or returns "" if nil.
func timeToStringValue(t *api_beta.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
