package role_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/role_v1/resource_role"
)

// minimalRoleModel builds a RoleModel with all nested object/list attributes
// explicitly null, suitable as a base for roleModelToDto/roleDtoToModel
// round-trip tests that only care about top-level scalar fields.
func minimalRoleModel() resource_role.RoleModel {
	return resource_role.RoleModel{
		AccessModelMetadata:     resource_role.NewAccessModelMetadataValueNull(),
		AccessProfiles:          types.ListNull(resource_role.AccessProfilesValue{}.Type(context.Background())),
		AccessRequestConfig:     resource_role.NewAccessRequestConfigValueNull(),
		AdditionalOwners:        types.ListNull(resource_role.AdditionalOwnersValue{}.Type(context.Background())),
		Created:                 types.StringNull(),
		Description:             types.StringValue("a test role"),
		DimensionRefs:           types.ListNull(resource_role.DimensionRefsValue{}.Type(context.Background())),
		Dimensional:             types.BoolNull(),
		Enabled:                 types.BoolValue(true),
		Entitlements:            types.ListNull(resource_role.EntitlementsValue{}.Type(context.Background())),
		Id:                      types.StringNull(),
		LegacyMembershipInfo:    resource_role.NewLegacyMembershipInfoValueNull(),
		Membership:              resource_role.NewMembershipValueNull(),
		Modified:                types.StringNull(),
		Name:                    types.StringValue("test-role"),
		Owner:                   resource_role.OwnerValue{},
		PrivilegeLevel:          types.StringNull(),
		Requestable:             types.BoolValue(false),
		RevocationRequestConfig: resource_role.NewRevocationRequestConfigValueNull(),
		Segments:                types.ListNull(types.StringType),
	}
}

// ownerModel returns a populated, non-null OwnerValue for tests that need a
// valid Required owner attribute (owner is Required in the schema, matching
// the API's required: [name, owner]).
func ownerModel(t *testing.T, id, name, typ string) resource_role.OwnerValue {
	t.Helper()
	attrs := map[string]attr.Value{
		"id":   types.StringValue(id),
		"name": types.StringValue(name),
		"type": types.StringValue(typ),
	}
	v, diags := resource_role.NewOwnerValue(resource_role.OwnerValue{}.AttributeTypes(context.Background()), attrs)
	if diags.HasError() {
		t.Fatalf("NewOwnerValue returned diagnostics: %v", diags)
	}
	return v
}

func TestRoleModelToDto(t *testing.T) {
	ctx := context.Background()
	model := minimalRoleModel()

	ownerId := "owner-id"
	ownerType := "IDENTITY"
	model.Owner = ownerModel(t, ownerId, "", ownerType)

	dto, diags := roleModelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("roleModelToDto returned diagnostics: %v", diags)
	}

	if dto.Name != "test-role" {
		t.Errorf("Name = %q, want %q", dto.Name, "test-role")
	}
	if !dto.Description.IsSet() || dto.Description.Get() == nil || *dto.Description.Get() != "a test role" {
		t.Errorf("Description = %v, want %q", dto.Description, "a test role")
	}
	if dto.Owner.Id == nil || *dto.Owner.Id != ownerId {
		t.Errorf("Owner.Id = %v, want %q", dto.Owner.Id, ownerId)
	}
	if dto.Owner.Type == nil || *dto.Owner.Type != ownerType {
		t.Errorf("Owner.Type = %v, want %q", dto.Owner.Type, ownerType)
	}
	if dto.Enabled == nil || !*dto.Enabled {
		t.Errorf("Enabled = %v, want true", dto.Enabled)
	}
	if dto.Requestable == nil || *dto.Requestable {
		t.Errorf("Requestable = %v, want false", dto.Requestable)
	}
}

func TestRoleDtoToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()
	fallback := minimalRoleModel()

	roleId := "role-id"
	ownerId := "owner-id"
	ownerType := "IDENTITY"
	description := "a test role"
	enabled := true
	requestable := false

	dto := &api_beta.Role{
		Id:   &roleId,
		Name: "test-role",
		Owner: api_beta.OwnerReference{
			Id:   &ownerId,
			Type: &ownerType,
		},
		Description: *api_beta.NewNullableString(&description),
		Enabled:     &enabled,
		Requestable: &requestable,
	}

	model, diags := roleDtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("roleDtoToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != roleId {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), roleId)
	}
	if model.Name.ValueString() != "test-role" {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), "test-role")
	}
	if model.Description.ValueString() != description {
		t.Errorf("Description = %q, want %q", model.Description.ValueString(), description)
	}
	if model.Owner.Id.ValueString() != ownerId {
		t.Errorf("Owner.Id = %q, want %q", model.Owner.Id.ValueString(), ownerId)
	}
	if !model.Enabled.ValueBool() {
		t.Errorf("Enabled = %v, want true", model.Enabled.ValueBool())
	}
	if model.Requestable.ValueBool() {
		t.Errorf("Requestable = %v, want false", model.Requestable.ValueBool())
	}
}

func TestRoleStructToMap(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		m, err := roleStructToMap(nil)
		if err != nil {
			t.Fatalf("roleStructToMap(nil) returned error: %v", err)
		}
		if m != nil {
			t.Errorf("roleStructToMap(nil) = %v, want nil", m)
		}
	})

	t.Run("struct input round-trips through JSON", func(t *testing.T) {
		id := "owner-id"
		typ := "IDENTITY"
		owner := api_beta.OwnerReference{Id: &id, Type: &typ}

		m, err := roleStructToMap(owner)
		if err != nil {
			t.Fatalf("roleStructToMap returned error: %v", err)
		}
		if m["id"] != "owner-id" {
			t.Errorf("m[id] = %v, want %q", m["id"], "owner-id")
		}
		if m["type"] != "IDENTITY" {
			t.Errorf("m[type] = %v, want %q", m["type"], "IDENTITY")
		}
	})
}

func TestRoleSliceToArrayInner(t *testing.T) {
	id1, id2 := "ap-1", "ap-2"
	refs := []api_beta.AccessProfileRef{
		{Id: &id1},
		{Id: &id2},
	}

	arr, err := roleSliceToArrayInner(refs)
	if err != nil {
		t.Fatalf("roleSliceToArrayInner returned error: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("len(arr) = %d, want 2", len(arr))
	}
	for i, want := range []string{id1, id2} {
		if arr[i].MapmapOfStringAny == nil {
			t.Fatalf("arr[%d].MapmapOfStringAny is nil, want populated map", i)
		}
		got := (*arr[i].MapmapOfStringAny)["id"]
		if got != want {
			t.Errorf("arr[%d][id] = %v, want %q", i, got, want)
		}
	}
}

func TestRoleJSONPatchReplace(t *testing.T) {
	name := "new-name"
	op := roleJSONPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&name))

	if op.Op != "replace" {
		t.Errorf("Op = %q, want %q", op.Op, "replace")
	}
	if op.Path != "/name" {
		t.Errorf("Path = %q, want %q", op.Path, "/name")
	}
	if op.Value == nil {
		t.Fatal("Value is nil, want populated")
	}
}
