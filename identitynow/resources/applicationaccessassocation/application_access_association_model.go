package applicationaccessassocation

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ApplicationAccessAssociation
type ApplicationAccessAssociation struct {
	Id               types.String `tfsdk:"id"`
	ApplicationId    types.String `tfsdk:"application_id"`
	AccessProfileIds types.Set    `tfsdk:"access_profile_ids"`
}
