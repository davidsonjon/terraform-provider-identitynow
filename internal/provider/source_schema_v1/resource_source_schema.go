// Package source_schema_v1 is a pilot implementation of the Source Schema
// resource/data source generated from SailPoint's per-service v1 OpenAPI
// spec (api-specs/idn/apis/sources), covering the
// `/sources/v1/{sourceId}/schemas[/{schemaId}]` family of endpoints - a
// sub-resource of the existing sources_v1.SourceResource
// (identitynow_source_v1) that resource's own "Known Limitations" section
// previously deferred to "a future, separate resource/data source".
//
// Scope note: this target deliberately does NOT cover
// getAccountsSchemaV1/importAccountsSchemaV1/getEntitlementsSchemaV1/
// importEntitlementsSchemaV1 (the `/sources/v1/{id}/schemas/accounts` and
// `/sources/v1/{id}/schemas/entitlements` CSV template download/upload
// endpoints, scoped to Delimited File sources only). Those are `text/csv`
// download and `multipart/form-data` file-upload operations, not structured
// JSON CRUD - a poor fit for a Terraform resource/data source's declarative
// model, and out of scope for tfplugingen-openapi/-framework entirely.
//
// "configuration" dynamic-shape decision
// ----------------------------------------
// A Schema's "configuration" property is a free-form `type: object` bag
// ("Holds any extra configuration data that the schema may require.") with
// no fixed set of keys - the same "arbitrary JSON blob" shape as
// transform_v1's "attributes"/source_v1's "connectorAttributes". It is
// hand-added as a jsontypes.Normalized JSON-string CustomType, the
// established pattern (see the IdentityNow agent's Project Context
// "Dynamic/discriminated-union attributes-style fields" bullet).
//
// "id" / "schema_id" redundancy
// ------------------------------
// The Schema DTO has its own "id" property, and tfplugingen-openapi
// separately synthesizes a "schema_id" attribute from the {schemaId} path
// parameter (the two are always equal in practice - the API never lets them
// diverge). Both were forced Computed-only on the resource via
// schema_overrides_source_schema_v1.yml's computed_only_overrides (letting
// practitioners set either would be meaningless, since the server always
// assigns them and any configured value would be silently overwritten) -
// see resource_source_schema_planmodifiers.go. The data source keeps
// "schema_id" as its Required lookup key input, alongside "id" as another
// Computed output field with the same value.
package source_schema_v1

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

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/sources"

	"terraform-provider-identitynow/internal/provider/source_schema_v1/resource_source_schema"
	"terraform-provider-identitynow/internal/provider/util"
)

// clientProvider is satisfied by internal/provider.identitynowProvider without
// this package needing to import it (which would create an import cycle).
type clientProvider interface {
	GetClient() *sailpoint.APIClient
}

var (
	_ resource.Resource                = (*sourceSchemaResource)(nil)
	_ resource.ResourceWithConfigure   = (*sourceSchemaResource)(nil)
	_ resource.ResourceWithImportState = (*sourceSchemaResource)(nil)
)

func NewSourceSchemaResource() resource.Resource {
	return &sourceSchemaResource{}
}

type sourceSchemaResource struct {
	client *sailpoint.APIClient
}

// sourceSchemaResourceModel mirrors resource_source_schema.SourceSchemaModel
// plus the hand-added "configuration" field - kept as a distinct,
// hand-written struct since Go doesn't allow adding a field to an imported
// generated struct type (see transform_v1's identical rationale).
type sourceSchemaResourceModel struct {
	Attributes         types.List           `tfsdk:"attributes"`
	Configuration      jsontypes.Normalized `tfsdk:"configuration"`
	Created            types.String         `tfsdk:"created"`
	DisplayAttribute   types.String         `tfsdk:"display_attribute"`
	Features           types.List           `tfsdk:"features"`
	HierarchyAttribute types.String         `tfsdk:"hierarchy_attribute"`
	Id                 types.String         `tfsdk:"id"`
	IdentityAttribute  types.String         `tfsdk:"identity_attribute"`
	IncludePermissions types.Bool           `tfsdk:"include_permissions"`
	Modified           types.String         `tfsdk:"modified"`
	Name               types.String         `tfsdk:"name"`
	NativeObjectType   types.String         `tfsdk:"native_object_type"`
	SchemaId           types.String         `tfsdk:"schema_id"`
	SourceId           types.String         `tfsdk:"source_id"`
}

func (r *sourceSchemaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_schema_v1"
}

