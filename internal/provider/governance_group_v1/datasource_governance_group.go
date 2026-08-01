// See resource_governance_group.go in this package for design notes and
// known limitations shared by both the resource and this data source.
package governance_group_v1

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/governance_groups"

	"terraform-provider-identitynow/internal/provider/governance_group_v1/datasource_governance_group"
)

var (
	_ datasource.DataSource              = (*governanceGroupDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*governanceGroupDataSource)(nil)
)

func NewGovernanceGroupDataSource() datasource.DataSource {
	return &governanceGroupDataSource{}
}

type governanceGroupDataSource struct {
	client *sailpoint.APIClient
}

func (d *governanceGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group_v1"
}

func (d *governanceGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_governance_group.GovernanceGroupDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Governance Group from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Governance Group](https://documentation.sailpoint.com/saas/help/common/governance_groups.html) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see the \"Known Limitations & Live Testing Notes\" section below before relying " +
		"on it in production configurations."
}

func (d *governanceGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *governanceGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_governance_group.GovernanceGroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Governance Group data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.GovernanceGroupsAPI.
		GetWorkgroupV1(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Governance Group data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Governance Group", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Governance Group data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors dtoToModel in resource_governance_group.go but
// against the data source's generated model/value types (a separate Go
// package emitted by tfplugingen-framework, so the types are not identical
// even though they're structurally the same).
func datasourceDtoToModel(ctx context.Context, dto *governance_groups.WorkgroupDto, fallback datasource_governance_group.GovernanceGroupModel) (datasource_governance_group.GovernanceGroupModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		model.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	}
	if dto.MemberCount != nil {
		model.MemberCount = types.Int64Value(*dto.MemberCount)
	}
	if dto.ConnectionCount != nil {
		model.ConnectionCount = types.Int64Value(*dto.ConnectionCount)
	}
	if dto.Created != nil {
		model.Created = types.StringValue(dto.Created.Format(time.RFC3339))
	}
	if dto.Modified != nil {
		model.Modified = types.StringValue(dto.Modified.Format(time.RFC3339))
	}

	owner, d := datasource_governance_group.OwnerValue{}.FromApi_betaWorkgroupDtoOwner(ctx, dto.Owner)
	diags.Append(d...)
	model.Owner = owner

	return model, diags
}
