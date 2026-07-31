package application_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/application_v1/resource_application"
)

// minimalApplicationModel builds an applicationResourceModel with all
// nested object/list attributes explicitly null/known, suitable as a base
// for applicationModelToCreateDto/applicationDtoToModel round-trip tests
// that only care about a subset of fields.
func minimalApplicationModel() applicationResourceModel {
	return applicationResourceModel{
		AccessProfileIds:        types.SetNull(types.StringType),
		AccountSource:           resource_application.NewAccountSourceValueNull(),
		AppCenterEnabled:        types.BoolValue(true),
		CloudAppId:              types.StringNull(),
		Created:                 types.StringNull(),
		Description:             types.StringValue("a test application"),
		Enabled:                 types.BoolValue(true),
		Id:                      types.StringNull(),
		MatchAllAccounts:        types.BoolValue(false),
		Modified:                types.StringNull(),
		Name:                    types.StringValue("test-application"),
		Owner:                   resource_application.NewOwnerValueNull(),
		ProvisionRequestEnabled: types.BoolValue(false),
	}
}

// accountSourceModel returns a populated, non-null AccountSourceValue for
// tests that need a valid Required account_source.id attribute.
func accountSourceModel(t *testing.T, id, name string) resource_application.AccountSourceValue {
	t.Helper()
	ctx := context.Background()
	attrs := map[string]attr.Value{
		"id":                          types.StringValue(id),
		"name":                        types.StringValue(name),
		"password_policies":           types.ListNull(resource_application.PasswordPoliciesValue{}.Type(ctx)),
		"type":                        types.StringValue("SOURCE"),
		"use_for_password_management": types.BoolNull(),
	}
	v, diags := resource_application.NewAccountSourceValue(resource_application.AccountSourceValue{}.AttributeTypes(ctx), attrs)
	if diags.HasError() {
		t.Fatalf("NewAccountSourceValue returned diagnostics: %v", diags)
	}
	return v
}

// ownerModel returns a populated, non-null OwnerValue for tests that need a
// valid owner attribute (owner.id/owner.type are Required once configured).
func ownerModel(t *testing.T, id, name, typ string) resource_application.OwnerValue {
	t.Helper()
	ctx := context.Background()
	attrs := map[string]attr.Value{
		"id":   types.StringValue(id),
		"name": types.StringValue(name),
		"type": types.StringValue(typ),
	}
	v, diags := resource_application.NewOwnerValue(resource_application.OwnerValue{}.AttributeTypes(ctx), attrs)
	if diags.HasError() {
		t.Fatalf("NewOwnerValue returned diagnostics: %v", diags)
	}
	return v
}

func TestApplicationModelToCreateDto(t *testing.T) {
	ctx := context.Background()
	model := minimalApplicationModel()
	model.AccountSource = accountSourceModel(t, "source-id", "")

	dto, diags := applicationModelToCreateDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("applicationModelToCreateDto returned diagnostics: %v", diags)
	}

	if dto.Name != "test-application" {
		t.Errorf("Name = %q, want %q", dto.Name, "test-application")
	}
	if dto.Description != "a test application" {
		t.Errorf("Description = %q, want %q", dto.Description, "a test application")
	}
	if dto.AccountSource.Id != "source-id" {
		t.Errorf("AccountSource.Id = %q, want %q", dto.AccountSource.Id, "source-id")
	}
	if dto.MatchAllAccounts == nil || *dto.MatchAllAccounts {
		t.Errorf("MatchAllAccounts = %v, want false", dto.MatchAllAccounts)
	}
}

func TestApplicationDtoToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()
	fallback := minimalApplicationModel()

	appId := "app-id"
	name := "test-application"
	description := "a test application"
	enabled := true
	provisionRequestEnabled := false
	appCenterEnabled := true
	matchAllAccounts := false
	ownerId := "owner-id"
	ownerType := api_beta.DTOTYPE_IDENTITY
	sourceId := "source-id"

	dto := &api_beta.SourceApp{
		Id:                      &appId,
		Name:                    &name,
		Description:             &description,
		Enabled:                 &enabled,
		ProvisionRequestEnabled: &provisionRequestEnabled,
		AppCenterEnabled:        &appCenterEnabled,
		MatchAllAccounts:        &matchAllAccounts,
		Owner: *api_beta.NewNullableBaseReferenceDto(&api_beta.BaseReferenceDto{
			Id:   &ownerId,
			Type: &ownerType,
		}),
		AccountSource: *api_beta.NewNullableSourceAppAccountSource(&api_beta.SourceAppAccountSource{
			Id: &sourceId,
		}),
	}

	model, diags := applicationDtoToModel(ctx, dto, []string{"ap-1", "ap-2"}, fallback)
	if diags.HasError() {
		t.Fatalf("applicationDtoToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != appId {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), appId)
	}
	if model.Name.ValueString() != name {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), name)
	}
	if model.Owner.Id.ValueString() != ownerId {
		t.Errorf("Owner.Id = %q, want %q", model.Owner.Id.ValueString(), ownerId)
	}
	if model.Owner.OwnerType.ValueString() != string(ownerType) {
		t.Errorf("Owner.OwnerType = %q, want %q", model.Owner.OwnerType.ValueString(), ownerType)
	}
	if model.AccountSource.Id.ValueString() != sourceId {
		t.Errorf("AccountSource.Id = %q, want %q", model.AccountSource.Id.ValueString(), sourceId)
	}

	var accessProfileIds []string
	if diags := model.AccessProfileIds.ElementsAs(ctx, &accessProfileIds, false); diags.HasError() {
		t.Fatalf("ElementsAs returned diagnostics: %v", diags)
	}
	if len(accessProfileIds) != 2 || accessProfileIds[0] != "ap-1" || accessProfileIds[1] != "ap-2" {
		t.Errorf("AccessProfileIds = %v, want [ap-1 ap-2]", accessProfileIds)
	}
}

