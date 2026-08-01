// This file implements identitynow_governance_group_connections_v1, the
// second follow-up item noted in resource_governance_group.go's package doc
// ("Deferred" members/connections sub-resources).
//
// GET /workgroups/v1/{workgroupId}/connections has no corresponding write
// endpoint at all - connections are established from the *referencing*
// object's side (e.g. a role/access profile/SOD policy/source that names
// this governance group as an owner/reviewer), not from the governance
// group's own API. A read-only data source is therefore the only sensible
// Terraform shape for this endpoint; there is no equivalent
// identitynow_governance_group_connections_v1 *resource*.
//
// Like the members resource, this is hand-written (not codegen'd) since it
// has no single-item CRUD shape to generate from - it's a plain list query.
package governance_group_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/governance_groups"
)

// governanceGroupConnectionsListPageLimit matches GET .../connections'
// documented maximum "limit" value (50, same as GET .../members - unlike
// most other v1 list endpoints' 250 - per the shared limit50.yaml parameter
// definition both endpoints use).
const governanceGroupConnectionsListPageLimit = 50

var (
	_ datasource.DataSource              = (*governanceGroupConnectionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*governanceGroupConnectionsDataSource)(nil)
)

func NewGovernanceGroupConnectionsDataSource() datasource.DataSource {
	return &governanceGroupConnectionsDataSource{}
}

type governanceGroupConnectionsDataSource struct {
	client *sailpoint.APIClient
}

// GovernanceGroupConnectionModel mirrors one entry of WorkgroupConnectionDto
// (a flattened ConnectedObject plus connectionType) - hand-written since this
// data source has no generated schema/model.
type GovernanceGroupConnectionModel struct {
	ObjectId          types.String `tfsdk:"object_id"`
	ObjectType        types.String `tfsdk:"object_type"`
	ObjectName        types.String `tfsdk:"object_name"`
	ObjectDescription types.String `tfsdk:"object_description"`
	ConnectionType    types.String `tfsdk:"connection_type"`
}

// governanceGroupConnectionAttrTypes is the attr.Type map shared by the
// schema's NestedAttributeObject and the Read method's types.ListValueFrom
// call, kept as a single function so the two can't drift out of sync.
func governanceGroupConnectionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"object_id":          types.StringType,
		"object_type":        types.StringType,
		"object_name":        types.StringType,
		"object_description": types.StringType,
		"connection_type":    types.StringType,
	}
}

type GovernanceGroupConnectionsDataSourceModel struct {
	GovernanceGroupId types.String `tfsdk:"governance_group_id"`
	Connections       types.List   `tfsdk:"connections"`
}

func (d *governanceGroupConnectionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group_connections_v1"
}

func (d *governanceGroupConnectionsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the objects (roles, access profiles, SOD policies, sources) connected to a Governance Group in IdentityNow/ISC.",
		MarkdownDescription: "Lists the objects (roles, access profiles, SOD policies, sources) connected to a " +
			"[Governance Group](https://documentation.sailpoint.com/saas/help/common/governance_groups.html) via " +
			"`GET /workgroups/v1/{workgroupId}/connections`. Connections are read-only here - they are established " +
			"from the referencing object's own configuration (e.g. a role naming this governance group as an owner), " +
			"not from the governance group side.\n\n" +
			"~> This is a `_v1` pilot data source.",
		Attributes: map[string]schema.Attribute{
			"governance_group_id": schema.StringAttribute{
				Required:            true,
				Description:         "ID of the Governance Group to list connections for.",
				MarkdownDescription: "ID of the Governance Group to list connections for.",
			},
			"connections": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Objects connected to the Governance Group.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"object_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "ID of the connected object.",
						},
						"object_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Type of the connected object (`ACCESS_PROFILE`, `ROLE`, `SOD_POLICY`, or `SOURCE`).",
						},
						"object_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable name of the connected object.",
						},
						"object_description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Description of the connected object.",
						},
						"connection_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "How the object is connected (`AccessRequestReviewer`, `Owner`, or `ManagementWorkgroup`).",
						},
					},
				},
			},
		},
	}
}

func (d *governanceGroupConnectionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	cp, ok := req.ProviderData.(clientProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected a provider client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = cp.GetClient()
}

func (d *governanceGroupConnectionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config GovernanceGroupConnectionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workgroupID := config.GovernanceGroupId.ValueString()

	tflog.Debug(ctx, "Reading Governance Group connections", map[string]interface{}{"governance_group_id": workgroupID})

	var dtos []governance_groups.WorkgroupConnectionDto
	var offset int32
	for {
		page, httpResp, err := d.client.GovernanceGroupsAPI.
			ListConnectionsV1(ctx, workgroupID).
			Offset(offset).
			Limit(governanceGroupConnectionsListPageLimit).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error listing Governance Group connections", map[string]interface{}{"governance_group_id": workgroupID, "error": err.Error()})
			resp.Diagnostics.AddError("Error listing Governance Group connections", errDetail(err, httpResp))
			return
		}
		dtos = append(dtos, page...)
		if len(page) < governanceGroupConnectionsListPageLimit {
			break
		}
		offset += governanceGroupConnectionsListPageLimit
	}

	models := make([]GovernanceGroupConnectionModel, 0, len(dtos))
	for i := range dtos {
		dto := dtos[i]
		model := GovernanceGroupConnectionModel{}
		if dto.ConnectionType != nil {
			model.ConnectionType = types.StringValue(*dto.ConnectionType)
		}
		if dto.Object != nil {
			if dto.Object.Id != nil {
				model.ObjectId = types.StringValue(*dto.Object.Id)
			}
			if dto.Object.Type != nil {
				model.ObjectType = types.StringValue(string(*dto.Object.Type))
			}
			if dto.Object.Name != nil {
				model.ObjectName = types.StringValue(*dto.Object.Name)
			}
			if dto.Object.Description.IsSet() && dto.Object.Description.Get() != nil {
				model.ObjectDescription = types.StringValue(*dto.Object.Description.Get())
			}
		}
		models = append(models, model)
	}

	elemType := types.ObjectType{AttrTypes: governanceGroupConnectionAttrTypes()}

	connectionsList, diags := types.ListValueFrom(ctx, elemType, models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Connections = connectionsList

	tflog.Debug(ctx, "Read Governance Group connections", map[string]interface{}{"governance_group_id": workgroupID, "count": len(dtos)})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
