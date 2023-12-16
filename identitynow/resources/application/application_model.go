package application

import (
	"context"
	"strconv"

	cc "github.com/davidsonjon/golang-sdk/cc"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// ListApplications200ResponseInner struct for ListApplications200ResponseInner
type ListApplications200ResponseInner struct {
	Id                      types.String `tfsdk:"id"`
	AppId                   types.String `tfsdk:"app_id"`
	ServiceId               types.String `tfsdk:"service_id"`
	ServiceAppId            types.String `tfsdk:"service_app_id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	AppCenterEnabled        types.Bool   `tfsdk:"app_center_enabled"`
	ProvisionRequestEnabled types.Bool   `tfsdk:"provision_request_enabled"`
	LaunchpadEnabled        types.Bool   `tfsdk:"launchpad_enabled"`
	Owner                   types.Object `tfsdk:"owner"`
	DateCreated             types.String `tfsdk:"date_created"`
	// LastUpdated             types.String `tfsdk:"last_updated"`
	AccessProfileIds types.List   `tfsdk:"access_profile_ids"`
	AccountServiceId types.String `tfsdk:"account_service_id"`

	// Icon *string `json:"icon"`

	// ControlType *string `json:"controlType"`
	// Mobile *bool `json:"mobile"`
	// PrivateApp *bool `json:"privateApp"`
	// ScriptName *string `json:"scriptName"`
	// Status *string `json:"status"`

	// Health *ListApplications200ResponseInnerHealth `json:"health"`
	// EnableSso *bool `json:"enableSso"`
	// SsoMethod *string `json:"ssoMethod"`
	// HasLinks *bool `json:"hasLinks"`
	// HasAutomations *bool `json:"hasAutomations"`
	// StepUpAuthData map[string]interface{} `json:"stepUpAuthData"`
	// StepUpAuthType *string `json:"stepUpAuthType"`
	// UsageAnalytics *bool `json:"usageAnalytics"`
	// UsageCertRequired *bool `json:"usageCertRequired"`
	// UsageCertText map[string]interface{} `json:"usageCertText"`

	// PasswordManaged *bool `json:"passwordManaged"`

	// DefaultAccessProfile map[string]interface{} `json:"defaultAccessProfile"`
	// Service *string `json:"service"`
	// SelectedSsoMethod *string `json:"selectedSsoMethod"`
	// SupportedSsoMethods *float32 `json:"supportedSsoMethods"`
	// OffNetworkBlockedRoles map[string]interface{} `json:"offNetworkBlockedRoles"`
	// SupportedOffNetwork *string `json:"supportedOffNetwork"`

	// LauncherCount *float32 `json:"launcherCount"`
	// AccountServiceName *string `json:"accountServiceName"`
	// AccountServiceExternalId *string `json:"accountServiceExternalId"`
	// AccountServiceMatchAllAccounts *bool `json:"accountServiceMatchAllAccounts"`
	// ExternalId *string `json:"externalId"`
	// AccountServiceUseForPasswordManagement *bool `json:"accountServiceUseForPasswordManagement"`
	// AccountServicePolicyId *string `json:"accountServicePolicyId"`
	// AccountServicePolicyName *string `json:"accountServicePolicyName"`
	// RequireStrongAuthn *bool `json:"requireStrongAuthn"`
	// AccountServicePolicies []ListApplications200ResponseInnerAccountServicePoliciesInner `json:"accountServicePolicies"`
	// XsdVersion *string `json:"xsdVersion"`
	// AppProfiles []ListApplications200ResponseInnerAppProfilesInner `json:"appProfiles"`
	// PasswordServiceId *float32 `json:"passwordServiceId"`

	// AdditionalProperties map[string]interface{}
}

// ListApplications200ResponseInnerOwner struct for ListApplications200ResponseInnerOwner
type ListApplications200ResponseInnerOwner struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

var listApplications200ResponseInnerOwnerTypes = map[string]attr.Type{
	"name": types.StringType,
	"id":   types.StringType,
}

// ApplicationAccessAssociation
type ApplicationAccessAssociation struct {
	Id               types.String `tfsdk:"id"`
	ApplicationId    types.String `tfsdk:"application_id"`
	AccessProfileIds types.List   `tfsdk:"access_profile_ids"`
}

// type ApplicationAccessProfilesItems struct {
// 	Id types.String `tfsdk:"id"`
// }

func parseAttributesApplication(app *ListApplications200ResponseInner, ccApp *cc.GetApplication200Response, diags *diag.Diagnostics) {
	tflog.Info(context.Background(), "parseAttributesApplication")

	app.Id = types.StringPointerValue(ccApp.Id)
	app.AppId = types.StringPointerValue(ccApp.AppId)
	app.ServiceId = types.StringPointerValue(ccApp.ServiceId)
	app.ServiceAppId = types.StringPointerValue(ccApp.ServiceAppId)
	app.Name = types.StringPointerValue(ccApp.Name)
	app.Description = types.StringPointerValue(ccApp.Description)
	app.AppCenterEnabled = types.BoolPointerValue(ccApp.AppCenterEnabled)
	app.ProvisionRequestEnabled = types.BoolPointerValue(ccApp.ProvisionRequestEnabled)
	app.LaunchpadEnabled = types.BoolPointerValue(ccApp.LaunchpadEnabled)
	app.DateCreated = types.StringValue(strconv.FormatFloat(float64(*ccApp.DateCreated), 'f', -1, 32))
	// app.LastUpdated = types.StringValue(strconv.FormatFloat(float64(*ccApp.LastUpdated), 'f', -1, 32))
	app.AccountServiceId = types.StringValue(strconv.FormatFloat(float64(*ccApp.AccountServiceId), 'f', -1, 32))

	owner, ok := types.ObjectValue(listApplications200ResponseInnerOwnerTypes, map[string]attr.Value{
		"name": types.StringPointerValue(ccApp.Owner.Name),
		"id":   types.StringPointerValue(ccApp.Owner.Id),
	})
	if ok.HasError() {
		diags.Append(ok...)
	}
	app.Owner = owner

}

func parseAccessProfiles(app *ListApplications200ResponseInner, appProfiles SailApplicationAccessProfiles, diags *diag.Diagnostics) {
	tflog.Info(context.Background(), "parseAccessProfiles")
	elements := []attr.Value{}
	for _, v := range appProfiles.Items {
		elements = append(elements, types.StringValue(v.Id))
	}
	listValue, ok := types.ListValue(types.StringType, elements)
	if ok.HasError() {
		diags.Append(ok...)
	}
	app.AccessProfileIds = listValue
}
