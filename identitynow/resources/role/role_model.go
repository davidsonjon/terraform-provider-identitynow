package role

import (
	"context"
	"log"

	"github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/metadataattribute"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Role A Role
type Role struct {
	// The id of the Role. This field must be left null when creating a Role, otherwise a 400 Bad Request error will result.
	Id types.String `tfsdk:"id"`
	// The human-readable display name of the Role
	Name types.String `tfsdk:"name"`
	// Date the Role was created
	Created types.String `tfsdk:"created"`
	// A human-readable description of the Role
	Description    types.String            `tfsdk:"description"`
	Owner          *OwnerReference         `tfsdk:"owner"`
	AccessProfiles []AccessProfileRef      `tfsdk:"access_profiles"`
	Entitlements   []EntitlementRef        `tfsdk:"entitlements"`
	Membership     *RoleMembershipSelector `tfsdk:"membership"`
	// Whether the Role is enabled or not.
	Enabled types.Bool `tfsdk:"enabled"`
	// Whether the Role can be the target of access requests.
	Requestable             types.Bool      `tfsdk:"requestable"`
	AccessRequestConfig     *Requestability `tfsdk:"access_request_config"`
	RevocationRequestConfig *Revocability   `tfsdk:"revocation_request_config"`
	// List of IDs of segments, if any, to which this Role is assigned.
	Segments types.List `tfsdk:"segments"`
	// Whether the Role is dimensional.
	Dimensional types.Bool `tfsdk:"dimensional"`
	// List of references to dimensions to which this Role is assigned. This field is only relevant if the Role is dimensional.
	// DimensionRefs       []DimensionRef                     `tfsdk:"dimensionRefs"`
	AccessModelMetadata []metadataattribute.AttributeDTO `tfsdk:"access_model_metadata"`
}

type AccessProfileRef struct {
	Type types.String `tfsdk:"type"`
	// Identity id
	Id types.String `tfsdk:"id"`
	// Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

var AccessProfileSchemeObject = map[string]attr.Type{
	"type": types.StringType,
	"id":   types.StringType,
	"name": types.StringType,
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

var RequestabilityType = map[string]attr.Type{
	"approval_schemes":         types.ListType{ElemType: ApprovalSchemeObject},
	"denial_comments_required": types.BoolType,
	"comments_required":        types.BoolType,
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

var ApprovalSchemeObject = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"approver_type": types.StringType,
		"approver_id":   types.StringType,
	},
}

var RevocabilityType = map[string]attr.Type{
	"approval_schemes": types.ListType{ElemType: ApprovalSchemeObject},
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

var MembershipKey = map[string]attr.Type{
	"type":      types.StringType,
	"property":  types.StringType,
	"source_id": types.StringType,
}

var IdentityObject = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"type":       types.StringType,
		"id":         types.StringType,
		"name":       types.StringType,
		"alias_name": types.StringType,
	},
}

var MembershipType = map[string]attr.Type{
	"type":       types.StringType,
	"identities": types.ListType{ElemType: IdentityObject},
	"criteria": types.ObjectType{
		AttrTypes: MembershipLevel1Object},
}

var MembershipLevel1Object = map[string]attr.Type{
	"operation":    types.StringType,
	"key":          types.ObjectType{AttrTypes: MembershipKey},
	"string_value": types.StringType,
	"children":     types.ListType{ElemType: MembershipLevel2Object},
}

var MembershipLevel2Object = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"operation":    types.StringType,
		"key":          types.ObjectType{AttrTypes: MembershipKey},
		"string_value": types.StringType,
		"children":     types.ListType{ElemType: MembershipLevel3Object},
	},
}

var MembershipLevel3Object = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"operation":    types.StringType,
		"key":          types.ObjectType{AttrTypes: MembershipKey},
		"string_value": types.StringType,
	},
}

type RoleMembershipSelector struct {
	Type     types.String        `tfsdk:"type"`
	Criteria *RoleCriteriaLevel1 `tfsdk:"criteria"`
	// Defines role membership as being exclusive to the specified Identities, when type is IDENTITY_LIST.
	Identities []RoleMembershipIdentity `tfsdk:"identities"`
	// AdditionalProperties map[string]interface{}
}

