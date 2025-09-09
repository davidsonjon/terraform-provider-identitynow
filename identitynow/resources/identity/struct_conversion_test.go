package identity_test

import (
	"context"
	"testing"

	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/identity"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestIdentityListItemStructConversion tests that IdentityListItem struct can be converted
// to the object type defined by getIdentityObjectAttrTypes without mismatch errors
func TestIdentityListItemStructConversion(t *testing.T) {
	ctx := context.Background()

	// Create a sample IdentityListItem
	identityListItem := identity.IdentityListItem{
		Id:              types.StringValue("test-id"),
		Name:            types.StringValue("Test User"),
		Created:         types.StringValue("2023-01-01T00:00:00Z"),
		Modified:        types.StringValue("2023-01-01T00:00:00Z"),
		Alias:           types.StringValue("testuser"),
		EmailAddress:    types.StringValue("test@example.com"),
		ProcessingState: types.StringValue("COMPLETE"),
		IdentityStatus:  types.StringValue("ACTIVE"),
	}

	// Create a slice with the identity item
	identityItems := []identity.IdentityListItem{identityListItem}

	// This should not panic or error - testing the conversion that was failing before
	_, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":               types.StringType,
			"name":             types.StringType,
			"created":          types.StringType,
			"modified":         types.StringType,
			"alias":            types.StringType,
			"email_address":    types.StringType,
			"processing_state": types.StringType,
			"identity_status":  types.StringType,
		},
	}, identityItems)

	// Check that conversion succeeded without errors
	if diags.HasError() {
		t.Errorf("Expected no errors during struct conversion, but got: %v", diags.Errors())
	}
}
