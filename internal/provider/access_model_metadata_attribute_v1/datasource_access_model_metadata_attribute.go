// See resource_access_model_metadata_attribute.go in this package for design
// notes and known limitations shared by both the resource and this data
// source.
package access_model_metadata_attribute_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/access_model_metadata"

	"terraform-provider-identitynow/internal/provider/access_model_metadata_attribute_v1/datasource_access_model_metadata_attribute"
)

var (
	_ datasource.DataSource              = (*accessModelMetadataAttributeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accessModelMetadataAttributeDataSource)(nil)
)

func NewAccessModelMetadataAttributeDataSource() datasource.DataSource {
	return &accessModelMetadataAttributeDataSource{}
}

type accessModelMetadataAttributeDataSource struct {
	client *sailpoint.APIClient
}

func (d *accessModelMetadataAttributeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_model_metadata_attribute_v1"
}

func (d *accessModelMetadataAttributeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_access_model_metadata_attribute.AccessModelMetadataAttributeDataSourceSchema(ctx)
	resp.Schema.Description = "Reads an Access Model Metadata Attribute from IdentityNow/ISC by key."
	resp.Schema.MarkdownDescription = "Reads an [Access Model Metadata](https://documentation.sailpoint.com/saas/help/access/metadata.html) " +
		"Attribute from IdentityNow/ISC by `key`.\n\n" +
		"~> This is a `_v1` pilot data source - see the resource documentation's \"Known Limitations & Live Testing Notes\" " +
		"section before relying on it in production configurations."
}

func (d *accessModelMetadataAttributeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accessModelMetadataAttributeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_access_model_metadata_attribute.AccessModelMetadataAttributeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Access Model Metadata Attribute data source", map[string]interface{}{"key": config.Key.ValueString()})

	dto, httpResp, err := d.client.AccessModelMetadataAPI.
		GetAccessModelMetadataAttributeV1(ctx, config.Key.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Access Model Metadata Attribute data source", map[string]interface{}{"key": config.Key.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Access Model Metadata Attribute", errDetail(err, httpResp))
		return
	}

	state, err := datasourceDtoToModel(ctx, dto, config)
	if err != nil {
		resp.Diagnostics.AddError("Error converting Access Model Metadata Attribute response", err.Error())
		return
	}

	tflog.Debug(ctx, "Read Access Model Metadata Attribute data source", map[string]interface{}{"key": state.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors ammDtoToModel in resource_access_model_metadata_attribute.go
// but against the data source's (separately generated) model type.
func datasourceDtoToModel(ctx context.Context, dto *access_model_metadata.AttributeDTO, fallback datasource_access_model_metadata_attribute.AccessModelMetadataAttributeModel) (datasource_access_model_metadata_attribute.AccessModelMetadataAttributeModel, error) {
	model := fallback

	model.Key = types.StringPointerValue(dto.Key)
	model.Name = types.StringPointerValue(dto.Name)
	model.Multiselect = types.BoolPointerValue(dto.Multiselect)
	model.Description = types.StringPointerValue(dto.Description)
	model.Status = types.StringPointerValue(dto.Status)
	model.Type = types.StringPointerValue(dto.Type)

	if dto.ObjectTypes != nil {
		listVal, diags := types.ListValueFrom(ctx, types.StringType, dto.ObjectTypes)
		if diags.HasError() {
			return model, fmt.Errorf("could not convert object_types: %v", diags)
		}
		model.ObjectTypes = listVal
	} else {
		model.ObjectTypes = types.ListNull(types.StringType)
	}

	values := make([]datasource_access_model_metadata_attribute.ValuesValue, 0, len(dto.Values))
	for i := range dto.Values {
		v, diags := datasource_access_model_metadata_attribute.ValuesValue{}.FromApi_betaAttributeValueDTO(ctx, &dto.Values[i])
		if diags.HasError() {
			return model, fmt.Errorf("could not convert values[%d]: %v", i, diags)
		}
		values = append(values, v)
	}
	listVal, diags := types.ListValueFrom(ctx, datasource_access_model_metadata_attribute.ValuesValue{}.Type(ctx), values)
	if diags.HasError() {
		return model, fmt.Errorf("could not convert values: %v", diags)
	}
	model.Values = listVal

	return model, nil
}
