// Package workflow_v1 is a pilot implementation of the workflow
// resource/data sources generated from SailPoint's new per-service v1
// OpenAPI spec (api-specs/idn/apis/workflows), following the same
// hand-written CRUD pattern established by role_v1/service_desk_integration_v1.
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model/value types in resource_workflow,
// datasource_workflow, backed by the golang-sdk v2 api_beta.WorkflowsAPI
// client (the SDK does not yet publish a per-service v1 package; v1 is the
// stabilization of what was beta).
//
// Codegen note: the spec's list/create/get/put/patch responses wrap the
// Workflow object in a top-level `allOf` (base Workflow properties + a
// {name,owner,description,definition,enabled,trigger} WorkflowBody wrapper),
// which tfplugingen-openapi cannot decompose ("schema composition is
// currently not supported") - resolved by running
// scripts/flatten_openapi_allof.py against api-specs/dereferenced/deref-workflows.v1.yaml
// before `make gen-api-v1` (see the transform_v1 precedent for the same
// whole-response-allOf pattern; if this spec is ever re-bundled from a newer
// upstream revision, the flattening step must be re-applied first).
//
// Scope: only the core CRUD surface (list/get/create/put/delete workflows)
// is implemented. The following related Workflows API operations are
// deliberately out of scope for this pilot (no resource/data source manages
// them) - see the package's "Known Limitations" doc section for the full
// rationale:
//   - PATCH /workflows/v1/{id} (JSON Patch) - PUT already gives a full,
//     simpler replace semantics for every mutable field, matching
//     transform_v1's identical PUT-over-PATCH choice.
//   - POST /workflows/v1/{id}/test, GET/DELETE /workflow-executions/v1/*,
//     GET /workflows/v1/{id}/executions - execution history/testing are
//     transient, non-declarative operations, not resource state.
//   - GET /workflows/v1/{id}/external/oauth-clients, POST .../execute/external/{id} -
//     external-trigger invocation plumbing, not workflow configuration.
//   - GET /workflow-library/v1(/actions|/triggers|/operators) - read-only
//     catalog/metadata endpoints describing what CAN go in a workflow
//     definition's steps, not a workflow's own attributes; would be
//     data-source candidates for a future session, not required to manage
//     a workflow itself.
//
// "definition"/"trigger.attributes" dynamic-shape decision (see the
// 2026-07-24 dynamic-attributes-pattern-research entry in
// .github/agents/identitynow-terraform-provider-developer.knowledge.md,
// and this agent's Project Context bullet on the same pattern): a
// workflow's "definition.steps" is `type: object, additionalProperties: true`
// - a free-form map of step definitions whose own shape varies per step
// "type" (action/approval/success/etc.), and a trigger's "attributes" is an
// anyOf across 3 shapes (event/external/scheduled) keyed by the sibling
// "trigger.type" field. Both are hand-added as jsontypes.Normalized
// JSON-string CustomTypes rather than modeled as static nested blocks,
// exactly like transform_v1's "attributes" and source_v1's
// "connector_attributes". Unlike transform (whose dynamic field was
// top-level), "trigger.attributes" lives inside a nested single_nested
// block ("trigger") alongside two ordinary, well-typed sibling fields
// ("type", "displayName") - tfplugingen-framework generates a single fixed
// Go value type per single_nested block at codegen time with no supported
// way to hand-insert an extra field into it afterward, so the *entire*
// "trigger" object (not just "attributes") is schema.ignores'd and
// hand-written in full here (schema + a plain types.Object-backed model),
// following segment_v1's "visibility_criteria" precedent for a fully
// hand-rolled nested block.
package workflow_v1

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
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/util"
	"terraform-provider-identitynow/internal/provider/workflow_v1/resource_workflow"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

