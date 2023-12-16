package governancegroup

import (
	"context"

	"github.com/davidsonjon/golang-sdk/beta"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// WorkgroupDto struct for WorkgroupDto
type WorkgroupDto struct {
	// ID of the object to which this reference applies
	Id types.String `tfsdk:"id"`
	// Name of the Governance Group
	Name types.String `tfsdk:"name"`
	// Description of the Governance Group
	Description types.String `tfsdk:"description"`
	// Number of members in the Governance Group.
	MemberCount types.Int64 `tfsdk:"member_count"`
	// Number of connections in the Governance Group.
	ConnectionCount types.Int64 `tfsdk:"connection_count"`
	// AdditionalProperties map[string]interface{}
	Owner      types.Object `tfsdk:"owner"`
	Membership types.Set    `tfsdk:"membership"`
}

type BaseReferenceDto1 struct {
	Type types.String `tfsdk:"type"`
	// ID of the object to which this reference applies
	Id types.String `tfsdk:"id"`
	// Human-readable display name of the object to which this reference applies
	Name types.String `tfsdk:"name"`
	// AdditionalProperties map[string]interface{}
}

var baseReferenceDto1Types = map[string]attr.Type{
	"name": types.StringType,
	"id":   types.StringType,
	"type": types.StringType,
}

func convertWorkgroupBeta(ctx context.Context, wg *WorkgroupDto) *beta.WorkgroupDto {

	betaWg := beta.WorkgroupDto{}

	betaWg.Name = wg.Name.ValueStringPointer()
	betaWg.Description = wg.Description.ValueStringPointer()

	wgOwner := BaseReferenceDto1{}

	wg.Owner.As(ctx, &wgOwner, basetypes.ObjectAsOptions{})

	betaWgOwner := beta.OwnerDto{}
	betaWgOwner.Id = wgOwner.Id.ValueStringPointer()

	betaWg.Owner = &betaWgOwner

	return &betaWg
}
