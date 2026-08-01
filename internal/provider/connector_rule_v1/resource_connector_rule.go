// Package connector_rule_v1 is a pilot implementation of the connector rule
// resource/data source generated from SailPoint's per-service v1 OpenAPI spec
// (api-specs/idn/apis/connector-rule-management), following the same
// hand-written CRUD pattern established by role_v1/transform_v1. This is an
// "extra" target with no counterpart in the reference davidsonjon/identitynow
// provider - see the dated knowledge.md entry for the full pipeline writeup.
//
// Codegen note: the spec's ConnectorRuleResponse wraps ConnectorRuleCreateRequest's
// properties plus {id, created, modified} in a top-level `allOf` that
// tfplugingen-openapi cannot decompose. This was resolved by flattening the 5
// affected `allOf` occurrences directly in
// api-specs/dereferenced/deref-connector-rule-management.v1.yaml (via
// scripts/flatten_openapi_allof.py, the same script/approach used for
// transform_v1) before running `make gen-api-v1`; if this spec is ever
// re-bundled from a newer upstream revision, that flattening step must be
// re-applied first.
//
// Type-mapping note: generator_config_connector_rule_v1.yml intentionally
// applies zero type_mappings (no type_mappings_connector_rule_v1.yml file
// exists). Two candidates were investigated and rejected:
//   - "source_code" (connector_rule_management.SourceCode{Version, Script string}) looked safe
//     at a glance (plain strings, no Nullable fields), but tfplugingen-framework's
//     generated single_nested To/FromApi converters unconditionally call
//     .ValueStringPointer()/types.StringPointerValue() on every leaf field,
//     which only compiles if the SDK struct's fields are *string pointers -
//     connector_rule_management.SourceCode's fields are plain (non-pointer) strings, so mapping
//     it broke the build. This is a new, distinct sub-case of the "leaf-only
//     mapping" pitfall (previously only the NullableString-incompatibility case
//     was documented) - see the knowledge.md entry.
//   - "signature"/its nested "input"/"output" (connector_rule_management.Argument) has a
//     Type field typed NullableString, the same pre-existing incompatibility
//     as sdk-issues.md #4 (AdditionalOwnerRef.Name/EntitlementRef.Name).
//
// As a result "signature" and "source_code" remain generated single_nested
// blocks (resource_connector_rule.SignatureValue/SourceCodeValue etc.), and
// this file hand-converts between those generated Value types and the SDK's
// connector_rule_management.ConnectorRuleCreateRequestSignature/SourceCode/Argument types.
package connector_rule_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/connector_rule_management"

	"terraform-provider-identitynow/internal/provider/connector_rule_v1/resource_connector_rule"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

// connectorRuleGuidanceMarkdown is shared between the resource and data
// source schema descriptions.
const connectorRuleGuidanceMarkdown = "" +
	"### `name`/`type` are immutable\n\n" +
	"The API documents `name` and `type` as immutable once a connector rule is created; only `description`, " +
	"`signature`, `source_code`, and `attributes` can be changed via a subsequent update. This resource enforces that " +
	"by requiring replacement whenever `name` or `type` changes in config, rather than sending a PUT the API might " +
	"reject.\n\n" +
	"### `attributes`\n\n" +
	"`attributes` is a raw JSON object (via `jsontypes.Normalized`) with no fixed shape - unlike `identitynow_transform_v1`'s " +
	"`attributes` (a discriminated union keyed by `type`), the connector rule API documents this simply as an opaque " +
	"`map[string]object`. Practitioners write raw JSON; drift detection is semantic (not textual) JSON comparison."

var (
	_ resource.Resource                = (*connectorRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*connectorRuleResource)(nil)
	_ resource.ResourceWithImportState = (*connectorRuleResource)(nil)
)

func NewConnectorRuleResource() resource.Resource {
	return &connectorRuleResource{}
}

type connectorRuleResource struct {
	client *sailpoint.APIClient
}

// connectorRuleResourceModel mirrors resource_connector_rule.ConnectorRuleModel
// plus the hand-added "attributes" field the generator was told to ignore
// (see the package doc and generator_config_connector_rule_v1.yml). Kept as a
// distinct, hand-written struct (rather than embedding the generated model)
// since Go doesn't allow adding a field to an imported struct type, and
// req.Plan.Get/resp.State.Set match purely on `tfsdk` tags, not on which
// struct type declares them.
type connectorRuleResourceModel struct {
	Created     types.String                            `tfsdk:"created"`
	Description types.String                            `tfsdk:"description"`
	Id          types.String                            `tfsdk:"id"`
	Modified    types.String                            `tfsdk:"modified"`
	Name        types.String                            `tfsdk:"name"`
	Signature   resource_connector_rule.SignatureValue  `tfsdk:"signature"`
	SourceCode  resource_connector_rule.SourceCodeValue `tfsdk:"source_code"`
	Type        types.String                            `tfsdk:"type"`
	Attributes  jsontypes.Normalized                    `tfsdk:"attributes"`
}

