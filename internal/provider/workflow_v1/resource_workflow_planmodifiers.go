package workflow_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const workflowTriggerDescription = "The trigger that starts the workflow. \"attributes\" is a raw JSON object whose " +
	"shape depends on \"type\" - see the resource/data source's top-level description for the shape each trigger " +
	"\"type\" expects."

// basetypesObjectAsOptions returns the (currently zero-value) options used
// whenever a hand-written "trigger" types.Object is decoded via .As(...) -
// factored into a helper purely so every call site stays consistent if a
// future need arises to set UnhandledNullAsEmpty/UnhandledUnknownAsEmpty.
func basetypesObjectAsOptions() basetypes.ObjectAsOptions {
	return basetypes.ObjectAsOptions{}
}

// applyWorkflowTriggerField hand-adds the "trigger" nested block that
// generator_config_workflow_v1.yml deliberately tells tfplugingen-openapi/
// -framework to ignore in full (see the package doc for why the whole
// block, not just "attributes", had to be excluded and hand-written). The
// data source's Computed-only equivalent is
// applyWorkflowDataSourceTriggerField below.
func applyWorkflowTriggerField(attrs *map[string]resourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]resourceschema.Attribute{}
	}
	(*attrs)["trigger"] = resourceschema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		Description:         workflowTriggerDescription,
		MarkdownDescription: workflowTriggerDescription,
		Attributes: map[string]resourceschema.Attribute{
			// "type"/"attributes" are Required whenever a trigger block is
			// present at all - both are `required` in the API's own
			// WorkflowTrigger schema - but the "trigger" block itself
			// stays Optional (a workflow may have no trigger configured
			// yet).
			"type": resourceschema.StringAttribute{
				Required:            true,
				Description:         "The trigger type (EVENT, EXTERNAL, or SCHEDULED).",
				MarkdownDescription: "The trigger type (`EVENT`, `EXTERNAL`, or `SCHEDULED`).",
			},
			"display_name": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "The trigger display name.",
				MarkdownDescription: "The trigger display name.",
			},
			"attributes": resourceschema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Required:            true,
				Description:         "Workflow trigger attributes, as a raw JSON object whose shape depends on \"type\".",
				MarkdownDescription: "Workflow trigger attributes, as a raw JSON object whose shape depends on `type`.",
			},
		},
	}
}

// applyWorkflowDataSourceTriggerField is the data source ("Computed"-only)
// equivalent of applyWorkflowTriggerField.
func applyWorkflowDataSourceTriggerField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	(*attrs)["trigger"] = datasourceschema.SingleNestedAttribute{
		Computed:            true,
		Description:         workflowTriggerDescription,
		MarkdownDescription: workflowTriggerDescription,
		Attributes: map[string]datasourceschema.Attribute{
			"type": datasourceschema.StringAttribute{
				Computed:            true,
				Description:         "The trigger type (EVENT, EXTERNAL, or SCHEDULED).",
				MarkdownDescription: "The trigger type (`EVENT`, `EXTERNAL`, or `SCHEDULED`).",
			},
			"display_name": datasourceschema.StringAttribute{
				Computed:            true,
				Description:         "The trigger display name.",
				MarkdownDescription: "The trigger display name.",
			},
			"attributes": datasourceschema.StringAttribute{
				CustomType:          jsontypes.NormalizedType{},
				Computed:            true,
				Description:         "Workflow trigger attributes, as a raw JSON object whose shape depends on \"type\".",
				MarkdownDescription: "Workflow trigger attributes, as a raw JSON object whose shape depends on `type`.",
			},
		},
	}
}

// applyWorkflowDataSourceDefinitionField is the data source ("Computed"-only)
// equivalent of applyWorkflowDefinitionField.
func applyWorkflowDataSourceDefinitionField(attrs *map[string]datasourceschema.Attribute) {
	if *attrs == nil {
		*attrs = map[string]datasourceschema.Attribute{}
	}
	desc := "The map of steps that the workflow will execute, as a raw JSON object (`{\"start\": \"...\", \"steps\": " +
		"{...}}`). Each step's own shape varies by its \"type\" - see " +
		"https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects."
	(*attrs)["definition"] = datasourceschema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}
