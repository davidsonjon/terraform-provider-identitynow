package sources_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/sources_v1/resource_source"
)

// minimalModel builds a sourceResourceModel with all nested object/list
// attributes explicitly null, suitable as a base for modelToDto/dtoToModel
// round-trip tests that only care about top-level scalar fields.
func minimalModel() sourceResourceModel {
	return sourceResourceModel{
		AccountCorrelationConfig:  resource_source.NewAccountCorrelationConfigValueNull(),
		AccountCorrelationRule:    resource_source.NewAccountCorrelationRuleValueNull(),
		Authoritative:             types.BoolNull(),
		BeforeProvisioningRule:    resource_source.NewBeforeProvisioningRuleValueNull(),
		Category:                  types.StringNull(),
		Cluster:                   resource_source.NewClusterValueNull(),
		ConnectionType:            types.StringNull(),
		Connector:                 types.StringValue("jdbc"),
		ConnectorAttributes:       jsontypes.NewNormalizedNull(),
		ConnectorClass:            types.StringNull(),
		ConnectorId:               types.StringNull(),
		ConnectorImplementationId: types.StringNull(),
		ConnectorName:             types.StringNull(),
		Created:                   types.StringNull(),
		CredentialProviderEnabled: types.BoolNull(),
		DeleteThreshold:           types.Int64Null(),
		Description:               types.StringValue("a test source"),
		Features:                  types.ListNull(types.StringType),
		Healthy:                   types.BoolNull(),
		Id:                        types.StringNull(),
		ManagementWorkgroup:       resource_source.NewManagementWorkgroupValueNull(),
		ManagerCorrelationMapping: resource_source.NewManagerCorrelationMappingValueNull(),
		ManagerCorrelationRule:    resource_source.NewManagerCorrelationRuleValueNull(),
		Modified:                  types.StringNull(),
		Name:                      types.StringValue("test-source"),
		Owner:                     resource_source.NewOwnerValueNull(),
		PasswordPolicies:          types.ListNull(resource_source.PasswordPoliciesValue{}.Type(context.Background())),
		Schemas:                   types.ListNull(resource_source.SchemasValue{}.Type(context.Background())),
		Since:                     types.StringNull(),
		Status:                    types.StringNull(),
		Type:                      types.StringValue("JDBC"),
	}
}

func TestModelToDto(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}

	if dto.Name != "test-source" {
		t.Errorf("Name = %q, want %q", dto.Name, "test-source")
	}
	if dto.Connector != "jdbc" {
		t.Errorf("Connector = %q, want %q", dto.Connector, "jdbc")
	}
	if dto.Description == nil || *dto.Description != "a test source" {
		t.Errorf("Description = %v, want %q", dto.Description, "a test source")
	}
	if dto.Type == nil || *dto.Type != "JDBC" {
		t.Errorf("Type = %v, want %q", dto.Type, "JDBC")
	}
	if dto.Owner.Get() != nil {
		t.Errorf("Owner = %v, want nil for a null OwnerValue", dto.Owner.Get())
	}
	if dto.Cluster.Get() != nil {
		t.Errorf("Cluster = %v, want nil for a null ClusterValue", dto.Cluster.Get())
	}
	if dto.ConnectorAttributes != nil {
		t.Errorf("ConnectorAttributes = %v, want nil for a null connector_attributes", dto.ConnectorAttributes)
	}
	// Category is never sent - Computed-only, server silently ignores it.
}

func TestModelToDto_ConnectorAttributes(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()
	model.ConnectorAttributes = jsontypes.NewNormalizedValue(`{"host":"db.example.com","port":5236}`)

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}

	if dto.ConnectorAttributes["host"] != "db.example.com" {
		t.Errorf("ConnectorAttributes[host] = %v, want %q", dto.ConnectorAttributes["host"], "db.example.com")
	}
	if dto.ConnectorAttributes["port"] != float64(5236) {
		t.Errorf("ConnectorAttributes[port] = %v, want %v", dto.ConnectorAttributes["port"], float64(5236))
	}
}

func TestDtoToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel()

	id := "src-id"
	dto := &api_beta.Source{
		Id:        &id,
		Name:      "test-source",
		Connector: "jdbc",
		Type:      strPtr("JDBC"),
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != "src-id" {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), "src-id")
	}
	if model.Name.ValueString() != "test-source" {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), "test-source")
	}
	if model.Connector.ValueString() != "jdbc" {
		t.Errorf("Connector = %q, want %q", model.Connector.ValueString(), "jdbc")
	}
}

