// Package transform_v1 is a pilot implementation of the transform
// resource/data source generated from SailPoint's new per-service v1 OpenAPI
// spec (api-specs/idn/apis/transforms), following the same hand-written CRUD
// pattern established by role_v1/service_desk_integration_v1.
//
// These hand-written wrappers implement resource.Resource / datasource.DataSource
// around the generated schema/model types in resource_transform and
// datasource_transform, backed by the golang-sdk v3 transforms.TransformsAPI
// client (the SDK does not yet publish a per-service v1 package; v1 is the
// stabilization of what was beta).
//
// Codegen note: the spec's create/get/put responses wrap the Transform object
// in a top-level `allOf` (base Transform properties + an {id, internal}
// wrapper) that tfplugingen-openapi cannot decompose ("schema composition is
// currently not supported"). This was resolved by flattening the 4 affected
// `allOf` occurrences directly in api-specs/dereferenced/deref-transforms.v1.yaml
// (a one-time, scripted edit - see the 2026-07-24 knowledge entry) before
// running `make gen-api-v1`; if this spec is ever re-bundled from a newer
// upstream revision, that flattening step must be re-applied first.
//
// "attributes" dynamic-shape decision (see the 2026-07-24
// dynamic-attributes-pattern-research entry in
// .github/agents/identitynow-terraform-provider-developer.knowledge.md):
// transform's "attributes" field is a discriminated union across ~35 "type"
// values, several of which have genuinely arbitrary-depth children (every
// variant's "input" property is `type: object, additionalProperties: true`
// holding another full nested transform definition; "concat"'s "values" is an
// array of arbitrary objects). Enumerating all ~35 variants as static nested
// blocks (the pipeline's usual oneOf/discriminator handling) was judged
// impractical and would still not model the recursive "input"/"values"
// depth. "attributes" is therefore hand-added (via schema.ignores in
// generator_config_transform_v1.yml + applyTransformAttributesField in
// resource_transform_planmodifiers.go) as a jsontypes.Normalized JSON-string
// CustomType - HashiCorp's documented alternative to schema.DynamicAttribute
// for exactly this "arbitrary/polymorphic JSON blob" scenario. Practitioners
// write raw JSON matching whatever shape the transform's "type" requires;
// jsontypes.Normalized provides semantic (not textual) equality, so
// whitespace/key-ordering differences between config and the API's
// round-tripped response don't produce false diffs.
package transform_v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/transforms"

	"terraform-provider-identitynow/internal/provider/transform_v1/resource_transform"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

// transformGuidanceMarkdown is shared between the resource and data source
// schema descriptions. Synthesized from (a) SailPoint's official transforms
// guide (https://developer.sailpoint.com/docs/extensibility/transforms/ and
// .../operations, fetched 2026-07-24) and (b) a live, read-only listing of 50
// real transforms from a sandbox tenant (2026-07-24) - see the matching
// knowledge-file entry for the raw sample data this was derived from.
const transformGuidanceMarkdown = "" +
	"### Working with \"attributes\"\n\n" +
	"`attributes` is a raw JSON string (via `jsontypes.Normalized`) because its shape is a discriminated union keyed by " +
	"`type` - each of the ~35 supported `type` values expects different sub-properties, and several (e.g. `lower`, " +
	"`concat`, `lookup`, `replaceAll`) can nest another full transform definition arbitrarily deep via an `input` " +
	"sub-property (implicit input if omitted, explicit input if a nested `{\"type\": ..., \"attributes\": {...}}` object " +
	"is supplied). There is no hard nesting limit, though SailPoint's own guidance cautions that deeply nested transforms " +
	"become harder to read/maintain, and the whole transform document cannot exceed 400KB. A real 3-level nested example " +
	"observed in a live tenant: a `lower` transform whose `input` was a `concat` transform, whose `input` was itself a " +
	"`lookup` transform keyed off two `identityAttribute` values - i.e. `lower(lookup(concat(identityAttribute, identityAttribute)))`.\n\n" +
	"Only the root-level transform's `name` is meaningful - nested transform objects passed via `attributes.input`/`attributes.values` " +
	"do not have (and should not be given) their own `name`.\n\n" +
	"### `type` string caveats\n\n" +
	"Validate the literal `type` string against this schema's `stringvalidator.OneOf(...)` list rather than against " +
	"SailPoint's human-readable operation names in the UI/docs - a handful of documented operation names (e.g. \"Join\", " +
	"\"Get End of String\", \"Generate Random String\") do not have a distinct, matching `type` enum value in the current " +
	"v1 API; some are instead reachable as a specific `attributes.operation` value on a `type = \"rule\"` transform " +
	"(confirmed live: a `type = \"rule\"` transform with `attributes.operation = \"getReferenceIdentityAttribute\"`) rather " +
	"than being their own top-level `type`.\n\n" +
	"### `internal` (SailPoint-managed) transforms\n\n" +
	"Transforms with `internal = true` (SailPoint-managed built-ins, e.g. `ToUpper`/`Remove Diacritical Marks`) were " +
	"observed live to omit the `attributes` key entirely from `GET` responses rather than returning `{}`. This provider " +
	"normalizes that omission to an empty JSON object (`\"{}\"`) in state/data-source output for predictable diffing; " +
	"these built-ins are not expected to be practitioner-managed via this resource (attempting to `Update`/`Delete` one " +
	"has not been tested and may be rejected by the API)."