const workflowGuidanceMarkdown = "" +
	"### Known Limitations & Live Testing Notes\n\n" +
	"- This is a `_v1` pilot resource. Only core CRUD (create/read/update/delete a workflow's own configuration) is " +
	"implemented - workflow execution/testing (`POST .../test`), execution history (`GET .../executions`, " +
	"`GET /workflow-executions/v1/...`), external-trigger invocation, and the read-only `/workflow-library/v1` " +
	"action/trigger/operator catalogs are all out of scope for this pilot (see the package doc for the full list).\n" +
	"- `enabled` workflows **cannot be deleted** - the live API rejects `DELETE` on an enabled workflow. Disable a " +
	"workflow (`enabled = false`) before destroying it.\n" +
	"- `definition` is a raw JSON string (`{\"start\": ..., \"steps\": {...}}`) because each step's shape varies by its " +
	"own `type` (action/approval/success/etc.) with genuinely free-form `additionalProperties`. See " +
	"https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects.\n" +
	"- `trigger.attributes` is likewise a raw JSON string, since its shape depends entirely on the sibling " +
	"`trigger.type` (`EVENT` -> `{id, filter.$, description, attributeToFilter, formDefinitionId}`, `EXTERNAL` -> " +
	"`{name, description, clientId, url}`, `SCHEDULED` -> `{frequency, timeZone, cronString, weeklyDays, weeklyTimes, " +
	"yearlyTimes}`). See https://developer.sailpoint.com/docs/extensibility/event-triggers/available for event " +
	"trigger ids.\n" +
	"- Update uses a full `PUT` (replacing every mutable field at once) rather than `PATCH`/JSON-Patch, mirroring " +
	"transform_v1's identical choice - simpler, and every field workflows expose is mutable via `PUT` per the API's " +
	"own docs.\n" +
	"- Phase B (live `terraform plan`/`apply` against a real sandbox tenant) is a pending follow-up for this pilot - " +
	"see the pipeline task's final report for details."

var (
	_ resource.Resource                = (*workflowResource)(nil)
	_ resource.ResourceWithConfigure   = (*workflowResource)(nil)
	_ resource.ResourceWithImportState = (*workflowResource)(nil)
)

func NewWorkflowResource() resource.Resource {
	return &workflowResource{}
}

type workflowResource struct {
	client *sailpoint.APIClient
}

// triggerModel is the hand-written model for the "trigger" nested block
// (schema.ignores'd in full - see the package doc). Backed by a plain
// types.Object at the parent model level (workflowResourceModel.Trigger),
// following segment_v1's "visibility_criteria" precedent for a fully
// hand-rolled nested block.
type triggerModel struct {
	Type        types.String         `tfsdk:"type"`
	DisplayName types.String         `tfsdk:"display_name"`
	Attributes  jsontypes.Normalized `tfsdk:"attributes"`
}

func triggerAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":         types.StringType,
		"display_name": types.StringType,
		"attributes":   jsontypes.NormalizedType{},
	}
}

// workflowResourceModel mirrors resource_workflow.WorkflowModel's
// generator-managed fields (owner/creator/modified_by reuse the generated
// custom value types directly - they were `associated_external_type` mapped,
// not hand-written) plus the two hand-added fields the generator was told
// to ignore in full ("definition", "trigger" - see the package doc). Kept as
// a distinct, hand-written struct (rather than embedding the generated
// model) since Go doesn't allow adding a field to an imported struct type,
// and req.Plan.Get/resp.State.Set match purely on `tfsdk` tags, not on which
// struct type declares them.
type workflowResourceModel struct {
	Created        types.String                      `tfsdk:"created"`
	Creator        resource_workflow.CreatorValue    `tfsdk:"creator"`
	Definition     jsontypes.Normalized              `tfsdk:"definition"`
	Description    types.String                      `tfsdk:"description"`
	Enabled        types.Bool                        `tfsdk:"enabled"`
	ExecutionCount types.Int64                       `tfsdk:"execution_count"`
	FailureCount   types.Int64                       `tfsdk:"failure_count"`
	Id             types.String                      `tfsdk:"id"`
	Modified       types.String                      `tfsdk:"modified"`
	ModifiedBy     resource_workflow.ModifiedByValue `tfsdk:"modified_by"`
	Name           types.String                      `tfsdk:"name"`
	Owner          resource_workflow.OwnerValue      `tfsdk:"owner"`
	Trigger        types.Object                      `tfsdk:"trigger"`
}

