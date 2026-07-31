package identity_profile_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/identity_profile_v1/resource_identity_profile"
)

// minimalModel builds an identityProfileResourceModel with all nested
// object attributes explicitly null, suitable as a base for
// modelToDto/dtoToModel round-trip tests that only care about top-level
// scalar fields - mirrors sources_v1/resource_source_test.go's helper of
// the same name.
func minimalModel() identityProfileResourceModel {
	return identityProfileResourceModel{
		AuthoritativeSource:              resource_identity_profile.NewAuthoritativeSourceValueNull(),
		Created:                          types.StringNull(),
		Description:                      types.StringValue("a test identity profile"),
		HasTimeBasedAttr:                 types.BoolNull(),
		Id:                               types.StringNull(),
		IdentityAttributeConfig:          jsontypes.NewNormalizedNull(),
		IdentityCount:                    types.Int64Null(),
		IdentityExceptionReportReference: resource_identity_profile.NewIdentityExceptionReportReferenceValueNull(),
		IdentityRefreshRequired:          types.BoolNull(),
		Modified:                         types.StringNull(),
		Name:                             types.StringValue("test-identity-profile"),
		Owner:                            resource_identity_profile.NewOwnerValueNull(),
		Priority:                         types.Int64Null(),
	}
}

func TestModelToDto(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}

	if dto.Name.Get() == nil || *dto.Name.Get() != "test-identity-profile" {
		t.Errorf("Name = %v, want %q", dto.Name.Get(), "test-identity-profile")
	}
	if dto.Description.Get() == nil || *dto.Description.Get() != "a test identity profile" {
		t.Errorf("Description = %v, want %q", dto.Description.Get(), "a test identity profile")
	}
	if dto.Owner.Get() != nil {
		t.Errorf("Owner = %v, want nil for a null OwnerValue", dto.Owner.Get())
	}
	if dto.Priority != nil {
		t.Errorf("Priority = %v, want nil", dto.Priority)
	}
	if dto.IdentityAttributeConfig != nil {
		t.Errorf("IdentityAttributeConfig = %v, want nil for a null identity_attribute_config", dto.IdentityAttributeConfig)
	}
}

func TestModelToDto_AuthoritativeSourceAndOwner(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()

	authSource, diags := resource_identity_profile.NewAuthoritativeSourceValue(
		model.AuthoritativeSource.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue("src-id"),
			"name": types.StringValue("src-name"),
			"type": types.StringValue("SOURCE"),
		},
	)
	if diags.HasError() {
		t.Fatalf("failed to build test AuthoritativeSourceValue: %v", diags)
	}
	model.AuthoritativeSource = authSource

	owner, diags := resource_identity_profile.NewOwnerValue(
		model.Owner.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringValue("owner-id"),
			"name": types.StringValue("owner-name"),
			"type": types.StringValue("IDENTITY"),
		},
	)
	if diags.HasError() {
		t.Fatalf("failed to build test OwnerValue: %v", diags)
	}
	model.Owner = owner

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}

	if dto.AuthoritativeSource.Id == nil || *dto.AuthoritativeSource.Id != "src-id" {
		t.Errorf("AuthoritativeSource.Id = %v, want %q", dto.AuthoritativeSource.Id, "src-id")
	}
	if dto.Owner.Get() == nil || dto.Owner.Get().Id == nil || *dto.Owner.Get().Id != "owner-id" {
		t.Errorf("Owner.Id = %v, want %q", dto.Owner.Get(), "owner-id")
	}
}

func TestModelToDto_IdentityAttributeConfig(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()
	model.IdentityAttributeConfig = jsontypes.NewNormalizedValue(`{"enabled":true}`)

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}
	if dto.IdentityAttributeConfig == nil {
		t.Fatal("IdentityAttributeConfig is nil, want populated")
	}
	if dto.IdentityAttributeConfig.Enabled == nil || !*dto.IdentityAttributeConfig.Enabled {
		t.Errorf("IdentityAttributeConfig.Enabled = %v, want true", dto.IdentityAttributeConfig.Enabled)
	}
}

func TestDtoToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel()

	id := "idprofile-id"
	dto := &api_beta.IdentityProfile{
		Id:   &id,
		Name: *api_beta.NewNullableString(strPtr("test-identity-profile")),
		AuthoritativeSource: api_beta.IdentityProfileAllOfAuthoritativeSource{
			Id:   strPtr("src-id"),
			Name: strPtr("src-name"),
			Type: strPtr("SOURCE"),
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != "idprofile-id" {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), "idprofile-id")
	}
	if model.Name.ValueString() != "test-identity-profile" {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), "test-identity-profile")
	}
	if model.AuthoritativeSource.Id.ValueString() != "src-id" {
		t.Errorf("AuthoritativeSource.Id = %q, want %q", model.AuthoritativeSource.Id.ValueString(), "src-id")
	}
}

// TestDtoToModel_IdentityAttributeConfig_FallbackConfigured exercises the
// branch where the practitioner's fallback model already has a configured
// identity_attribute_config value: dtoToModel must preserve that value
// verbatim rather than overwriting it with the live API's (potentially
// auto-populated/enriched) value, mirroring sources_v1's
// connector_attributes fallback-preservation pattern - see the comment
// above this logic in dtoToModel.
func TestDtoToModel_IdentityAttributeConfig_FallbackConfigured(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel()
	configured := `{"enabled":true}`
	fallback.IdentityAttributeConfig = jsontypes.NewNormalizedValue(configured)

	dto := &api_beta.IdentityProfile{
		Name: *api_beta.NewNullableString(strPtr("test-identity-profile")),
		AuthoritativeSource: api_beta.IdentityProfileAllOfAuthoritativeSource{
			Id: strPtr("src-id"),
		},
		IdentityAttributeConfig: &api_beta.IdentityAttributeConfig{
			Enabled: boolPtr(true),
			AttributeTransforms: []api_beta.IdentityAttributeTransform{
				{IdentityAttributeName: strPtr("uid")},
			},
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.IdentityAttributeConfig.ValueString() != configured {
		t.Errorf("IdentityAttributeConfig = %q, want practitioner's configured value %q preserved verbatim",
			model.IdentityAttributeConfig.ValueString(), configured)
	}
}

// TestDtoToModel_IdentityAttributeConfig_FallbackNull exercises the branch
// where the fallback model has no configured identity_attribute_config
// value (e.g. after `terraform import`, or a freshly created profile where
// none was set): dtoToModel must populate it from the API's response so
// unmanaged/imported profiles still get a real value instead of null.
func TestDtoToModel_IdentityAttributeConfig_FallbackNull(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel() // IdentityAttributeConfig is null by default

	dto := &api_beta.IdentityProfile{
		Name: *api_beta.NewNullableString(strPtr("test-identity-profile")),
		AuthoritativeSource: api_beta.IdentityProfileAllOfAuthoritativeSource{
			Id: strPtr("src-id"),
		},
		IdentityAttributeConfig: &api_beta.IdentityAttributeConfig{
			Enabled: boolPtr(true),
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.IdentityAttributeConfig.IsNull() {
		t.Fatal("IdentityAttributeConfig is null, want populated from API response")
	}
}

func TestStructToMap(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		m, err := structToMap(nil)
		if err != nil {
			t.Fatalf("structToMap(nil) returned error: %v", err)
		}
		if m != nil {
			t.Errorf("structToMap(nil) = %v, want nil", m)
		}
	})

	t.Run("struct input round-trips through JSON", func(t *testing.T) {
		src := api_beta.IdentityProfileAllOfAuthoritativeSource{Id: strPtr("src-id"), Name: strPtr("src-name"), Type: strPtr("SOURCE")}

		m, err := structToMap(src)
		if err != nil {
			t.Fatalf("structToMap returned error: %v", err)
		}
		if m["id"] != "src-id" {
			t.Errorf("m[id] = %v, want %q", m["id"], "src-id")
		}
		if m["name"] != "src-name" {
			t.Errorf("m[name] = %v, want %q", m["name"], "src-name")
		}
	})
}

func TestJsonPatchReplace(t *testing.T) {
	name := "new-name"
	op := jsonPatchReplace("/name", api_beta.StringAsUpdateMultiHostSourcesRequestInnerValue(&name))

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

func TestOptionalStringPatch(t *testing.T) {
	if ops := optionalStringPatch("/name", nil); ops != nil {
		t.Errorf("optionalStringPatch(nil) = %v, want nil", ops)
	}
	v := "new-name"
	ops := optionalStringPatch("/name", &v)
	if len(ops) != 1 || ops[0].Path != "/name" || ops[0].Op != "replace" {
		t.Errorf("optionalStringPatch(&v) = %v, want one replace op at /name", ops)
	}
}

func TestOptionalInt64Patch(t *testing.T) {
	if ops := optionalInt64Patch("/priority", nil); ops != nil {
		t.Errorf("optionalInt64Patch(nil) = %v, want nil", ops)
	}
	v := int64(30)
	ops := optionalInt64Patch("/priority", &v)
	if len(ops) != 1 || ops[0].Path != "/priority" || ops[0].Op != "replace" {
		t.Errorf("optionalInt64Patch(&v) = %v, want one replace op at /priority", ops)
	}
}

func TestIdentityAttributeConfigToApi(t *testing.T) {
	t.Run("null value", func(t *testing.T) {
		cfg, diags := identityAttributeConfigToApi(jsontypes.NewNormalizedNull())
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigToApi(null) returned diagnostics: %v", diags)
		}
		if cfg != nil {
			t.Errorf("identityAttributeConfigToApi(null) = %v, want nil", cfg)
		}
	})

	t.Run("unknown value", func(t *testing.T) {
		cfg, diags := identityAttributeConfigToApi(jsontypes.NewNormalizedUnknown())
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigToApi(unknown) returned diagnostics: %v", diags)
		}
		if cfg != nil {
			t.Errorf("identityAttributeConfigToApi(unknown) = %v, want nil", cfg)
		}
	})

	t.Run("empty string value", func(t *testing.T) {
		cfg, diags := identityAttributeConfigToApi(jsontypes.NewNormalizedValue(""))
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigToApi(\"\") returned diagnostics: %v", diags)
		}
		if cfg != nil {
			t.Errorf("identityAttributeConfigToApi(\"\") = %v, want nil", cfg)
		}
	})

	t.Run("valid JSON object", func(t *testing.T) {
		cfg, diags := identityAttributeConfigToApi(jsontypes.NewNormalizedValue(`{"enabled":true}`))
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigToApi returned diagnostics: %v", diags)
		}
		if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
			t.Errorf("cfg.Enabled = %v, want true", cfg)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, diags := identityAttributeConfigToApi(jsontypes.NewNormalizedValue(`not-json`))
		if !diags.HasError() {
			t.Fatal("identityAttributeConfigToApi(invalid JSON) returned no diagnostics, want an error")
		}
	})
}

