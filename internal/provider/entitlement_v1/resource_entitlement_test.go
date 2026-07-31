package entitlement_v1

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/entitlement_v1/resource_entitlement"
)

func minimalEntitlementModel() entitlementResourceModel {
	ctx := context.Background()
	return entitlementResourceModel{
		AccessModelMetadata:    resource_entitlement.NewAccessModelMetadataValueNull(),
		Attribute:              types.StringNull(),
		Attributes:             jsontypes.NewNormalizedNull(),
		CloudGoverned:          types.BoolNull(),
		Created:                types.StringNull(),
		Description:            types.StringNull(),
		DirectPermissions:      types.ListNull(resource_entitlement.DirectPermissionsValue{}.Type(ctx)),
		Id:                     types.StringNull(),
		ManuallyUpdatedFields:  types.ObjectNull(entitlementManuallyUpdatedFieldsAttrTypes()),
		Modified:               types.StringNull(),
		Name:                   types.StringNull(),
		Owner:                  resource_entitlement.NewOwnerValueNull(),
		PrivilegeLevel:         resource_entitlement.NewPrivilegeLevelValueNull(),
		Requestable:            types.BoolNull(),
		Segments:               types.ListNull(types.StringType),
		SourceId:               types.StringNull(),
		Source:                 resource_entitlement.NewSourceValueNull(),
		SourceSchemaObjectType: types.StringNull(),
		Tags:                   types.ListNull(types.StringType),
		Value:                  types.StringNull(),
	}
}

func entitlementOwnerModel(t *testing.T, id, name, ownerType string) resource_entitlement.OwnerValue {
	t.Helper()
	ctx := context.Background()
	v, diags := resource_entitlement.NewOwnerValue(
		resource_entitlement.OwnerValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue(id),
			"name": types.StringValue(name),
			"type": types.StringValue(ownerType),
		},
	)
	if diags.HasError() {
		t.Fatalf("NewOwnerValue returned diagnostics: %v", diags)
	}
	return v
}

func stringListValue(t *testing.T, values ...string) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("ListValueFrom returned diagnostics: %v", diags)
	}
	return list
}

type entitlementPatchExpectation struct {
	op       string
	path     string
	value    interface{}
	hasValue bool
}

func assertEntitlementPatchOps(t *testing.T, ops []api_beta.JsonPatchOperation, want []entitlementPatchExpectation) {
	t.Helper()
	if len(ops) != len(want) {
		t.Fatalf("len(ops) = %d, want %d, ops = %+v", len(ops), len(want), ops)
	}

	for i := range want {
		if ops[i].Op != want[i].op {
			t.Errorf("ops[%d].Op = %q, want %q", i, ops[i].Op, want[i].op)
		}
		if ops[i].Path != want[i].path {
			t.Errorf("ops[%d].Path = %q, want %q", i, ops[i].Path, want[i].path)
		}
		if !want[i].hasValue {
			if ops[i].Value != nil {
				t.Errorf("ops[%d].Value = %v, want nil", i, ops[i].Value)
			}
			continue
		}
		if ops[i].Value == nil {
			t.Fatalf("ops[%d].Value is nil, want populated", i)
		}
		b, err := json.Marshal(ops[i].Value)
		if err != nil {
			t.Fatalf("json.Marshal(ops[%d].Value) returned error: %v", i, err)
		}
		var got interface{}
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("json.Unmarshal(ops[%d].Value) returned error: %v", i, err)
		}
		if !reflect.DeepEqual(got, want[i].value) {
			t.Errorf("ops[%d].Value = %#v, want %#v", i, got, want[i].value)
		}
	}
}

