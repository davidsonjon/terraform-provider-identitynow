package accessprofile

import (
	"log"

	v3 "github.com/davidsonjon/golang-sdk/v3"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AccessProfiles -
type AccessProfiles struct {
	AccessProfiles []AccessProfile `tfsdk:"governance_groups"`
}

type AccessProfile struct {
	// The ID of the Access Profile
	Id types.String `tfsdk:"id"`
	// Name of the Access Profile
	Name types.String `tfsdk:"name" json:"name"`
	// Information about the Access Profile
	Description types.String `json:"description" tfsdk:"description"`
	// Date the Access Profile was created
	Created types.String `tfsdk:"created"`
	// Date the Access Profile was last modified.
	// Modified types.String `tfsdk:"modified"`
	// Whether the Access Profile is enabled. If the Access Profile is enabled then you must include at least one Entitlement.
	Enabled types.Bool `tfsdk:"enabled"`
	// SourceID types.String `tfsdk:"source_id"`
	// OwnerID types.String `tfsdk:"owner_id"`
	Owner  *OwnerReference         `tfsdk:"owner"`
	Source *AccessProfileSourceRef `tfsdk:"source"`
	// A list of entitlements associated with the Access Profile. If enabled is false this is allowed to be empty otherwise it needs to contain at least one Entitlement.
	Entitlements []EntitlementRef `tfsdk:"entitlements"`
	// Whether the Access Profile is requestable via access request. Currently, making an Access Profile non-requestable is only supported  for customers enabled with the new Request Center. Otherwise, attempting to create an Access Profile with a value  **false** in this field results in a 400 error.
	Requestable             types.Bool      `tfsdk:"requestable"`
	AccessRequestConfig     *Requestability `tfsdk:"access_request_config"`
	RevocationRequestConfig *Revocability   `tfsdk:"revocation_request_config"`

	// List of IDs of segments, if any, to which this Access Profile is assigned.
	// Segments []string `tfsdk:"segments"`
	// ProvisioningCriteria NullableProvisioningCriteriaLevel1 `tfsdk:"provisioning_criteria"`
	// AdditionalProperties map[string]interface{}
}

func (ap *AccessProfile) parseConfiguredAttributes(v3ap *v3.AccessProfile) {
	log.Print("parseConfiguredAttributes")

	ap.Id = types.StringPointerValue(v3ap.Id)
	ap.Name = types.StringValue(v3ap.Name)
	ap.Description = types.StringValue(*v3ap.Description.Get())
	ap.Enabled = types.BoolPointerValue(v3ap.Enabled)

	ap.Owner = &OwnerReference{}
	ap.Owner.Name = types.StringValue(v3ap.Owner.GetName())
	ap.Owner.Id = types.StringValue(v3ap.Owner.GetId())
	ap.Owner.Type = types.StringValue((v3ap.Owner.GetType()))

	ap.Source = &AccessProfileSourceRef{}
	ap.Source.Id = types.StringPointerValue(v3ap.Source.Id)
	ap.Source.Name = types.StringPointerValue(v3ap.Source.Name)
	ap.Source.Type = types.StringPointerValue(v3ap.Source.Type)

	ap.Entitlements = []EntitlementRef{}

	for _, e := range v3ap.Entitlements {
		entitlement := EntitlementRef{
			// Name: types.StringPointerValue(e.Name),
			Id:   types.StringPointerValue(e.Id),
			Type: types.StringPointerValue(e.Type),
		}
		ap.Entitlements = append(ap.Entitlements, entitlement)
	}

	ap.Requestable = types.BoolPointerValue(v3ap.Requestable)

	if v3ap.AccessRequestConfig != nil {
		ap.AccessRequestConfig = &Requestability{}
		ap.AccessRequestConfig.CommentsRequired = types.BoolPointerValue(v3ap.AccessRequestConfig.CommentsRequired)
		ap.AccessRequestConfig.DenialCommentsRequired = types.BoolPointerValue(v3ap.AccessRequestConfig.DenialCommentsRequired)

		for _, a := range v3ap.AccessRequestConfig.ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				// ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			appId, err := a.ApproverId.MarshalJSON()
			if err != nil {
				log.Printf("error ApproverId.MarshalJSON: %+v\n", a.ApproverId)
			}
			if appId != nil {
				approval.ApproverId = types.StringPointerValue(a.ApproverId.Get())
			} else {
				approval.ApproverId = types.StringNull()
			}

			ap.AccessRequestConfig.ApprovalSchemes = append(ap.AccessRequestConfig.ApprovalSchemes, approval)
		}
	}
	//  else {
	// 	ap.AccessRequestConfig = nil
	// }

	log.Printf("v3ap.RevocationRequestConfig: %+v\n", v3ap.RevocationRequestConfig)

	if v3ap.RevocationRequestConfig != nil {
		ap.RevocationRequestConfig = &Revocability{}
		ap.RevocationRequestConfig.CommentsRequired = types.BoolPointerValue(v3ap.RevocationRequestConfig.CommentsRequired.Get())
		ap.RevocationRequestConfig.DenialCommentsRequired = types.BoolPointerValue(v3ap.RevocationRequestConfig.DenialCommentsRequired.Get())

		for _, a := range v3ap.RevocationRequestConfig.ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				// ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			if *a.ApproverId.Get() == "" {
				approval.ApproverId = types.StringNull()
			} else {
				approval.ApproverId = types.StringPointerValue(a.ApproverId.Get())
			}
			ap.RevocationRequestConfig.ApprovalSchemes = append(ap.RevocationRequestConfig.ApprovalSchemes, approval)
		}
	} else {
		ap.RevocationRequestConfig = nil
	}
}

