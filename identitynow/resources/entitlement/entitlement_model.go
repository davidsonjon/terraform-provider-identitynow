package entitlement

import (
	beta "github.com/davidsonjon/golang-sdk/beta"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Entitlement struct for Entitlement
type Entitlement struct {
	// The entitlement id
	Id types.String `tfsdk:"id"`
	// The entitlement name
	Name types.String `tfsdk:"name"`
	// Time when the entitlement was created
	Created types.String `tfsdk:"created"`
	// Time when the entitlement was last modified
	Modified types.String `tfsdk:"modified"`
	// The entitlement attribute name
	Attribute types.String `tfsdk:"attribute"`
	// The value of the entitlement
	Value types.String `tfsdk:"value"`
	// The object type of the entitlement from the source schema
	SourceSchemaObjectType types.String `tfsdk:"source_schema_object_type"`
	// True if the entitlement is privileged
	Privileged types.Bool `tfsdk:"privileged"`
	// True if the entitlement is cloud governed
	CloudGoverned types.Bool `tfsdk:"cloud_governed"`
	// The description of the entitlement
	Description types.String `tfsdk:"description"`
	// True if the entitlement is requestable
	Requestable types.Bool `tfsdk:"requestable"`
	// A map of free-form key-value pairs from the source system
	// Attributes map[string]interface{} `tfsdk:"attributes"`
	SourceID types.String `tfsdk:"source_id"`
	OwnerID  types.String `tfsdk:"owner_id"`
	// Source types.Object `tfsdk:"source"`
	// Owner types.Object `tfsdk:"owner"`
	// DirectPermissions []PermissionDto `tfsdk:"directPermissions"`
	// // List of IDs of segments, if any, to which this Entitlement is assigned.
	// Segments []string `tfsdk:"segments"`
	// ManuallyUpdatedFields *ManuallyUpdatedFieldsDTO `tfsdk:"manuallyUpdatedFields"`
	// AdditionalProperties map[string]interface{}
}

// EntitlementSource struct for EntitlementSource
type EntitlementSource struct {
	// The source ID
	Id types.String `tfsdk:"id"`
	// The source type, will always be \"SOURCE\"
	Type types.String `tfsdk:"type"`
	// The source name
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

var entitlementSourceTypes = map[string]attr.Type{
	"name": types.StringType,
	"id":   types.StringType,
	"type": types.StringType,
}

// OwnerReferenceDto Simplified DTO for the owner object of the entitlement
type OwnerReferenceDto struct {
	// The owner id for the entitlement
	Id types.String `tfsdk:"id"`
	// The owner name for the entitlement
	Name types.String `tfsdk:"name"`
	// The type of the owner. Initially only type IDENTITY is supported
	Type types.String `tfsdk:"type"`
	// AdditionalProperties map[string]interface{}
}

var ownerReferenceDtoTypes = map[string]attr.Type{
	"name": types.StringType,
	"id":   types.StringType,
	"type": types.StringType,
}

func convertEntitlementBeta(ent *Entitlement) *beta.Entitlement {

	betaEnt := beta.Entitlement{}

	betaEnt.Privileged = ent.Privileged.ValueBoolPointer()
	betaEnt.Requestable = ent.Requestable.ValueBoolPointer()

	return &betaEnt
}