func TestEntitlementResourceDtoToModel(t *testing.T) {
	ctx := context.Background()

	t.Run("populated dto", func(t *testing.T) {
		fallback := minimalEntitlementModel()

		created := api_beta.SailPointTime{Time: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}
		modified := api_beta.SailPointTime{Time: time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)}
		dto := api_beta.NewEntitlement()
		dto.SetId("entitlement-id")
		dto.SetName("Finance Access")
		dto.SetAttribute("memberOf")
		dto.SetDescription("Finance access entitlement")
		dto.SetValue("CN=Finance,OU=Groups")
		dto.SetSourceSchemaObjectType("group")
		dto.SetCloudGoverned(true)
		dto.SetRequestable(true)
		dto.SetCreated(created)
		dto.SetModified(modified)
		dto.SetSegments([]string{"segment-a", "segment-b"})
		dto.SetAttributes(map[string]interface{}{"enabled": true, "rank": float64(7)})
		dto.SetDirectPermissions([]api_beta.PermissionDto{{Rights: []string{"read", "write"}, Target: strPtr("accounts")}})
		dto.SetOwner(api_beta.EntitlementOwner{Id: strPtr("owner-id"), Name: strPtr("Owner Name"), Type: strPtr("IDENTITY")})

		source := api_beta.NewEntitlementSource()
		source.SetId("source-id")
		source.SetName("HR Source")
		source.SetType("SOURCE")
		dto.SetSource(*source)

		manual := api_beta.NewEntitlementManuallyUpdatedFields()
		manual.SetDISPLAY_NAME(true)
		manual.SetDESCRIPTION(false)
		dto.SetManuallyUpdatedFields(*manual)

		metadata := api_beta.NewEntitlementAccessModelMetadata()
		metadata.SetAttributes([]api_beta.AttributeDTO{{
			Key:         strPtr("environment"),
			Name:        strPtr("Environment"),
			Multiselect: boolPtr(true),
			Status:      strPtr("ACTIVE"),
			Type:        strPtr("custom"),
			ObjectTypes: []string{"entitlement"},
			Description: strPtr("Environment classification"),
			Values: []api_beta.AttributeValueDTO{{
				Name:   strPtr("Production"),
				Status: strPtr("ACTIVE"),
				Value:  strPtr("prod"),
			}},
		}})
		dto.SetAccessModelMetadata(*metadata)
		dto.AdditionalProperties = map[string]interface{}{
			"privilegeLevel": map[string]interface{}{
				"direct":    "HIGH",
				"effective": "HIGH",
				"inherited": "NONE",
				"setBy":     "governance",
				"setByType": "SYSTEM",
			},
			"tags": []string{"tag-1", "tag-2"},
		}

		model, diags := entitlementResourceDtoToModel(ctx, dto, fallback)
		if diags.HasError() {
			t.Fatalf("entitlementResourceDtoToModel returned diagnostics: %v", diags)
		}

		if model.Id.ValueString() != "entitlement-id" {
			t.Errorf("Id = %q, want %q", model.Id.ValueString(), "entitlement-id")
		}
		if model.Name.ValueString() != "Finance Access" {
			t.Errorf("Name = %q, want %q", model.Name.ValueString(), "Finance Access")
		}
		if model.Attribute.ValueString() != "memberOf" {
			t.Errorf("Attribute = %q, want %q", model.Attribute.ValueString(), "memberOf")
		}
		if model.Description.ValueString() != "Finance access entitlement" {
			t.Errorf("Description = %q, want %q", model.Description.ValueString(), "Finance access entitlement")
		}
		if model.Value.ValueString() != "CN=Finance,OU=Groups" {
			t.Errorf("Value = %q, want %q", model.Value.ValueString(), "CN=Finance,OU=Groups")
		}
		if model.SourceSchemaObjectType.ValueString() != "group" {
			t.Errorf("SourceSchemaObjectType = %q, want %q", model.SourceSchemaObjectType.ValueString(), "group")
		}
		if !model.CloudGoverned.ValueBool() {
			t.Errorf("CloudGoverned = false, want true")
		}
		if !model.Requestable.ValueBool() {
			t.Errorf("Requestable = false, want true")
		}
		if model.Created.ValueString() != created.Format(time.RFC3339) {
			t.Errorf("Created = %q, want %q", model.Created.ValueString(), created.Format(time.RFC3339))
		}
		if model.Modified.ValueString() != modified.Format(time.RFC3339) {
			t.Errorf("Modified = %q, want %q", model.Modified.ValueString(), modified.Format(time.RFC3339))
		}
		if model.Owner.Id.ValueString() != "owner-id" {
			t.Errorf("Owner.Id = %q, want %q", model.Owner.Id.ValueString(), "owner-id")
		}
		if model.Owner.Name.ValueString() != "Owner Name" {
			t.Errorf("Owner.Name = %q, want %q", model.Owner.Name.ValueString(), "Owner Name")
		}
		if model.Owner.OwnerType.ValueString() != "IDENTITY" {
			t.Errorf("Owner.OwnerType = %q, want %q", model.Owner.OwnerType.ValueString(), "IDENTITY")
		}
		if model.Source.Id.ValueString() != "source-id" {
			t.Errorf("Source.Id = %q, want %q", model.Source.Id.ValueString(), "source-id")
		}
		if model.Source.Name.ValueString() != "HR Source" {
			t.Errorf("Source.Name = %q, want %q", model.Source.Name.ValueString(), "HR Source")
		}
		if model.Source.SourceType.ValueString() != "SOURCE" {
			t.Errorf("Source.SourceType = %q, want %q", model.Source.SourceType.ValueString(), "SOURCE")
		}

		var segments []string
		if diags := model.Segments.ElementsAs(ctx, &segments, false); diags.HasError() {
			t.Fatalf("Segments.ElementsAs returned diagnostics: %v", diags)
		}
		if !reflect.DeepEqual(segments, []string{"segment-a", "segment-b"}) {
			t.Errorf("Segments = %v, want %v", segments, []string{"segment-a", "segment-b"})
		}

		var permissions []resource_entitlement.DirectPermissionsValue
		if diags := model.DirectPermissions.ElementsAs(ctx, &permissions, false); diags.HasError() {
			t.Fatalf("DirectPermissions.ElementsAs returned diagnostics: %v", diags)
		}
		if len(permissions) != 1 {
			t.Fatalf("len(permissions) = %d, want 1", len(permissions))
		}
		if permissions[0].Target.ValueString() != "accounts" {
			t.Errorf("DirectPermissions[0].Target = %q, want %q", permissions[0].Target.ValueString(), "accounts")
		}
		var rights []string
		if diags := permissions[0].Rights.ElementsAs(ctx, &rights, false); diags.HasError() {
			t.Fatalf("DirectPermissions[0].Rights.ElementsAs returned diagnostics: %v", diags)
		}
		if !reflect.DeepEqual(rights, []string{"read", "write"}) {
			t.Errorf("DirectPermissions[0].Rights = %v, want %v", rights, []string{"read", "write"})
		}

		var metadataAttrs []resource_entitlement.AttributesValue
		if diags := model.AccessModelMetadata.Attributes.ElementsAs(ctx, &metadataAttrs, false); diags.HasError() {
			t.Fatalf("AccessModelMetadata.Attributes.ElementsAs returned diagnostics: %v", diags)
		}
		if len(metadataAttrs) != 1 {
			t.Fatalf("len(metadataAttrs) = %d, want 1", len(metadataAttrs))
		}
		if metadataAttrs[0].Key.ValueString() != "environment" {
			t.Errorf("AccessModelMetadata.Attributes[0].Key = %q, want %q", metadataAttrs[0].Key.ValueString(), "environment")
		}
		if metadataAttrs[0].Name.ValueString() != "Environment" {
			t.Errorf("AccessModelMetadata.Attributes[0].Name = %q, want %q", metadataAttrs[0].Name.ValueString(), "Environment")
		}
		if !metadataAttrs[0].Multiselect.ValueBool() {
			t.Errorf("AccessModelMetadata.Attributes[0].Multiselect = false, want true")
		}
		var metadataValues []resource_entitlement.ValuesValue
		if diags := metadataAttrs[0].Values.ElementsAs(ctx, &metadataValues, false); diags.HasError() {
			t.Fatalf("AccessModelMetadata.Attributes[0].Values.ElementsAs returned diagnostics: %v", diags)
		}
		if len(metadataValues) != 1 || metadataValues[0].Value.ValueString() != "prod" {
			t.Errorf("metadataValues = %+v, want one value with Value %q", metadataValues, "prod")
		}

		if model.PrivilegeLevel.Direct.ValueString() != "HIGH" {
			t.Errorf("PrivilegeLevel.Direct = %q, want %q", model.PrivilegeLevel.Direct.ValueString(), "HIGH")
		}
		if model.PrivilegeLevel.SetBy.ValueString() != "governance" {
			t.Errorf("PrivilegeLevel.SetBy = %q, want %q", model.PrivilegeLevel.SetBy.ValueString(), "governance")
		}

		var tags []string
		if diags := model.Tags.ElementsAs(ctx, &tags, false); diags.HasError() {
			t.Fatalf("Tags.ElementsAs returned diagnostics: %v", diags)
		}
		if !reflect.DeepEqual(tags, []string{"tag-1", "tag-2"}) {
			t.Errorf("Tags = %v, want %v", tags, []string{"tag-1", "tag-2"})
		}

		if model.Attributes.ValueString() != `{"enabled":true,"rank":7}` {
			t.Errorf("Attributes = %q, want %q", model.Attributes.ValueString(), `{"enabled":true,"rank":7}`)
		}
		manualAttrs := model.ManuallyUpdatedFields.Attributes()
		if manualAttrs["display_name"].(types.Bool).ValueBool() != true {
			t.Errorf("ManuallyUpdatedFields.display_name = %v, want true", manualAttrs["display_name"])
		}
		if manualAttrs["description"].(types.Bool).ValueBool() != false {
			t.Errorf("ManuallyUpdatedFields.description = %v, want false", manualAttrs["description"])
		}
	})

	t.Run("nil nested values map to nulls", func(t *testing.T) {
		model, diags := entitlementResourceDtoToModel(ctx, &api_beta.Entitlement{}, minimalEntitlementModel())
		if diags.HasError() {
			t.Fatalf("entitlementResourceDtoToModel returned diagnostics: %v", diags)
		}

		if !model.Id.IsNull() {
			t.Errorf("Id.IsNull() = false, want true")
		}
		if !model.Owner.IsNull() {
			t.Errorf("Owner.IsNull() = false, want true")
		}
		if !model.Source.IsNull() {
			t.Errorf("Source.IsNull() = false, want true")
		}
		if !model.Attributes.IsNull() {
			t.Errorf("Attributes.IsNull() = false, want true")
		}
		if !model.ManuallyUpdatedFields.IsNull() {
			t.Errorf("ManuallyUpdatedFields.IsNull() = false, want true")
		}
		if !model.AccessModelMetadata.IsNull() {
			t.Errorf("AccessModelMetadata.IsNull() = false, want true")
		}
		if !model.DirectPermissions.IsNull() {
			t.Errorf("DirectPermissions.IsNull() = false, want true")
		}
		if !model.Segments.IsNull() {
			t.Errorf("Segments.IsNull() = false, want true")
		}
	})
}

