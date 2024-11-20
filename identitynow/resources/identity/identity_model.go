package identity

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Identity
type Identity struct {
	// System-generated unique ID of the Object
	Id types.String `tfsdk:"id"`
	// Name of the Object
	Name types.String `tfsdk:"name"`
	// Creation date of the Object
	Created types.String `tfsdk:"created"`
	// Last modification date of the Object
	Modified types.String `tfsdk:"modified"`
	// Alternate unique identifier for the identity
	Alias types.String `tfsdk:"alias"`
	// The email address of the identity
	EmailAddress types.String `tfsdk:"email_address"`
	// The processing state of the identity
	ProcessingState types.String `tfsdk:"processing_state"`
	// The identity's status in the system
	IdentityStatus     types.String `tfsdk:"identity_status"`
	UseCallerIdentity  types.Bool   `tfsdk:"use_caller_identity"`
	CallerIdentityUsed types.Bool   `tfsdk:"caller_identity_used"`

	// ManagerRef *BaseReferenceDto1 `json:"managerRef"`
	// // Whether this identity is a manager of another identity
	// IsManager *bool `json:"isManager"`
	// // The last time the identity was refreshed by the system
	// LastRefresh *time.Time `json:"lastRefresh"`
	// // A map with the identity attributes for the identity
	// Attributes map[string]interface{} `json:"attributes"`
	// LifecycleState *LifecycleStateDto `json:"lifecycleState"`
	// AdditionalProperties map[string]interface{}
}