func (r *sourceSchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_source_schema.SourceSchemaResourceSchema(ctx)
	resp.Schema.Description = "Manages a Schema on an existing Source in IdentityNow/ISC."
	resp.Schema.MarkdownDescription = "Manages a [Schema](https://documentation.sailpoint.com/saas/help/accounts/schema.html) " +
		"(account or group/entitlement attribute definitions) on an existing [Source](https://documentation.sailpoint.com/saas/help/sources/index.html) " +
		"in IdentityNow/ISC.\n\n" +
		"~> This is a `_v1` pilot resource - see \"Known Limitations & Live Testing Notes\" below before relying on it in " +
		"production configurations."
	applySourceSchemaConfigurationField(&resp.Schema.Attributes, true)
	applySourceSchemaPlanModifiers(&resp.Schema)
}

func (r *sourceSchemaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sourceSchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	sourceID, schemaID, err := idToParts(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_id"), sourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schema_id"), schemaID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), schemaID)...)
}

func (r *sourceSchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sourceSchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := plan.SourceId.ValueString()
	tflog.Debug(ctx, "Creating Source Schema", map[string]interface{}{"source_id": sourceID, "name": plan.Name.ValueString()})

	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, httpResp, err := r.client.SourcesAPI.
		CreateSourceSchemaV1(ctx, sourceID).
		Schema(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error creating Source Schema", map[string]interface{}{"source_id": sourceID, "error": err.Error()})
		resp.Diagnostics.AddError("Error creating Source Schema", errDetail(err, httpResp))
		return
	}

	state, diags := dtoToModel(ctx, apiResp, sourceID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created Source Schema", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sourceSchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sourceSchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	schemaID := state.SchemaId.ValueString()
	tflog.Debug(ctx, "Reading Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})

	apiResp, httpResp, err := r.client.SourcesAPI.
		GetSourceSchemaV1(ctx, sourceID, schemaID).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source Schema not found, removing from state", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Error reading Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID, "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Source Schema", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, sourceID, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceSchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sourceSchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state sourceSchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	schemaID := state.SchemaId.ValueString()
	tflog.Debug(ctx, "Updating Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})

	// source_id/schema_id/id are all RequiresReplace or Computed-only (see
	// resource_source_schema_planmodifiers.go), so Update is only ever
	// reached for changes to the other mutable fields. A full PUT
	// (putSourceSchemaV1) replacing the whole document is simplest and
	// correct here - the API requires "id" to remain present (unchanged) in
	// the body, so it is populated from state below (mirrors transform_v1's
	// full-replace Update).
	dto, diags := modelToDto(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	dto.SetId(schemaID)

	apiResp, httpResp, err := r.client.SourcesAPI.
		PutSourceSchemaV1(ctx, sourceID, schemaID).
		Schema(*dto).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error updating Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID, "error": err.Error()})
		resp.Diagnostics.AddError("Error updating Source Schema", errDetail(err, httpResp))
		return
	}

	newState, diags := dtoToModel(ctx, apiResp, sourceID, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Updated Source Schema", map[string]interface{}{"id": newState.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sourceSchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sourceSchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sourceID := state.SourceId.ValueString()
	schemaID := state.SchemaId.ValueString()
	tflog.Debug(ctx, "Deleting Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})

	httpResp, err := r.client.SourcesAPI.
		DeleteSourceSchemaV1(ctx, sourceID, schemaID).
		Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			tflog.Warn(ctx, "Source Schema already absent on delete", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})
			return
		}
		tflog.Error(ctx, "Error deleting Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID, "error": err.Error()})
		resp.Diagnostics.AddError("Error deleting Source Schema", errDetail(err, httpResp))
		return
	}

	tflog.Info(ctx, "Deleted Source Schema", map[string]interface{}{"source_id": sourceID, "schema_id": schemaID})
}

// idToParts splits this resource's ImportState id ("source_id/schema_id",
// matching source_provisioning_policy_v1's identical delimiter convention).
func idToParts(id string) (sourceID, schemaID string, err error) {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			sourceID, schemaID = id[:i], id[i+1:]
			if sourceID == "" || schemaID == "" {
				break
			}
			return sourceID, schemaID, nil
		}
	}
	return "", "", fmt.Errorf("expected import id in the form \"source_id/schema_id\", got: %q", id)
}

// configurationToMap decodes the practitioner-supplied "configuration" JSON
// string into a map[string]interface{}, matching transform_v1's
// attributesToMap pattern. Null/empty decodes to nil (omitted on the wire).
func configurationToMap(v jsontypes.Normalized) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil, diags
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &m); err != nil {
		diags.AddError(
			"Invalid \"configuration\" JSON",
			fmt.Sprintf("Could not decode \"configuration\" as a JSON object: %s", err.Error()),
		)
		return nil, diags
	}
	return m, diags
}

