package entitlement

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

var _ datasource.DataSource = &EntitlementsDataSource{}

func NewEntitlementsDataSource() datasource.DataSource {
	return &EntitlementsDataSource{}
}

type EntitlementsDataSource struct {
	client *sailpoint.APIClient
}

// EntitlementsDataSourceModel represents the data source configuration model
type EntitlementsDataSourceModel struct {
	Filter       types.String `tfsdk:"filter"`
	SourceID     types.String `tfsdk:"source_id"`
	Limit        types.Int64  `tfsdk:"limit"`
	Entitlements types.List   `tfsdk:"entitlements"`
}

func (d *EntitlementsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlements"
}

func (d *EntitlementsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Get the shared entitlement schema attributes
	entitlementAttributes := GetEntitlementSchemaAttributes()
	
	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlements data source for querying multiple entitlements with filter support",

		Attributes: map[string]schema.Attribute{
			"filter": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter expression to query entitlements (e.g., 'source.id eq \"xxx\"')",
			},
			"source_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Source ID to filter entitlements (convenience field for source.id filter)",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of entitlements to return (default: 250)",
			},
			"entitlements": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of entitlements matching the filter criteria",
				NestedObject: schema.NestedAttributeObject{
					Attributes: entitlementAttributes,
				},
			},
		},
	}
}

func (d *EntitlementsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EntitlementsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EntitlementsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build filter from either explicit filter or source_id convenience field
	var filter string
	if !data.Filter.IsNull() && !data.Filter.IsUnknown() {
		filter = data.Filter.ValueString()
	} else if !data.SourceID.IsNull() && !data.SourceID.IsUnknown() {
		filter = fmt.Sprintf(`source.id eq "%s"`, data.SourceID.ValueString())
	}

	// Set default limit if not specified
	limit := int32(250)
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		limit = int32(data.Limit.ValueInt64())
	}

	// Call the ListEntitlements API
	apiRequest := d.client.Beta.EntitlementsAPI.ListEntitlements(ctx)
	if filter != "" {
		apiRequest = apiRequest.Filters(filter)
	}
	apiRequest = apiRequest.Limit(limit)

	entitlements, httpResp, err := apiRequest.Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.ListEntitlements",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.ListEntitlements",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	// Convert entitlements using the shared conversion function
	var entitlementModels []Entitlement
	for _, entitlement := range entitlements {
		entitlementModel, diags := ConvertBetaEntitlementToModel(ctx, entitlement)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		entitlementModels = append(entitlementModels, entitlementModel)
	}

	// Convert to Terraform list type
	entitlementsList, listDiags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: getEntitlementObjectAttrTypes(),
	}, entitlementModels)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Entitlements = entitlementsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// getEntitlementObjectAttrTypes returns the attribute types for the Entitlement object
// This uses the existing types from the model
func getEntitlementObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                        types.StringType,
		"name":                      types.StringType,
		"created":                   types.StringType,
		"modified":                  types.StringType,
		"attribute":                 types.StringType,
		"value":                     types.StringType,
		"source_schema_object_type": types.StringType,
		"privileged":                types.BoolType,
		"cloud_governed":            types.BoolType,
		"description":               types.StringType,
		"requestable":               types.BoolType,
		"source_id":                 types.StringType,
		"owner": types.ObjectType{
			AttrTypes: OwnerSchemeObject,
		},
		"access_model_metadata": types.ListType{
			ElemType: AttributeDTOObject,
		},
	}
}