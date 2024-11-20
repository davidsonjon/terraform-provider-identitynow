package segment

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"

	v3 "github.com/davidsonjon/golang-sdk/v2/api_v3"
)

var _ resource.Resource = &SegmentResource{}
var _ resource.ResourceWithImportState = &SegmentResource{}

func NewSegmentResource() resource.Resource {
	return &SegmentResource{}
}

type SegmentResource struct {
	client *sailpoint.APIClient
}

func (r *SegmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment"
}

func (r *SegmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Segment resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The segment id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The segment name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the segment was created",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the segment was last modified",
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The segment attribute name",
			},
			"visibility_criteria": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"operator": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "visibility_criteria operator",
					},
					"attribute": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "visibility_criteria attribute",
					},
					"value": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "visibility_criteria value type",
							},
							"value": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "visibility_criteria value value",
							},
						},
						Optional:            true,
						MarkdownDescription: "visibility_criteria value",
					},
					"children": schema.SetNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"operator": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "children operator",
								},
								"attribute": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "children attribute",
								},
								"value": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "children value type",
										},
										"value": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "children value value",
										},
									},
									Optional:            true,
									MarkdownDescription: "children value",
								},
								"children": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "children - always null",
								},
							},
						},
						Optional:            true,
						MarkdownDescription: "children",
					},
				},
				Required:            true,
				MarkdownDescription: "visibility_criteria",
			},
			"active": schema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "True if the segment is active",
			},
		},
	}
}

func (r *SegmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

	r.client = config.APIClient
}

func (r *SegmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Segment

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	segmentReq := *v3.NewSegment()
	segmentReq.Name = data.Name.ValueStringPointer()
	segmentReq.Description = data.Description.ValueStringPointer()
	segmentReq.Active = data.Active.ValueBoolPointer()
	// segmentReq.Created = "2024-03-04T15:31:17.333172Z"

	segmentReq.VisibilityCriteria = v3.NewSegmentVisibilityCriteriaWithDefaults()
	segmentReq.VisibilityCriteria.Expression = &v3.Expression{
		Operator: data.VisibilityCriteria.Operator.ValueStringPointer(),
		Children: []v3.ExpressionChildrenInner{},
	}

	if segmentReq.VisibilityCriteria.Expression.Value.IsSet() {
		segmentReq.VisibilityCriteria.Expression.Attribute = *v3.NewNullableString(data.VisibilityCriteria.Attribute.ValueStringPointer())
	}

	if segmentReq.VisibilityCriteria.Expression.Value.IsSet() {
		segVisCritValue := v3.NewNullableValue(&v3.Value{Type: data.VisibilityCriteria.Value.Type.ValueStringPointer(),
			Value: data.VisibilityCriteria.Value.Value.ValueStringPointer()})
		segmentReq.VisibilityCriteria.Expression.Value = *segVisCritValue
	}

	for _, c := range data.VisibilityCriteria.Children {
		childValue := v3.NewNullableValue(&v3.Value{Type: c.Value.Type.ValueStringPointer(),
			Value: c.Value.Value.ValueStringPointer()})
		child := v3.ExpressionChildrenInner{
			Operator:  c.Operator.ValueStringPointer(),
			Attribute: *v3.NewNullableString(c.Attribute.ValueStringPointer()),
			Value:     *childValue,
			Children:  v3.NullableString{},
		}

		segmentReq.VisibilityCriteria.Expression.Children = append(segmentReq.VisibilityCriteria.Expression.Children, child)
	}

	segment, httpResp, err := r.client.V3.SegmentsAPI.CreateSegment(context.Background()).Segment(segmentReq).Execute()
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Segment

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	segment, httpResp, err := r.client.V3.SegmentsAPI.GetSegment(ctx, data.Id.ValueString()).Execute()
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update Segment
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planAp := convertSegmentV3(&plan)
	stateAp := convertSegmentV3(&state)

	patch, err := jsondiff.Compare(stateAp, planAp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	tflog.Info(ctx, fmt.Sprintf("patch: %v", patch))

	var requestBody []map[string]interface{}

	for _, p := range patch {
		requestBody = append(requestBody, map[string]interface{}{
			"op":    p.Type,
			"path":  p.Path,
			"value": p.Value,
		})
	}

	segment, httpResp, err := r.client.V3.SegmentsAPI.PatchSegment(ctx, plan.Id.ValueString()).RequestBody(requestBody).Execute()
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

	parseAttributes(&update, segment, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *SegmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Segment

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.V3.SegmentsAPI.DeleteSegment(ctx, state.Id.ValueString()).Execute()
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

	tflog.Info(ctx, fmt.Sprintf("Full HTTP response DeleteSegment: %v", httpResp))

}

func (r *SegmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
