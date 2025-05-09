package metadataattribute

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"
)

var _ resource.Resource = &MetadataAttributeResource{}
var _ resource.ResourceWithImportState = &MetadataAttributeResource{}

func NewMetadataAttributeResource() resource.Resource {
	return &MetadataAttributeResource{}
}

type MetadataAttributeResource struct {
	client *sailpoint.APIClient
}

func (r *MetadataAttributeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_model_metadata"
}

func (r *MetadataAttributeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Access Profile resource",

		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Technical name of the Attribute. This is unique and cannot be changed after creation.",
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				MarkdownDescription: "The display name of the key.",
			},
			"multiselect": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Indicates whether the attribute can have multiple values.",
			},
			"status": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The status of the Attribute.",
			},
			"type": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The type of the Attribute. This can be either `custom` or `governance`.",
			},
			"object_types": schema.ListAttribute{
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
					listvalidator.ValueStringsAre(stringvalidator.OneOf("entitlement", "role", "general")),
				},
				Required:            true,
				MarkdownDescription: "An array of object types this attributes values can be applied to. Possible values are `all` or `entitlement`. Value `all` means this attribute can be used with all object types that are supported.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The description of the Attribute.",
			},
			"values": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Technical name of the Attribute value. This is unique and cannot be changed after creation.",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The display name of the Attribute value.",
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The status of the Attribute value.",
						},
					},
				},
				Optional: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *MetadataAttributeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

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

