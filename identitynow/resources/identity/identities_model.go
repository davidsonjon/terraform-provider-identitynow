package identity

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// IdentitiesDataSourceModel represents the data source configuration model for the plural identities data source
type IdentitiesDataSourceModel struct {
	Filters    types.String `tfsdk:"filters"`
	Limit      types.Int64  `tfsdk:"limit"`
	Identities types.List   `tfsdk:"identities"`
}

// IdentityListItem represents a single identity item in the plural identities data source
// This is separate from the singular Identity struct to avoid field mismatches
type IdentityListItem struct {
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
	IdentityStatus types.String `tfsdk:"identity_status"`
}
