package accessprofile

import (
	"log"

	v3 "github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	Name types.String `tfsdk:"name"`
	// Information about the Access Profile
	Description types.String `tfsdk:"description"`
	// Date the Access Profile was created
	Created types.String `tfsdk:"created"`
	// Date the Access Profile was last modified.
	// Modified types.String `tfsdk:"modified"`
	// Whether the Access Profile is enabled. If the Access Profile is enabled then you must include at least one Entitlement.
	Enabled types.Bool              `tfsdk:"enabled"`
	Owner   *OwnerReference         `tfsdk:"owner"`
	Source  *AccessProfileSourceRef `tfsdk:"source"`
	// A list of entitlements associated with the Access Profile. If enabled is false this is allowed to be empty otherwise it needs to contain at least one Entitlement.
	Entitlements []EntitlementRef `tfsdk:"entitlements"`
	// Whether the Access Profile is requestable via access request. Currently, making an Access Profile non-requestable is only supported  for customers enabled with the new Request Center. Otherwise, attempting to create an Access Profile with a value  **false** in this field results in a 400 error.
	Requestable             types.Bool      `tfsdk:"requestable"`
	AccessRequestConfig     *Requestability `tfsdk:"access_request_config"`
	RevocationRequestConfig *Revocability   `tfsdk:"revocation_request_config"`

	// List of IDs of segments, if any, to which this Access Profile is assigned.
	// Segments []string `tfsdk:"segments"`
	ProvisioningCriteria *ProvisioningCriteriaLevel1 `tfsdk:"provisioning_criteria"`
	// AdditionalProperties map[string]interface{}
}

var ProvisioningCriteriaLevel3Object = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"operation": types.StringType,
		"attribute": types.StringType,
		"value":     types.StringType,
	},
}

var ProvisioningCriteriaLevel2Object = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"operation": types.StringType,
		"attribute": types.StringType,
		"value":     types.StringType,
		"children":  types.ListType{ElemType: ProvisioningCriteriaLevel3Object},
	},
}

var ProvisioningCriteriaLevel1Type = map[string]attr.Type{
	"operation": types.StringType,
	"attribute": types.StringType,
	"value":     types.StringType,
	"children":  types.ListType{ElemType: ProvisioningCriteriaLevel2Object},
}

type ProvisioningCriteriaLevel1 struct {
	Operation types.String `tfsdk:"operation"`
	// Name of the Account attribute to be tested. If **operation** is one of EQUALS, NOT_EQUALS, CONTAINS, or HAS, this field is required. Otherwise, specifying it is an error.
	Attribute types.String `tfsdk:"attribute"`
	// String value to test the Account attribute with regard to the specified operation. If the operation is one of EQUALS, NOT_EQUALS, or CONTAINS, this field is required. Otherwise, specifying it is an error. If the Attribute is not String-typed, it will be converted to the appropriate type.
	Value types.String `tfsdk:"value"`
	// Array of child criteria. Required if the operation is AND or OR, otherwise it must be left null. A maximum of three levels of criteria are supported, including leaf nodes.
	Children []ProvisioningCriteriaLevel2 `tfsdk:"children"`
	// AdditionalProperties map[string]interface{}
}

type ProvisioningCriteriaLevel2 struct {
	Operation types.String `tfsdk:"operation"`
	// Name of the Account attribute to be tested. If **operation** is one of EQUALS, NOT_EQUALS, CONTAINS, or HAS, this field is required. Otherwise, specifying it is an error.
	Attribute types.String `tfsdk:"attribute"`
	// String value to test the Account attribute with regard to the specified operation. If the operation is one of EQUALS, NOT_EQUALS, or CONTAINS, this field is required. Otherwise, specifying it is an error. If the Attribute is not String-typed, it will be converted to the appropriate type.
	Value types.String `tfsdk:"value"`
	// Array of child criteria. Required if the operation is AND or OR, otherwise it must be left null. A maximum of three levels of criteria are supported, including leaf nodes.
	Children []ProvisioningCriteriaLevel3 `tfsdk:"children"`
	// AdditionalProperties map[string]interface{}
}