var (
	_ resource.Resource                = (*transformResource)(nil)
	_ resource.ResourceWithConfigure   = (*transformResource)(nil)
	_ resource.ResourceWithImportState = (*transformResource)(nil)
)

func NewTransformResource() resource.Resource {
	return &transformResource{}
}

type transformResource struct {
	client *sailpoint.APIClient
}

// transformResourceModel mirrors resource_transform.TransformModel plus the
// hand-added "attributes" field the generator was told to ignore (see package
// doc). Kept as a distinct, hand-written struct (rather than embedding the
// generated model) since Go doesn't allow adding a field to an imported
// struct type, and req.Plan.Get/resp.State.Set match purely on `tfsdk` tags,
// not on which struct type declares them.
type transformResourceModel struct {
	Id         types.String         `tfsdk:"id"`
	Internal   types.Bool           `tfsdk:"internal"`
	Name       types.String         `tfsdk:"name"`
	Type       types.String         `tfsdk:"type"`
	Attributes jsontypes.Normalized `tfsdk:"attributes"`
}

func (r *transformResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transform_v1"
}

func (r *transformResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_transform.TransformResourceSchema(ctx)
	resp.Schema.Description = "Manages a Transform in IdentityNow/ISC. Transforms manipulate attribute data (e.g. from a " +
		"source account or identity) without requiring custom rule code."
	resp.Schema.MarkdownDescription = "Manages a [Transform](https://developer.sailpoint.com/docs/extensibility/transforms/) " +
		"in IdentityNow/ISC. Transforms manipulate attribute data (e.g. from a source account or identity) without requiring " +
		"custom rule code.\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below before relying on it in " +
		"production configurations.\n\n" +
		transformGuidanceMarkdown
	applyTransformAttributesField(&resp.Schema.Attributes, true)
	applyTransformUseStateForUnknown(&resp.Schema)
}

