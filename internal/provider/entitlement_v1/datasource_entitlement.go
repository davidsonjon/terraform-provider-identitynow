// See resource_entitlement.go in this package for the adopt-existing design
// notes and hand-added schema field rationale shared by the resource and these
// data sources.
package entitlement_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/entitlements"

	"terraform-provider-identitynow/internal/provider/entitlement_v1/datasource_entitlement"
)

var (
	_ datasource.DataSource              = (*entitlementDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*entitlementDataSource)(nil)
)

func NewEntitlementDataSource() datasource.DataSource {
	return &entitlementDataSource{}
}

type entitlementDataSource struct {
	client *sailpoint.APIClient
}

type entitlementDataSourceModel struct {
	AccessModelMetadata    datasource_entitlement.AccessModelMetadataValue `tfsdk:"access_model_metadata"`
	Attribute              types.String                                    `tfsdk:"attribute"`
	Attributes             jsontypes.Normalized                            `tfsdk:"attributes"`
	CloudGoverned          types.Bool                                      `tfsdk:"cloud_governed"`
	Created                types.String                                    `tfsdk:"created"`
	Description            types.String                                    `tfsdk:"description"`
	DirectPermissions      types.List                                      `tfsdk:"direct_permissions"`
	Id                     types.String                                    `tfsdk:"id"`
	ManuallyUpdatedFields  types.Object                                    `tfsdk:"manually_updated_fields"`
	Modified               types.String                                    `tfsdk:"modified"`
	Name                   types.String                                    `tfsdk:"name"`
	Owner                  datasource_entitlement.OwnerValue               `tfsdk:"owner"`
	PrivilegeLevel         datasource_entitlement.PrivilegeLevelValue      `tfsdk:"privilege_level"`
	Requestable            types.Bool                                      `tfsdk:"requestable"`
	Segments               types.List                                      `tfsdk:"segments"`
	Source                 datasource_entitlement.SourceValue              `tfsdk:"source"`
	SourceSchemaObjectType types.String                                    `tfsdk:"source_schema_object_type"`
	Tags                   types.List                                      `tfsdk:"tags"`
	Value                  types.String                                    `tfsdk:"value"`
}

func (d *entitlementDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement_v1"
}

func (d *entitlementDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		Description: "Reads an Entitlement from IdentityNow/ISC by id.",
		MarkdownDescription: "Reads an Entitlement from IdentityNow/ISC by `id`. Returns the same attributes as the " +
			"`identitynow_entitlement_v1` resource, including the hand-added `attributes` JSON blob and " +
			"`manually_updated_fields` object.",
		Attributes: entitlementDataSourceAttributes(ctx),
	}
}

func entitlementDataSourceAttributes(ctx context.Context) map[string]datasourceschema.Attribute {
	attrs := datasource_entitlement.EntitlementDataSourceSchema(ctx).Attributes
	out := make(map[string]datasourceschema.Attribute, len(attrs)+2)
	for k, v := range attrs {
		out[k] = v
	}
	applyEntitlementDataSourceAttributesField(&out)
	applyEntitlementDataSourceManuallyUpdatedFieldsField(&out)
	return out
}

func applyEntitlementDataSourceAttributesField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	desc := "Raw source-system attributes for this entitlement, represented as a normalized JSON object because the shape is connector-specific and truly dynamic."
	(*attrs)["attributes"] = datasourceschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

func applyEntitlementDataSourceManuallyUpdatedFieldsField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	desc := "Flags describing whether selected entitlement fields were manually updated after first aggregation."
	(*attrs)["manually_updated_fields"] = datasourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
		Attributes: map[string]datasourceschema.Attribute{
			"display_name": datasourceschema.BoolAttribute{
				Computed:            true,
				Description:         "Whether the entitlement display name was manually updated.",
				MarkdownDescription: "Whether the entitlement display name was manually updated.",
			},
			"description": datasourceschema.BoolAttribute{
				Computed:            true,
				Description:         "Whether the entitlement description was manually updated.",
				MarkdownDescription: "Whether the entitlement description was manually updated.",
			},
		},
	}
}

