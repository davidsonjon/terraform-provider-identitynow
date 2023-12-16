package entitlement

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk"
	beta "github.com/davidsonjon/golang-sdk/beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &EntitlementDataSource{}

func NewEntitlementDataSource() datasource.DataSource {
	return &EntitlementDataSource{}
}

type EntitlementDataSource struct {
	client *sailpoint.APIClient
}

func (d *EntitlementDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement"
}

func (d *EntitlementDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlement data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The entitlement id",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The entitlement name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the entitlement was created",
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the entitlement was last modified",
			},
			"attribute": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The entitlement attribute name",
			},
			"value": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The value of the entitlement",
			},
			"source_schema_object_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The object type of the entitlement from the source schema",
			},
			"privileged": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is privileged",
			},
			"cloud_governed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is cloud governed",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the entitlement",
			},
			"requestable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is requestable",
			},
			"source_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Source ID of the entitlement",
			},
			"owner_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Owner ID of the entitlement",
			},
		},
	}
}

func (d *EntitlementDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *EntitlementDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Entitlement

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entitlement, httpResp, err := d.client.Beta.EntitlementsApi.GetEntitlement(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsApi.GetEntitlement",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsApi.GetEntitlement",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	parseAttributes(&data, entitlement, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func parseAttributes(ent *Entitlement, betaEnt *beta.Entitlement, diags *diag.Diagnostics) {
	ent.Id = types.StringPointerValue(betaEnt.Id)
	ent.Name = types.StringPointerValue(betaEnt.Name)
	ent.Created = types.StringValue(betaEnt.Created.String())
	ent.Modified = types.StringValue(betaEnt.Modified.String())
	ent.Attribute = types.StringPointerValue(betaEnt.Attribute)
	ent.Value = types.StringPointerValue(betaEnt.Value)
	ent.SourceSchemaObjectType = types.StringPointerValue(betaEnt.SourceSchemaObjectType)
	ent.Privileged = types.BoolPointerValue(betaEnt.Privileged)
	ent.CloudGoverned = types.BoolPointerValue(betaEnt.CloudGoverned)
	ent.Description = types.StringPointerValue(betaEnt.Description)
	ent.Requestable = types.BoolPointerValue(betaEnt.Requestable)

	ent.SourceID = types.StringPointerValue(betaEnt.Source.Id)

	if betaEnt.Owner != nil {
		if betaEnt.Owner.Id != nil {
			ent.OwnerID = types.StringPointerValue(betaEnt.Owner.Id)
		}
	} else {
		ent.OwnerID = types.StringNull()
	}

}