// normalizedConfigurationFromAPI re-encodes an API-returned "configuration"
// map as a jsontypes.Normalized JSON string, matching transform_v1's
// normalizedAttributesFromAPI pattern. A nil map is normalized to an empty
// JSON object "{}" for predictable diffing.
func normalizedConfigurationFromAPI(m map[string]interface{}) (jsontypes.Normalized, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m == nil {
		m = map[string]interface{}{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		diags.AddError(
			"Error encoding \"configuration\" from API response",
			fmt.Sprintf("Could not re-encode the API's \"configuration\" value as JSON: %s", err.Error()),
		)
		return jsontypes.NewNormalizedNull(), diags
	}
	return jsontypes.NewNormalizedValue(string(b)), diags
}

// attributesListToAPI converts the resource's "attributes" types.List (each
// element a resource_source_schema.AttributesValue) into
// []sources.AttributeDefinition.
func attributesListToAPI(ctx context.Context, l types.List) ([]sources.AttributeDefinition, diag.Diagnostics) {
	var diags diag.Diagnostics
	if l.IsNull() || l.IsUnknown() {
		return nil, diags
	}
	var values []resource_source_schema.AttributesValue
	diags.Append(l.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make([]sources.AttributeDefinition, 0, len(values))
	for _, v := range values {
		ad := sources.NewAttributeDefinition()
		if !v.Name.IsNull() && !v.Name.IsUnknown() {
			ad.SetName(v.Name.ValueString())
		}
		if !v.NativeName.IsNull() && !v.NativeName.IsUnknown() {
			ad.SetNativeName(v.NativeName.ValueString())
		}
		if !v.AttributesType.IsNull() && !v.AttributesType.IsUnknown() {
			t := sources.AttributeDefinitionType(v.AttributesType.ValueString())
			ad.Type = &t
		}
		if !v.Description.IsNull() && !v.Description.IsUnknown() {
			ad.SetDescription(v.Description.ValueString())
		}
		if !v.IsMulti.IsNull() && !v.IsMulti.IsUnknown() {
			ad.SetIsMulti(v.IsMulti.ValueBool())
		}
		if !v.IsEntitlement.IsNull() && !v.IsEntitlement.IsUnknown() {
			ad.SetIsEntitlement(v.IsEntitlement.ValueBool())
		}
		if !v.IsGroup.IsNull() && !v.IsGroup.IsUnknown() {
			ad.SetIsGroup(v.IsGroup.ValueBool())
		}
		ad.Schema = schemaValueToNullableRef(v.Schema)
		out = append(out, *ad)
	}
	return out, diags
}

// schemaValueToNullableRef converts the nested "schema" basetypes.ObjectValue
// (attr.Value form, as stored on AttributesValue.Schema) into an
// sources.NullableAttributeDefinitionSchema by reading its "id"/"name"/"type"
// string attributes directly.
func schemaValueToNullableRef(obj types.Object) sources.NullableAttributeDefinitionSchema {
	var out sources.NullableAttributeDefinitionSchema
	if obj.IsNull() || obj.IsUnknown() {
		return out
	}
	attrs := obj.Attributes()
	ref := sources.NewAttributeDefinitionSchema()
	if v, ok := attrs["id"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
		ref.SetId(v.ValueString())
	}
	if v, ok := attrs["name"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
		ref.SetName(v.ValueString())
	}
	if v, ok := attrs["type"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
		ref.SetType(v.ValueString())
	}
	out.Set(ref)
	return out
}

// attributesListFromAPI converts an API-returned []sources.AttributeDefinition
// into the resource's "attributes" types.List.
func attributesListFromAPI(ctx context.Context, defs []sources.AttributeDefinition) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := resource_source_schema.AttributesValue{}.Type(ctx)
	if defs == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]resource_source_schema.AttributesValue, 0, len(defs))
	for _, ad := range defs {
		schemaObj, d := schemaRefToObjectValue(ad.Schema)
		diags.Append(d...)

		attrType := ""
		if ad.Type != nil {
			attrType = string(*ad.Type)
		}

		v, d := resource_source_schema.NewAttributesValue(
			resource_source_schema.AttributesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"description":    types.StringPointerValue(ad.Description),
				"is_entitlement": types.BoolPointerValue(ad.IsEntitlement),
				"is_group":       types.BoolPointerValue(ad.IsGroup),
				"is_multi":       types.BoolPointerValue(ad.IsMulti),
				"name":           types.StringPointerValue(ad.Name),
				"native_name":    nullableStringToStringValue(ad.NativeName),
				"schema":         schemaObj,
				"type":           types.StringValue(attrType),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return list, diags
}

// schemaRefToObjectValue converts an sources.NullableAttributeDefinitionSchema
// API response value into a types.Object matching the "schema" nested
// attribute's shape (id/name/type strings).
func schemaRefToObjectValue(ref sources.NullableAttributeDefinitionSchema) (types.Object, diag.Diagnostics) {
	attrTypes := map[string]attr.Type{
		"id":   types.StringType,
		"name": types.StringType,
		"type": types.StringType,
	}
	if !ref.IsSet() || ref.Get() == nil {
		return types.ObjectNull(attrTypes), nil
	}
	r := ref.Get()
	return types.ObjectValue(attrTypes, map[string]attr.Value{
		"id":   types.StringPointerValue(r.Id),
		"name": types.StringPointerValue(r.Name),
		"type": types.StringPointerValue(r.Type),
	})
}

// nullableStringToStringValue converts an sources.NullableString into a
// types.String.
func nullableStringToStringValue(ns sources.NullableString) types.String {
	if !ns.IsSet() || ns.Get() == nil {
		return types.StringNull()
	}
	return types.StringValue(*ns.Get())
}

// modelToDto builds an sources.Schema request body from the plan model.
func modelToDto(ctx context.Context, plan sourceSchemaResourceModel) (*sources.Schema, diag.Diagnostics) {
	config, diags := configurationToMap(plan.Configuration)
	if diags.HasError() {
		return nil, diags
	}

	attrs, d := attributesListToAPI(ctx, plan.Attributes)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	var features []string
	if !plan.Features.IsNull() && !plan.Features.IsUnknown() {
		d = plan.Features.ElementsAs(ctx, &features, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
	}

	dto := sources.NewSchema()
	dto.SetName(plan.Name.ValueString())
	if !plan.NativeObjectType.IsNull() && !plan.NativeObjectType.IsUnknown() {
		dto.SetNativeObjectType(plan.NativeObjectType.ValueString())
	}
	if !plan.IdentityAttribute.IsNull() && !plan.IdentityAttribute.IsUnknown() {
		dto.SetIdentityAttribute(plan.IdentityAttribute.ValueString())
	}
	if !plan.DisplayAttribute.IsNull() && !plan.DisplayAttribute.IsUnknown() {
		dto.SetDisplayAttribute(plan.DisplayAttribute.ValueString())
	}
	if !plan.HierarchyAttribute.IsNull() && !plan.HierarchyAttribute.IsUnknown() {
		dto.SetHierarchyAttribute(plan.HierarchyAttribute.ValueString())
	}
	if !plan.IncludePermissions.IsNull() && !plan.IncludePermissions.IsUnknown() {
		dto.SetIncludePermissions(plan.IncludePermissions.ValueBool())
	}
	if features != nil {
		dto.SetFeatures(features)
	}
	if config != nil {
		dto.SetConfiguration(config)
	}
	dto.SetAttributes(attrs)

	return dto, diags
}

// dtoToModel converts an sources.Schema API response into the resource's
// state model.
func dtoToModel(ctx context.Context, dto *sources.Schema, sourceID string, fallback sourceSchemaResourceModel) (sourceSchemaResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.SourceId = types.StringValue(sourceID)
	model.Id = types.StringValue(dto.GetId())
	model.SchemaId = types.StringValue(dto.GetId())
	model.Name = types.StringPointerValue(dto.Name)
	model.NativeObjectType = types.StringPointerValue(dto.NativeObjectType)
	model.IdentityAttribute = types.StringPointerValue(dto.IdentityAttribute)
	model.DisplayAttribute = types.StringPointerValue(dto.DisplayAttribute)
	model.HierarchyAttribute = types.StringPointerValue(dto.HierarchyAttribute.Get())
	model.IncludePermissions = types.BoolPointerValue(dto.IncludePermissions)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified.Get())

	if dto.Features != nil {
		featuresList, d := types.ListValueFrom(ctx, types.StringType, dto.Features)
		diags.Append(d...)
		model.Features = featuresList
	} else {
		model.Features = types.ListNull(types.StringType)
	}

	config, d := normalizedConfigurationFromAPI(dto.Configuration)
	diags.Append(d...)
	model.Configuration = config

	attrsList, d := attributesListFromAPI(ctx, dto.Attributes)
	diags.Append(d...)
	model.Attributes = attrsList

	return model, diags
}

// timeToStringValue formats an *sources.SailPointTime as RFC3339, or
// returns a null types.String if nil (matches sources_v1's identical
// helper).
func timeToStringValue(t *sources.SailPointTime) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

// errDetail delegates to the shared util.SailpointErrorDetail helper (see
// transform_v1/role_v1/service_desk_integration_v1 for the same pattern).
func errDetail(err error, httpResp *http.Response) string {
	return util.SailpointErrorDetail(err, httpResp)
}
