package governancegroup

import (
	"context"
	"fmt"
	"log"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &GovernanceGroupDataSource{}

func NewGovernanceGroupDataSource() datasource.DataSource {
	return &GovernanceGroupDataSource{}
}

type GovernanceGroupDataSource struct {
	client *sailpoint.APIClient
}

func (d *GovernanceGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_group"
}

func (d *GovernanceGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Governance Group data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Governance group ID.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Governance group name.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Governance group description.",
			},
			"member_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of members in the governance group.",
			},
			"connection_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of connections in the governance group.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner's DTO type.",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner's identity ID.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Owner's display name.",
					},
				},
				Computed:            true,
				MarkdownDescription: "Owner",
			},
			"membership": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Identity's DTO type.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Identity ID.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Identity's display name.",
						},
					},
				},
				Computed:            true,
				MarkdownDescription: "membership",
			},
		},
	}
}

func (d *GovernanceGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(config.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = config.APIClient
}

func (d *GovernanceGroupDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *GovernanceGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkgroupDto

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.IsNull() {

		filters := fmt.Sprintf(`name eq "%v"`, data.Name.ValueString())

		workgroups, httpResp, err := d.client.Beta.GovernanceGroupsAPI.ListWorkgroups(context.Background()).Filters(filters).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroups",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroups",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}

		switch len(workgroups) {
		case 0:
			resp.Diagnostics.AddError(
				"No workgroup found",
				"Error: No workgroup found for value",
			)
			return
		case 1:
			data.Id = types.StringPointerValue(workgroups[0].Id)
		default:
			resp.Diagnostics.AddError(
				"More than one workgroup found",
				fmt.Sprintf("Error: %v workgroups found with query, only results with 1 will return data", len(workgroups)),
			)
			return
		}

	}

	workgroup, httpResp, err := d.client.Beta.GovernanceGroupsAPI.GetWorkgroup(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.GetWorkgroup",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.GetWorkgroup",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Id = types.StringPointerValue(workgroup.Id)
	data.Name = types.StringPointerValue(workgroup.Name)
	data.Description = types.StringPointerValue(workgroup.Description)
	data.MemberCount = types.Int64PointerValue(workgroup.MemberCount)
	data.ConnectionCount = types.Int64PointerValue(workgroup.ConnectionCount)

	workgroupMembers, httpResp, err := d.client.Beta.GovernanceGroupsAPI.ListWorkgroupMembers(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.GovernanceGroupsAPI.ListWorkgroupMembers",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	elements := []attr.Value{}
	for _, v := range workgroupMembers {
		member, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
			"name": types.StringPointerValue(v.Name),
			"id":   types.StringPointerValue(v.Id),
			"type": types.StringPointerValue((*string)(v.Type)),
		})
		if ok.HasError() {
			resp.Diagnostics.Append(ok...)
		}
		log.Printf("member`: %v\n", member)

		elements = append(elements, member)
	}

	listValue := types.SetValueMust(types.ObjectType{AttrTypes: baseReferenceDto1Types}, elements)

	data.Membership = listValue

	owner, ok := types.ObjectValue(baseReferenceDto1Types, map[string]attr.Value{
		"name": types.StringPointerValue(workgroup.Owner.Name),
		"id":   types.StringPointerValue(workgroup.Owner.Id),
		"type": types.StringPointerValue((*string)(workgroup.Owner.Type)),
	})
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}

	data.Owner = owner

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
