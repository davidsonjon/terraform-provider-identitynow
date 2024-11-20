package application

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SourceApp struct for SourceApp
type SourceApp struct {
	// The source app id
	Id types.String `tfsdk:"id"`
	// The deprecated source app id
	CloudAppId types.String `tfsdk:"cloud_app_id"`
	// The source app name
	Name types.String `tfsdk:"name"`
	// Time when the source app was created
	Created types.String `tfsdk:"created"`
	// Time when the source app was last modified
	Modified types.String `tfsdk:"modified"`
	// True if the source app is enabled
	Enabled types.Bool `tfsdk:"enabled"`
	// True if the source app is provision request enabled
	ProvisionRequestEnabled types.Bool `tfsdk:"provision_request_enabled"`
	// The description of the source app
	Description types.String `tfsdk:"description"`
	// True if the source app match all accounts
	MatchAllAccounts types.Bool `tfsdk:"match_all_accounts"`
	// True if the source app is shown in the app center
	AppCenterEnabled types.Bool   `tfsdk:"appcenter_enabled"`
	AccountSourceId  types.String `tfsdk:"account_source_id"`
	// // The owner of source app
	Owner            types.Object `tfsdk:"owner"`
	AccessProfileIds types.Set    `tfsdk:"access_profile_ids"`
}

// type BaseReferenceDto1 struct {
// 	Type types.String `tfsdk:"type"`
// 	// ID of the object to which this reference applies
// 	Id types.String `tfsdk:"id"`
// 	// Human-readable display name of the object to which this reference applies
// 	Name types.String `tfsdk:"name"`
// 	// AdditionalProperties map[types.String]interface{}
// }

// ListApplications200ResponseInnerOwner struct for ListApplications200ResponseInnerOwner
type ListApplications200ResponseInnerOwner struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`
	// AdditionalProperties map[types.String]interface{}
}

var listApplications200ResponseInnerOwnerTypes = map[string]attr.Type{
	"name": types.StringType,
	"id":   types.StringType,
	"type": types.StringType,
}

func (m ListApplications200ResponseInnerOwner) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType,
		"id":   types.StringType,
		"type": types.StringType,
	}
}