type ProvisioningCriteriaLevel3 struct {
	Operation types.String `tfsdk:"operation"`
	// Name of the Account attribute to be tested. If **operation** is one of EQUALS, NOT_EQUALS, CONTAINS, or HAS, this field is required. Otherwise, specifying it is an error.
	Attribute types.String `tfsdk:"attribute"`
	// String value to test the Account attribute with regard to the specified operation. If the operation is one of EQUALS, NOT_EQUALS, or CONTAINS, this field is required. Otherwise, specifying it is an error. If the Attribute is not String-typed, it will be converted to the appropriate type.
	Value types.String `tfsdk:"value"`
	// AdditionalProperties map[string]interface{}
}

func (ap *AccessProfile) parseConfiguredAttributes(v3ap *v3.AccessProfile) {
	log.Print("parseConfiguredAttributes")

	ap.Id = types.StringPointerValue(v3ap.Id)
	ap.Name = types.StringValue(v3ap.Name)
	ap.Description = types.StringPointerValue(v3ap.Description.Get())
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

	if v3ap.AccessRequestConfig.Get() == nil {
		ap.AccessRequestConfig = nil
	} else {
		ap.AccessRequestConfig = &Requestability{}
		ap.AccessRequestConfig.CommentsRequired = types.BoolPointerValue(v3ap.AccessRequestConfig.Get().CommentsRequired.Get())
		ap.AccessRequestConfig.DenialCommentsRequired = types.BoolPointerValue(v3ap.AccessRequestConfig.Get().DenialCommentsRequired.Get())

		for _, a := range v3ap.AccessRequestConfig.Get().ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			ap.AccessRequestConfig.ApprovalSchemes = append(ap.AccessRequestConfig.ApprovalSchemes, approval)
		}
	}

	if v3ap.RevocationRequestConfig.Get() == nil || len(v3ap.RevocationRequestConfig.Get().ApprovalSchemes) == 0 {
		ap.RevocationRequestConfig = nil
	} else {
		ap.RevocationRequestConfig = &Revocability{}

		for _, a := range v3ap.RevocationRequestConfig.Get().ApprovalSchemes {
			approval := AccessProfileApprovalScheme{
				ApproverType: types.StringPointerValue(a.ApproverType),
				ApproverId:   types.StringPointerValue(a.ApproverId.Get()),
			}
			ap.RevocationRequestConfig.ApprovalSchemes = append(ap.RevocationRequestConfig.ApprovalSchemes, approval)
		}
	}

	if v3ap.ProvisioningCriteria.Get() == nil {
		ap.ProvisioningCriteria = nil
	} else {
		ap.ProvisioningCriteria = &ProvisioningCriteriaLevel1{}

		ap.ProvisioningCriteria.Operation = types.StringPointerValue((*string)(v3ap.ProvisioningCriteria.Get().Operation.Ptr()))
		ap.ProvisioningCriteria.Attribute = types.StringPointerValue(v3ap.ProvisioningCriteria.Get().Attribute.Get())
		ap.ProvisioningCriteria.Value = types.StringPointerValue(v3ap.ProvisioningCriteria.Get().Value.Get())
		ap.ProvisioningCriteria.Children = []ProvisioningCriteriaLevel2{}

		for _, c1 := range v3ap.ProvisioningCriteria.Get().Children {
			child1 := ProvisioningCriteriaLevel2{}
			child1.Operation = types.StringPointerValue((*string)(c1.Operation.Ptr()))
			child1.Attribute = types.StringPointerValue(c1.Attribute.Get())
			child1.Value = types.StringPointerValue(c1.Value.Get())

			for _, c2 := range c1.Children {
				child2 := ProvisioningCriteriaLevel3{}
				child2.Operation = types.StringPointerValue((*string)(c2.Operation.Ptr()))
				child2.Attribute = types.StringPointerValue(c2.Attribute.Get())
				child2.Value = types.StringPointerValue(c2.Value)

				child1.Children = append(child1.Children, child2)
			}

			ap.ProvisioningCriteria.Children = append(ap.ProvisioningCriteria.Children, child1)
		}
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

var RequestabilityType = map[string]attr.Type{
	"comments_required":        types.BoolType,
	"denial_comments_required": types.BoolType,
	"approval_schemes":         types.ListType{ElemType: AccessProfileApprovalSchemeObject},
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

var AccessProfileApprovalSchemeObject = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"approver_type": types.StringType,
		"approver_id":   types.StringType,
	},
}