func TestEntitlementStringPatchOps(t *testing.T) {
	tests := []struct {
		name  string
		plan  types.String
		state types.String
		want  []entitlementPatchExpectation
	}{
		{
			name:  "unchanged returns no ops",
			plan:  types.StringValue("same"),
			state: types.StringValue("same"),
		},
		{
			name:  "null state becomes add",
			plan:  types.StringValue("new"),
			state: types.StringNull(),
			want:  []entitlementPatchExpectation{{op: "add", path: "/name", value: "new", hasValue: true}},
		},
		{
			name:  "existing value becomes replace",
			plan:  types.StringValue("new"),
			state: types.StringValue("old"),
			want:  []entitlementPatchExpectation{{op: "replace", path: "/name", value: "new", hasValue: true}},
		},
		{
			name:  "planned null becomes remove",
			plan:  types.StringNull(),
			state: types.StringValue("old"),
			want:  []entitlementPatchExpectation{{op: "remove", path: "/name"}},
		},
		{
			name:  "unknown plan is ignored",
			plan:  types.StringUnknown(),
			state: types.StringValue("old"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEntitlementPatchOps(t, entitlementStringPatchOps("/name", tt.plan, tt.state), tt.want)
		})
	}
}

func TestEntitlementBoolPatchOps(t *testing.T) {
	tests := []struct {
		name  string
		plan  types.Bool
		state types.Bool
		want  []entitlementPatchExpectation
	}{
		{
			name:  "null state becomes add",
			plan:  types.BoolValue(true),
			state: types.BoolNull(),
			want:  []entitlementPatchExpectation{{op: "add", path: "/requestable", value: true, hasValue: true}},
		},
		{
			name:  "existing value becomes replace",
			plan:  types.BoolValue(true),
			state: types.BoolValue(false),
			want:  []entitlementPatchExpectation{{op: "replace", path: "/requestable", value: true, hasValue: true}},
		},
		{
			name:  "planned null is ignored",
			plan:  types.BoolNull(),
			state: types.BoolValue(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEntitlementPatchOps(t, entitlementBoolPatchOps("/requestable", tt.plan, tt.state), tt.want)
		})
	}
}