func (r *MetadataAttributeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AttributeDTO

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	objectType := make([]types.String, 0, len(data.ObjectTypes.Elements()))
	diags := data.ObjectTypes.ElementsAs(ctx, &objectType, false)
	resp.Diagnostics.Append(diags...)

	key := data.Name.ValueString()

	switch objectType[0].ValueString() {
	case "entitlement":
		key = "ent" + key
	case "role":
		key = "role" + key
	}

	attribute := beta.AttributeDTO{}
	attribute.Key = &key
	attribute.Name = data.Name.ValueStringPointer()
	attribute.Status = beta.PtrString("active")
	attribute.Type = beta.PtrString("custom")
	objectTypes := []string{"entitlement"}
	attribute.ObjectTypes = objectTypes
	if data.Description.IsNull() {
		attribute.Description = beta.PtrString("")
	} else {
		attribute.Description = data.Description.ValueStringPointer()
	}
	attribute.Multiselect = data.Multiselect.ValueBoolPointer()
	values := []beta.AttributeValueDTO{}
	for _, v := range data.Values {
		values = append(values, beta.AttributeValueDTO{
			Name:   v.Name.ValueStringPointer(),
			Value:  v.Value.ValueStringPointer(),
			Status: beta.PtrString("active"),
		})
	}
	attribute.Values = values

	attributeResult, httpResp, err := r.client.Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttribute(context.Background(), &attribute).Execute()
	if err != nil {
		tflog.Info(ctx, fmt.Sprintf("err: %v", err))
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	data.Key = types.StringPointerValue(attributeResult.Key)
	data.Name = types.StringPointerValue(attributeResult.Name)
	data.Description = types.StringPointerValue(attributeResult.Description)
	data.Multiselect = types.BoolPointerValue(attributeResult.Multiselect)
	data.Status = types.StringPointerValue(attributeResult.Status)
	data.Type = types.StringPointerValue(attributeResult.Type)
	dataValues := []AttributeValueDTO{}
	for _, v := range attributeResult.Values {
		value := AttributeValueDTO{
			Value:  types.StringPointerValue(v.Value),
			Name:   types.StringPointerValue(v.Name),
			Status: types.StringPointerValue(v.Status),
		}
		dataValues = append(dataValues, value)
	}
	data.Values = dataValues

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MetadataAttributeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AttributeDTO

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	key := data.Key.ValueString()

	metadata, httpResp, err := r.client.Beta.AccessModelMetadataAPI.GetAccessModelMetadataAttribute(context.Background(), key).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.GetAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling V3.AccessProfilesAPI.GetAccessModelMetadataAttribute",
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
	if metadata.Description == nil {
		data.Description = types.StringValue("")
	} else {
		data.Description = types.StringPointerValue(metadata.Description)
	}

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
		if *v.Status == "active" {
			attributeValues = append(attributeValues, value)
		}
	}
	data.Values = attributeValues
	// data.ObjectTypes = types.StringSlicePointerValue(metadata.ObjectTypes)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MetadataAttributeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update AttributeDTO
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planMap := map[string]beta.AttributeValueDTO{}
	for _, v := range plan.Values {
		planMap[v.Value.ValueString()] = beta.AttributeValueDTO{
			Name:   v.Name.ValueStringPointer(),
			Value:  v.Value.ValueStringPointer(),
			Status: beta.PtrString("active"),
		}
	}

	stateMap := map[string]beta.AttributeValueDTO{}
	for _, v := range state.Values {
		stateMap[v.Value.ValueString()] = beta.AttributeValueDTO{
			Name:   v.Name.ValueStringPointer(),
			Value:  v.Value.ValueStringPointer(),
			Status: v.Status.ValueStringPointer(),
		}
	}

	for k := range stateMap {
		if _, ok := planMap[k]; !ok {
			tflog.Info(ctx, fmt.Sprintf("Deleting AttributeValue of:%v", k))
			value := k // string | Value of the Attribute.
			if *stateMap[k].Status == "active" {
				httpResp, err := r.client.Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttributeValue(context.Background(), state.Key.ValueString(), value).Execute()
				if err != nil {
					sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
					if isSailpointError {
						resp.Diagnostics.AddError(
							"Error when calling Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttributeValue",
							fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
						)
					} else {
						resp.Diagnostics.AddError(
							"Error when calling Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttributeValue",
							fmt.Sprintf("Error: %s, see debug info for more information", err),
						)
					}

					tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
					return
				}
			}
		}
	}

	for k, v := range planMap {
		if _, ok := stateMap[k]; !ok {
			tflog.Info(ctx, fmt.Sprintf("Creating AttributeValue of:%v", k))
			key := state.Key.ValueString()
			value := v

			_, httpResp, err := r.client.Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttributeValue(context.Background(), key, value).Execute()
			if err != nil {
				sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
				if isSailpointError {
					resp.Diagnostics.AddError(
						"Error when calling Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttributeValue",
						fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
					)
				} else {
					resp.Diagnostics.AddError(
						"Error when calling Beta.AccessModelMetadataAPI.CreateAccessModelMetadataAttributeValue",
						fmt.Sprintf("Error: %s, see debug info for more information", err),
					)
				}

				tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
				return
			}
		}
	}

	planAp := convertAttribute(&plan, &resp.Diagnostics)
	stateAp := convertAttribute(&state, &resp.Diagnostics)

	jsonPatchOperation := []beta.JsonPatchOperation{}

	patch, err := jsondiff.Compare(stateAp, planAp, jsondiff.Ignores("/values",
		"/status"))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when calling PatchAccessProfile",
			fmt.Sprintf("Error: %v, see debug info for more information", err),
		)

		return
	}

	for _, p := range patch {
		patch := *beta.NewJsonPatchOperationWithDefaults()

		op, err := p.MarshalJSON()
		if err != nil {
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	attributeResult, httpResp, err := r.client.Beta.AccessModelMetadataAPI.PatchAccessModelMetadataAttribute(context.Background(), state.Key.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.PatchAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.PatchAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))

		return
	}

	update.Key = types.StringPointerValue(attributeResult.Key)
	update.Name = types.StringPointerValue(attributeResult.Name)
	update.Description = types.StringPointerValue(attributeResult.Description)
	update.Multiselect = types.BoolPointerValue(attributeResult.Multiselect)
	update.Status = types.StringPointerValue(attributeResult.Status)
	update.Type = types.StringPointerValue(attributeResult.Type)
	dataValues := []AttributeValueDTO{}
	for _, v := range attributeResult.Values {
		value := AttributeValueDTO{
			Value:  types.StringPointerValue(v.Value),
			Name:   types.StringPointerValue(v.Name),
			Status: types.StringPointerValue(v.Status),
		}
		if *v.Status == "active" {
			dataValues = append(dataValues, value)
		}
	}
	update.Values = dataValues

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *MetadataAttributeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AttributeDTO

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttribute(context.Background(), state.Key.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.AccessModelMetadataAPI.DeleteAccessModelMetadataAttribute",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}

		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

}

func (r *MetadataAttributeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
