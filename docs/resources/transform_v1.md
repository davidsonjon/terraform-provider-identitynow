---
page_title: "identitynow_transform_v1 Resource - identitynow"
subcategory: "Transforms"
description: |-
  Manages a Transform https://developer.sailpoint.com/docs/extensibility/transforms/ in IdentityNow/ISC. Transforms manipulate attribute data (e.g. from a source account or identity) without requiring custom rule code.
  ~> This is a _v1 pilot resource - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.
  Working with "attributes"
  attributes is a raw JSON string (via jsontypes.Normalized) because its shape is a discriminated union keyed by type - each of the ~35 supported type values expects different sub-properties, and several (e.g. lower, concat, lookup, replaceAll) can nest another full transform definition arbitrarily deep via an input sub-property (implicit input if omitted, explicit input if a nested {"type": ..., "attributes": {...}} object is supplied). There is no hard nesting limit, though SailPoint's own guidance cautions that deeply nested transforms become harder to read/maintain, and the whole transform document cannot exceed 400KB. A real 3-level nested example observed in a live tenant: a lower transform whose input was a concat transform, whose input was itself a lookup transform keyed off two identityAttribute values - i.e. lower(lookup(concat(identityAttribute, identityAttribute))).
  Only the root-level transform's name is meaningful - nested transform objects passed via attributes.input/attributes.values do not have (and should not be given) their own name.
  type string caveats
  Validate the literal type string against this schema's stringvalidator.OneOf(...) list rather than against SailPoint's human-readable operation names in the UI/docs - a handful of documented operation names (e.g. "Join", "Get End of String", "Generate Random String") do not have a distinct, matching type enum value in the current v1 API; some are instead reachable as a specific attributes.operation value on a type = "rule" transform (confirmed live: a type = "rule" transform with attributes.operation = "getReferenceIdentityAttribute") rather than being their own top-level type.
  internal (SailPoint-managed) transforms
  Transforms with internal = true (SailPoint-managed built-ins, e.g. ToUpper/Remove Diacritical Marks) were observed live to omit the attributes key entirely from GET responses rather than returning {}. This provider normalizes that omission to an empty JSON object ("{}") in state/data-source output for predictable diffing; these built-ins are not expected to be practitioner-managed via this resource (attempting to Update/Delete one has not been tested and may be rejected by the API).
---

# identitynow_transform_v1 (Resource)