func TestEntitlementListPatchOps(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		plan  types.List
		state types.List
		want  []entitlementPatchExpectation
	}{
		{
			name:  "unchanged returns no ops",
			plan:  stringListValue(t, "seg-a"),
			state: stringListValue(t, "seg-a"),
		},
		{
			name:  "null state becomes add",
			plan:  stringListValue(t, "seg-a", "seg-b"),
			state: types.ListNull(types.StringType),
			want:  []entitlementPatchExpectation{{op: "add", path: "/segments", value: []interface{}{"seg-a", "seg-b"}, hasValue: true}},
		},
		{
			name:  "existing value becomes replace",
			plan:  stringListValue(t, "seg-b"),
			state: stringListValue(t, "seg-a"),
			want:  []entitlementPatchExpectation{{op: "replace", path: "/segments", value: []interface{}{"seg-b"}, hasValue: true}},
		},
		{
			name:  "planned null becomes remove",
			plan:  types.ListNull(types.StringType),
			state: stringListValue(t, "seg-a"),
			want:  []entitlementPatchExpectation{{op: "remove", path: "/segments"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, diags := entitlementListPatchOps(ctx, "/segments", tt.plan, tt.state)
			if diags.HasError() {
				t.Fatalf("entitlementListPatchOps returned diagnostics: %v", diags)
			}
			assertEntitlementPatchOps(t, ops, tt.want)
		})
	}
}