func (ap *AccessProfile) parseComputedAttributes(v3ap *v3.AccessProfile) {

	ap.Id = types.StringValue(*v3ap.Id)
	ap.Created = types.StringValue(v3ap.Created.String())
	// ap.Modified = types.StringValue(v3ap.Modified.String())

}

type OwnerReference struct {
	Type types.String `tfsdk:"type"`
	// Identity id
	Id types.String `tfsdk:"id"`
	// Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

// AccessProfileSourceRef struct for AccessProfileSourceRef
type AccessProfileSourceRef struct {
	// The ID of the Source with with which the Access Profile is associated
	Id types.String `tfsdk:"id"`
	// The type of the Source, will always be SOURCE
	Type types.String `tfsdk:"type"`
	// The display name of the associated Source
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

// EntitlementRef struct for EntitlementRef
type EntitlementRef struct {
	// The ID of the Entitlement
	Id types.String `tfsdk:"id"`
	// The type of the Entitlement, will always be ENTITLEMENT
	Type types.String `tfsdk:"type"`
	// The display name of the Entitlement
	// Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
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

// Revocability struct for Revocability
type Revocability struct {
	// Whether the requester of the containing object must provide comments justifying the request
	CommentsRequired types.Bool `tfsdk:"comments_required"`
	// Whether an approver must provide comments when denying the request
	DenialCommentsRequired types.Bool `tfsdk:"denial_comments_required"`
	// List describing the steps in approving the revocation request
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

func convertAccessProfileV3(ap *AccessProfile) *v3.AccessProfile {

	accessProfile := v3.AccessProfile{}
	accessProfile.Name = ap.Name.ValueString()
	accessProfile.Description.Set(ap.Description.ValueStringPointer())
	accessProfile.Requestable = ap.Requestable.ValueBoolPointer()
	accessProfile.Enabled = ap.Enabled.ValueBoolPointer()

	accessProfile.Entitlements = []v3.EntitlementRef{}

	for _, ap := range ap.Entitlements {
		entitlement := v3.EntitlementRef{
			Id:   ap.Id.ValueStringPointer(),
			Type: ap.Type.ValueStringPointer(),
		}
		accessProfile.Entitlements = append(accessProfile.Entitlements, entitlement)
	}

	accessProfile.Owner.SetName(ap.Owner.Name.ValueString())
	accessProfile.Owner.SetId(ap.Owner.Id.ValueString())
	accessProfile.Owner.SetType(ap.Owner.Type.ValueString())

	accessProfileSourceRef := v3.AccessProfileSourceRef{}
	accessProfileSourceRef.Name = ap.Source.Name.ValueStringPointer()
	accessProfileSourceRef.Id = ap.Source.Id.ValueStringPointer()

	accessProfile.Source = accessProfileSourceRef

	if ap.AccessRequestConfig != nil {
		accessRequest := v3.NewRequestability()
		accessRequest.CommentsRequired = ap.AccessRequestConfig.CommentsRequired.ValueBoolPointer()
		accessRequest.DenialCommentsRequired = ap.AccessRequestConfig.DenialCommentsRequired.ValueBoolPointer()

		accessRequestSchema := []v3.AccessProfileApprovalScheme{}
		for _, ar := range ap.AccessRequestConfig.ApprovalSchemes {
			as := v3.AccessProfileApprovalScheme{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if ar.ApproverId.IsNull() {
				as.SetApproverId("")
			} else {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRequestSchema = append(accessRequestSchema, as)
		}
		accessRequest.ApprovalSchemes = accessRequestSchema
		accessProfile.AccessRequestConfig = accessRequest
	}

	if ap.RevocationRequestConfig != nil {
		accessRevoke := v3.NewRevocability()
		accessRevoke.CommentsRequired.Set(ap.RevocationRequestConfig.CommentsRequired.ValueBoolPointer())
		accessRevoke.DenialCommentsRequired.Set(ap.RevocationRequestConfig.DenialCommentsRequired.ValueBoolPointer())

		accessRevokeSchema := []v3.AccessProfileApprovalScheme{}
		for _, ar := range ap.RevocationRequestConfig.ApprovalSchemes {
			as := v3.AccessProfileApprovalScheme{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if ar.ApproverId.IsNull() {
				as.SetApproverId("")
			} else {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRevokeSchema = append(accessRevokeSchema, as)
		}
		accessRevoke.ApprovalSchemes = accessRevokeSchema
		accessProfile.RevocationRequestConfig = accessRevoke
	}

	return &accessProfile
}