func (r *workflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_v1"
}

func (r *workflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_workflow.WorkflowResourceSchema(ctx)
	resp.Schema.Description = "Manages a Workflow in IdentityNow/ISC. Workflows automate repeatable processes " +
		"(e.g. sending notifications, calling external systems) in response to an event, schedule, or external trigger."
	resp.Schema.MarkdownDescription = "Manages a [Workflow](https://developer.sailpoint.com/docs/extensibility/workflows/) " +
		"in IdentityNow/ISC. Workflows automate repeatable processes (e.g. sending notifications, calling external " +
		"systems) in response to an event, schedule, or external trigger.\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations.\n\n" +
		workflowGuidanceMarkdown
	applyWorkflowDefinitionField(&resp.Schema.Attributes)
	applyWorkflowTriggerField(&resp.Schema.Attributes)
	applyWorkflowUseStateForUnknown(&resp.Schema)
}

func (r *workflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *workflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Workflow", map[string]interface{}{"name": plan.Name.ValueString()})

	owner, diags := plan.Owner.ToApi_betaWorkflowBodyOwner(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if owner == nil {
		owner = api_beta.NewWorkflowBodyOwner()
	}

	createReq := api_beta.NewCreateWorkflowRequest(plan.Name.ValueString(), *owner)

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		createReq.Description = plan.Description.ValueStringPointer()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		createReq.Enabled = plan.Enabled.ValueBoolPointer()
	}

	def, diags := definitionToAPI(plan.Definition)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.Definition = def

	trg, diags := triggerObjectToAPI(ctx, plan.Trigger)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.Trigger = trg

	apiResp, httpResp, err := r.client.Beta.WorkflowsAPI.
		CreateWorkflow(ctx).
		CreateWorkflowRequest(*createReq).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Workflow", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Workflow", errDetail(err, httpResp))
		return
	}

	state, diags := workflowDtoToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Workflow", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Workflow", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.Beta.WorkflowsAPI.
		GetWorkflow(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Workflow not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Workflow", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Workflow", errDetail(err, httpResp))
		return
	}

	newState, diags := workflowDtoToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Workflow", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *workflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workflowResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state workflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Workflow", map[string]interface{}{"id": state.Id.ValueString()})

	// A full PUT replacing the whole document is simplest here - the API's
	// PUT is documented as "a full update," and unlike transform_v1's PUT
	// (where name/type are actually immutable), every field a workflow
	// exposes is mutable via PUT. See the package doc for why PATCH/JSON
	// Patch was not used instead.
	body := api_beta.NewWorkflowBody()
	body.Name = plan.Name.ValueStringPointer()

	owner, diags := plan.Owner.ToApi_betaWorkflowBodyOwner(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Owner = owner

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body.Description = plan.Description.ValueStringPointer()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		body.Enabled = plan.Enabled.ValueBoolPointer()
	}

	def, diags := definitionToAPI(plan.Definition)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Definition = def

	trg, diags := triggerObjectToAPI(ctx, plan.Trigger)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body.Trigger = trg

	apiResp, httpResp, err := r.client.Beta.WorkflowsAPI.
		PutWorkflow(ctx, state.Id.ValueString()).
		WorkflowBody(*body).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Workflow", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Workflow", errDetail(err, httpResp))
		return
	}

	newState, diags := workflowDtoToModel(ctx, apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Workflow", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *workflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Workflow", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.Beta.WorkflowsAPI.
		DeleteWorkflow(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Workflow already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Workflow", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError(
			"Error deleting Workflow",
			errDetail(err, httpResp)+" Note: enabled workflows cannot be deleted - disable the workflow (enabled = false) first.",
		)
		return
	}

	tflog.Info(ctx, "Deleted Workflow", map[string]interface{}{"id": state.Id.ValueString()})
}