func TestEntitlementOwnerPatchOps(t *testing.T) {
	tests := []struct {
		name  string
		plan  resource_entitlement.OwnerValue
		state resource_entitlement.OwnerValue
		want  []entitlementPatchExpectation
	}{
		{
			name:  "unchanged returns no ops",
			plan:  entitlementOwnerModel(t, "owner-id", "Owner Name", "IDENTITY"),
			state: entitlementOwnerModel(t, "owner-id", "Different Name", "IDENTITY"),
		},
		{
			name:  "null state becomes add",
			plan:  entitlementOwnerModel(t, "owner-id", "Owner Name", "IDENTITY"),
			state: resource_entitlement.NewOwnerValueNull(),
			want:  []entitlementPatchExpectation{{op: "add", path: "/owner", value: map[string]interface{}{"id": "owner-id", "type": "IDENTITY"}, hasValue: true}},
		},
		{
			name:  "existing value becomes replace",
			plan:  entitlementOwnerModel(t, "owner-2", "Owner Two", "GOVERNANCE_GROUP"),
			state: entitlementOwnerModel(t, "owner-1", "Owner One", "IDENTITY"),
			want:  []entitlementPatchExpectation{{op: "replace", path: "/owner", value: map[string]interface{}{"id": "owner-2", "type": "GOVERNANCE_GROUP"}, hasValue: true}},
		},
		{
			name:  "planned null becomes remove",
			plan:  resource_entitlement.NewOwnerValueNull(),
			state: entitlementOwnerModel(t, "owner-id", "Owner Name", "IDENTITY"),
			want:  []entitlementPatchExpectation{{op: "remove", path: "/owner"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, diags := entitlementOwnerPatchOps(tt.plan, tt.state)
			if diags.HasError() {
				t.Fatalf("entitlementOwnerPatchOps returned diagnostics: %v", diags)
			}
			assertEntitlementPatchOps(t, ops, tt.want)
		})
	}
}

func TestEntitlementResourcePatchOps(t *testing.T) {
	ctx := context.Background()
	state := minimalEntitlementModel()
	state.Name = types.StringValue("Current Name")
	state.Description = types.StringValue("Current Description")
	state.Requestable = types.BoolValue(false)
	state.Segments = stringListValue(t, "segment-a")
	state.Owner = entitlementOwnerModel(t, "owner-id", "Owner Name", "IDENTITY")

	t.Run("no changes produces no ops", func(t *testing.T) {
		plan := state
		ops, diags := entitlementResourcePatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("entitlementResourcePatchOps returned diagnostics: %v", diags)
		}
		assertEntitlementPatchOps(t, ops, nil)
	})

	t.Run("replace ops cover every writable field", func(t *testing.T) {
		plan := state
		plan.Name = types.StringValue("New Name")
		plan.Description = types.StringValue("New Description")
		plan.Requestable = types.BoolValue(true)
		plan.Segments = stringListValue(t, "segment-b", "segment-c")
		plan.Owner = entitlementOwnerModel(t, "owner-2", "Owner Two", "GOVERNANCE_GROUP")

		ops, diags := entitlementResourcePatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("entitlementResourcePatchOps returned diagnostics: %v", diags)
		}
		assertEntitlementPatchOps(t, ops, []entitlementPatchExpectation{
			{op: "replace", path: "/name", value: "New Name", hasValue: true},
			{op: "replace", path: "/description", value: "New Description", hasValue: true},
			{op: "replace", path: "/requestable", value: true, hasValue: true},
			{op: "replace", path: "/segments", value: []interface{}{"segment-b", "segment-c"}, hasValue: true},
			{op: "replace", path: "/owner", value: map[string]interface{}{"id": "owner-2", "type": "GOVERNANCE_GROUP"}, hasValue: true},
		})
	})

	t.Run("null prior values produce add ops", func(t *testing.T) {
		plan := minimalEntitlementModel()
		plan.Name = types.StringValue("Configured Name")
		plan.Description = types.StringValue("Configured Description")
		plan.Requestable = types.BoolValue(true)
		plan.Segments = stringListValue(t, "segment-a")
		plan.Owner = entitlementOwnerModel(t, "owner-id", "Owner Name", "IDENTITY")

		ops, diags := entitlementResourcePatchOps(ctx, plan, minimalEntitlementModel())
		if diags.HasError() {
			t.Fatalf("entitlementResourcePatchOps returned diagnostics: %v", diags)
		}
		assertEntitlementPatchOps(t, ops, []entitlementPatchExpectation{
			{op: "add", path: "/name", value: "Configured Name", hasValue: true},
			{op: "add", path: "/description", value: "Configured Description", hasValue: true},
			{op: "add", path: "/requestable", value: true, hasValue: true},
			{op: "add", path: "/segments", value: []interface{}{"segment-a"}, hasValue: true},
			{op: "add", path: "/owner", value: map[string]interface{}{"id": "owner-id", "type": "IDENTITY"}, hasValue: true},
		})
	})

	t.Run("planned null values remove nullable fields", func(t *testing.T) {
		plan := state
		plan.Name = types.StringNull()
		plan.Description = types.StringNull()
		plan.Segments = types.ListNull(types.StringType)
		plan.Owner = resource_entitlement.NewOwnerValueNull()

		ops, diags := entitlementResourcePatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("entitlementResourcePatchOps returned diagnostics: %v", diags)
		}
		assertEntitlementPatchOps(t, ops, []entitlementPatchExpectation{
			{op: "remove", path: "/name"},
			{op: "remove", path: "/description"},
			{op: "remove", path: "/segments"},
			{op: "remove", path: "/owner"},
		})
	})
}