var RevocabilityType = map[string]attr.Type{
	"approval_schemes": types.ListType{ElemType: AccessProfileApprovalSchemeObject},
}

// Revocability struct for Revocability
type Revocability struct {
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
		commentsRequired := v3.NullableBool{}
		commentsRequired.Set(ap.AccessRequestConfig.CommentsRequired.ValueBoolPointer())
		denialCommentsRequired := v3.NullableBool{}
		denialCommentsRequired.Set(ap.AccessRequestConfig.DenialCommentsRequired.ValueBoolPointer())
		accessRequest.CommentsRequired = commentsRequired
		accessRequest.DenialCommentsRequired = denialCommentsRequired

		accessRequestSchema := []v3.AccessProfileApprovalScheme{}
		for _, ar := range ap.AccessRequestConfig.ApprovalSchemes {
			as := v3.AccessProfileApprovalScheme{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if !ar.ApproverId.IsNull() {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRequestSchema = append(accessRequestSchema, as)
		}
		accessRequest.ApprovalSchemes = accessRequestSchema
		nullableRequestability := *v3.NewNullableRequestability(accessRequest)
		accessProfile.AccessRequestConfig = nullableRequestability
	}

	if ap.RevocationRequestConfig != nil {
		accessRevoke := v3.NewRevocability()

		accessRevokeSchema := []v3.AccessProfileApprovalScheme{}
		for _, ar := range ap.RevocationRequestConfig.ApprovalSchemes {
			as := v3.AccessProfileApprovalScheme{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if !ar.ApproverId.IsNull() {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRevokeSchema = append(accessRevokeSchema, as)
		}
		accessRevoke.ApprovalSchemes = accessRevokeSchema
		accessProfile.RevocationRequestConfig = *v3.NewNullableRevocability(accessRevoke)
	}

	if ap.ProvisioningCriteria != nil {
		provisioningCriteriaLevel11 := v3.NullableProvisioningCriteriaLevel1{}
		provisioningCriteriaLevel1 := v3.ProvisioningCriteriaLevel1{}

		provisioningCriteriaLevel1.Attribute = *v3.NewNullableString(ap.ProvisioningCriteria.Attribute.ValueStringPointer())
		provisioningCriteriaLevel1.Operation = (*v3.ProvisioningCriteriaOperation)(ap.ProvisioningCriteria.Operation.ValueStringPointer())
		provisioningCriteriaLevel1.Value = *v3.NewNullableString(ap.ProvisioningCriteria.Value.ValueStringPointer())

		for _, c1 := range ap.ProvisioningCriteria.Children {
			provisioningCriteriaLevel2 := *v3.NewProvisioningCriteriaLevel2()

			provisioningCriteriaLevel2.Attribute = *v3.NewNullableString(c1.Attribute.ValueStringPointer())
			provisioningCriteriaLevel2.Operation = (*v3.ProvisioningCriteriaOperation)(c1.Operation.ValueStringPointer())
			provisioningCriteriaLevel2.Value = *v3.NewNullableString(c1.Value.ValueStringPointer())

			for _, c2 := range c1.Children {
				provisioningCriteriaLevel3 := *v3.NewProvisioningCriteriaLevel3()

				provisioningCriteriaLevel3.Attribute = *v3.NewNullableString(c2.Attribute.ValueStringPointer())
				provisioningCriteriaLevel3.Operation = (*v3.ProvisioningCriteriaOperation)(c2.Operation.ValueStringPointer())
				provisioningCriteriaLevel3.Value = c2.Value.ValueStringPointer()

				provisioningCriteriaLevel2.Children = append(provisioningCriteriaLevel2.Children, provisioningCriteriaLevel3)
			}

			provisioningCriteriaLevel1.Children = append(provisioningCriteriaLevel1.Children, provisioningCriteriaLevel2)
		}

		provisioningCriteriaLevel11.Set(&provisioningCriteriaLevel1)

		accessProfile.ProvisioningCriteria = provisioningCriteriaLevel11
	}

	return &accessProfile
}
