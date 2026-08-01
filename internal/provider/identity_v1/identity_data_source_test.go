package identity_v1

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sailpoint-oss/golang-sdk/v3/identities"
)

func TestIdentityDataSourceDTOToModel(t *testing.T) {
	ctx := context.Background()
	created := identities.SailPointTime{Time: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)}
	modified := identities.SailPointTime{Time: time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)}
	lastRefresh := identities.SailPointTime{Time: time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)}

	dto := identities.NewIdentity("Test User")
	dto.SetId("identity-id")
	dto.SetAlias("M200082")
	dto.SetEmailAddress("test.user@example.com")
	dto.SetProcessingState("PROCESSED")
	dto.SetIdentityStatus("ACTIVE")
	dto.SetIsManager(true)
	dto.SetCreated(created)
	dto.SetModified(modified)
	dto.SetLastRefresh(lastRefresh)
	dto.SetAttributes(map[string]interface{}{"department": "Engineering", "employeeNumber": float64(7)})
	dto.SetLifecycleState(*identities.NewIdentityLifecycleState("active", true))

	managerRef := identities.NewIdentityManagerRef()
	managerRef.SetId("manager-id")
	managerRef.SetName("Manager Name")
	managerRef.SetType("IDENTITY")
	dto.SetManagerRef(*managerRef)

	fallback := identityDataSourceModel{
		Id:           types.StringValue("fallback-id"),
		Alias:        types.StringValue("fallback-alias"),
		EmailAddress: types.StringValue("fallback@example.com"),
	}

	model, diags := identityDataSourceDTOToModel(ctx, dto, fallback)
	if diags.HasError() {
		t.Fatalf("identityDataSourceDTOToModel returned diagnostics: %v", diags)
	}

	if model.Id.ValueString() != "identity-id" {
		t.Errorf("Id = %q, want %q", model.Id.ValueString(), "identity-id")
	}
	if model.Alias.ValueString() != "M200082" {
		t.Errorf("Alias = %q, want %q", model.Alias.ValueString(), "M200082")
	}
	if model.EmailAddress.ValueString() != "test.user@example.com" {
		t.Errorf("EmailAddress = %q, want %q", model.EmailAddress.ValueString(), "test.user@example.com")
	}
	if model.Name.ValueString() != "Test User" {
		t.Errorf("Name = %q, want %q", model.Name.ValueString(), "Test User")
	}
	if model.IdentityStatus.ValueString() != "ACTIVE" {
		t.Errorf("IdentityStatus = %q, want %q", model.IdentityStatus.ValueString(), "ACTIVE")
	}
	if !model.IsManager.ValueBool() {
		t.Error("IsManager = false, want true")
	}
	if model.ProcessingState.ValueString() != "PROCESSED" {
		t.Errorf("ProcessingState = %q, want %q", model.ProcessingState.ValueString(), "PROCESSED")
	}
	if model.Created.ValueString() != created.Format(time.RFC3339) {
		t.Errorf("Created = %q, want %q", model.Created.ValueString(), created.Format(time.RFC3339))
	}
	if model.Modified.ValueString() != modified.Format(time.RFC3339) {
		t.Errorf("Modified = %q, want %q", model.Modified.ValueString(), modified.Format(time.RFC3339))
	}
	if model.LastRefresh.ValueString() != lastRefresh.Format(time.RFC3339) {
		t.Errorf("LastRefresh = %q, want %q", model.LastRefresh.ValueString(), lastRefresh.Format(time.RFC3339))
	}
	if model.LifecycleState.StateName.ValueString() != "active" {
		t.Errorf("LifecycleState.StateName = %q, want %q", model.LifecycleState.StateName.ValueString(), "active")
	}
	if !model.LifecycleState.ManuallyUpdated.ValueBool() {
		t.Error("LifecycleState.ManuallyUpdated = false, want true")
	}
	if model.ManagerRef.Id.ValueString() != "manager-id" {
		t.Errorf("ManagerRef.Id = %q, want %q", model.ManagerRef.Id.ValueString(), "manager-id")
	}
	if model.ManagerRef.Name.ValueString() != "Manager Name" {
		t.Errorf("ManagerRef.Name = %q, want %q", model.ManagerRef.Name.ValueString(), "Manager Name")
	}
	if model.ManagerRef.ManagerRefType.ValueString() != "IDENTITY" {
		t.Errorf("ManagerRef.Type = %q, want %q", model.ManagerRef.ManagerRefType.ValueString(), "IDENTITY")
	}

	var gotAttributes map[string]interface{}
	if err := json.Unmarshal([]byte(model.Attributes.ValueString()), &gotAttributes); err != nil {
		t.Fatalf("json.Unmarshal(attributes) returned error: %v", err)
	}
	if gotAttributes["department"] != "Engineering" {
		t.Errorf("attributes.department = %v, want %q", gotAttributes["department"], "Engineering")
	}
	if gotAttributes["employeeNumber"] != float64(7) {
		t.Errorf("attributes.employeeNumber = %v, want %v", gotAttributes["employeeNumber"], float64(7))
	}
}