func TestOwnerToPatchMap(t *testing.T) {
	t.Run("null owner returns nil map", func(t *testing.T) {
		m, diags := ownerToPatchMap(resource_application.NewOwnerValueNull())
		if diags.HasError() {
			t.Fatalf("ownerToPatchMap returned diagnostics: %v", diags)
		}
		if m != nil {
			t.Errorf("m = %v, want nil", m)
		}
	})

	t.Run("populated owner returns id/type map", func(t *testing.T) {
		owner := ownerModel(t, "owner-id", "", "IDENTITY")
		m, diags := ownerToPatchMap(owner)
		if diags.HasError() {
			t.Fatalf("ownerToPatchMap returned diagnostics: %v", diags)
		}
		if m["id"] != "owner-id" {
			t.Errorf("m[id] = %v, want %q", m["id"], "owner-id")
		}
		if m["type"] != "IDENTITY" {
			t.Errorf("m[type] = %v, want %q", m["type"], "IDENTITY")
		}
	})
}

func TestOwnerFromAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("nil dto returns null owner", func(t *testing.T) {
		v, diags := ownerFromAPI(ctx, nil)
		if diags.HasError() {
			t.Fatalf("ownerFromAPI returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("v.IsNull() = false, want true")
		}
	})

	t.Run("populated dto", func(t *testing.T) {
		id := "owner-id"
		typ := api_beta.DTOTYPE_IDENTITY
		v, diags := ownerFromAPI(ctx, &api_beta.BaseReferenceDto{Id: &id, Type: &typ})
		if diags.HasError() {
			t.Fatalf("ownerFromAPI returned diagnostics: %v", diags)
		}
		if v.Id.ValueString() != id {
			t.Errorf("Id = %q, want %q", v.Id.ValueString(), id)
		}
		if v.OwnerType.ValueString() != string(typ) {
			t.Errorf("OwnerType = %q, want %q", v.OwnerType.ValueString(), typ)
		}
	})
}

func TestStringSetToArrayInner(t *testing.T) {
	ctx := context.Background()

	t.Run("null set returns nil", func(t *testing.T) {
		arr, diags := stringSetToArrayInner(ctx, types.SetNull(types.StringType))
		if diags.HasError() {
			t.Fatalf("stringSetToArrayInner returned diagnostics: %v", diags)
		}
		if arr != nil {
			t.Errorf("arr = %v, want nil", arr)
		}
	})

	t.Run("populated set", func(t *testing.T) {
		set, diags := types.SetValueFrom(ctx, types.StringType, []string{"ap-1", "ap-2"})
		if diags.HasError() {
			t.Fatalf("SetValueFrom returned diagnostics: %v", diags)
		}
		arr, diags := stringSetToArrayInner(ctx, set)
		if diags.HasError() {
			t.Fatalf("stringSetToArrayInner returned diagnostics: %v", diags)
		}
		if len(arr) != 2 {
			t.Fatalf("len(arr) = %d, want 2", len(arr))
		}
		got := map[string]bool{}
		for _, a := range arr {
			if a.String == nil {
				t.Fatalf("arr entry .String is nil, want populated")
			}
			got[*a.String] = true
		}
		if !got["ap-1"] || !got["ap-2"] {
			t.Errorf("arr = %v, want to contain ap-1 and ap-2", arr)
		}
	})
}

func TestApplicationJSONPatchReplace(t *testing.T) {
	name := "new-name"
	op := applicationJSONPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&name))

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

func TestApplicationUpdatePatchOps(t *testing.T) {
	ctx := context.Background()
	state := minimalApplicationModel()
	state.AccountSource = accountSourceModel(t, "source-id", "")
	state.Owner = ownerModel(t, "owner-id", "", "IDENTITY")

	t.Run("no changes produces no ops", func(t *testing.T) {
		plan := state
		ops, diags := applicationUpdatePatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("applicationUpdatePatchOps returned diagnostics: %v", diags)
		}
		if len(ops) != 0 {
			t.Errorf("len(ops) = %d, want 0, ops = %+v", len(ops), ops)
		}
	})

	t.Run("description change produces one op", func(t *testing.T) {
		plan := state
		plan.Description = types.StringValue("updated description")
		ops, diags := applicationUpdatePatchOps(ctx, plan, state)
		if diags.HasError() {
			t.Fatalf("applicationUpdatePatchOps returned diagnostics: %v", diags)
		}
		if len(ops) != 1 {
			t.Fatalf("len(ops) = %d, want 1, ops = %+v", len(ops), ops)
		}
		if ops[0].Path != "/description" {
			t.Errorf("ops[0].Path = %q, want %q", ops[0].Path, "/description")
		}
	})
}

func TestApplicationCreatePatchOps(t *testing.T) {
	ctx := context.Background()
	plan := minimalApplicationModel()
	plan.Owner = ownerModel(t, "owner-id", "", "IDENTITY")
	set, diags := types.SetValueFrom(ctx, types.StringType, []string{"ap-1"})
	if diags.HasError() {
		t.Fatalf("SetValueFrom returned diagnostics: %v", diags)
	}
	plan.AccessProfileIds = set

	ops, diags := applicationCreatePatchOps(ctx, plan)
	if diags.HasError() {
		t.Fatalf("applicationCreatePatchOps returned diagnostics: %v", diags)
	}

	paths := map[string]bool{}
	for _, op := range ops {
		paths[op.Path] = true
	}
	if !paths["/owner"] {
		t.Errorf("ops missing /owner, got %+v", ops)
	}
	if !paths["/accessProfiles"] {
		t.Errorf("ops missing /accessProfiles, got %+v", ops)
	}
}