// TestDtoToModel_ConnectorAttributes_FallbackConfigured exercises the branch
// where the practitioner's fallback model already has a configured
// connector_attributes value: dtoToModel must preserve that value verbatim
// even though the API response's connectorAttributes has been enriched with
// extra server-injected keys, to avoid a "Provider produced inconsistent
// result after apply" error (see the comment in dtoToModel above this logic).
func TestDtoToModel_ConnectorAttributes_FallbackConfigured(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel()
	configured := `{"host":"db.example.com"}`
	fallback.ConnectorAttributes = jsontypes.NewNormalizedValue(configured)

	dto := &api_beta.Source{
		Name:      "test-source",
		Connector: "jdbc",
		ConnectorAttributes: map[string]interface{}{
			"host":    "db.example.com",
			"healthy": true,
			"since":   "2024-01-01T00:00:00Z",
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.ConnectorAttributes.ValueString() != configured {
		t.Errorf("ConnectorAttributes = %q, want practitioner's configured value %q preserved verbatim",
			model.ConnectorAttributes.ValueString(), configured)
	}
}

// TestDtoToModel_ConnectorAttributes_FallbackNull exercises the branch where
// the fallback model has no configured connector_attributes value (e.g.
// after `terraform import`): dtoToModel must populate it from the API's
// response so imported/unmanaged sources still get a real value instead of
// null.
func TestDtoToModel_ConnectorAttributes_FallbackNull(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel() // ConnectorAttributes is null by default

	dto := &api_beta.Source{
		Name:      "test-source",
		Connector: "jdbc",
		ConnectorAttributes: map[string]interface{}{
			"host": "db.example.com",
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.ConnectorAttributes.IsNull() {
		t.Fatal("ConnectorAttributes is null, want populated from API response")
	}
	if model.ConnectorAttributes.ValueString() != `{"host":"db.example.com"}` {
		t.Errorf("ConnectorAttributes = %q, want %q", model.ConnectorAttributes.ValueString(), `{"host":"db.example.com"}`)
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
		cluster := &api_beta.MultiHostIntegrationsCluster{Id: "cluster-id", Name: "cluster-name", Type: "CLUSTER"}

		m, err := structToMap(cluster)
		if err != nil {
			t.Fatalf("structToMap returned error: %v", err)
		}
		if m["id"] != "cluster-id" {
			t.Errorf("m[id] = %v, want %q", m["id"], "cluster-id")
		}
		if m["name"] != "cluster-name" {
			t.Errorf("m[name] = %v, want %q", m["name"], "cluster-name")
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

func TestOptionalBoolPatch(t *testing.T) {
	if ops := optionalBoolPatch("/authoritative", nil); ops != nil {
		t.Errorf("optionalBoolPatch(nil) = %v, want nil", ops)
	}
	v := true
	ops := optionalBoolPatch("/authoritative", &v)
	if len(ops) != 1 || ops[0].Path != "/authoritative" || ops[0].Op != "replace" {
		t.Errorf("optionalBoolPatch(&v) = %v, want one replace op at /authoritative", ops)
	}
}

func TestOptionalInt32Patch(t *testing.T) {
	if ops := optionalInt32Patch("/deleteThreshold", nil); ops != nil {
		t.Errorf("optionalInt32Patch(nil) = %v, want nil", ops)
	}
	v := int32(10)
	ops := optionalInt32Patch("/deleteThreshold", &v)
	if len(ops) != 1 || ops[0].Path != "/deleteThreshold" || ops[0].Op != "replace" {
		t.Errorf("optionalInt32Patch(&v) = %v, want one replace op at /deleteThreshold", ops)
	}
}

func TestConnectorAttributesToMap(t *testing.T) {
	t.Run("null value", func(t *testing.T) {
		m, diags := connectorAttributesToMap(jsontypes.NewNormalizedNull())
		if diags.HasError() {
			t.Fatalf("connectorAttributesToMap(null) returned diagnostics: %v", diags)
		}
		if m != nil {
			t.Errorf("connectorAttributesToMap(null) = %v, want nil", m)
		}
	})

	t.Run("unknown value", func(t *testing.T) {
		m, diags := connectorAttributesToMap(jsontypes.NewNormalizedUnknown())
		if diags.HasError() {
			t.Fatalf("connectorAttributesToMap(unknown) returned diagnostics: %v", diags)
		}
		if m != nil {
			t.Errorf("connectorAttributesToMap(unknown) = %v, want nil", m)
		}
	})

	t.Run("empty string value", func(t *testing.T) {
		m, diags := connectorAttributesToMap(jsontypes.NewNormalizedValue(""))
		if diags.HasError() {
			t.Fatalf("connectorAttributesToMap(\"\") returned diagnostics: %v", diags)
		}
		if m != nil {
			t.Errorf("connectorAttributesToMap(\"\") = %v, want nil", m)
		}
	})

	t.Run("valid JSON object", func(t *testing.T) {
		m, diags := connectorAttributesToMap(jsontypes.NewNormalizedValue(`{"host":"db.example.com"}`))
		if diags.HasError() {
			t.Fatalf("connectorAttributesToMap returned diagnostics: %v", diags)
		}
		if m["host"] != "db.example.com" {
			t.Errorf("m[host] = %v, want %q", m["host"], "db.example.com")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, diags := connectorAttributesToMap(jsontypes.NewNormalizedValue(`not-json`))
		if !diags.HasError() {
			t.Fatal("connectorAttributesToMap(invalid JSON) returned no diagnostics, want an error")
		}
	})
}

func TestMergeConnectorAttributes(t *testing.T) {
	t.Run("both empty returns nil", func(t *testing.T) {
		if m := mergeConnectorAttributes(nil, nil); m != nil {
			t.Errorf("mergeConnectorAttributes(nil, nil) = %v, want nil", m)
		}
	})

	t.Run("configured keys win over live keys on conflict", func(t *testing.T) {
		live := map[string]interface{}{"host": "old.example.com", "healthy": true, "since": "2024-01-01"}
		configured := map[string]interface{}{"host": "new.example.com"}

		merged := mergeConnectorAttributes(live, configured)

		if merged["host"] != "new.example.com" {
			t.Errorf("merged[host] = %v, want %q (configured should win)", merged["host"], "new.example.com")
		}
		if merged["healthy"] != true {
			t.Errorf("merged[healthy] = %v, want true (live-only key should be preserved)", merged["healthy"])
		}
		if merged["since"] != "2024-01-01" {
			t.Errorf("merged[since] = %v, want %q (live-only key should be preserved)", merged["since"], "2024-01-01")
		}
	})

	t.Run("configured-only when live is nil", func(t *testing.T) {
		configured := map[string]interface{}{"host": "db.example.com"}
		merged := mergeConnectorAttributes(nil, configured)
		if merged["host"] != "db.example.com" {
			t.Errorf("merged[host] = %v, want %q", merged["host"], "db.example.com")
		}
		if len(merged) != 1 {
			t.Errorf("len(merged) = %d, want 1", len(merged))
		}
	})

	t.Run("live-only when configured is nil", func(t *testing.T) {
		live := map[string]interface{}{"healthy": true}
		merged := mergeConnectorAttributes(live, nil)
		if merged["healthy"] != true {
			t.Errorf("merged[healthy] = %v, want true", merged["healthy"])
		}
	})
}

func TestNormalizedConnectorAttributesFromAPI(t *testing.T) {
	t.Run("nil map returns null", func(t *testing.T) {
		v, diags := normalizedConnectorAttributesFromAPI(nil)
		if diags.HasError() {
			t.Fatalf("normalizedConnectorAttributesFromAPI(nil) returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("normalizedConnectorAttributesFromAPI(nil) = %v, want null", v)
		}
	})

	t.Run("populated map returns JSON string", func(t *testing.T) {
		v, diags := normalizedConnectorAttributesFromAPI(map[string]interface{}{"host": "db.example.com"})
		if diags.HasError() {
			t.Fatalf("normalizedConnectorAttributesFromAPI returned diagnostics: %v", diags)
		}
		if v.ValueString() != `{"host":"db.example.com"}` {
			t.Errorf("normalizedConnectorAttributesFromAPI = %q, want %q", v.ValueString(), `{"host":"db.example.com"}`)
		}
	})
}

func TestClusterFromAPI(t *testing.T) {
	ctx := context.Background()

	t.Run("nil cluster returns null ClusterValue", func(t *testing.T) {
		v, diags := clusterFromAPI(ctx, nil)
		if diags.HasError() {
			t.Fatalf("clusterFromAPI(nil) returned diagnostics: %v", diags)
		}
		if !v.IsNull() {
			t.Errorf("clusterFromAPI(nil) = %v, want null", v)
		}
	})

	t.Run("populated cluster maps fields", func(t *testing.T) {
		cluster := &api_beta.MultiHostIntegrationsCluster{Id: "cluster-id", Name: "cluster-name", Type: "CLUSTER"}
		v, diags := clusterFromAPI(ctx, cluster)
		if diags.HasError() {
			t.Fatalf("clusterFromAPI returned diagnostics: %v", diags)
		}
		if v.Id.ValueString() != "cluster-id" {
			t.Errorf("Id = %q, want %q", v.Id.ValueString(), "cluster-id")
		}
		if v.Name.ValueString() != "cluster-name" {
			t.Errorf("Name = %q, want %q", v.Name.ValueString(), "cluster-name")
		}
		if v.ClusterType.ValueString() != "CLUSTER" {
			t.Errorf("ClusterType = %q, want %q", v.ClusterType.ValueString(), "CLUSTER")
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