func (d *entitlementDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *entitlementDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config entitlementDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Entitlement data source", map[string]interface{}{"id": config.Id.ValueString()})

	dto, httpResp, err := d.client.EntitlementsAPI.
		GetEntitlementV1(ctx, config.Id.ValueString()).
		Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Entitlement data source", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
		resp.Diagnostics.AddError("Error reading Entitlement", entitlementErrDetail(err, httpResp))
		return
	}

	state, diags := entitlementDataSourceDtoToModel(ctx, dto, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func entitlementDataSourceDtoToModel(ctx context.Context, dto *entitlements.EntitlementV2, fallback entitlementDataSourceModel) (entitlementDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := fallback

	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	} else {
		model.Id = types.StringNull()
	}
	model.Name = types.StringPointerValue(dto.Name)
	model.Attribute = types.StringPointerValue(dto.Attribute)
	model.Description = types.StringPointerValue(dto.Description.Get())
	model.Value = types.StringPointerValue(dto.Value)
	model.SourceSchemaObjectType = types.StringPointerValue(dto.SourceSchemaObjectType)
	model.CloudGoverned = types.BoolPointerValue(dto.CloudGoverned)
	model.Requestable = types.BoolPointerValue(dto.Requestable)
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	v, d := entitlementDatasourceAccessModelMetadataFromAPI(ctx, dto.AccessModelMetadata)
	diags.Append(d...)
	model.AccessModelMetadata = v

	directPermissions, d := entitlementDatasourceDirectPermissionsFromAPI(ctx, dto.DirectPermissions)
	diags.Append(d...)
	model.DirectPermissions = directPermissions

	owner, d := entitlementDatasourceOwnerFromAPI(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	privilegeLevel, d := entitlementDatasourcePrivilegeLevelFromAPI(ctx, dto.PrivilegeLevel)
	diags.Append(d...)
	model.PrivilegeLevel = privilegeLevel

	segments, d := stringListValueFromAPI(ctx, dto.Segments)
	diags.Append(d...)
	model.Segments = segments

	source, d := entitlementDatasourceSourceFromAPI(ctx, dto.Source)
	diags.Append(d...)
	model.Source = source

	tags, d := entitlementTagsFromAPI(ctx, dto.Tags)
	diags.Append(d...)
	model.Tags = tags

	attributes, d := normalizedJSONFromMap(dto.Attributes)
	diags.Append(d...)
	model.Attributes = attributes

	manuallyUpdatedFields, d := entitlementManuallyUpdatedFieldsFromAPI(dto.ManuallyUpdatedFields)
	diags.Append(d...)
	model.ManuallyUpdatedFields = manuallyUpdatedFields

	return model, diags
}

func entitlementDatasourceAccessModelMetadataFromAPI(ctx context.Context, dto *entitlements.EntitlementV2AccessModelMetadata) (datasource_entitlement.AccessModelMetadataValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if dto == nil {
		return datasource_entitlement.NewAccessModelMetadataValueNull(), diags
	}

	attributesList, d := entitlementDatasourceAttributeDTOListFromAPI(ctx, dto.Attributes)
	diags.Append(d...)

	v, d := datasource_entitlement.NewAccessModelMetadataValue(
		datasource_entitlement.AccessModelMetadataValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"attributes": attributesList},
	)
	diags.Append(d...)
	return v, diags
}

