package source

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Source -
type Source struct {
	// the id of the Source
	Id types.String `tfsdk:"id"`
	// Human-readable name of the source
	Name types.String `tfsdk:"name"`
	// Human-readable description of the source
	Description         types.String `tfsdk:"description"`
	ConnectorAttributes types.Object `tfsdk:"connector_attributes"`

	// Owner SourceOwner `json:"owner"`
	// Cluster *SourceCluster `json:"cluster"`
	// AccountCorrelationConfig *SourceAccountCorrelationConfig `json:"accountCorrelationConfig"`
	// AccountCorrelationRule *SourceAccountCorrelationRule `json:"accountCorrelationRule"`
	// ManagerCorrelationMapping *ManagerCorrelationMapping `json:"managerCorrelationMapping"`
	// ManagerCorrelationRule *SourceManagerCorrelationRule `json:"managerCorrelationRule"`
	// BeforeProvisioningRule *SourceBeforeProvisioningRule `json:"beforeProvisioningRule"`
	// // List of references to Schema objects
	// Schemas []SourceSchemasInner `json:"schemas"`
	// // List of references to the associated PasswordPolicy objects.
	// PasswordPolicies []SourcePasswordPoliciesInner `json:"passwordPolicies"`
	// // Optional features that can be supported by a source.
	// Features []SourceFeature `json:"features"`
	// // Specifies the type of system being managed e.g. Active Directory, Workday, etc.. If you are creating a Delimited File source, you must set the `provisionasCsv` query parameter to `true`.
	// Type *string `json:"type"`
	// // Connector script name.
	// Connector string `json:"connector"`
	// // The fully qualified name of the Java class that implements the connector interface.
	// ConnectorClass *string `json:"connectorClass"`
	// // Connector specific configuration; will differ from type to type.
	// // Number from 0 to 100 that specifies when to skip the delete phase.
	// DeleteThreshold *int32 `json:"deleteThreshold"`
	// // When true indicates the source is referenced by an IdentityProfile.
	// Authoritative *bool `json:"authoritative"`
	// ManagementWorkgroup *SourceManagementWorkgroup `json:"managementWorkgroup"`
	// // When true indicates a healthy source
	// Healthy *bool `json:"healthy"`
	// // A status identifier, giving specific information on why a source is healthy or not
	// Status *string `json:"status"`
	// // Timestamp showing when a source health check was last performed
	// Since *string `json:"since"`
	// // The id of connector
	// ConnectorId *string `json:"connectorId"`
	// // The name of the connector that was chosen on source creation
	// ConnectorName *string `json:"connectorName"`
	// // The type of connection (direct or file)
	// ConnectionType *string `json:"connectionType"`
	// // The connector implementation id
	// ConnectorImplementationId *string `json:"connectorImplementationId"`
	// AdditionalProperties map[string]interface{}
}

// ConnectorAttributes struct for ConnectorAttributes
type ConnectorAttributes struct {
	// The source ID
	Id types.String `tfsdk:"cloud_external_id"`
}

var connectorAttributesTypes = map[string]attr.Type{
	"cloud_external_id": types.StringType,
}

// Source -
type SourceLoadWait struct {
	SourceId types.String `tfsdk:"source_id"`
	Wait     types.Bool   `tfsdk:"wait_for_active_jobs"`
	Triggers types.Map    `tfsdk:"triggers"`
}