// RoleCriteriaLevel1 Defines STANDARD type Role membership
type RoleCriteriaLevel1 struct {
	Operation types.String     `tfsdk:"operation"`
	Key       *RoleCriteriaKey `tfsdk:"key"`
	// String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.
	StringValue types.String `tfsdk:"string_value"`
	// Array of child criteria. Required if the operation is AND or OR, otherwise it must be left null. A maximum of three levels of criteria are supported, including leaf nodes. Additionally, AND nodes can only be children or OR nodes and vice-versa.
	Children []RoleCriteriaLevel2 `tfsdk:"children"`
	// AdditionalProperties map[string]interface{}
}

// RoleCriteriaLevel2 Defines STANDARD type Role membership
type RoleCriteriaLevel2 struct {
	Operation types.String     `tfsdk:"operation"`
	Key       *RoleCriteriaKey `tfsdk:"key"`
	// String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.
	StringValue types.String `tfsdk:"string_value"`
	// Array of child criteria. Required if the operation is AND or OR, otherwise it must be left null. A maximum of three levels of criteria are supported, including leaf nodes. Additionally, AND nodes can only be children or OR nodes and vice-versa.
	Children []RoleCriteriaLevel3 `tfsdk:"children"`
	// AdditionalProperties map[string]interface{}
}

// RoleCriteriaLevel3 Defines STANDARD type Role membership
type RoleCriteriaLevel3 struct {
	Operation types.String     `tfsdk:"operation"`
	Key       *RoleCriteriaKey `tfsdk:"key"`
	// String value to test the Identity attribute, Account attribute, or Entitlement specified in the key with regard to the specified operation. If this criteria is a leaf node, that is, if the operation is one of EQUALS, NOT_EQUALS, CONTAINS, STARTS_WITH, or ENDS_WITH, this field is required. Otherwise, specifying it is an error.
	StringValue types.String `tfsdk:"string_value"`
	// AdditionalProperties map[string]interface{}
}

type RoleCriteriaKey struct {
	Type types.String `tfsdk:"type"`
	// The name of the attribute or entitlement to which the associated criteria applies.
	Property types.String `tfsdk:"property"`
	// ID of the Source from which an account attribute or entitlement is drawn. Required if type is ACCOUNT or ENTITLEMENT
	SourceId types.String `tfsdk:"source_id"`
	// AdditionalProperties map[string]interface{}
}

// RoleMembershipIdentity A reference to an Identity in an IDENTITY_LIST role membership criteria.
type RoleMembershipIdentity struct {
	Type types.String `tfsdk:"type"`
	// Identity id
	Id types.String `tfsdk:"id"`
	// Human-readable display name of the Identity.
	Name types.String `tfsdk:"name"`
	// User name of the Identity
	AliasName types.String `tfsdk:"alias_name"`
	// AdditionalProperties map[string]interface{}
}

