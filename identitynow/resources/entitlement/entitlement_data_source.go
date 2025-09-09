package entitlement

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
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
	// Get the shared entitlement schema attributes
	attributes := GetEntitlementSchemaAttributes()

	// Override the 'id' attribute to be required for the single entitlement data source
	attributes["id"] = schema.StringAttribute{
		Required:            true,
		MarkdownDescription: "The entitlement id",
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlement data source",
		Attributes:          attributes,
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

	entitlement, httpResp, err := d.client.Beta.EntitlementsAPI.GetEntitlement(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsApi.GetEntitlement",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
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

	// Use the shared conversion function
	convertedEntitlement, diags := ConvertBetaEntitlementToModel(ctx, *entitlement)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the converted data, but preserve the original ID from the request
	data = convertedEntitlement
	data.Id = types.StringValue(data.Id.ValueString()) // Ensure ID is set

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// parseAttributes is a wrapper function to maintain compatibility with entitlement_resource.go
// This function is deprecated and should be replaced with ConvertBetaEntitlementToModel
func parseAttributes(ent *Entitlement, betaEnt *beta.Entitlement, diags *diag.Diagnostics) {
	convertedEnt, convertDiags := ConvertBetaEntitlementToModel(context.Background(), *betaEnt)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return
	}
	*ent = convertedEnt
}
