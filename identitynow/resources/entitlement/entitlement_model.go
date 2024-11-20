package entitlement

import (
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
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
	// OwnerID  types.String `tfsdk:"owner_id"`
	Owner *OwnerReference `tfsdk:"owner"`
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

func convertEntitlementBeta(ent *Entitlement) *beta.Entitlement {
	betaEnt := beta.Entitlement{}

	betaEnt.Privileged = ent.Privileged.ValueBoolPointer()
	if ent.Requestable.IsNull() {
		betaEnt.Requestable = beta.PtrBool(false)
	} else {
		betaEnt.Requestable = ent.Requestable.ValueBoolPointer()
	}
	owner := beta.EntitlementOwner{}
	owner.Id = betaEnt.Owner.Id
	owner.Name = betaEnt.Owner.Name
	owner.Type = betaEnt.Owner.Type

	betaEnt.Owner = &owner

	return &betaEnt
}

// Entitlement struct for Entitlement
type EntitlementRequestConfig struct {
	// The entitlement id
	Id                  types.String    `tfsdk:"id"`
	AccessRequestConfig *Requestability `tfsdk:"access_request_config"`
}

// Requestability struct for Requestability
type Requestability struct {
	// Whether the requester of the containing object must provide comments justifying the request
	CommentsRequired types.Bool `tfsdk:"comments_required"`
	// Whether an approver must provide comments when denying the request
	DenialCommentsRequired types.Bool `tfsdk:"denial_comments_required"`
	// List describing the steps in approving the request
	ApprovalSchemes []AccessProfileApprovalScheme `tfsdk:"approval_schemes"`
	// AdditionalProperties map[string]interface{}
}

// AccessProfileApprovalScheme struct for AccessProfileApprovalScheme
type AccessProfileApprovalScheme struct {
	// Describes the individual or group that is responsible for an approval step. Values are as follows. **APP_OWNER**: The owner of the Application  **OWNER**: Owner of the associated Access Profile or Role  **SOURCE_OWNER**: Owner of the Source associated with an Access Profile  **MANAGER**: Manager of the Identity making the request  **GOVERNANCE_GROUP**: A Governance Group, the ID of which is specified by the **approverId** field
	ApproverType types.String `tfsdk:"approver_type"`
	// Id of the specific approver, used only when approverType is GOVERNANCE_GROUP
	ApproverId types.String `tfsdk:"approver_id"`
	// AdditionalProperties map[string]interface{}
}

type OwnerReference struct {
	Type types.String `tfsdk:"type"`
	// Identity id
	Id types.String `tfsdk:"id"`
	// Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

var OwnerSchemeObject = map[string]attr.Type{
	"type": types.StringType,
	"id":   types.StringType,
	"name": types.StringType,
}
