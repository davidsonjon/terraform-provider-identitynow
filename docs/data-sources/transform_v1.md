---
page_title: "identitynow_transform_v1 Data Source - identitynow"
subcategory: "Transforms"
description: |-
  Reads a Transform https://developer.sailpoint.com/docs/extensibility/transforms/ from IdentityNow/ISC by id.
  ~> This is a _v1 pilot data source - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.
  Working with "attributes"
  attributes is a raw JSON string (via jsontypes.Normalized) because its shape is a discriminated union keyed by type - each of the ~35 supported type values expects different sub-properties, and several (e.g. lower, concat, lookup, replaceAll) can nest another full transform definition arbitrarily deep via an input sub-property (implicit input if omitted, explicit input if a nested {"type": ..., "attributes": {...}} object is supplied). There is no hard nesting limit, though SailPoint's own guidance cautions that deeply nested transforms become harder to read/maintain, and the whole transform document cannot exceed 400KB. A real 3-level nested example observed in a live tenant: a lower transform whose input was a concat transform, whose input was itself a lookup transform keyed off two identityAttribute values - i.e. lower(lookup(concat(identityAttribute, identityAttribute))).
  Only the root-level transform's name is meaningful - nested transform objects passed via attributes.input/attributes.values do not have (and should not be given) their own name.
  type string caveats
  Validate the literal type string against this schema's stringvalidator.OneOf(...) list rather than against SailPoint's human-readable operation names in the UI/docs - a handful of documented operation names (e.g. "Join", "Get End of String", "Generate Random String") do not have a distinct, matching type enum value in the current v1 API; some are instead reachable as a specific attributes.operation value on a type = "rule" transform (confirmed live: a type = "rule" transform with attributes.operation = "getReferenceIdentityAttribute") rather than being their own top-level type.
  internal (SailPoint-managed) transforms
  Transforms with internal = true (SailPoint-managed built-ins, e.g. ToUpper/Remove Diacritical Marks) were observed live to omit the attributes key entirely from GET responses rather than returning {}. This provider normalizes that omission to an empty JSON object ("{}") in state/data-source output for predictable diffing; these built-ins are not expected to be practitioner-managed via this resource (attempting to Update/Delete one has not been tested and may be rejected by the API).
---

# identitynow_transform_v1 (Data Source)

Reads a [Transform](https://developer.sailpoint.com/docs/extensibility/transforms/) from IdentityNow/ISC by `id`.

~> This is a `_v1` pilot data source - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.

### Working with "attributes"

`attributes` is a raw JSON string (via `jsontypes.Normalized`) because its shape is a discriminated union keyed by `type` - each of the ~35 supported `type` values expects different sub-properties, and several (e.g. `lower`, `concat`, `lookup`, `replaceAll`) can nest another full transform definition arbitrarily deep via an `input` sub-property (implicit input if omitted, explicit input if a nested `{"type": ..., "attributes": {...}}` object is supplied). There is no hard nesting limit, though SailPoint's own guidance cautions that deeply nested transforms become harder to read/maintain, and the whole transform document cannot exceed 400KB. A real 3-level nested example observed in a live tenant: a `lower` transform whose `input` was a `concat` transform, whose `input` was itself a `lookup` transform keyed off two `identityAttribute` values - i.e. `lower(lookup(concat(identityAttribute, identityAttribute)))`.

Only the root-level transform's `name` is meaningful - nested transform objects passed via `attributes.input`/`attributes.values` do not have (and should not be given) their own `name`.

### `type` string caveats

Validate the literal `type` string against this schema's `stringvalidator.OneOf(...)` list rather than against SailPoint's human-readable operation names in the UI/docs - a handful of documented operation names (e.g. "Join", "Get End of String", "Generate Random String") do not have a distinct, matching `type` enum value in the current v1 API; some are instead reachable as a specific `attributes.operation` value on a `type = "rule"` transform (confirmed live: a `type = "rule"` transform with `attributes.operation = "getReferenceIdentityAttribute"`) rather than being their own top-level `type`.

### `internal` (SailPoint-managed) transforms

Transforms with `internal = true` (SailPoint-managed built-ins, e.g. `ToUpper`/`Remove Diacritical Marks`) were observed live to omit the `attributes` key entirely from `GET` responses rather than returning `{}`. This provider normalizes that omission to an empty JSON object (`"{}"`) in state/data-source output for predictable diffing; these built-ins are not expected to be practitioner-managed via this resource (attempting to `Update`/`Delete` one has not been tested and may be rejected by the API).

## Example Usage

```terraform
# The data source's "id" is commonly chained from a resource's output so
# Terraform can defer the lookup to apply time (see the resource example),
# but it can also be a known transform id looked up directly, as shown here.
data "identitynow_transform_v1" "example" {
  id = "2c91808576ddc7060176de5040574aa0"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) ID of the transform to retrieve

### Read-Only

- `attributes` (String) Meta-data about the transform, as a raw JSON object. Values are specific to the transform's "type" - see https://developer.sailpoint.com/docs/extensibility/transforms/operations for the shape each "type" expects.
- `internal` (Boolean) Indicates whether this is an internal SailPoint-created transform or a customer-created transform
- `name` (String) Unique name of this transform
- `type` (String) The type of transform operation

## Known Limitations & Live Testing Notes

See the [`identitynow_transform_v1` resource documentation](../resources/transform_v1.md#known-limitations--live-testing-notes)
for the full list of limitations and live-testing findings shared by both the
resource and this data source (both are backed by the same underlying
model/conversion code).