func TestIdentityAttributeConfigFromAPI(t *testing.T) {
	t.Run("nil cfg returns null", func(t *testing.T) {
		v, diags := identityAttributeConfigFromAPI(nil)
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigFromAPI(nil) returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("identityAttributeConfigFromAPI(nil) = %v, want null", v)
		}
	})

	t.Run("populated cfg returns JSON string", func(t *testing.T) {
		v, diags := identityAttributeConfigFromAPI(&api_beta.IdentityAttributeConfig{Enabled: boolPtr(true)})
		if diags.HasError() {
			t.Fatalf("identityAttributeConfigFromAPI returned diagnostics: %v", diags)
		}
		if v.IsNull() {
			t.Fatal("identityAttributeConfigFromAPI(populated) is null, want a value")
		}
	})
}

func TestTimeToStringValue(t *testing.T) {
	if v := timeToStringValue(nil); !v.IsNull() {
		t.Errorf("timeToStringValue(nil) = %v, want null", v)
	}

	tm := api_beta.SailPointTime{}
	if err := tm.UnmarshalJSON([]byte(`"2024-01-02T03:04:05Z"`)); err != nil {
		t.Fatalf("failed to build test SailPointTime: %v", err)
	}
	v := timeToStringValue(&tm)
	if v.IsNull() {
		t.Fatal("timeToStringValue(populated) is null, want a value")
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
