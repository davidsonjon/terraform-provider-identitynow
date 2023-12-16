package application

// SailApplication
type SailApplication struct {
	AppId                          string `json:"appId,omitempty"`
	Id                             string `json:"id,omitempty"`
	ServiceId                      string `json:"serviceId,omitempty"`
	ServiceAppId                   string `json:"serviceAppId,omitempty"`
	Name                           string `json:"name,omitempty"`
	Description                    string `json:"description,omitempty"`
	AccountServiceMatchAllAccounts bool   `json:"accountServiceMatchAllAccounts,omitempty"`

	AccountServiceExternalId                string `json:"accountServiceExternalId,omitempty"`
	AccountServiceId                        int    `json:"accountServiceId,omitempty"`
	AccountServiceName                      string `json:"accountServiceName,omitempty"`
	AccountServicePolicyId                  string `json:"accountServicePolicyId,omitempty"`
	AccountServicePolicyName                string `json:"accountServicePolicyName,omitempty"`
	AccountServiceUseForPasswordManagement  bool   `json:"accountServiceUseForPasswordManagement,omitempty"`
	AppCenterEnabled                        bool   `json:"appCenterEnabled,omitempty"`
	ControlType                             string `json:"controlType,omitempty"`
	DateCreated                             int    `json:"dateCreated,omitempty"`
	EnableSso                               bool   `json:"enableSso,omitempty"`
	ExternalId                              string `json:"externalId,omitempty"`
	HasAutomations                          bool   `json:"hasAutomations,omitempty"`
	HasLinks                                bool   `json:"hasLinks,omitempty"`
	Icon                                    string `json:"icon,omitempty"`
	LastUpdated                             int    `json:"lastUpdated,omitempty"`
	LauncherCount                           int    `json:"launcherCount,omitempty"`
	LaunchpadEnabled                        bool   `json:"launchpadEnabled,omitempty"`
	Mobile                                  bool   `json:"mobile,omitempty"`
	PasswordManaged                         bool   `json:"passwordManaged,omitempty"`
	PasswordServiceId                       int    `json:"passwordServiceId,omitempty"`
	PasswordServiceName                     string `json:"passwordServiceName,omitempty"`
	PasswordServicePolicyId                 string `json:"passwordServicePolicyId,omitempty"`
	PasswordServicePolicyName               string `json:"passwordServicePolicyName,omitempty"`
	PasswordServiceUseForPasswordManagement bool   `json:"passwordServiceUseForPasswordManagement,omitempty"`
	PrivateApp                              bool   `json:"privateApp,omitempty"`
	ProvisionRequestEnabled                 bool   `json:"provisionRequestEnabled,omitempty"`
	RequireStrongAuthn                      bool   `json:"requireStrongAuthn,omitempty"`
	ScriptName                              string `json:"scriptName,omitempty"`
	SelectedSsoMethod                       string `json:"selectedSsoMethod,omitempty"`
	Service                                 string `json:"service,omitempty"`
	SsoMethod                               string `json:"ssoMethod,omitempty"`
	Status                                  string `json:"status,omitempty"`
	StepUpAuthType                          string `json:"stepUpAuthType,omitempty"`
	SupportedOffNetwork                     string `json:"supportedOffNetwork,omitempty"`
	SupportedSsoMethods                     int    `json:"supportedSsoMethods,omitempty"`
	UsageAnalytics                          bool   `json:"usageAnalytics,omitempty"`
	UsageCertRequired                       bool   `json:"usageCertRequired,omitempty"`
	XsdVersion                              string `json:"xsdVersion,omitempty"`

	Owner SailApplicationOwner `json:"owner,omitempty"`
	// AppProfiles []ApplicationAppProfiles `json:"appProfiles,omitempty"`
	// Health      ApplicationHealth        `json:"health,omitempty"`

	AccessProfileIds []string // garbage? `json:"accessProfileIds,omitempty"`
	// accountServicePolicies                  Object[] accountServicePolicies=System.Object[]

	// defaultAccessProfile                    object defaultAccessProfile=null

	// offNetworkBlockedRoles                  object offNetworkBlockedRoles=null
	// passwordServicePolicies                 Object[] passwordServicePolicies=System.Object[]
	// stepUpAuthData                          object stepUpAuthData=null
	// usageCertText                           object usageCertText=null
}

// SailApplicationOwner
type SailApplicationOwner struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// SailApplicationAppProfiles
type SailApplicationAppProfiles struct {
	Id          int    `json:"id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	DateCreated string `json:"dateCreated,omitempty"`
	XsdVersion  string `json:"xsdVersion,omitempty"`
}

// SailApplicationHealth
type SailApplicationHealth struct {
	Status      string `json:"status,omitempty"`
	LastChanged string `json:"lastChanged,omitempty"`
	Since       int    `json:"since,omitempty"`
	Healthy     bool   `json:"healthy,omitempty"`
}

// SailApplicationCreate
type SailApplicationCreate struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	AppType     string `json:"appType,omitempty"`
}

// SailApplicationUpdate
type SailApplicationUpdate struct {
	Name                    string   `json:"alias,omitempty"` // need to use alias as name is not overwritten
	Description             string   `json:"description,omitempty"`
	OwnerId                 string   `json:"ownerId,omitempty"`
	AccountServiceId        int      `json:"accountServiceId,omitempty"`
	ProvisionRequestEnabled bool     `json:"provisionRequestEnabled,omitempty"`
	LaunchpadEnabled        bool     `json:"launchpadEnabled,omitempty"`
	AccessProfileIds        []string `json:"accessProfileIds,omitempty"`
}

// SailApplicationAccessProfiles
type SailApplicationAccessProfiles struct {
	Count int                                  `json:"count,omitempty"`
	Items []SailApplicationAccessProfilesItems `json:"items,omitempty"`
}

// SailApplicationAccessProfilesItems
type SailApplicationAccessProfilesItems struct {
	Id          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// SailApplicationAccessAssociation
type SailApplicationAccessAssociation struct {
	AccessProfileIds []string `json:"accessProfileIds"`
}
