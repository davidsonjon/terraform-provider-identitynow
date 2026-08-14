package source_actions_v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
)

func testActionMetadataAndSchema(t *testing.T, a action.Action, wantTypeName string) {
	t.Helper()

	var metaResp action.MetadataResponse
	a.Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "identitynow"}, &metaResp)
	if metaResp.TypeName != wantTypeName {
		t.Fatalf("TypeName = %q, want %q", metaResp.TypeName, wantTypeName)
	}

	var schemaResp action.SchemaResponse
	a.Schema(context.Background(), action.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() produced diagnostics: %v", schemaResp.Diagnostics)
	}
	if _, ok := schemaResp.Schema.Attributes["source_id"]; !ok {
		t.Fatal("schema is missing required source_id attribute")
	}
	if _, ok := schemaResp.Schema.Attributes["timeout"]; !ok {
		t.Fatal("schema is missing timeout attribute")
	}
}

func TestAggregateAccountsAction_MetadataAndSchema(t *testing.T) {
	testActionMetadataAndSchema(t, NewAggregateAccountsAction(), "identitynow_aggregate_accounts")
}

func TestAggregateEntitlementsAction_MetadataAndSchema(t *testing.T) {
	testActionMetadataAndSchema(t, NewAggregateEntitlementsAction(), "identitynow_aggregate_entitlements")
}

func TestSyncSourceAttributesAction_MetadataAndSchema(t *testing.T) {
	testActionMetadataAndSchema(t, NewSyncSourceAttributesAction(), "identitynow_sync_source_attributes")
}

func TestAggregateAccountsAction_ConfigureUnexpectedType(t *testing.T) {
	a := &AggregateAccountsAction{}
	var resp action.ConfigureResponse
	a.Configure(context.Background(), action.ConfigureRequest{ProviderData: "not-a-client-provider"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure() with an unexpected ProviderData type produced no diagnostics, want an error")
	}
}

func TestAggregateAccountsAction_ConfigureNilProviderData(t *testing.T) {
	a := &AggregateAccountsAction{}
	var resp action.ConfigureResponse
	a.Configure(context.Background(), action.ConfigureRequest{ProviderData: nil}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure() with nil ProviderData produced diagnostics: %v", resp.Diagnostics)
	}
}