func (r *connectorRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_rule_v1"
}

func (r *connectorRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_connector_rule.ConnectorRuleResourceSchema(ctx)
	resp.Schema.Description = "Manages a Connector Rule in IdentityNow/ISC. Connector rules are custom logic invoked " +
		"during connector operations (e.g. before/after account provisioning, build map)."
	resp.Schema.MarkdownDescription = "Manages a [Connector Rule](https://developer.sailpoint.com/docs/extensibility/rules/) " +
		"in IdentityNow/ISC. Connector rules are custom logic invoked during connector operations (e.g. before/after " +
		"account provisioning, build map).\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations.\n\n" +
		connectorRuleGuidanceMarkdown
	applyConnectorRuleAttributesField(&resp.Schema.Attributes, true)
	applyConnectorRuleUseStateForUnknown(&resp.Schema)
	applyConnectorRuleRequiresReplace(&resp.Schema)
}

func (r *connectorRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectorRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *connectorRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectorRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Connector Rule", map[string]interface{}{"name": plan.Name.ValueString(), "type": plan.Type.ValueString()})

	sourceCode, diags := sourceCodeModelToAPI(plan.SourceCode)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	signature, diags := signatureModelToAPI(ctx, plan.Signature)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := attributesToMap(plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto := connector_rule_management.NewConnectorRuleCreateRequest(plan.Name.ValueString(), plan.Type.ValueString(), *sourceCode)
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		dto.SetDescription(plan.Description.ValueString())
	}
	if signature != nil {
		dto.SetSignature(*signature)
	}
	dto.SetAttributes(attrs)

	apiResp, httpResp, err := r.client.ConnectorRuleManagementAPI.
		CreateConnectorRuleV1(ctx).
		ConnectorRuleCreateRequest(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Connector Rule", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Connector Rule", errDetail(err, httpResp))
		return
	}

	state, diags := connectorRuleResponseToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Connector Rule", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectorRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Connector Rule", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.ConnectorRuleManagementAPI.
		GetConnectorRuleV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Connector Rule not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Connector Rule", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Connector Rule", errDetail(err, httpResp))
		return
	}

	newState, diags := connectorRuleResponseToModel(ctx, apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Connector Rule", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *connectorRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan connectorRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state connectorRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Connector Rule", map[string]interface{}{"id": state.Id.ValueString()})

	sourceCode, diags := sourceCodeModelToAPI(plan.SourceCode)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	signature, diags := signatureModelToAPI(ctx, plan.Signature)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := attributesToMap(plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto := connector_rule_management.NewConnectorRuleUpdateRequest(plan.Name.ValueString(), plan.Type.ValueString(), *sourceCode, state.Id.ValueString())
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		dto.SetDescription(plan.Description.ValueString())
	}
	if signature != nil {
		dto.SetSignature(*signature)
	}
	dto.SetAttributes(attrs)

	apiResp, httpResp, err := r.client.ConnectorRuleManagementAPI.
		PutConnectorRuleV1(ctx, state.Id.ValueString()).
		ConnectorRuleUpdateRequest(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Connector Rule", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Connector Rule", errDetail(err, httpResp))
		return
	}

	newState, diags := connectorRuleResponseToModel(ctx, apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Connector Rule", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *connectorRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Connector Rule", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.ConnectorRuleManagementAPI.
		DeleteConnectorRuleV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Connector Rule already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Connector Rule", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Connector Rule", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Connector Rule", map[string]interface{}{"id": state.Id.ValueString()})
}

// sourceCodeModelToAPI converts the generated SourceCodeValue (a
// schema.Required single_nested block, so always known/non-null once past
// plan validation) into connector_rule_management.SourceCode.
func sourceCodeModelToAPI(v resource_connector_rule.SourceCodeValue) (*connector_rule_management.SourceCode, diag.Diagnostics) {
	var diags diag.Diagnostics
	return connector_rule_management.NewSourceCode(v.Version.ValueString(), v.Script.ValueString()), diags
}

