package service_desk_integration_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/service_desk_integration_v1/resource_service_desk_integration"
)

// minimalModel builds a ServiceDeskIntegrationModel with all nested
// object/list attributes explicitly null, suitable as a base for
// modelToDto/dtoToModel round-trip tests that only care about top-level
// scalar fields.
func minimalModel() resource_service_desk_integration.ServiceDeskIntegrationModel {
	return resource_service_desk_integration.ServiceDeskIntegrationModel{
		Attributes:             resource_service_desk_integration.NewAttributesValueNull(),
		BeforeProvisioningRule: resource_service_desk_integration.NewBeforeProvisioningRuleValueNull(),
		Cluster:                types.StringNull(),
		ClusterRef:             resource_service_desk_integration.NewClusterRefValueNull(),
		Created:                types.StringNull(),
		Description:            types.StringValue("a test integration"),
		Id:                     types.StringNull(),
		ManagedSources:         types.ListNull(types.StringType),
		Modified:               types.StringNull(),
		Name:                   types.StringValue("test-sdi"),
		OwnerRef:               resource_service_desk_integration.NewOwnerRefValueNull(),
		ProvisioningConfig:     resource_service_desk_integration.NewProvisioningConfigValueNull(),
		Type:                   types.StringValue("WEBHOOK"),
	}
}

func TestModelToDto(t *testing.T) {
	ctx := context.Background()
	model := minimalModel()

	dto, diags := modelToDto(ctx, model)
	if diags.HasError() {
		t.Fatalf("modelToDto returned diagnostics: %v", diags)
	}

	if dto.Name != "test-sdi" {
		t.Errorf("Name = %q, want %q", dto.Name, "test-sdi")
	}
	if dto.Description != "a test integration" {
		t.Errorf("Description = %q, want %q", dto.Description, "a test integration")
	}
	if dto.Type != "WEBHOOK" {
		t.Errorf("Type = %q, want %q", dto.Type, "WEBHOOK")
	}
	// "attributes" has no schema-defined properties; always sent as an empty object.
	if dto.Attributes == nil || len(dto.Attributes) != 0 {
		t.Errorf("Attributes = %v, want empty non-nil map", dto.Attributes)
	}
	if dto.OwnerRef != nil {
		t.Errorf("OwnerRef = %v, want nil for a null OwnerRefValue", dto.OwnerRef)
	}
	if dto.ClusterRef != nil {
		t.Errorf("ClusterRef = %v, want nil for a null ClusterRefValue", dto.ClusterRef)
	}
	if dto.ProvisioningConfig != nil {
		t.Errorf("ProvisioningConfig = %v, want nil for a null ProvisioningConfigValue", dto.ProvisioningConfig)
	}
}

func TestDtoToModel_RoundTrip(t *testing.T) {
	ctx := context.Background()
	fallback := minimalModel()

	ownerId := "owner-id"
	dto := &api_beta.ServiceDeskIntegrationDto{
		Name:        "test-sdi",
		Description: "a test integration",
		Type:        "WEBHOOK",
		OwnerRef: &api_beta.OwnerDto{
			Id: &ownerId,
		},
		Attributes: map[string]interface{}{},
		AdditionalProperties: map[string]interface{}{
			"id": "abc123",
		},
	}

	model, diags := dtoToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("dtoToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != "abc123" {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), "abc123")
	}
	if model.Name.ValueString() != "test-sdi" {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), "test-sdi")
	}
	if model.OwnerRef.Id.ValueString() != "owner-id" {
		t.Errorf("OwnerRef.Id = %q, want %q", model.OwnerRef.Id.ValueString(), "owner-id")
	}
}

func TestDtoID(t *testing.T) {
	tests := []struct {
		name string
		dto  *api_beta.ServiceDeskIntegrationDto
		want string
	}{
		{"nil dto", nil, ""},
		{"nil AdditionalProperties", &api_beta.ServiceDeskIntegrationDto{}, ""},
		{
			"id present as string",
			&api_beta.ServiceDeskIntegrationDto{AdditionalProperties: map[string]interface{}{"id": "abc123"}},
			"abc123",
		},
		{
			"id present but wrong type",
			&api_beta.ServiceDeskIntegrationDto{AdditionalProperties: map[string]interface{}{"id": 42}},
			"",
		},
		{
			"id absent",
			&api_beta.ServiceDeskIntegrationDto{AdditionalProperties: map[string]interface{}{"other": "value"}},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dtoID(tt.dto); got != tt.want {
				t.Errorf("dtoID() = %q, want %q", got, tt.want)
			}
		})
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
		id := "owner-id"
		name := "owner-name"
		owner := &api_beta.OwnerDto{Id: &id, Name: &name}

		m, err := structToMap(owner)
		if err != nil {
			t.Fatalf("structToMap returned error: %v", err)
		}
		if m["id"] != "owner-id" {
			t.Errorf("m[id] = %v, want %q", m["id"], "owner-id")
		}
		if m["name"] != "owner-name" {
			t.Errorf("m[name] = %v, want %q", m["name"], "owner-name")
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
