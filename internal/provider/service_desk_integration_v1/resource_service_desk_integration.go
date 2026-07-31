// Package service_desk_integration_v1 is a pilot implementation of the
// service-desk-integration resource/data source generated from SailPoint's new
// per-service v1 OpenAPI spec (api-specs/idn/apis/service-desk-integration).
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_service_desk_integration and
// datasource_service_desk_integration, backed by the golang-sdk v2
// api_beta.ServiceDeskIntegrationAPI client (the SDK does not yet publish a
// per-service v1 package; v1 is the stabilization of what was beta).
//
// Known limitations (tracked for follow-up before promoting out of the _v1 pilot
// package into internal/provider/service_desk_integration):
//   - "attributes" has no schema-defined properties (the OpenAPI spec models it as a
//     bare object with no properties), so it is always treated as an empty object on
//     write and read.
//   - "provisioning_config.managed_resource_refs" and
//     "provisioning_config.plan_initializer_script" are pass-through only (state
//     mirrors config) rather than parsed from the API response, since
//     api_beta.ProvisioningConfig was intentionally not given an
//     associated_external_type mapping (see the tfplugingen-openapi-type-reviewer
//     knowledge file: parent blocks with nested list_nested/single_nested children
//     broke framework generation for the role target when mapped).
//   - Reads of objects with a non-empty provisioningConfig.managedResourceRefs
//     work around a confirmed upstream golang-sdk defect (mistyped
//     Type/Id/Name fields on ProvisioningConfigManagedResourceRefsInner) via
//     the fallback decode in sdk_fallback.go, which discards
//     managedResourceRefs entirely - consistent with the pass-through-only
//     handling above, since this package never reads that field anyway.
package service_desk_integration_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/service_desk_integration_v1/resource_service_desk_integration"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*serviceDeskIntegrationResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceDeskIntegrationResource)(nil)
	_ resource.ResourceWithImportState = (*serviceDeskIntegrationResource)(nil)
)

func NewServiceDeskIntegrationResource() resource.Resource {
	return &serviceDeskIntegrationResource{}
}

type serviceDeskIntegrationResource struct {
	client *sailpoint.APIClient
}

func (r *serviceDeskIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_desk_integration_v1"
}

func (r *serviceDeskIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_service_desk_integration.ServiceDeskIntegrationResourceSchema(ctx)
	resp.Schema.Description = "Manages a Service Desk Integration in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Service Desk Integration](https://documentation.sailpoint.com/saas/help/integration/help/landing-service-desk.html) " +
		"in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource - see the \"Known Limitations & Live Testing Notes\" section below before relying on " +
		"it in production configurations."
	applyServiceDeskIntegrationUseStateForUnknown(&resp.Schema)
}