// workflowDtoToModel converts an api_beta.Workflow API response into the
// resource's state model.
func workflowDtoToModel(ctx context.Context, dto *api_beta.Workflow) (workflowResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := workflowResourceModel{
		Creator:    resource_workflow.NewCreatorValueNull(),
		ModifiedBy: resource_workflow.NewModifiedByValueNull(),
		Owner:      resource_workflow.NewOwnerValueNull(),
		Trigger:    types.ObjectNull(triggerAttrTypes()),
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
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	} else {
		model.Description = types.StringNull()
	}
	if dto.Enabled != nil {
		model.Enabled = types.BoolValue(*dto.Enabled)
	}
	if dto.ExecutionCount != nil {
		model.ExecutionCount = types.Int64Value(int64(*dto.ExecutionCount))
	}
	if dto.FailureCount != nil {
		model.FailureCount = types.Int64Value(int64(*dto.FailureCount))
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	if dto.Owner != nil {
		owner, d := resource_workflow.NewOwnerValueNull().FromApi_betaWorkflowBodyOwner(ctx, dto.Owner)
		diags.Append(d...)
		model.Owner = owner
	}
	if dto.Creator != nil {
		creator, d := resource_workflow.NewCreatorValueNull().FromApi_betaWorkflowAllOfCreator(ctx, dto.Creator)
		diags.Append(d...)
		model.Creator = creator
	}
	if dto.ModifiedBy != nil {
		modifiedBy, d := resource_workflow.NewModifiedByValueNull().FromApi_betaWorkflowModifiedBy(ctx, dto.ModifiedBy)
		diags.Append(d...)
		model.ModifiedBy = modifiedBy
	}

	def, d := definitionFromAPI(dto.Definition)
	diags.Append(d...)
	model.Definition = def

	trg, d := triggerAPIToObject(ctx, dto.Trigger)
	diags.Append(d...)
	model.Trigger = trg

	return model, diags
}

// definitionToAPI decodes the practitioner-supplied "definition" JSON string
// into an *api_beta.WorkflowDefinition. A null/unknown/empty value returns
// nil (definition genuinely omitted - valid for a workflow with no steps
// configured yet).
func definitionToAPI(v jsontypes.Normalized) (*api_beta.WorkflowDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, diags
	}
	var def api_beta.WorkflowDefinition
	if err := json.Unmarshal([]byte(v.ValueString()), &def); err != nil {
		diags.AddError(
			"Invalid \"definition\" JSON",
			fmt.Sprintf("Could not decode \"definition\" as a JSON object: %s", err.Error()),
		)
		return nil, diags
	}
	return &def, diags
}

// definitionFromAPI re-encodes an API-returned *api_beta.WorkflowDefinition
// as a jsontypes.Normalized JSON string.
func definitionFromAPI(def *api_beta.WorkflowDefinition) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if def == nil {
		return jsontypes.NewNormalizedNull(), diags
	}
	b, err := json.Marshal(def)
	if err != nil {
		diags.AddError(
			"Error encoding \"definition\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"definition\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

// triggerObjectToAPI converts the hand-written "trigger" types.Object
// (schema-native, see the package doc) into an *api_beta.WorkflowTrigger. A
// null/unknown object returns nil (no trigger configured).
func triggerObjectToAPI(ctx context.Context, obj types.Object) (*api_beta.WorkflowTrigger, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}

	var m triggerModel
	diags.Append(obj.As(ctx, &m, basetypesObjectAsOptions())...)
	if diags.HasError() {
		return nil, diags
	}

	var attrs map[string]interface{}
	if !m.Attributes.IsNull() && !m.Attributes.IsUnknown() && m.Attributes.ValueString() != "" {
		if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
			diags.AddError(
				"Invalid \"trigger.attributes\" JSON",
				fmt.Sprintf("Could not decode \"trigger.attributes\" as a JSON object: %s", err.Error()),
			)
			return nil, diags
		}
	} else {
		attrs = map[string]interface{}{}
	}

	trg := api_beta.NewWorkflowTrigger(m.Type.ValueString(), attrs)
	if !m.DisplayName.IsNull() && !m.DisplayName.IsUnknown() {
		trg.DisplayName = *api_beta.NewNullableString(m.DisplayName.ValueStringPointer())
	}
	return trg, diags
}