func TestIdentitiesListItemFromDTO(t *testing.T) {
	ctx := context.Background()
	created := identities.SailPointTime{Time: time.Date(2026, time.April, 5, 6, 7, 8, 0, time.UTC)}
	modified := identities.SailPointTime{Time: time.Date(2026, time.May, 6, 7, 8, 9, 0, time.UTC)}
	lastRefresh := identities.SailPointTime{Time: time.Date(2026, time.June, 7, 8, 9, 10, 0, time.UTC)}

	dto := identities.NewIdentity("Another User")
	dto.SetId("identity-2")
	dto.SetAlias("A200001")
	dto.SetEmailAddress("another.user@example.com")
	dto.SetProcessingState("COMPLETE")
	dto.SetIdentityStatus("ACTIVE")
	dto.SetIsManager(false)
	dto.SetCreated(created)
	dto.SetModified(modified)
	dto.SetLastRefresh(lastRefresh)
	dto.SetLifecycleState(*identities.NewIdentityLifecycleState("inactive", false))

	managerRef := identities.NewIdentityManagerRef()
	managerRef.SetId("manager-2")
	managerRef.SetName("Second Manager")
	managerRef.SetType("IDENTITY")
	dto.SetManagerRef(*managerRef)

	item, diags := identitiesListItemFromDTO(ctx, dto)
	if diags.HasError() {
		t.Fatalf("identitiesListItemFromDTO returned diagnostics: %v", diags)
	}

	if item.Id.ValueString() != "identity-2" {
		t.Errorf("Id = %q, want %q", item.Id.ValueString(), "identity-2")
	}
	if item.Alias.ValueString() != "A200001" {
		t.Errorf("Alias = %q, want %q", item.Alias.ValueString(), "A200001")
	}
	if item.EmailAddress.ValueString() != "another.user@example.com" {
		t.Errorf("EmailAddress = %q, want %q", item.EmailAddress.ValueString(), "another.user@example.com")
	}
	if item.Name.ValueString() != "Another User" {
		t.Errorf("Name = %q, want %q", item.Name.ValueString(), "Another User")
	}
	if item.IdentityStatus.ValueString() != "ACTIVE" {
		t.Errorf("IdentityStatus = %q, want %q", item.IdentityStatus.ValueString(), "ACTIVE")
	}
	if item.IsManager.ValueBool() {
		t.Error("IsManager = true, want false")
	}
	if item.ProcessingState.ValueString() != "COMPLETE" {
		t.Errorf("ProcessingState = %q, want %q", item.ProcessingState.ValueString(), "COMPLETE")
	}
	if item.Created.ValueString() != created.Format(time.RFC3339) {
		t.Errorf("Created = %q, want %q", item.Created.ValueString(), created.Format(time.RFC3339))
	}
	if item.Modified.ValueString() != modified.Format(time.RFC3339) {
		t.Errorf("Modified = %q, want %q", item.Modified.ValueString(), modified.Format(time.RFC3339))
	}
	if item.LastRefresh.ValueString() != lastRefresh.Format(time.RFC3339) {
		t.Errorf("LastRefresh = %q, want %q", item.LastRefresh.ValueString(), lastRefresh.Format(time.RFC3339))
	}

	lifecycleAttrs := item.LifecycleState.Attributes()
	if lifecycleAttrs["state_name"].(types.String).ValueString() != "inactive" {
		t.Errorf("LifecycleState.state_name = %q, want %q", lifecycleAttrs["state_name"].(types.String).ValueString(), "inactive")
	}
	if lifecycleAttrs["manually_updated"].(types.Bool).ValueBool() {
		t.Error("LifecycleState.manually_updated = true, want false")
	}

	managerAttrs := item.ManagerRef.Attributes()
	if managerAttrs["id"].(types.String).ValueString() != "manager-2" {
		t.Errorf("ManagerRef.id = %q, want %q", managerAttrs["id"].(types.String).ValueString(), "manager-2")
	}
	if managerAttrs["name"].(types.String).ValueString() != "Second Manager" {
		t.Errorf("ManagerRef.name = %q, want %q", managerAttrs["name"].(types.String).ValueString(), "Second Manager")
	}
	if managerAttrs["type"].(types.String).ValueString() != "IDENTITY" {
		t.Errorf("ManagerRef.type = %q, want %q", managerAttrs["type"].(types.String).ValueString(), "IDENTITY")
	}
}

