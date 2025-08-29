package entitlement

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GetEntitlementSchemaAttributes returns the common schema attributes used by both
// the single entitlement data source and the multiple entitlements data source
func GetEntitlementSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
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
	}
}