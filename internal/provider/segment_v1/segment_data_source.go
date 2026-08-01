package segment_v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	sailpoint "github.com/sailpoint-oss/golang-sdk/v3"
	"github.com/sailpoint-oss/golang-sdk/v3/segments"

	"terraform-provider-identitynow/internal/provider/segment_v1/datasource_segment"
)

const segmentLookupPageSize int32 = 250

var (
	_ datasource.DataSource                     = (*segmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*segmentDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*segmentDataSource)(nil)
)

type segmentDataSourceModel struct {
	Active             types.Bool                    `tfsdk:"active"`
	Created            types.String                  `tfsdk:"created"`
	Description        types.String                  `tfsdk:"description"`
	Id                 types.String                  `tfsdk:"id"`
	Modified           types.String                  `tfsdk:"modified"`
	Name               types.String                  `tfsdk:"name"`
	Owner              datasource_segment.OwnerValue `tfsdk:"owner"`
	VisibilityCriteria types.Object                  `tfsdk:"visibility_criteria"`
}

func NewSegmentDataSource() datasource.DataSource {
	return &segmentDataSource{}
}

type segmentDataSource struct {
	client *sailpoint.APIClient
}

func (d *segmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment_v1"
}

func (d *segmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = segmentDataSourceSchema(ctx)
	resp.Schema.Description = "Reads a Segment from IdentityNow/ISC by id or exact name."
	resp.Schema.MarkdownDescription = "Reads a Segment from IdentityNow/ISC by `id` or exact `name`. Exactly one of those arguments must be set. Returns the same generated fields as the Phase 1 codegen plus the hand-written `visibility_criteria` tree."
}

func segmentDataSourceSchema(ctx context.Context) datasourceschema.Schema {
	s := datasource_segment.SegmentDataSourceSchema(ctx)

	idAttr := s.Attributes["id"].(datasourceschema.StringAttribute)
	idAttr.Required = false
	idAttr.Optional = true
	idAttr.Computed = true
	idAttr.Description = "The segment ID to retrieve. Exactly one of `id` or `name` must be set."
	idAttr.MarkdownDescription = "The segment ID to retrieve. Exactly one of `id` or `name` must be set."
	s.Attributes["id"] = idAttr

	nameAttr := s.Attributes["name"].(datasourceschema.StringAttribute)
	nameAttr.Optional = true
	nameAttr.Computed = true
	nameAttr.Description = "The segment name to retrieve by exact match when `id` is not set. Exactly one of `id` or `name` must be set."
	nameAttr.MarkdownDescription = "The segment name to retrieve by exact match when `id` is not set. Exactly one of `id` or `name` must be set."
	s.Attributes["name"] = nameAttr

	applyDataSourceVisibilityCriteriaField(&s.Attributes)
	return s
}

func (d *segmentDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *segmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *segmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config segmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dto, diags := d.lookupSegment(ctx, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, diags := segmentDataSourceDTOToModel(ctx, dto, config.Id)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *segmentDataSource) lookupSegment(ctx context.Context, config segmentDataSourceModel) (*segments.Segment, diag.Diagnostics) {
	var diags diag.Diagnostics

	if !config.Id.IsNull() && !config.Id.IsUnknown() && config.Id.ValueString() != "" {
		tflog.Debug(ctx, "Reading Segment data source by id", map[string]interface{}{"id": config.Id.ValueString()})
		dto, httpResp, err := d.client.SegmentsAPI.
			GetSegmentV1(ctx, config.Id.ValueString()).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error reading Segment data source by id", map[string]interface{}{"id": config.Id.ValueString(), "error": err.Error()})
			diags.AddError("Error reading Segment", segmentErrDetail(err, httpResp))
			return nil, diags
		}
		return dto, diags
	}

	lookupName := strings.TrimSpace(config.Name.ValueString())
	tflog.Debug(ctx, "Reading Segment data source by name", map[string]interface{}{"name": lookupName})

	matches := make([]segments.Segment, 0, 2)
	var offset int32
	for {
		items, httpResp, err := d.client.SegmentsAPI.
			ListSegmentsV1(ctx).
			Limit(segmentLookupPageSize).
			Offset(offset).
			Execute()
		if err != nil {
			tflog.Error(ctx, "Error listing Segments for name lookup", map[string]interface{}{"name": lookupName, "error": err.Error()})
			diags.AddError("Error reading Segment by name", segmentErrDetail(err, httpResp))
			return nil, diags
		}

		for i := range items {
			if items[i].Name != nil && *items[i].Name == lookupName {
				matches = append(matches, items[i])
			}
		}

		if len(items) < int(segmentLookupPageSize) {
			break
		}
		offset += segmentLookupPageSize
	}

	switch len(matches) {
	case 0:
		diags.AddError(
			"Segment not found by name",
			fmt.Sprintf("No segment with exact name %q was found. Set `id` instead if the segment name is not unique or has changed.", lookupName),
		)
		return nil, diags
	case 1:
		return &matches[0], diags
	default:
		ids := make([]string, 0, len(matches))
		for i := range matches {
			if matches[i].Id != nil && *matches[i].Id != "" {
				ids = append(ids, *matches[i].Id)
			}
		}
		diags.AddError(
			"Segment name is not unique",
			fmt.Sprintf("Found %d segments with exact name %q. Use `id` instead. Matching segment ids: %s.", len(matches), lookupName, strings.Join(ids, ", ")),
		)
		return nil, diags
	}
}

func segmentDataSourceDTOToModel(ctx context.Context, dto *segments.Segment, fallbackID types.String) (segmentDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := segmentDataSourceModel{
		Active:             types.BoolNull(),
		Created:            types.StringNull(),
		Description:        types.StringNull(),
		Id:                 fallbackID,
		Modified:           types.StringNull(),
		Name:               types.StringNull(),
		Owner:              datasource_segment.NewOwnerValueNull(),
		VisibilityCriteria: types.ObjectNull(visibilityCriteriaAttrTypes()),
	}

	if dto == nil {
		return model, diags
	}
	if dto.Id != nil {
		model.Id = types.StringValue(*dto.Id)
	}
	if dto.Name != nil {
		model.Name = types.StringValue(*dto.Name)
	}
	if dto.Description != nil {
		model.Description = types.StringValue(*dto.Description)
	}
	if dto.Active != nil {
		model.Active = types.BoolValue(*dto.Active)
	}
	model.Created = timeToStringValue(dto.Created)
	model.Modified = timeToStringValue(dto.Modified)

	owner, d := datasource_segment.OwnerValue{}.FromApi_betaOwnerReferenceSegments(ctx, dto.Owner.Get())
	diags.Append(d...)
	model.Owner = owner

	vc, d := visibilityCriteriaObjectFromAPI(ctx, dto.VisibilityCriteria)
	diags.Append(d...)
	model.VisibilityCriteria = vc

	return model, diags
}
