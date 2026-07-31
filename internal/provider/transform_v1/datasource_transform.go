// See resource_transform.go in this package for design notes and known
// limitations shared by both the resource and this data source.
package transform_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v2"
	"github.com/sailpoint-oss/golang-sdk/v2/api_beta"

	"terraform-provider-identitynow/internal/provider/transform_v1/datasource_transform"
)

var (
	_ datasource.DataSource              = (*transformDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*transformDataSource)(nil)
)

func NewTransformDataSource() datasource.DataSource {
	return &transformDataSource{}
}

type transformDataSource struct {
	client *sailpoint.APIClient
}

// transformDataSourceModel mirrors datasource_transform.TransformModel plus
// the hand-added "attributes" field - see transformResourceModel's doc
// comment in resource_transform.go for why this can't just embed the
// generated model type.
type transformDataSourceModel struct {
	Id         types.String         `tfsdk:"id"`
	Internal   types.Bool           `tfsdk:"internal"`
	Name       types.String         `tfsdk:"name"`
	Type       types.String         `tfsdk:"type"`
	Attributes jsontypes.Normalized `tfsdk:"attributes"`
}

func (d *transformDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transform_v1"
}

func (d *transformDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_transform.TransformDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Transform from IdentityNow/ISC by id."
	resp.Schema.MarkdownDescription = "Reads a [Transform](https://developer.sailpoint.com/docs/extensibility/transforms/) " +
		"from IdentityNow/ISC by `id`.\n\n" +
		"~> This is a `_v1` pilot data source - see \"Known Limitations & Live Testing Notes\" below before relying on it " +
		"in production configurations.\n\n" +
		transformGuidanceMarkdown
	applyTransformAttributesFieldDataSource(&resp.Schema.Attributes)
}

// applyTransformAttributesFieldDataSource mirrors applyTransformAttributesField
// (resource_transform_planmodifiers.go) but against a
// datasource/schema.Attribute map (a distinct Go type from
// resource/schema.Attribute, so it can't share the same function signature).
func applyTransformAttributesFieldDataSource(attrs *map[string]dsschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]dsschema.Attribute{}
	}
	desc := "Meta-data about the transform, as a raw JSON object. Values are specific to the transform's \"type\" - " +
		"see https://developer.sailpoint.com/docs/extensibility/transforms/operations for the shape each \"type\" expects."
	(*attrs)["attributes"] = dsschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

func (d *transformDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *transformDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config transformDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Transform data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.Beta.TransformsAPI.
		GetTransform(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Transform data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Transform", errDetail(err, httpResp))
		return
	}

	state, diags := datasourceDtoToModel(dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Read Transform data source", map[string]interface{}{"id": state.Id.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// datasourceDtoToModel mirrors transformReadToModel in resource_transform.go
// but against the data source's model type.
func datasourceDtoToModel(dto *api_beta.TransformRead, fallback transformDataSourceModel) (transformDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	model.Id = types.StringValue(dto.Id)
	model.Internal = types.BoolValue(dto.Internal)
	model.Name = types.StringValue(dto.Name)
	model.Type = types.StringValue(dto.Type)

	model.Attributes, diags = normalizedAttributesFromAPI(dto.Attributes)

	return model, diags
}
