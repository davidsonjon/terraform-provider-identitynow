package identity

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &IdentitiesDataSource{}

func NewIdentitiesDataSource() datasource.DataSource {
	return &IdentitiesDataSource{}
}

type IdentitiesDataSource struct {
	client *sailpoint.APIClient
}

func (d *IdentitiesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identities"
}

func (d *IdentitiesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Identities data source for querying multiple identities with filter support",

		Attributes: map[string]schema.Attribute{
			"filters": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter expression to query identities (e.g., 'alias sw \"alice\"', 'email eq \"test@example.com\"', or 'firstname eq \"John\"') filtering is support for the following fields and operators:\n\n **id**: eq, in\n\n **name**: eq, sw\n\n **alias**: eq, sw\n\n **firstname**: eq, sw\n\n **lastname**: eq, sw\n\n **email**: eq, sw\n\n **cloudStatus**: eq\n\n **processingState**: eq\n\n **correlated**: eq\n\n **protected**: eq",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of identities to return (default: 250, max: 250)",
			},
			"identities": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of identities matching the filter criteria",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "System-generated unique ID of the Object",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the Object",
						},
						"created": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Creation date of the Object",
						},
						"modified": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Last modification date of the Object",
						},
						"alias": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Alternate unique identifier for the identity",
						},
						"email_address": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The email address of the identity",
						},
						"processing_state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The processing state of the identity",
						},
						"identity_status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identity's status in the system",
						},
					},
				},
			},
		},
	}
}

func (d *IdentitiesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentitiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentitiesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default limit if not specified
	limit := int32(250)
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		requestedLimit := int32(data.Limit.ValueInt64())
		if requestedLimit > 250 {
			resp.Diagnostics.AddWarning(
				"Limit exceeds maximum",
				"The requested limit exceeds the API maximum of 250. Using 250 instead.",
			)
		} else {
			limit = requestedLimit
		}
	}

	// Call the ListIdentities API
	apiRequest := d.client.Beta.IdentitiesAPI.ListIdentities(ctx)

	// Apply filter if provided
	if !data.Filters.IsNull() && !data.Filters.IsUnknown() {
		apiRequest = apiRequest.Filters(data.Filters.ValueString())
	}

	apiRequest = apiRequest.Limit(limit)

	identities, httpResp, err := apiRequest.Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.IdentitiesAPI.ListIdentities",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.IdentitiesAPI.ListIdentities",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	// Convert identities to our model format
	var identityModels []IdentityListItem
	for _, identity := range identities {
		identityModel := IdentityListItem{
			Id:              types.StringPointerValue(identity.Id),
			Name:            types.StringValue(identity.Name),
			Created:         types.StringPointerValue(identity.Created),
			Modified:        types.StringPointerValue(identity.Modified),
			Alias:           types.StringPointerValue(identity.Alias),
			EmailAddress:    types.StringPointerValue(identity.EmailAddress.Get()),
			ProcessingState: types.StringPointerValue(identity.ProcessingState.Get()),
			IdentityStatus:  types.StringPointerValue(identity.IdentityStatus),
		}

		identityModels = append(identityModels, identityModel)
	}

	// Convert to Terraform list type
	identitiesList, listDiags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: getIdentityObjectAttrTypes(),
	}, identityModels)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Identities = identitiesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// getIdentityObjectAttrTypes returns the attribute types for the Identity object
func getIdentityObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":               types.StringType,
		"name":             types.StringType,
		"created":          types.StringType,
		"modified":         types.StringType,
		"alias":            types.StringType,
		"email_address":    types.StringType,
		"processing_state": types.StringType,
		"identity_status":  types.StringType,
	}
}