func convertRoleV3(role *Role) api_v3.Role {

	newRole := api_v3.Role{}
	newRole.Name = role.Name.ValueString()
	description := api_v3.NullableString{}
	description.Set(role.Description.ValueStringPointer())
	newRole.Description = description
	newRole.Enabled = role.Enabled.ValueBoolPointer()
	newRole.Requestable = role.Requestable.ValueBoolPointer()

	newRole.Owner.SetId(role.Owner.Id.ValueString())
	newRole.Owner.SetType(role.Owner.Type.ValueString())

	newRole.Entitlements = []api_v3.EntitlementRef{}

	for _, entitlement := range role.Entitlements {
		entitlement := api_v3.EntitlementRef{
			Id:   entitlement.Id.ValueStringPointer(),
			Type: entitlement.Type.ValueStringPointer(),
		}
		newRole.Entitlements = append(newRole.Entitlements, entitlement)
	}

	newRole.AccessProfiles = []api_v3.AccessProfileRef{}

	for _, accessProfile := range role.AccessProfiles {
		entitlement := api_v3.AccessProfileRef{
			Id:   accessProfile.Id.ValueStringPointer(),
			Type: accessProfile.Type.ValueStringPointer(),
		}
		newRole.AccessProfiles = append(newRole.AccessProfiles, entitlement)
	}

	if role.AccessRequestConfig != nil {
		accessRequest := api_v3.NewRequestabilityForRole()
		commentsRequired := api_v3.NullableBool{}
		commentsRequired.Set(role.AccessRequestConfig.CommentsRequired.ValueBoolPointer())
		denialCommentsRequired := api_v3.NullableBool{}
		denialCommentsRequired.Set(role.AccessRequestConfig.DenialCommentsRequired.ValueBoolPointer())
		accessRequest.CommentsRequired = commentsRequired
		accessRequest.DenialCommentsRequired = denialCommentsRequired

		accessRequestSchema := []api_v3.ApprovalSchemeForRole{}
		for _, ar := range role.AccessRequestConfig.ApprovalSchemes {
			as := api_v3.ApprovalSchemeForRole{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if !ar.ApproverId.IsNull() {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRequestSchema = append(accessRequestSchema, as)
		}
		accessRequest.ApprovalSchemes = accessRequestSchema
		newRole.AccessRequestConfig = accessRequest
	}

	if role.RevocationRequestConfig != nil {
		accessRevoke := api_v3.NewRevocabilityForRole()

		accessRevokeSchema := []api_v3.ApprovalSchemeForRole{}
		for _, ar := range role.RevocationRequestConfig.ApprovalSchemes {
			as := api_v3.ApprovalSchemeForRole{}
			as.SetApproverType(ar.ApproverType.ValueString())
			if !ar.ApproverId.IsNull() {
				as.SetApproverId(ar.ApproverId.ValueString())
			}

			accessRevokeSchema = append(accessRevokeSchema, as)
		}
		accessRevoke.ApprovalSchemes = accessRevokeSchema
		newRole.RevocationRequestConfig = accessRevoke
	}

	if role.AccessModelMetadata != nil {
		accessmodelmetadata := &api_v3.AttributeDTOList{}

		for _, att := range role.AccessModelMetadata {
			metatdataAtts := api_v3.AttributeDTO{}
			metatdataAtts.Key = att.Key.ValueStringPointer()
			metatdataAtts.Name = att.Name.ValueStringPointer()
			metatdataAtts.Multiselect = att.Multiselect.ValueBoolPointer()
			metatdataAtts.Status = att.Status.ValueStringPointer()
			metatdataAtts.Type = att.Type.ValueStringPointer()
			metatdataAtts.Description = att.Description.ValueStringPointer()

			elements := make([]types.String, 0, len(att.ObjectTypes.Elements()))
			_ = att.ObjectTypes.ElementsAs(context.Background(), &elements, false)

			for _, v := range elements {
				metatdataAtts.ObjectTypes = append(metatdataAtts.ObjectTypes, v.ValueString())
			}

			for _, v := range att.Values {
				value := &api_v3.AttributeValueDTO{
					Value:  v.Value.ValueStringPointer(),
					Name:   v.Name.ValueStringPointer(),
					Status: v.Status.ValueStringPointer(),
				}
				metatdataAtts.Values = append(metatdataAtts.Values, *value)

			}

			accessmodelmetadata.Attributes = append(accessmodelmetadata.Attributes, metatdataAtts)
		}

		newRole.AccessModelMetadata = accessmodelmetadata
	}

	criteriaLevel1 := &api_v3.RoleCriteriaLevel1{}
	if role.Membership != nil {
		membership := &api_v3.RoleMembershipSelector{}
		membership.Type = (*api_v3.RoleMembershipSelectorType)(role.Membership.Type.ValueStringPointer())

		for _, i := range role.Membership.Identities {
			identity := api_v3.RoleMembershipIdentity{
				Id:        i.Id.ValueStringPointer(),
				Name:      *api_v3.NewNullableString(i.Name.ValueStringPointer()),
				AliasName: *api_v3.NewNullableString(i.AliasName.ValueStringPointer()),
				Type:      (*api_v3.DtoType)(i.Type.ValueStringPointer()),
			}

			membership.Identities = append(membership.Identities, identity)
		}

		if role.Membership.Criteria != nil {
			criteriaLevel2 := []api_v3.RoleCriteriaLevel2{}
			for _, c2 := range role.Membership.Criteria.Children {
				criteriaLevel2Key := api_v3.NullableRoleCriteriaKey{}
				if c2.Key != nil {
					test := &api_v3.RoleCriteriaKey{}
					test.Type = api_v3.RoleCriteriaKeyType(c2.Key.Type.ValueString())
					test.Property = c2.Key.Property.ValueString()
					test.SourceId = *api_v3.NewNullableString(c2.Key.SourceId.ValueStringPointer())
					criteriaLevel2Key.Set(test)
				}
				level2 := api_v3.RoleCriteriaLevel2{
					Operation:   (*api_v3.RoleCriteriaOperation)(c2.Operation.ValueStringPointer()),
					Key:         criteriaLevel2Key,
					StringValue: *api_v3.NewNullableString(c2.StringValue.ValueStringPointer()),
				}

				criteriaLevel3 := []api_v3.RoleCriteriaLevel3{}
				for _, c3 := range c2.Children {
					criteriaLevel3Key := api_v3.NullableRoleCriteriaKey{}

					if c3.Key != nil {
						test := &api_v3.RoleCriteriaKey{}
						test.Type = api_v3.RoleCriteriaKeyType(c3.Key.Type.ValueString())
						test.Property = c3.Key.Property.ValueString()
						if !c3.Key.SourceId.IsUnknown() {
							log.Printf("!c3.Key.SourceId.IsUnknown()!!")
							test.SourceId = *api_v3.NewNullableString(c3.Key.SourceId.ValueStringPointer())
						}
						if test.SourceId.Get() == nil {
							log.Printf("test.SourceId.Get() nil!!")
						} else {
							log.Printf("test.SourceId.Get() NOT nil!!")
						}
						criteriaLevel3Key.Set(test)
					}
					level3 := api_v3.RoleCriteriaLevel3{
						Operation:   (*api_v3.RoleCriteriaOperation)(c3.Operation.ValueStringPointer()),
						Key:         criteriaLevel3Key,
						StringValue: c3.StringValue.ValueStringPointer(),
					}

					criteriaLevel3 = append(criteriaLevel3, level3)
				}
				level2.Children = criteriaLevel3
				criteriaLevel2 = append(criteriaLevel2, level2)
			}

			criteriaLevel1.Operation = (*api_v3.RoleCriteriaOperation)(role.Membership.Criteria.Operation.ValueStringPointer())
			if role.Membership.Criteria.Key != nil {
				criteriaLevel1Key := &api_v3.RoleCriteriaKey{}
				criteriaLevel1Key.Type = api_v3.RoleCriteriaKeyType(role.Membership.Criteria.Key.Type.ValueString())
				criteriaLevel1Key.Property = role.Membership.Criteria.Key.Property.ValueString()
				criteriaLevel1Key.SourceId = *api_v3.NewNullableString(role.Membership.Criteria.Key.SourceId.ValueStringPointer())

				criteriaLevel1.Key = *api_v3.NewNullableRoleCriteriaKey(criteriaLevel1Key)
			}
			criteriaLevel1.StringValue = *api_v3.NewNullableString(role.Membership.Criteria.StringValue.ValueStringPointer())
			criteriaLevel1.Children = criteriaLevel2

			nullableCriteria := api_v3.NewNullableRoleCriteriaLevel1(criteriaLevel1)
			membership.Criteria = *nullableCriteria
		}

		nullableMembership := api_v3.NewNullableRoleMembershipSelector(membership)

		newRole.Membership = *nullableMembership
	}

	return newRole
}
