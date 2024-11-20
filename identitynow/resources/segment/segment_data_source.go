package segment

import (
	"context"
	"fmt"
	"log"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	v3 "github.com/davidsonjon/golang-sdk/v2/api_v3"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &SegmentDataSource{}

func NewSegmentDataSource() datasource.DataSource {
	return &SegmentDataSource{}
}

type SegmentDataSource struct {
	client *sailpoint.APIClient
}

func (d *SegmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment"
}

func (d *SegmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Segment data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The segment id",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The segment name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the segment was created",
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the segment was last modified",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The segment attribute name",
			},
			"visibility_criteria": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"operator": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "visibility_criteria operator",
					},
					"attribute": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "visibility_criteria attribute",
					},
					"value": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "visibility_criteria value type",
							},
							"value": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "visibility_criteria value value",
							},
						},
						Computed:            true,
						MarkdownDescription: "visibility_criteria value",
					},
					"children": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"operator": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "children operator",
								},
								"attribute": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "children attribute",
								},
								"value": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "children value type",
										},
										"value": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "children value value",
										},
									},
									Computed:            true,
									MarkdownDescription: "children value",
								},
								"children": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "children - always null",
								},
							},
						},
						Computed:            true,
						MarkdownDescription: "children",
					},
				},
				Computed:            true,
				MarkdownDescription: "visibility_criteria",
			},
			"active": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the segment is active",
			},
		},
	}
}

func (d *SegmentDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(
			path.MatchRoot("id"),
			path.MatchRoot("name"),
		),
	}
}

func (d *SegmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *SegmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Segment

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Name.IsNull() {
		segments, httpResp, err := d.client.V3.SegmentsAPI.ListSegments(ctx).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling GetApplication",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling GetApplication",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		}

		for _, s := range segments {
			if *s.Name == data.Name.ValueString() {
				data.Id = types.StringPointerValue(s.Id)
			}
		}
	}

	segment, httpResp, err := d.client.V3.SegmentsAPI.GetSegment(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.SegmentsApi.GetSegment",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.SegmentsApi.GetSegment",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	parseAttributes(&data, segment, &resp.Diagnostics)

	tflog.Info(ctx, fmt.Sprintf("data.VisibilityCriteria: %v", data.VisibilityCriteria))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func parseAttributes(seg *Segment, v3Seg *v3.Segment, diags *diag.Diagnostics) {
	seg.Id = types.StringPointerValue(v3Seg.Id)
	seg.Name = types.StringPointerValue(v3Seg.Name)
	seg.Created = types.StringValue(v3Seg.Created.String())
	seg.Modified = types.StringValue(v3Seg.Modified.String())
	seg.Description = types.StringPointerValue(v3Seg.Description)
	seg.Active = types.BoolPointerValue(v3Seg.Active)

	seg.VisibilityCriteria = &VisibilityCriteria{
		Operator:  types.StringPointerValue(v3Seg.VisibilityCriteria.Expression.Operator),
		Attribute: types.StringPointerValue(v3Seg.VisibilityCriteria.Expression.Attribute.Get()),
	}

	expression := v3Seg.VisibilityCriteria.GetExpression()

	if expression.Value.IsSet() {
		seg.VisibilityCriteria.Value = &Value{
			Type:  types.StringPointerValue(expression.GetValue().Type),
			Value: types.StringPointerValue(expression.GetValue().Value),
		}
	} else {
		seg.VisibilityCriteria.Value = nil
	}

	for _, c := range expression.Children {
		log.Printf("c.Operator:%v", c.Operator)

		seg.VisibilityCriteria.Children = append(seg.VisibilityCriteria.Children, VisibilityCriteria{
			Operator:  types.StringValue(c.GetOperator()),
			Attribute: types.StringPointerValue(c.Attribute.Get()),
			Value: &Value{
				Type:  types.StringPointerValue(c.Value.Get().Type),
				Value: types.StringPointerValue(c.Value.Get().Value),
			},
		})
	}

	// if v3Seg.Owner != nil {
	// 	if v3Seg.Owner.Id != nil {
	// 		seg.OwnerID = types.StringPointerValue(v3Seg.Owner.Id)
	// 	}
	// } else {
	// 	seg.Owner = types.StringNull()
	// }

}
