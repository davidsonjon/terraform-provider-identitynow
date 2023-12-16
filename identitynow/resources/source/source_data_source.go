// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package source

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk"
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

var _ datasource.DataSource = &SourceDataSource{}

func NewSourceDataSource() datasource.DataSource {
	return &SourceDataSource{}
}

type SourceDataSource struct {
	client *sailpoint.APIClient
}

func (d *SourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source"
}

func (d *SourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Source data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The id of the Source",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Human-readable name of the source",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable description of the source",
			},
			"connector_attributes": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"cloud_external_id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Cloud External ID (old source ID)",
					},
				},
				Computed:            true,
				MarkdownDescription: "Connector Attributes",
			},
		},
	}
}

func (d *SourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *SourceDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *SourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Source

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.IsNull() {
		filter := fmt.Sprintf(`name eq "%v"`, data.Name.ValueString())

		sources, httpResp, err := d.client.V3.SourcesApi.ListSources(context.Background()).Filters(filter).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling V3.SourcesApi.ListSources",
					fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling V3.SourcesApi.ListSources",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}

		switch len(sources) {
		case 0:
			resp.Diagnostics.AddError(
				"No identities found",
				"Error: No sources found for value",
			)
			return
		case 1:
			data.Id = types.StringPointerValue(sources[0].Id)
		default:
			resp.Diagnostics.AddError(
				"More than one identity found",
				fmt.Sprintf("Error: %T sources found with query, only results with 1 will return data", len(sources)),
			)
			return
		}
	}

	source, httpResp, err := d.client.V3.SourcesApi.GetSource(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s", sailpointError.FormattedMessage),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SourcesApi.GetSource",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Id = types.StringPointerValue(source.Id)
	data.Name = types.StringValue(source.Name)
	data.Description = types.StringPointerValue(source.Description)

	cAttr, ok := types.ObjectValue(connectorAttributesTypes, map[string]attr.Value{
		"cloud_external_id": types.StringValue(source.ConnectorAttributes["cloudExternalId"].(string)),
	})
	if ok.HasError() {
		resp.Diagnostics.Append(ok...)
	}
	data.ConnectorAttributes = cAttr

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