// triggerAPIToObject converts an API-returned *api_beta.WorkflowTrigger into
// the hand-written "trigger" types.Object.
func triggerAPIToObject(ctx context.Context, trg *api_beta.WorkflowTrigger) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if trg == nil {
		return types.ObjectNull(triggerAttrTypes()), diags
	}

	attrsJSON, err := json.Marshal(trg.Attributes)
	if err != nil {
		diags.AddError(
			"Error encoding \"trigger.attributes\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"trigger.attributes\" value as JSON: %s", err.Error()),
		)
		return types.ObjectNull(triggerAttrTypes()), diags
	}

	m := triggerModel{
		Type:       types.StringValue(trg.Type),
		Attributes: jsontypes.NewNormalizedValue(string(attrsJSON)),
	}
	if name := trg.DisplayName.Get(); name != nil {
		m.DisplayName = types.StringValue(*name)
	} else {
		m.DisplayName = types.StringNull()
	}

	obj, d := types.ObjectValueFrom(ctx, triggerAttrTypes(), m)
	diags.Append(d...)
	return obj, diags
}

// applyWorkflowDefinitionField hand-adds the "definition" attribute that
// generator_config_workflow_v1.yml deliberately tells tfplugingen-openapi/
// -framework to ignore (see the package doc). Optional+Computed since a
// workflow can exist without a definition (e.g. a placeholder/template
// workflow) and the API doesn't require one at create time. The data
// source's Computed-only equivalent is applyWorkflowDataSourceDefinitionField
// in resource_workflow_planmodifiers.go.
func applyWorkflowDefinitionField(attrs *map[string]resourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]resourceschema.Attribute{}
	}
	desc := "The map of steps that the workflow will execute, as a raw JSON object (`{\"start\": \"...\", \"steps\": " +
		"{...}}`). Each step's own shape varies by its \"type\" - see " +
		"https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects."
	(*attrs)["definition"] = resourceschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Optional:            true,
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

// applyWorkflowUseStateForUnknown patches resource_workflow.WorkflowResourceSchema's
// generated Attributes map with plan modifiers on Computed-only attributes
// with no schema Default - the systemic tfplugingen-framework gap already
// documented for role_v1/service_desk_integration_v1/transform_v1 (it never
// emits any PlanModifiers, so every unconfigured Computed attribute is
// proposed as Unknown on every plan, not just when it actually changes).
func applyWorkflowUseStateForUnknown(s *resourceschema.Schema) {
	for _, name := range []string{"id", "created", "modified"} {
		if a, ok := s.Attributes[name].(resourceschema.StringAttribute); ok {
			a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
			s.Attributes[name] = a
		}
	}
}

// errDetail delegates to the shared util.SailpointErrorDetail helper so all
// _v1 targets surface the same richer detail (HTTP status, detailCode,
// trackingId, and message text) in resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}

// timeToStringValue converts an *api_beta.SailPointTime (a thin wrapper
// around time.Time) to an RFC3339 types.String, or types.StringNull() if
// nil - matching the existing pattern used by sources_v1/access_profile_v1
// for their own Created/Modified fields (see the SDK type-shape reference
// catalog's WorkgroupDto entry).
func timeToStringValue(t *api_beta.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