// sourceCodeFromAPI converts an connector_rule_management.SourceCode API response into the
// generated SourceCodeValue.
func sourceCodeFromAPI(ctx context.Context, dto connector_rule_management.SourceCode) (resource_connector_rule.SourceCodeValue, diag.Diagnostics) {
	return resource_connector_rule.NewSourceCodeValue(
		resource_connector_rule.SourceCodeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"script":  types.StringValue(dto.Script),
			"version": types.StringValue(dto.Version),
		},
	)
}

// signatureModelToAPI converts the generated SignatureValue (a schema.Optional+Computed
// single_nested block) into *connector_rule_management.ConnectorRuleCreateRequestSignature,
// returning nil if the practitioner omitted "signature" entirely.
func signatureModelToAPI(ctx context.Context, v resource_connector_rule.SignatureValue) (*connector_rule_management.ConnectorRuleCreateRequestSignature, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() {
		return nil, diags
	}
	if v.IsUnknown() {
		diags.AddError("Unknown signature value", "\"signature\" must be fully known before the connector rule can be sent to the API.")
		return nil, diags
	}

	input, d := argumentListModelToAPI(ctx, v.Input)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	sig := connector_rule_management.NewConnectorRuleCreateRequestSignature(input)

	output, d := argumentObjectModelToAPI(v.Output)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	if output != nil {
		sig.SetOutput(*output)
	}

	return sig, diags
}

// argumentListModelToAPI converts the generated "input" basetypes.ListValue
// (elements are resource_connector_rule.InputValue) into []connector_rule_management.Argument.
func argumentListModelToAPI(ctx context.Context, list basetypes.ListValue) ([]connector_rule_management.Argument, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return []connector_rule_management.Argument{}, diags
	}

	var items []resource_connector_rule.InputValue
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make([]connector_rule_management.Argument, 0, len(items))
	for _, item := range items {
		arg := connector_rule_management.NewArgument(item.Name.ValueString())
		if !item.Description.IsNull() && !item.Description.IsUnknown() {
			arg.SetDescription(item.Description.ValueString())
		}
		if !item.InputType.IsNull() && !item.InputType.IsUnknown() {
			arg.SetType(item.InputType.ValueString())
		}
		out = append(out, *arg)
	}
	return out, diags
}

// argumentObjectModelToAPI converts the generated "output" basetypes.ObjectValue
// (a single resource_connector_rule.OutputValue) into *connector_rule_management.Argument,
// returning nil if the practitioner omitted "output" entirely.
func argumentObjectModelToAPI(obj basetypes.ObjectValue) (*connector_rule_management.Argument, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() {
		return nil, diags
	}
	if obj.IsUnknown() {
		diags.AddError("Unknown signature.output value", "\"signature.output\" must be fully known before the connector rule can be sent to the API.")
		return nil, diags
	}

	attrs := obj.Attributes()
	name, _ := attrs["name"].(basetypes.StringValue)
	description, _ := attrs["description"].(basetypes.StringValue)
	outputType, _ := attrs["type"].(basetypes.StringValue)

	arg := connector_rule_management.NewArgument(name.ValueString())
	if !description.IsNull() && !description.IsUnknown() {
		arg.SetDescription(description.ValueString())
	}
	if !outputType.IsNull() && !outputType.IsUnknown() {
		arg.SetType(outputType.ValueString())
	}
	return arg, diags
}

// signatureFromAPI converts an *connector_rule_management.ConnectorRuleCreateRequestSignature
// API response into the generated SignatureValue, returning a null
// SignatureValue if the API omitted "signature" entirely.
func signatureFromAPI(ctx context.Context, dto *connector_rule_management.ConnectorRuleCreateRequestSignature) (resource_connector_rule.SignatureValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_connector_rule.SignatureValue{}.AttributeTypes(ctx)
	if dto == nil {
		return resource_connector_rule.NewSignatureValueNull(), diags
	}

	input, d := argumentListFromAPI(ctx, dto.Input)
	diags.Append(d...)

	output, d := argumentObjectFromAPI(ctx, dto.Output)
	diags.Append(d...)

	v, d := resource_connector_rule.NewSignatureValue(
		attrTypes,
		map[string]attr.Value{
			"input":  input,
			"output": output,
		},
	)
	diags.Append(d...)
	return v, diags
}

