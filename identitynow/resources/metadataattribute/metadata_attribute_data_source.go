package metadataattribute

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &AccessModelMetadataDataSource{}

func NewAccessModelMetadataDataSource() datasource.DataSource {
	return &AccessModelMetadataDataSource{}
}

type AccessModelMetadataDataSource struct {
	client *sailpoint.APIClient
}

func (d *AccessModelMetadataDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_model_metadata"
}

func (d *AccessModelMetadataDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Identity data source",

		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Technical name of the Attribute. This is unique and cannot be changed after creation.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display name of the key.",
			},
			"multiselect": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Indicates whether the attribute can have multiple values.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The status of the Attribute.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The type of the Attribute. This can be either `custom` or `governance`.",
			},
			"object_types": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the Attribute.",
			},
			"values": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"value": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Technical name of the Attribute value. This is unique and cannot be changed after creation.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The display name of the Attribute value.",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The status of the Attribute value.",
						},
					},
				},
				Computed: true,
			},
		},
	}
}

func (d *AccessModelMetadataDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *AccessModelMetadataDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AttributeDTO

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	key := data.Key.ValueString()

	metadata, httpResp, err := d.client.Beta.AccessModelMetadataAPI.GetAccessModelMetadataAttribute(context.Background(), key).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.CreateAccessProfile",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.CreateAccessProfile",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Key = types.StringPointerValue(metadata.Key)
	data.Name = types.StringPointerValue(metadata.Name)
	data.Multiselect = types.BoolPointerValue(metadata.Multiselect)
	data.Status = types.StringPointerValue(metadata.Status)
	data.Type = types.StringPointerValue(metadata.Type)
	data.Description = types.StringPointerValue(metadata.Description)

	objectTypes, diags := types.ListValueFrom(ctx, types.StringType, metadata.ObjectTypes)
	data.ObjectTypes = objectTypes
	resp.Diagnostics.Append(diags...)

	attributeValues := []AttributeValueDTO{}

	for _, v := range metadata.Values {
		value := AttributeValueDTO{
			Value:  types.StringPointerValue(v.Value),
			Name:   types.StringPointerValue(v.Name),
			Status: types.StringPointerValue(v.Status),
		}
		attributeValues = append(attributeValues, value)
	}
	data.Values = attributeValues
	// data.ObjectTypes = types.StringSlicePointerValue(metadata.ObjectTypes)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