func (r *transformResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *transformResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *transformResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan transformResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Transform", map[string]interface{}{"name": plan.Name.ValueString(), "type": plan.Type.ValueString()})

	attrs, diags := attributesToMap(plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto := transforms.NewTransform(plan.Name.ValueString(), plan.Type.ValueString(), attrs)

	apiResp, httpResp, err := r.client.TransformsAPI.
		CreateTransformV1(ctx).
		Transform(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Transform", map[string]interface{}{"name": plan.Name.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Transform", errDetail(err, httpResp))
		return
	}

	state, diags := transformReadToModel(apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Transform", map[string]interface{}{"id": state.Id.ValueString(), "name": state.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *transformResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state transformResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Transform", map[string]interface{}{"id": state.Id.ValueString()})

	apiResp, httpResp, err := r.client.TransformsAPI.
		GetTransformV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Transform not found, removing from state", map[string]interface{}{"id": state.Id.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Transform", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Transform", errDetail(err, httpResp))
		return
	}

	newState, diags := transformReadToModel(apiResp, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Transform", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *transformResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan transformResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state transformResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Transform", map[string]interface{}{"id": state.Id.ValueString()})

	// Per the API's own description: "Only the 'attributes' field is
	// mutable. Attempting to change other properties (ex. 'name' and 'type')
	// will result in an error." A full PUT replacing the whole document is
	// simplest and correct here since name/type are immutable anyway (any
	// config change to them will already have been rejected/require
	// replacement well before Update is reached, given they're plain
	// Required, non-ForceNew-marked strings today - tracked as a follow-up:
	// consider adding stringplanmodifier.RequiresReplace() to name/type once
	// this is promoted out of the _v1 pilot).
	attrs, diags := attributesToMap(plan.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto := transforms.NewTransform(plan.Name.ValueString(), plan.Type.ValueString(), attrs)

	apiResp, httpResp, err := r.client.TransformsAPI.
		UpdateTransformV1(ctx, state.Id.ValueString()).
		Transform(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Transform", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Transform", errDetail(err, httpResp))
		return
	}

	newState, diags := transformReadToModel(apiResp, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Transform", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *transformResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state transformResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Transform", map[string]interface{}{"id": state.Id.ValueString()})

	httpResp, err := r.client.TransformsAPI.
		DeleteTransformV1(ctx, state.Id.ValueString()).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Transform already absent on delete", map[string]interface{}{"id": state.Id.ValueString()})
			return
		}
		tflog.Error(ctx, "Error deleting Transform", map[string]interface{}{"id": state.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Transform", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Transform", map[string]interface{}{"id": state.Id.ValueString()})
}

// attributesToMap decodes the practitioner-supplied "attributes" JSON string
// into a map[string]interface{} suitable for transforms.NewTransform. A null
// jsontypes.Normalized (attribute genuinely omitted, though it's schema.Required
// so this should only occur for an empty-object literal like "{}") decodes to
// an empty map rather than nil, since the API requires "attributes" to be
// present (even if empty) on every request.
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

// transformReadToModel converts an transforms.TransformRead API response into
// the resource's state model. "attributes" is round-tripped through
// jsontypes.NewNormalizedValue so the state reflects the API's canonical
// representation (useful for drift detection on transform types where the
// API might reorder/default sub-keys), while jsontypes.Normalized's semantic
// equality still avoids false diffs from whitespace/key-ordering alone.
func transformReadToModel(dto *transforms.TransformRead, fallback transformResourceModel) (transformResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.Id = types.StringValue(dto.Id)
	model.Internal = types.BoolValue(dto.Internal)
	model.Name = types.StringValue(dto.Name)
	model.Type = types.StringValue(dto.Type)

	model.Attributes, diags = normalizedAttributesFromAPI(dto.Attributes)

	return model, diags
}

// normalizedAttributesFromAPI re-encodes an API-returned "attributes" map as
// a jsontypes.Normalized JSON string. Confirmed via a live read-only listing
// against a sandbox tenant (2026-07-24): SailPoint-managed "internal": true
// transforms (e.g. the built-in "ToUpper"/"Remove Diacritical Marks") can
// omit "attributes" from the response body entirely rather than returning
// "{}", which would otherwise decode to a nil Go map and marshal to the JSON
// literal "null" (technically valid JSON, but an odd/surprising value to
// diff against a schema.Required "attributes" that practitioner configs
// always populate as a real object). Normalize nil to an empty JSON object
// instead so drift/import behavior against these built-ins stays predictable.
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
// from the role_v1/service_desk_integration_v1 pilots) so all _v1 targets
// surface the same richer detail (HTTP status, detailCode, trackingId, and
// message text) in resp.Diagnostics.AddError output.
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