func entitlementDatasourceAttributeDTOListFromAPI(ctx context.Context, items []entitlements.AccessModelMetadata) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_entitlement.AttributesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_entitlement.AttributesValue, 0, len(items))
	for _, item := range items {
		valuesList, d := entitlementDatasourceAttributeValueDTOListFromAPI(ctx, item.Values)
		diags.Append(d...)

		objectTypes, d := types.ListValueFrom(ctx, types.StringType, item.ObjectTypes)
		diags.Append(d...)

		v, d := datasource_entitlement.NewAttributesValue(
			datasource_entitlement.AttributesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"description":  types.StringPointerValue(item.Description),
				"key":          types.StringPointerValue(item.Key),
				"multiselect":  types.BoolPointerValue(item.Multiselect),
				"name":         types.StringPointerValue(item.Name),
				"object_types": objectTypes,
				"status":       types.StringPointerValue(item.Status),
				"type":         types.StringPointerValue(item.Type),
				"values":       valuesList,
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementDatasourceAttributeValueDTOListFromAPI(ctx context.Context, items []entitlements.AccessModelMetadataValuesInner) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_entitlement.ValuesValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_entitlement.ValuesValue, 0, len(items))
	for _, item := range items {
		v, d := datasource_entitlement.NewValuesValue(
			datasource_entitlement.ValuesValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"name":   types.StringPointerValue(item.Name),
				"status": types.StringPointerValue(item.Status),
				"value":  types.StringPointerValue(item.Value),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementDatasourceDirectPermissionsFromAPI(ctx context.Context, items []entitlements.PermissionDTO) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := datasource_entitlement.DirectPermissionsValue{}.Type(ctx)
	if items == nil {
		return types.ListNull(elemType), diags
	}

	values := make([]datasource_entitlement.DirectPermissionsValue, 0, len(items))
	for _, item := range items {
		rights, d := types.ListValueFrom(ctx, types.StringType, item.Rights)
		diags.Append(d...)
		v, d := datasource_entitlement.NewDirectPermissionsValue(
			datasource_entitlement.DirectPermissionsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"rights": rights,
				"target": types.StringPointerValue(item.Target),
			},
		)
		diags.Append(d...)
		values = append(values, v)
	}

	listVal, d := types.ListValueFrom(ctx, elemType, values)
	diags.Append(d...)
	return listVal, diags
}

func entitlementDatasourceOwnerFromAPI(ctx context.Context, dto *entitlements.EntitlementV2Owner) (datasource_entitlement.OwnerValue, diag.Diagnostics) {
	if dto == nil {
		return datasource_entitlement.NewOwnerValueNull(), nil
	}
	return datasource_entitlement.NewOwnerValue(
		datasource_entitlement.OwnerValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(dto.Id),
			"name": types.StringPointerValue(dto.Name),
			"type": types.StringPointerValue(dto.Type),
		},
	)
}

func entitlementDatasourceSourceFromAPI(ctx context.Context, dto *entitlements.EntitlementV2Source) (datasource_entitlement.SourceValue, diag.Diagnostics) {
	if dto == nil {
		return datasource_entitlement.NewSourceValueNull(), nil
	}
	return datasource_entitlement.NewSourceValue(
		datasource_entitlement.SourceValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(dto.Id),
			"name": types.StringPointerValue(dto.Name),
			"type": types.StringPointerValue(dto.Type),
		},
	)
}

func entitlementDatasourcePrivilegeLevelFromAPI(ctx context.Context, pl *entitlements.EntitlementV2PrivilegeLevel) (datasource_entitlement.PrivilegeLevelValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	if pl == nil {
		return datasource_entitlement.NewPrivilegeLevelValueNull(), diags
	}
	v, d := datasource_entitlement.NewPrivilegeLevelValue(
		datasource_entitlement.PrivilegeLevelValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"direct":      types.StringPointerValue(pl.Direct),
			"effective":   types.StringPointerValue(pl.Effective),
			"inherited":   types.StringPointerValue(pl.Inherited.Get()),
			"set_by":      types.StringPointerValue(pl.SetBy),
			"set_by_type": types.StringPointerValue(pl.SetByType.Get()),
		},
	)
	diags.Append(d...)
	return v, diags
}