Manages a [Transform](https://developer.sailpoint.com/docs/extensibility/transforms/) in IdentityNow/ISC. Transforms manipulate attribute data (e.g. from a source account or identity) without requiring custom rule code.

~> This is a `_v1` pilot resource - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.

### Working with "attributes"

`attributes` is a raw JSON string (via `jsontypes.Normalized`) because its shape is a discriminated union keyed by `type` - each of the ~35 supported `type` values expects different sub-properties, and several (e.g. `lower`, `concat`, `lookup`, `replaceAll`) can nest another full transform definition arbitrarily deep via an `input` sub-property (implicit input if omitted, explicit input if a nested `{"type": ..., "attributes": {...}}` object is supplied). There is no hard nesting limit, though SailPoint's own guidance cautions that deeply nested transforms become harder to read/maintain, and the whole transform document cannot exceed 400KB. A real 3-level nested example observed in a live tenant: a `lower` transform whose `input` was a `concat` transform, whose `input` was itself a `lookup` transform keyed off two `identityAttribute` values - i.e. `lower(lookup(concat(identityAttribute, identityAttribute)))`.

Only the root-level transform's `name` is meaningful - nested transform objects passed via `attributes.input`/`attributes.values` do not have (and should not be given) their own `name`.

### `type` string caveats

Validate the literal `type` string against this schema's `stringvalidator.OneOf(...)` list rather than against SailPoint's human-readable operation names in the UI/docs - a handful of documented operation names (e.g. "Join", "Get End of String", "Generate Random String") do not have a distinct, matching `type` enum value in the current v1 API; some are instead reachable as a specific `attributes.operation` value on a `type = "rule"` transform (confirmed live: a `type = "rule"` transform with `attributes.operation = "getReferenceIdentityAttribute"`) rather than being their own top-level `type`.

### `internal` (SailPoint-managed) transforms

Transforms with `internal = true` (SailPoint-managed built-ins, e.g. `ToUpper`/`Remove Diacritical Marks`) were observed live to omit the `attributes` key entirely from `GET` responses rather than returning `{}`. This provider normalizes that omission to an empty JSON object (`"{}"`) in state/data-source output for predictable diffing; these built-ins are not expected to be practitioner-managed via this resource (attempting to `Update`/`Delete` one has not been tested and may be rejected by the API).

## Example Usage

```terraform
# A simple, single-level transform. "attributes" is a raw JSON string - its
# shape depends entirely on "type" (see the resource documentation for the
# full list of supported types and a link to SailPoint's Transform Operations
# reference).
resource "identitynow_transform_v1" "lower_department" {
  name = "Lowercase Department"
  type = "lower"

  # No "input" key below means an *implicit* input: the system supplies
  # whatever value is mapped to this transform (e.g. from an identity
  # profile's attribute mapping).
  attributes = jsonencode({})
}

# A nested/explicit-input example: a "lower" transform whose input is itself
# a "concat" transform joining two identity attributes. Nested transform
# objects (anything passed via "input"/"values") must NOT have their own
# "name" - only the root-level transform is named.
resource "identitynow_transform_v1" "lower_concat_example" {
  name = "Lowercase Concat Example"
  type = "lower"

  attributes = jsonencode({
    input = {
      type = "concat"
      attributes = {
        values = [
          { type = "identityAttribute", attributes = { name = "firstname" } },
          "-",
          { type = "identityAttribute", attributes = { name = "lastname" } },
        ]
      }
    }
  })
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `attributes` (String) Meta-data about the transform, as a raw JSON object. Values are specific to the transform's "type" - see https://developer.sailpoint.com/docs/extensibility/transforms/operations for the shape each "type" expects.
- `name` (String) Unique name of this transform
- `type` (String) The type of transform operation

### Read-Only

- `id` (String) Unique ID of this transform
- `internal` (Boolean) Indicates whether this is an internal SailPoint-created transform or a customer-created transform

## Known Limitations & Live Testing Notes

This resource is a `_v1` pilot: schema/model types are generated by
`tfplugingen-framework` from SailPoint's per-service `/transforms/v1` OpenAPI
spec, but Create/Read/Update/Delete are hand-written against the
`golang-sdk`'s `api_beta.TransformsAPI` client. The following was learned
from a scripted spec fix, a full live create/update/destroy cycle, and a
read-only sample of 50 real transforms pulled from a sandbox tenant - not
just from the spec:

- **The entire response body is wrapped in a top-level `allOf`.** Unlike
  `service_desk_integration_v1` (where `allOf` only affected a nested
  sub-field), `transforms`' create/get/put responses (and each list item)
  wrap the *whole* response schema in a 2-member `allOf` -
  `tfplugingen-openapi` cannot decompose this and silently skips mapping the
  entire response body, not just one field, unless the generator's WARN
  output is scrutinized. This was fixed with a one-time scripted structural
  merge against `api-specs/dereferenced/deref-transforms.v1.yaml`; the fix
  must be re-applied if this spec is ever re-bundled from a newer upstream
  revision.
- **`attributes` is a raw JSON string, not a typed block.** Its shape is a
  discriminated union keyed by `type` across ~35 supported values, several of
  which nest another full transform definition to an arbitrary depth via an
  `input` (or `values`) sub-property. Rather than statically enumerate all
  variants (impractical, and still can't model the recursive depth),
  `attributes` is excluded from codegen (`schema.ignores`) and hand-added as
  a `jsontypes.Normalized` JSON-string `CustomType`, giving semantic (not
  textual) equality so whitespace/key-ordering differences between your
  config and the API's round-tripped response don't produce false diffs.
- **`internal = true` transforms can omit `attributes` entirely.** A live,
  read-only listing found SailPoint-managed built-ins (e.g. `ToUpper`,
  `Remove Diacritical Marks`) returning no `attributes` key at all rather
  than `{}`. This provider normalizes that omission to an empty JSON object
  in state/data-source output. These built-ins are not expected to be
  practitioner-managed via this resource.
- **Only `attributes` is mutable per the API's own documentation** - changing
  `name`/`type` is expected to be rejected by the API. `Update` currently
  sends a full `PUT` replacing the whole document (simplest and correct given
  the above), but does not yet enforce `RequiresReplace()` on `name`/`type`
  at the plan-modifier level - tracked as a follow-up.
- **Confirmed via a full live lifecycle test** (`terraform apply` → `plan`
  showing no changes → `apply` with a changed `attributes` value → `plan`
  showing no changes → `destroy`), all against a real sandbox tenant
  transform of `type = "upper"`.