// argumentListFromAPI converts []connector_rule_management.Argument into the generated
// "input" basetypes.ListValue of InputValue elements.
func argumentListFromAPI(ctx context.Context, items []connector_rule_management.Argument) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_connector_rule.InputValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_connector_rule.InputValue, 0, len(items))
	for _, item := range items {
		description := types.StringNull()
		if item.Description.IsSet() && item.Description.Get() != nil {
			description = types.StringValue(*item.Description.Get())
		}
		inputType := types.StringNull()
		if item.Type.IsSet() && item.Type.Get() != nil {
			inputType = types.StringValue(*item.Type.Get())
		}

		v, d := resource_connector_rule.NewInputValue(
			resource_connector_rule.InputValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"name":        types.StringValue(item.Name),
				"description": description,
				"type":        inputType,
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

// argumentObjectFromAPI converts a NullableArgument API response into the
// generated "output" basetypes.ObjectValue (an OutputValue), returning a null
// object if the API's "output" was absent/null.
func argumentObjectFromAPI(ctx context.Context, dto connector_rule_management.NullableArgument) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := resource_connector_rule.OutputValue{}.AttributeTypes(ctx)
	if !dto.IsSet() || dto.Get() == nil {
		return types.ObjectNull(attrTypes), diags
	}
	item := dto.Get()

	description := types.StringNull()
	if item.Description.IsSet() && item.Description.Get() != nil {
		description = types.StringValue(*item.Description.Get())
	}
	outputType := types.StringNull()
	if item.Type.IsSet() && item.Type.Get() != nil {
		outputType = types.StringValue(*item.Type.Get())
	}

	v, d := resource_connector_rule.NewOutputValue(
		attrTypes,
		map[string]attr.Value{
			"name":        types.StringValue(item.Name),
			"description": description,
			"type":        outputType,
		},
	)
	diags.Append(d...)
	obj, d := v.ToObjectValue(ctx)
	diags.Append(d...)
	return obj, diags
}

// connectorRuleResponseToModel converts an connector_rule_management.ConnectorRuleResponse API
// response into the resource's state model.
func connectorRuleResponseToModel(ctx context.Context, dto *connector_rule_management.ConnectorRuleResponse, fallback connectorRuleResourceModel) (connectorRuleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.Id = types.StringValue(dto.Id)
	model.Created = types.StringValue(dto.Created)
	model.Name = types.StringValue(dto.Name)
	model.Type = types.StringValue(dto.Type)

	if dto.Description.IsSet() && dto.Description.Get() != nil {
		model.Description = types.StringValue(*dto.Description.Get())
	} else {
		model.Description = types.StringNull()
	}

	if dto.Modified.IsSet() && dto.Modified.Get() != nil {
		model.Modified = types.StringValue(*dto.Modified.Get())
	} else {
		model.Modified = types.StringNull()
	}

	sourceCode, d := sourceCodeFromAPI(ctx, dto.SourceCode)
	diags.Append(d...)
	model.SourceCode = sourceCode

	signature, d := signatureFromAPI(ctx, dto.Signature)
	diags.Append(d...)
	model.Signature = signature

	// "attributes" is intentionally NOT overwritten from dto.Attributes here -
	// see applyConnectorRuleAttributesField's doc comment in
	// resource_connector_rule_planmodifiers.go: the API server-injects a
	// "sourceVersion" key, and reflecting that (or any other out-of-band
	// change) back into state would violate Terraform's Required-attribute
	// consistency rules. model.Attributes keeps the fallback (plan/prior
	// state) value passed in by the caller.

	return model, diags
}

// attributesToMap decodes the practitioner-supplied "attributes" JSON string
// into a map[string]interface{} suitable for connector_rule_management.ConnectorRuleCreateRequest/
// UpdateRequest. A null jsontypes.Normalized decodes to an empty map rather
// than nil, since the API accepts (and this resource always sends)
// "attributes" as a real object, even if empty.
func attributesToMap(v jsontypes.Normalized) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return map[string]interface{}{}, diags
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError(
			"Invalid \"attributes\" JSON",
			fmt.Sprintf("Could not decode \"attributes\" as a JSON object: %s", err.Error()),
		)
		return nil, diags
	}
	return m, diags
}

// normalizedAttributesFromAPI re-encodes an API-returned "attributes" map as
// a jsontypes.Normalized JSON string, normalizing a nil map (e.g. if the API
// omits "attributes" for a rule with no custom attributes) to an empty JSON
// object for predictable diffing - mirroring transform_v1's identical
// normalizedAttributesFromAPI helper.
func normalizedAttributesFromAPI(attrs map[string]interface{}) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if attrs == nil {
		attrs = map[string]interface{}{}
	}
	attrsJSON, err := json.Marshal(attrs)
	if err != nil {
		diags.AddError(
			"Error encoding \"attributes\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"attributes\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(attrsJSON)), diags
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (adopted
// from the role_v1/transform_v1 pilots) so all _v1 targets surface the same
// richer detail (HTTP status, detailCode, trackingId, and message text) in
// resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
