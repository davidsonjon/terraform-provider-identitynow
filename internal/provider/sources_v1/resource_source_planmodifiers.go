package sources_v1

import (
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// applySourceConnectorAttributesField hand-adds the "connector_attributes"
// attribute that generator_config_sources_v1.yml deliberately tells
// tfplugingen-openapi/-framework to ignore (see the package doc for the full
// dynamic-shape rationale, identical to transform_v1's "attributes"). It is a
// plain schema.StringAttribute using jsontypes.NormalizedType{} as its
// CustomType, so practitioners write raw JSON matching whatever shape their
// source's connector "type" requires, and drift detection/equality is
// semantic (not textual) JSON comparison rather than a byte-for-byte string
// compare.
//
// Unlike transform_v1's Required "attributes", "connectorAttributes" is
// genuinely Optional in the Source schema - required=false always (both the
// resource and data source get an Optional+Computed / Computed-only copy
// respectively).
func applySourceConnectorAttributesField(attrs *map[string]schema.Attribute, computedOnly bool) {
	if *attrs == nil {
		*attrs = map[string]schema.Attribute{}
	}
	desc := "Connector-specific configuration. This configuration differs from connector type to connector type " +
		"(e.g. Active Directory vs. Workday vs. a Delimited File source each expect different sub-keys) - represented " +
		"as a raw JSON object via `jsontypes.Normalized` rather than a fixed schema, since tfplugingen-openapi/-framework " +
		"cannot generate a faithful static schema for a connector-type-discriminated union."
	if computedOnly {
		(*attrs)["connector_attributes"] = schema.StringAttribute{
			CustomType:          jsontypes.NormalizedType{},
			Computed:            true,
			Description:         desc,
			MarkdownDescription: desc,
		}
		return
	}
	(*attrs)["connector_attributes"] = schema.StringAttribute{
		CustomType:          jsontypes.NormalizedType{},
		Optional:            true,
		Computed:            true,
		Description:         desc,
		MarkdownDescription: desc,
	}
}

// applySourceUseStateForUnknown patches resource_source.SourceResourceSchema's
// generated Attributes map with stringplanmodifier.UseStateForUnknown() on a
// deliberately narrow set of scalar attributes, working around the same
// systemic tfplugingen-framework pipeline gap documented for every other
// _v1 pilot target (see the identitynow-terraform-provider-developer
// knowledge file's 2026-07-24 entries): tfplugingen-framework never emits
// any PlanModifiers, so every unconfigured Computed attribute is proposed as
// Unknown on every plan (not just when it actually changes), producing a
// permanent, spurious "(known after apply)" diff.
//
// Scope is intentionally narrow - only plain schema.StringAttribute leaves
// that are genuinely server-managed and never written by modelToDto, and
// whose value the live API does not spontaneously change on an unrelated
// Update (see role_v1's contrasting "modified" caveat, and
// governance_group_v1's "modified" caveat, for why a naive blanket
// UseStateForUnknown across every Computed-only field would be wrong here
// too):
//   - "id": server-assigned on Create, never configurable - safe.
//   - "created": server-assigned, never configurable - safe.
//
// Deliberately EXCLUDED (left showing "(known after apply)" until a real
// Read/Update populates them, matching the established convention):
//   - "modified", "connector_id", "connector_name", "connection_type",
//     "connector_implementation_id", "healthy", "status", "since" are all
//     genuinely volatile / refreshed unconditionally from the live API on
//     every Read (health/status/connector metadata can change outside
//     Terraform's own control, e.g. via a scheduled aggregation or a
//     connector-side change).
//   - "schemas"/"password_policies" are managed by their own out-of-scope
//     sub-resource endpoints (see package doc) - genuinely expected to
//     change outside this resource's own CRUD.
func applySourceUseStateForUnknown(s *schema.Schema) {
	patchString(s, "id")
	patchString(s, "created")
}

func patchString(s *schema.Schema, name string) {
	a, ok := s.Attributes[name].(schema.StringAttribute)
	if !ok {
		return
	}
	a.PlanModifiers = append(a.PlanModifiers, stringplanmodifier.UseStateForUnknown())
	s.Attributes[name] = a
}