func TestIdentityLookupSpecFromConfig(t *testing.T) {
	tests := []struct {
		name   string
		config identityDataSourceModel
		want   identityLookupSpec
	}{
		{
			name: "id lookup",
			config: identityDataSourceModel{
				Id: types.StringValue("identity-id"),
			},
			want: identityLookupSpec{LookupID: "identity-id"},
		},
		{
			name: "alias lookup trims whitespace",
			config: identityDataSourceModel{
				Alias: types.StringValue("  M200082  "),
			},
			want: identityLookupSpec{
				FilterPattern: "alias eq \"%s\"",
				LookupValue:   "M200082",
				LookupAttr:    "alias",
				LookupLabel:   "alias",
			},
		},
		{
			name: "email lookup trims whitespace",
			config: identityDataSourceModel{
				EmailAddress: types.StringValue("  test.user@example.com  "),
			},
			want: identityLookupSpec{
				FilterPattern: "email eq \"%s\"",
				LookupValue:   "test.user@example.com",
				LookupAttr:    "email_address",
				LookupLabel:   "email address",
			},
		},
		{
			name: "id wins if multiple are set",
			config: identityDataSourceModel{
				Id:           types.StringValue("identity-id"),
				Alias:        types.StringValue("alias"),
				EmailAddress: types.StringValue("test.user@example.com"),
			},
			want: identityLookupSpec{LookupID: "identity-id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identityLookupSpecFromConfig(tt.config)
			if got != tt.want {
				t.Errorf("identityLookupSpecFromConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIdentityFromMatches(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		got, diags := identityFromMatches(nil, "missing@example.com", "email_address", "email address")
		if got != nil {
			t.Fatalf("got = %v, want nil", got)
		}
		if !diags.HasError() {
			t.Fatal("diags.HasError() = false, want true")
		}
		if diags[0].Summary() != "Identity not found by email_address" {
			t.Errorf("Summary = %q, want %q", diags[0].Summary(), "Identity not found by email_address")
		}
		if !strings.Contains(diags[0].Detail(), "missing@example.com") {
			t.Errorf("Detail = %q, want to mention lookup value", diags[0].Detail())
		}
	})

	t.Run("single match", func(t *testing.T) {
		id := "identity-id"
		got, diags := identityFromMatches([]identities.Identity{{Id: &id}}, "M200082", "alias", "alias")
		if diags.HasError() {
			t.Fatalf("identityFromMatches returned diagnostics: %v", diags)
		}
		if got == nil || got.Id == nil || *got.Id != id {
			t.Fatalf("got = %+v, want id %q", got, id)
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		id1 := "identity-1"
		id2 := "identity-2"
		got, diags := identityFromMatches(
			[]identities.Identity{{Id: &id1}, {Id: &id2}},
			"M200082",
			"alias",
			"alias",
		)
		if got != nil {
			t.Fatalf("got = %v, want nil", got)
		}
		if !diags.HasError() {
			t.Fatal("diags.HasError() = false, want true")
		}
		if diags[0].Summary() != "Identity alias is not unique" {
			t.Errorf("Summary = %q, want %q", diags[0].Summary(), "Identity alias is not unique")
		}
		for _, wantSnippet := range []string{"identity-1", "identity-2", "M200082"} {
			if !strings.Contains(diags[0].Detail(), wantSnippet) {
				t.Errorf("Detail = %q, want to contain %q", diags[0].Detail(), wantSnippet)
			}
		}
	})
}

func TestIdentityDataSourceConfigValidators(t *testing.T) {
	validators := (&identityDataSource{}).ConfigValidators(context.Background())
	if len(validators) != 1 {
		t.Fatalf("len(validators) = %d, want 1", len(validators))
	}
}

func TestIdentitiesListItemFromDTO_NilDTO(t *testing.T) {
	ctx := context.Background()
	item, diags := identitiesListItemFromDTO(ctx, nil)
	if diags.HasError() {
		t.Fatalf("identitiesListItemFromDTO returned diagnostics: %v", diags)
	}
	if !item.Id.IsNull() {
		t.Errorf("Id.IsNull() = false, want true")
	}
	if !item.LifecycleState.IsNull() {
		t.Errorf("LifecycleState.IsNull() = false, want true")
	}
	if !item.ManagerRef.IsNull() {
		t.Errorf("ManagerRef.IsNull() = false, want true")
	}
}
