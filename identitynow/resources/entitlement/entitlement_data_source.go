package entitlement

import (
	"context"
	"fmt"

	sailpoint "github.com/davidsonjon/golang-sdk/v2"
	beta "github.com/davidsonjon/golang-sdk/v2/api_beta"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/config"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/resources/metadataattribute"
	"github.com/davidsonjon/terraform-provider-identitynow/identitynow/util"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &EntitlementDataSource{}

func NewEntitlementDataSource() datasource.DataSource {
	return &EntitlementDataSource{}
}

type EntitlementDataSource struct {
	client *sailpoint.APIClient
}

func (d *EntitlementDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement"
}

func (d *EntitlementDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Entitlement data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The entitlement id",
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
				Computed:            true,
				MarkdownDescription: "The value of the entitlement",
			},
			"source_schema_object_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The object type of the entitlement from the source schema",
			},
			"privileged": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is privileged",
			},
			"cloud_governed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is cloud governed",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The description of the entitlement, due to API limitations, may be set to an empty string (`\"\"`) but not **null**. Note: this attribute can be initially aggregated in from some sources and will be overwritten if set",
			},
			"requestable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "True if the entitlement is requestable",
			},
			"source_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Source ID of the entitlement",
			},
			"owner": schema.SingleNestedAttribute{
				MarkdownDescription: "The Owner of the entitlement",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Identity id",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Human-readable display name of the owner. It may be left null or omitted in a POST or PATCH. If set, it must match the current value of the owner's display name, otherwise a 400 Bad Request error will result.",
					},
					"type": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The type of the Source, will always be `IDENTITY`",
					},
				},
				Computed: true,
			},
			"access_model_metadata": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Computed:            true,
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
				},
			},
		},
	}
}

func (d *EntitlementDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {

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

func (d *EntitlementDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Entitlement

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	entitlement, httpResp, err := d.client.Beta.EntitlementsAPI.GetEntitlement(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		sailpointError, isSailpointError := util.SailpointErrorFromHTTPBody(httpResp)
		if isSailpointError {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsApi.GetEntitlement",
				fmt.Sprintf("Error: %s", *sailpointError.GetMessages()[0].Text),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error when calling Beta.EntitlementsApi.GetEntitlement",
				fmt.Sprintf("Error: %s, see debug info for more information", err),
			)
		}
		tflog.Info(ctx, fmt.Sprintf("Full HTTP response: %v", httpResp))
		return
	}

	parseAttributes(&data, entitlement, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func parseAttributes(ent *Entitlement, betaEnt *beta.Entitlement, diags *diag.Diagnostics) {
	ent.Id = types.StringPointerValue(betaEnt.Id)
	ent.Name = types.StringPointerValue(betaEnt.Name)
	ent.Created = types.StringValue(betaEnt.Created.String())
	ent.Modified = types.StringValue(betaEnt.Modified.String())
	ent.Attribute = types.StringPointerValue(betaEnt.Attribute.Get())
	ent.Value = types.StringPointerValue(betaEnt.Value)
	ent.SourceSchemaObjectType = types.StringPointerValue(betaEnt.SourceSchemaObjectType)
	ent.Privileged = types.BoolPointerValue(betaEnt.Privileged)
	ent.CloudGoverned = types.BoolPointerValue(betaEnt.CloudGoverned)
	ent.Description = types.StringPointerValue(betaEnt.Description.Get())
	ent.Requestable = types.BoolPointerValue(betaEnt.Requestable)

	ent.SourceID = types.StringPointerValue(betaEnt.Source.Id)

	if betaEnt.Owner != nil {
		owner := &OwnerReference{}
		owner.Id = types.StringPointerValue(betaEnt.Owner.Id)
		owner.Name = types.StringPointerValue(betaEnt.Owner.Name)
		owner.Type = types.StringPointerValue(betaEnt.Owner.Type)

		ent.Owner = owner
	}

	if len(betaEnt.AccessModelMetadata.Attributes) > 0 {
		metadata := []metadataattribute.AttributeDTO{}

		for _, att := range betaEnt.AccessModelMetadata.Attributes {
			metatdataAtts := metadataattribute.AttributeDTO{}
			metatdataAtts.Key = types.StringPointerValue(att.Key)
			metatdataAtts.Name = types.StringPointerValue(att.Name)
			metatdataAtts.Multiselect = types.BoolPointerValue(att.Multiselect)
			metatdataAtts.Status = types.StringPointerValue(att.Status)
			metatdataAtts.Type = types.StringPointerValue(att.Type)
			metatdataAtts.Description = types.StringPointerValue(att.Description)

			objectTypes, diags1 := types.ListValueFrom(context.Background(), types.StringType, att.ObjectTypes)
			metatdataAtts.ObjectTypes = objectTypes
			diags.Append(diags1...)

			for _, v := range att.Values {
				value := &metadataattribute.AttributeValueDTO{
					Value:  types.StringPointerValue(v.Value),
					Name:   types.StringPointerValue(v.Name),
					Status: types.StringPointerValue(v.Status),
				}
				metatdataAtts.Values = append(metatdataAtts.Values, *value)

			}
			metadata = append(metadata, metatdataAtts)

		}

		ent.AccessModelMetadata = metadata
	}

}