func TestEntitlementManuallyUpdatedFieldsFromAPI(t *testing.T) {
	t.Run("nil dto returns null object", func(t *testing.T) {
		v, diags := entitlementManuallyUpdatedFieldsFromAPI(nil)
		if diags.HasError() {
			t.Fatalf("entitlementManuallyUpdatedFieldsFromAPI returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("v.IsNull() = false, want true")
		}
	})

	t.Run("populated dto returns expected booleans", func(t *testing.T) {
		v, diags := entitlementManuallyUpdatedFieldsFromAPI(&api_beta.EntitlementManuallyUpdatedFields{
			DISPLAY_NAME: boolPtr(true),
			DESCRIPTION:  boolPtr(false),
		})
		if diags.HasError() {
			t.Fatalf("entitlementManuallyUpdatedFieldsFromAPI returned diagnostics: %v", diags)
		}
		attrs := v.Attributes()
		if attrs["display_name"].(types.Bool).ValueBool() != true {
			t.Errorf("display_name = %v, want true", attrs["display_name"])
		}
		if attrs["description"].(types.Bool).ValueBool() != false {
			t.Errorf("description = %v, want false", attrs["description"])
		}
	})
}

func TestNormalizedJSONFromMap(t *testing.T) {
	t.Run("nil map returns null", func(t *testing.T) {
		v, diags := normalizedJSONFromMap(nil)
		if diags.HasError() {
			t.Fatalf("normalizedJSONFromMap(nil) returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("v.IsNull() = false, want true")
		}
	})

	t.Run("populated map returns normalized json", func(t *testing.T) {
		v, diags := normalizedJSONFromMap(map[string]interface{}{"b": "two", "a": true})
		if diags.HasError() {
			t.Fatalf("normalizedJSONFromMap returned diagnostics: %v", diags)
		}
		if v.ValueString() != `{"a":true,"b":"two"}` {
			t.Errorf("v = %q, want %q", v.ValueString(), `{"a":true,"b":"two"}`)
		}
	})
}

func TestEntitlementResourceOwnerFromAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("nil dto returns null owner", func(t *testing.T) {
		v, diags := entitlementResourceOwnerFromAPI(ctx, nil)
		if diags.HasError() {
			t.Fatalf("entitlementResourceOwnerFromAPI returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("v.IsNull() = false, want true")
		}
	})

	t.Run("populated dto maps fields", func(t *testing.T) {
		v, diags := entitlementResourceOwnerFromAPI(ctx, &api_beta.EntitlementOwner{
			Id:   strPtr("owner-id"),
			Name: strPtr("Owner Name"),
			Type: strPtr("IDENTITY"),
		})
		if diags.HasError() {
			t.Fatalf("entitlementResourceOwnerFromAPI returned diagnostics: %v", diags)
		}
		if v.Id.ValueString() != "owner-id" {
			t.Errorf("Id = %q, want %q", v.Id.ValueString(), "owner-id")
		}
		if v.Name.ValueString() != "Owner Name" {
			t.Errorf("Name = %q, want %q", v.Name.ValueString(), "Owner Name")
		}
		if v.OwnerType.ValueString() != "IDENTITY" {
			t.Errorf("OwnerType = %q, want %q", v.OwnerType.ValueString(), "IDENTITY")
		}
	})
}

func TestEntitlementResourceSourceFromAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("nil dto returns null source", func(t *testing.T) {
		v, diags := entitlementResourceSourceFromAPI(ctx, nil)
		if diags.HasError() {
			t.Fatalf("entitlementResourceSourceFromAPI returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("v.IsNull() = false, want true")
		}
	})

	t.Run("populated dto maps fields", func(t *testing.T) {
		source := api_beta.NewEntitlementSource()
		source.SetId("source-id")
		source.SetName("HR Source")
		source.SetType("SOURCE")

		v, diags := entitlementResourceSourceFromAPI(ctx, source)
		if diags.HasError() {
			t.Fatalf("entitlementResourceSourceFromAPI returned diagnostics: %v", diags)
		}
		if v.Id.ValueString() != "source-id" {
			t.Errorf("Id = %q, want %q", v.Id.ValueString(), "source-id")
		}
		if v.Name.ValueString() != "HR Source" {
			t.Errorf("Name = %q, want %q", v.Name.ValueString(), "HR Source")
		}
		if v.SourceType.ValueString() != "SOURCE" {
			t.Errorf("SourceType = %q, want %q", v.SourceType.ValueString(), "SOURCE")
		}
	})
}

func strPtr(v string) *string { return &v }

func boolPtr(v bool) *bool { return &v }