func (r *serviceDeskIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceDeskIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *serviceDeskIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Service Desk Integration", map[string]interface{}{"name": plan.Name.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.Beta.ServiceDeskIntegrationAPI.
		CreateServiceDeskIntegration(ctx).
		ServiceDeskIntegrationDto(*dto).
		Execute()
	apiResp, httpResp, err = withManagedResourceRefsFallback(ctx, apiResp, httpResp, err)
	if err != nil {
		tflog.Error(ctx, "Error creating Service Desk Integration", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Service Desk Integration", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceDeskIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.Beta.ServiceDeskIntegrationAPI.
		GetServiceDeskIntegration(ctx, state.Id.ValueString()).
		Execute()
	apiResp, httpResp, err = withManagedResourceRefsFallback(ctx, apiResp, httpResp, err)
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Service Desk Integration not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Service Desk Integration", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Service Desk Integration", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *serviceDeskIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The v1 API updates via RFC 6902 JSON Patch. A "replace whole document"
	// patch of every top-level writable field is the simplest correct
	// approach for a pilot resource; a follow-up can move to a minimal diff.
	patch := []api_beta.JsonPatchOperation{
		jsonPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&dto.Name)),
		jsonPatchReplace("/description", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&dto.Description)),
		jsonPatchReplace("/type", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&dto.Type)),
		jsonPatchReplace("/attributes", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&dto.Attributes)),
	}
	if m, err := structToMap(dto.OwnerRef); err == nil && m != nil {
		patch = append(patch, jsonPatchReplace("/ownerRef", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
	}
	if m, err := structToMap(dto.ClusterRef); err == nil && m != nil {
		patch = append(patch, jsonPatchReplace("/clusterRef", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
	}
	if m, err := structToMap(dto.BeforeProvisioningRule); err == nil && m != nil {
		patch = append(patch, jsonPatchReplace("/beforeProvisioningRule", api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue(&m)))
	}

	tflog.Debug(ctx, "Patching Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString(), "patch_ops": len(patch)})

	apiResp, httpResp, err := r.client.Beta.ServiceDeskIntegrationAPI.
		PatchServiceDeskIntegration(ctx, state.Id.ValueString()).
		JsonPatchOperation(patch).
		Execute()
	apiResp, httpResp, err = withManagedResourceRefsFallback(ctx, apiResp, httpResp, err)
	if err != nil {
		tflog.Error(ctx, "Error updating Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Service Desk Integration", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Service Desk Integration", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *serviceDeskIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_service_desk_integration.ServiceDeskIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.Beta.ServiceDeskIntegrationAPI.
		DeleteServiceDeskIntegration(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Service Desk Integration already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Service Desk Integration", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Service Desk Integration", map[string]interface{}{"id": state.Id.ValueString()})
}

// modelToDto converts the Terraform plan/config model into the SDK create/update
// DTO shape. See package doc for the "attributes" and "provisioning_config" caveats.
//
// before_provisioning_rule/cluster_ref/owner_ref are Optional+Computed in the
// schema, so Terraform sends them as Unknown (not Null) on Create when the
// practitioner omits them from config - this is normal Optional+Computed
// behavior, not an error condition. The generated ToApi_beta*Dto() converters
// only handle Null/Known and error on Unknown, so Unknown is treated the same
// as Null here (send nothing; dtoToModel fills in the real value from the API
// response afterward).
func modelToDto(ctx context.Context, m resource_service_desk_integration.ServiceDeskIntegrationModel) (*api_beta.ServiceDeskIntegrationDto, diag.Diagnostics) {
	var diags diag.Diagnostics

	var beforeProvisioningRule *api_beta.BeforeProvisioningRuleDto
	if !m.BeforeProvisioningRule.IsUnknown() {
		var d diag.Diagnostics
		beforeProvisioningRule, d = m.BeforeProvisioningRule.ToApi_betaBeforeProvisioningRuleDto(ctx)
		diags.Append(d...)
	}
	var clusterRef *api_beta.SourceClusterDto
	if !m.ClusterRef.IsUnknown() {
		var d diag.Diagnostics
		clusterRef, d = m.ClusterRef.ToApi_betaSourceClusterDto(ctx)
		diags.Append(d...)
	}
	var ownerRef *api_beta.OwnerDto
	if !m.OwnerRef.IsUnknown() {
		var d diag.Diagnostics
		ownerRef, d = m.OwnerRef.ToApi_betaOwnerDto(ctx)
		diags.Append(d...)
	}
	if diags.HasError() {
		return nil, diags
	}

	var managedSources []string
	if !m.ManagedSources.IsNull() && !m.ManagedSources.IsUnknown() {
		diags.Append(m.ManagedSources.ElementsAs(ctx, &managedSources, false)...)
	}

	dto := api_beta.NewServiceDeskIntegrationDtoWithDefaults()
	dto.Name = m.Name.ValueString()
	dto.Description = m.Description.ValueString()
	dto.Type = m.Type.ValueString()
	// "attributes" has no schema-defined properties; always sent as an empty object.
	dto.Attributes = map[string]interface{}{}
	dto.OwnerRef = ownerRef
	dto.ClusterRef = clusterRef
	dto.BeforeProvisioningRule = beforeProvisioningRule
	if !m.Cluster.IsNull() && !m.Cluster.IsUnknown() {
		dto.Cluster = m.Cluster.ValueStringPointer()
	}
	if managedSources != nil {
		dto.ManagedSources = managedSources
	}
	if pc := provisioningConfigToDto(m.ProvisioningConfig); pc != nil {
		dto.ProvisioningConfig = pc
	}

	return dto, diags
}

// provisioningConfigToDto maps only the top-level scalar fields of
// provisioning_config. ManagedResourceRefs/PlanInitializerScript are
// intentionally left unset on write; see package doc.
func provisioningConfigToDto(v resource_service_desk_integration.ProvisioningConfigValue) *api_beta.ProvisioningConfig {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	pc := api_beta.NewProvisioningConfigWithDefaults()
	if !v.UniversalManager.IsNull() && !v.UniversalManager.IsUnknown() {
		pc.UniversalManager = v.UniversalManager.ValueBoolPointer()
	}
	if !v.NoProvisioningRequests.IsNull() && !v.NoProvisioningRequests.IsUnknown() {
		pc.NoProvisioningRequests = v.NoProvisioningRequests.ValueBoolPointer()
	}
	if !v.ProvisioningRequestExpiration.IsNull() && !v.ProvisioningRequestExpiration.IsUnknown() {
		exp := int32(v.ProvisioningRequestExpiration.ValueInt64())
		pc.ProvisioningRequestExpiration = &exp
	}
	return pc
}

// dtoToModel converts an API response DTO into the Terraform state model,
// preferring fields carried over from fallback (plan/prior state) for the
// parts of provisioning_config and attributes that are not read back from
// the API (see package doc).
func dtoToModel(ctx context.Context, dto *api_beta.ServiceDeskIntegrationDto, fallback resource_service_desk_integration.ServiceDeskIntegrationModel) (resource_service_desk_integration.ServiceDeskIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if id := dtoID(dto); id != "" {
		model.Id = types.StringValue(id)
	}
	model.Name = types.StringValue(dto.Name)
	model.Description = types.StringValue(dto.Description)
	model.Type = types.StringValue(dto.Type)
	model.Created = types.StringPointerValue(nil)
	model.Modified = types.StringPointerValue(nil)
	if dto.Cluster != nil {
		model.Cluster = types.StringPointerValue(dto.Cluster)
	}

	beforeProvisioningRule, d := resource_service_desk_integration.BeforeProvisioningRuleValue{}.FromApi_betaBeforeProvisioningRuleDto(ctx, dto.BeforeProvisioningRule)
	diags.Append(d...)
	model.BeforeProvisioningRule = beforeProvisioningRule

	clusterRef, d := resource_service_desk_integration.ClusterRefValue{}.FromApi_betaSourceClusterDto(ctx, dto.ClusterRef)
	diags.Append(d...)
	model.ClusterRef = clusterRef

	ownerRef, d := resource_service_desk_integration.OwnerRefValue{}.FromApi_betaOwnerDto(ctx, dto.OwnerRef)
	diags.Append(d...)
	model.OwnerRef = ownerRef

	if dto.ManagedSources != nil {
		listVal, d := types.ListValueFrom(ctx, types.StringType, dto.ManagedSources)
		diags.Append(d...)
		model.ManagedSources = listVal
	}

	attrsVal, d := resource_service_desk_integration.NewAttributesValue(map[string]attr.Type{}, map[string]attr.Value{})
	diags.Append(d...)
	model.Attributes = attrsVal

	// provisioning_config: refresh the scalar fields we can safely read back;
	// leave managed_resource_refs/plan_initializer_script as configured
	// (fallback) since api_beta.ProvisioningConfig isn't mapped that deeply.
	if dto.ProvisioningConfig != nil {
		model.ProvisioningConfig.UniversalManager = types.BoolPointerValue(dto.ProvisioningConfig.UniversalManager)
		model.ProvisioningConfig.NoProvisioningRequests = types.BoolPointerValue(dto.ProvisioningConfig.NoProvisioningRequests)
		if dto.ProvisioningConfig.ProvisioningRequestExpiration != nil {
			model.ProvisioningConfig.ProvisioningRequestExpiration = types.Int64Value(int64(*dto.ProvisioningConfig.ProvisioningRequestExpiration))
		}
	}

	return model, diags
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (adopted
// from the role_v1 pilot) so both v1 targets surface the same richer detail
// (HTTP status, detailCode, trackingId, and message text) in
// resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

// dtoID reads "id" out of AdditionalProperties: api_beta.ServiceDeskIntegrationDto
// (generated against the older beta spec) doesn't declare "id" as a typed field,
// but the real v1 API response includes it, so it lands in AdditionalProperties
// during JSON unmarshaling.
func dtoID(dto *api_beta.ServiceDeskIntegrationDto) string {
	if dto == nil || dto.AdditionalProperties == nil {
		return ""
	}
	if v, ok := dto.AdditionalProperties["id"].(string); ok {
		return v
	}
	return ""
}

func jsonPatchReplace(path string, value api_beta.UpdateMultiHostSourcesRequestInnerValue) api_beta.JsonPatchOperation {
	return api_beta.JsonPatchOperation{
		Op:    "replace",
		Path:  path,
		Value: &value,
	}
}

// structToMap round-trips an SDK model struct through JSON to get a
// map[string]interface{} suitable for api_beta.MapmapOfStringAnyAsUpdateMultiHostSourcesRequestInnerValue,
// since the JSON Patch value wrapper type doesn't accept typed structs directly.
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
