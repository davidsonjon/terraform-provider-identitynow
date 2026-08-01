package segment_v1

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/segments"

	"terraform-provider-identitynow/internal/provider/segment_v1/datasource_segments"
)

var (
	_ datasource.DataSource              = (*segmentsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*segmentsDataSource)(nil)
)

type segmentsDataSourceModel struct {
	Limit    types.Int64 `tfsdk:"limit"`
	Offset   types.Int64 `tfsdk:"offset"`
	Segments types.Set   `tfsdk:"segments"`
}

type segmentsListItemModel struct {
	Active             types.Bool   `tfsdk:"active"`
	Created            types.String `tfsdk:"created"`
	Description        types.String `tfsdk:"description"`
	Id                 types.String `tfsdk:"id"`
	Modified           types.String `tfsdk:"modified"`
	Name               types.String `tfsdk:"name"`
	Owner              types.Object `tfsdk:"owner"`
	VisibilityCriteria types.Object `tfsdk:"visibility_criteria"`
}

func NewSegmentsDataSource() datasource.DataSource {
	return &segmentsDataSource{}
}

type segmentsDataSource struct {
	client *sailpoint.APIClient
}

func (d *segmentsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segments_v1"
}

func (d *segmentsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = segmentsDataSourceSchema(ctx)
	resp.Schema.Description = "Lists Segments from IdentityNow/ISC, optionally paginated."
	resp.Schema.MarkdownDescription = "Lists Segments from IdentityNow/ISC via `GET /segments`, optionally paginated. Returns the same generated fields as Phase 1 codegen plus the hand-written `visibility_criteria` tree for each segment."
}

func (d *segmentsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *segmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config segmentsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading Segments data source")

	apiReq := d.client.SegmentsAPI.ListSegmentsV1(ctx)
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		apiReq = apiReq.Limit(int32(config.Limit.ValueInt64()))
	}
	if !config.Offset.IsNull() && !config.Offset.IsUnknown() {
		apiReq = apiReq.Offset(int32(config.Offset.ValueInt64()))
	}

	dtos, httpResp, err := apiReq.Execute()
	if err != nil {
		tflog.Error(ctx, "Error reading Segments data source", map[string]interface{}{"error": err.Error()})
		resp.Diagnostics.AddError("Error listing Segments", segmentErrDetail(err, httpResp))
		return
	}

	fullType := segmentsDataSourceSchema(ctx).Type().(basetypes.ObjectType)
	elemType := fullType.AttrTypes["segments"].(basetypes.SetType).ElemType

	items := make([]segmentsListItemModel, 0, len(dtos))
	for i := range dtos {
		item, diags := segmentListItemFromDTO(ctx, &dtos[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		items = append(items, item)
	}

	segmentsSet, diags := types.SetValueFrom(ctx, elemType, items)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	config.Segments = segmentsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func segmentsDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	s := datasource_segments.SegmentsDataSourceSchema(ctx)
	if segmentsAttr, ok := s.Attributes["segments"].(datasourceschema.SetNestedAttribute); ok {
		applyDataSourceVisibilityCriteriaField(&segmentsAttr.NestedObject.Attributes)
		// The generated NestedObject carries a fixed `CustomType` (SegmentsType)
		// derived at codegen time from the (visibilityCriteria-ignored)
		// generated model, which takes precedence over the Attributes map when
		// the framework computes .Type() - so without clearing it here, the
		// object type would silently omit the visibility_criteria field we just
		// added above, causing a "Struct defines fields not found in object"
		// conversion error at Read time. Clearing it makes .Type() derive the
		// object type dynamically from Attributes instead.
		segmentsAttr.NestedObject.CustomType = nil
		s.Attributes["segments"] = segmentsAttr
	}
	return s
}

func segmentListItemFromDTO(ctx context.Context, dto *segments.Segment) (segmentsListItemModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	item := segmentsListItemModel{
		Active:             types.BoolNull(),
		Created:            types.StringNull(),
		Description:        types.StringNull(),
		Id:                 types.StringNull(),
		Modified:           types.StringNull(),
		Name:               types.StringNull(),
		Owner:              types.ObjectNull(datasource_segments.OwnerValue{}.AttributeTypes(ctx)),
		VisibilityCriteria: types.ObjectNull(visibilityCriteriaAttrTypes()),
	}

	if dto == nil {
		return item, diags
	}
	if dto.Id != nil {
		item.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		item.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		item.Description = types.StringValue(*dto.Description)
	}
	if dto.Active != nil {
		item.Active = types.BoolValue(*dto.Active)
	}
	item.Created = timeToStringValue(dto.Created)
	item.Modified = timeToStringValue(dto.Modified)

	ownerObj, d := pluralSegmentOwnerObjectFromAPI(ctx, dto.Owner.Get())
	diags.Append(d...)
	item.Owner = ownerObj

	vc, d := visibilityCriteriaObjectFromAPI(ctx, dto.VisibilityCriteria)
	diags.Append(d...)
	item.VisibilityCriteria = vc

	return item, diags
}

func pluralSegmentOwnerObjectFromAPI(ctx context.Context, owner *segments.OwnerReferenceSegments) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if owner == nil {
		return types.ObjectNull(datasource_segments.OwnerValue{}.AttributeTypes(ctx)), diags
	}

	ownerValue, d := datasource_segments.NewOwnerValue(
		datasource_segments.OwnerValue{}.AttributeTypes(ctx),
		map[string]attr.Value{
			"id":   types.StringPointerValue(owner.Id),
			"name": types.StringPointerValue(owner.Name),
			"type": types.StringPointerValue(owner.Type),
		},
	)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(datasource_segments.OwnerValue{}.AttributeTypes(ctx)), diags
	}

	obj, d := ownerValue.ToObjectValue(ctx)
	diags.Append(d...)
	return obj, diags
}
