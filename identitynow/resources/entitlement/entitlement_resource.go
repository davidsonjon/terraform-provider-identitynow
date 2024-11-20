package entitlement

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/wI2L/jsondiff"
)

var _ resource.Resource = &EntitlementResource{}
var _ resource.ResourceWithImportState = &EntitlementResource{}

func NewEntitlementResource() resource.Resource {
	return &EntitlementResource{}
}

type EntitlementResource struct {
	client *sailpoint.APIClient
}

func (r *EntitlementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement"
}

func (r *EntitlementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlement resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The entitlement id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The entitlement name",
			},
			"created": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the entitlement was created",
			},
			"modified": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the entitlement was last modified",
			},
			"attribute": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The entitlement attribute name",
			},
			"value": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The value of the entitlement",
			},
			"source_schema_object_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The object type of the entitlement from the source schema",
			},
			"privileged": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "True if the entitlement is privileged",
			},
			"cloud_governed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is cloud governed",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the entitlement",
			},
			"requestable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "True if the entitlement is requestable",
			},
			"source_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The Source ID of the entitlement",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Identity id",
					},
					"name": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
					},
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("IDENTITY"),
						},
						MarkdownDescription: "The type of the Source, will always be `IDENTITY`",
					},
				},
				Optional: true,
				Computed: true,
				Default: objectdefault.StaticValue(
					types.ObjectNull(OwnerSchemeObject),
				),
			},
		},
	}
}

func (r EntitlementResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data Entitlement

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Id.IsNull() && (!data.SourceID.IsNull() || !data.Value.IsNull()) {
		resp.Diagnostics.AddAttributeError(
			path.Root("id"),
			"Conflicting attributes configured id, source_id and value",
			"Expected attribute configurations 1) id or 2) source_id and value to be configured.",
		)
		return
	}

}

func (r *EntitlementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(config.ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected sailpoint.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = config.APIClient
}

func (r *EntitlementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Entitlement

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Id.IsNull() || data.Id.IsUnknown() {
		source, httpResp, err := r.client.V3.SourcesAPI.GetSource(ctx, data.SourceID.ValueString()).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling V3.SourcesApi.GetSource",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling V3.SourcesApi.GetSource",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}

		filters := "source.id eq \"" + *source.Id + "\" and value eq \"" + data.Value.ValueString() + "\""

		entitlements, httpResp, err := r.client.Beta.EntitlementsAPI.ListEntitlements(ctx).Filters(filters).Limit(1).Execute()
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

		switch len(entitlements) {
		case 1:
			data.Id = types.StringPointerValue(entitlements[0].Id)
		default:
			resp.Diagnostics.AddError(
				"Couldn't Find Entitlement",
				fmt.Sprintf("Could not find value: %s in source:%v", data.Value.ValueString(), *source.Id),
			)
			return
		}

	}

	entitlement, httpResp, err := r.client.Beta.EntitlementsAPI.GetEntitlement(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlement",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlement",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	if entitlement == nil {
		resp.Diagnostics.AddError(
			"No entitlement found",
			fmt.Sprintf("Unexpected error retrieving SQL users: %s", err),
		)
		return
	}

	jsonPatchOperation := []beta.JsonPatchOperation{} // []JsonPatchOperation |

	newEntitlement := *entitlement
	newEntitlement.SetPrivileged(data.Privileged.ValueBool())
	newEntitlement.SetRequestable(data.Requestable.ValueBool())

	if data.Owner != nil {
		owner := beta.EntitlementOwner{
			Id:   data.Owner.Id.ValueStringPointer(),
			Name: data.Owner.Name.ValueStringPointer(),
			Type: data.Owner.Type.ValueStringPointer(),
		}
		newEntitlement.SetOwner(owner)
	} else {
		newEntitlement.Owner = nil
	}

	patch, err := jsondiff.Compare(entitlement, newEntitlement)
	if err != nil {
		// handle error
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
			// handle error
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)
		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	if patch != nil {
		patchEnt, httpResp, err := r.client.Beta.EntitlementsAPI.PatchEntitlement(ctx, data.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.EntitlementsAPI.PatchEntitlement",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.EntitlementsAPI.PatchEntitlement",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}
		parseAttributes(&data, patchEnt, &resp.Diagnostics)
	} else {
		parseAttributes(&data, entitlement, &resp.Diagnostics)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EntitlementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Entitlement

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entitlement, httpResp, err := r.client.Beta.EntitlementsAPI.GetEntitlement(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlement",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsAPI.GetEntitlement",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	parseAttributes(&data, entitlement, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EntitlementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, update Entitlement
	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &update)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	planEnt := convertEntitlementBeta(&plan)
	stateEnt := convertEntitlementBeta(&state)

	if update.Owner != nil {
		owner := beta.EntitlementOwner{
			Id:   update.Owner.Id.ValueStringPointer(),
			Name: update.Owner.Name.ValueStringPointer(),
			Type: update.Owner.Type.ValueStringPointer(),
		}
		stateEnt.SetOwner(owner)
	} else {
		stateEnt.Owner = nil
	}

	jsonPatchOperation := []beta.JsonPatchOperation{} // []JsonPatchOperation |

	patch, err := jsondiff.Compare(stateEnt, planEnt)
	if err != nil {
		// handle error
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
			// handle error
			resp.Diagnostics.AddError(
				"Error when calling Marshalling patch operation",
				fmt.Sprintf("Error: %v, see debug info for more information", err),
			)

			return
		}
		patch.UnmarshalJSON(op)

		jsonPatchOperation = append(jsonPatchOperation, patch)
	}

	if patch != nil {
		patchEnt, httpResp, err := r.client.Beta.EntitlementsAPI.PatchEntitlement(ctx, state.Id.ValueString()).JsonPatchOperation(jsonPatchOperation).Execute()
		if err != nil {
			sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
			if isSailpointError {
				resp.Diagnostics.AddError(
					"Error when calling Beta.EntitlementsAPI.PatchEntitlement",
					fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
				)
			} else {
				resp.Diagnostics.AddError(
					"Error when calling Beta.EntitlementsAPI.PatchEntitlement",
					fmt.Sprintf("Error: %s, see debug info for more information", err),
				)
			}
			tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
			return
		}
		parseAttributes(&plan, patchEnt, &resp.Diagnostics)
	} else {
		parseAttributes(&plan, planEnt, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &update)...)
}

func (r *EntitlementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Entitlement

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *EntitlementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
