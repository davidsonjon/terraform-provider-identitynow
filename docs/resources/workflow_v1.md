---
page_title: "identitynow_workflow_v1 Resource - identitynow"
subcategory: "Workflows"
description: |-
  Manages a Workflow https://developer.sailpoint.com/docs/extensibility/workflows/ in IdentityNow/ISC. Workflows automate repeatable processes (e.g. sending notifications, calling external systems) in response to an event, schedule, or external trigger.
  ~> This is a _v1 pilot resource - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.
  Known Limitations & Live Testing Notes
  This is a _v1 pilot resource. Only core CRUD (create/read/update/delete a workflow's own configuration) is implemented - workflow execution/testing (POST .../test), execution history (GET .../executions, GET /workflow-executions/v1/...), external-trigger invocation, and the read-only /workflow-library/v1 action/trigger/operator catalogs are all out of scope for this pilot (see the package doc for the full list).enabled workflows cannot be deleted - the live API rejects DELETE on an enabled workflow. Disable a workflow (enabled = false) before destroying it.definition is a raw JSON string ({"start": ..., "steps": {...}}) because each step's shape varies by its own type (action/approval/success/etc.) with genuinely free-form additionalProperties. See https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects.trigger.attributes is likewise a raw JSON string, since its shape depends entirely on the sibling trigger.type (EVENT -> {id, filter.$, description, attributeToFilter, formDefinitionId}, EXTERNAL -> {name, description, clientId, url}, SCHEDULED -> {frequency, timeZone, cronString, weeklyDays, weeklyTimes, yearlyTimes}). See https://developer.sailpoint.com/docs/extensibility/event-triggers/available for event trigger ids.Update uses a full PUT (replacing every mutable field at once) rather than PATCH/JSON-Patch, mirroring transform_v1's identical choice - simpler, and every field workflows expose is mutable via PUT per the API's own docs.Phase B (live terraform plan/apply against a real sandbox tenant) is a pending follow-up for this pilot - see the pipeline task's final report for details.
---

# identitynow_workflow_v1 (Resource)

Manages a [Workflow](https://developer.sailpoint.com/docs/extensibility/workflows/) in IdentityNow/ISC. Workflows automate repeatable processes (e.g. sending notifications, calling external systems) in response to an event, schedule, or external trigger.

~> This is a `_v1` pilot resource - see "Known Limitations & Live Testing Notes" below before relying on it in production configurations.

### Known Limitations & Live Testing Notes

- This is a `_v1` pilot resource. Only core CRUD (create/read/update/delete a workflow's own configuration) is implemented - workflow execution/testing (`POST .../test`), execution history (`GET .../executions`, `GET /workflow-executions/v1/...`), external-trigger invocation, and the read-only `/workflow-library/v1` action/trigger/operator catalogs are all out of scope for this pilot (see the package doc for the full list).
- `enabled` workflows **cannot be deleted** - the live API rejects `DELETE` on an enabled workflow. Disable a workflow (`enabled = false`) before destroying it.
- `definition` is a raw JSON string (`{"start": ..., "steps": {...}}`) because each step's shape varies by its own `type` (action/approval/success/etc.) with genuinely free-form `additionalProperties`. See https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects.
- `trigger.attributes` is likewise a raw JSON string, since its shape depends entirely on the sibling `trigger.type` (`EVENT` -> `{id, filter.$, description, attributeToFilter, formDefinitionId}`, `EXTERNAL` -> `{name, description, clientId, url}`, `SCHEDULED` -> `{frequency, timeZone, cronString, weeklyDays, weeklyTimes, yearlyTimes}`). See https://developer.sailpoint.com/docs/extensibility/event-triggers/available for event trigger ids.
- Update uses a full `PUT` (replacing every mutable field at once) rather than `PATCH`/JSON-Patch, mirroring transform_v1's identical choice - simpler, and every field workflows expose is mutable via `PUT` per the API's own docs.
- Phase B (live `terraform plan`/`apply` against a real sandbox tenant) is a pending follow-up for this pilot - see the pipeline task's final report for details.

## Example Usage

```terraform
# A simple event-triggered workflow that sends an email whenever an
# identity's manager attribute changes. "definition" and "trigger.attributes"
# are raw JSON strings - their shapes depend entirely on the step "type"s
# used in "definition.steps" and the trigger's own "type", respectively (see
# the resource documentation for links to SailPoint's Workflows guide).
resource "identitynow_workflow_v1" "send_email_on_manager_change" {
  name        = "Send Email on Manager Change"
  description = "Send an email to the identity whose manager attribute changed."
  enabled     = false # Workflows cannot be created in an enabled state.

  owner = {
    type = "IDENTITY"
    id   = "2c91808568c529c60168cca6f90c1313"
  }

  trigger = {
    type = "EVENT"
    attributes = jsonencode({
      id     = "idn:identity-attributes-changed"
      filter = "$.changes[?(@.attribute == 'manager')]"
    })
  }

  definition = jsonencode({
    start = "Send Email Test"
    steps = {
      "Send Email" = {
        actionId = "sp:send-email"
        attributes = {
          body            = "This is a test"
          from            = "sailpoint@sailpoint.com"
          "recipientId.$" = "$.identity.id"
          subject         = "test"
        }
        nextStep     = "success"
        selectResult = null
        type         = "action"
      }
      success = {
        type = "success"
      }
    }
  })
}

# A scheduled-trigger workflow example - "trigger.attributes" shape differs
# entirely (frequency/cronString/timeZone instead of id/filter).
resource "identitynow_workflow_v1" "nightly_report" {
  name        = "Nightly Report"
  description = "Run a scheduled report every night."
  enabled     = false

  owner = {
    type = "IDENTITY"
    id   = "2c91808568c529c60168cca6f90c1313"
  }

  trigger = {
    type = "SCHEDULED"
    attributes = jsonencode({
      cronString = "0 0 * * *"
    })
  }

  definition = jsonencode({
    start = "success"
    steps = {
      success = {
        type = "success"
      }
    }
  })
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the workflow
- `owner` (Attributes) The identity that owns the workflow.  The owner's permissions in IDN will determine what actions the workflow is allowed to perform.  Ownership can be changed by updating the owner in a PUT or PATCH request. (see [below for nested schema](#nestedatt--owner))

### Optional

- `definition` (String) The map of steps that the workflow will execute, as a raw JSON object (`{"start": "...", "steps": {...}}`). Each step's own shape varies by its "type" - see https://developer.sailpoint.com/docs/extensibility/workflows/ for the JSON schema each step type expects.
- `description` (String) Description of what the workflow accomplishes
- `enabled` (Boolean) Enable or disable the workflow.  Workflows cannot be created in an enabled state.
- `trigger` (Attributes) The trigger that starts the workflow. "attributes" is a raw JSON object whose shape depends on "type" - see the resource/data source's top-level description for the shape each trigger "type" expects. (see [below for nested schema](#nestedatt--trigger))

### Read-Only

- `created` (String) The date and time the workflow was created.
- `creator` (Attributes) Workflow creator's identity. (see [below for nested schema](#nestedatt--creator))
- `execution_count` (Number) The number of times this workflow has been executed.
- `failure_count` (Number) The number of times this workflow has failed during execution.
- `id` (String) Workflow ID. This is a UUID generated upon creation.
- `modified` (String) The date and time the workflow was modified.
- `modified_by` (Attributes) (see [below for nested schema](#nestedatt--modified_by))

<a id="nestedatt--owner"></a>
### Nested Schema for `owner`

Optional:

- `id` (String) The unique ID of the object
- `name` (String) The name of the object
- `type` (String) The type of object that is referenced


<a id="nestedatt--trigger"></a>
### Nested Schema for `trigger`

Required:

- `attributes` (String) Workflow trigger attributes, as a raw JSON object whose shape depends on `type`.
- `type` (String) The trigger type (`EVENT`, `EXTERNAL`, or `SCHEDULED`).

Optional:

- `display_name` (String) The trigger display name.


<a id="nestedatt--creator"></a>
### Nested Schema for `creator`

Read-Only:

- `id` (String) Workflow creator's identity ID.
- `name` (String) Workflow creator's display name.
- `type` (String) Workflow creator's DTO type.


<a id="nestedatt--modified_by"></a>
### Nested Schema for `modified_by`

Read-Only:

- `id` (String) Identity ID
- `name` (String) Human-readable display name of identity.
- `type` (String)

## Known Limitations & Live Testing Notes

This resource is a `_v1` pilot: schema/model types for most attributes are
generated by `tfplugingen-framework` from SailPoint's per-service
`/workflows/v1` OpenAPI spec, but `definition` and the entire `trigger`
block are hand-written (not generated), and Create/Read/Update/Delete are
hand-written against the `golang-sdk`'s `api_beta.WorkflowsAPI` client.

- **Scope: only core workflow CRUD is implemented.** The following related
  Workflows API operations are deliberately out of scope for this pilot (no
  resource/data source manages them):
  - `POST /workflows/v1/{id}/test` (test-run a workflow) and everything
    under `/workflow-executions/v1/*` (execution history/cancellation) -
    these are transient, non-declarative operations, not resource state.
  - `GET /workflows/v1/{id}/external/oauth-clients` and
    `POST .../execute/external/{id}` - external-trigger invocation plumbing.
  - `GET /workflow-library/v1(/actions|/triggers|/operators)` - read-only
    catalogs describing what can go inside `definition.steps`, not a
    workflow's own configuration; a candidate for a future data source, not
    required to manage a workflow itself.
- **The entire response body is wrapped in a top-level `allOf`,** exactly
  like `transform_v1`. `list`/`create`/`get`/`put`/`patch` responses each
  merge the base `Workflow` properties with a `WorkflowBody`-shaped wrapper -
  `tfplugingen-openapi` cannot decompose this and silently skips mapping the
  entire response body unless the generator's WARN output is scrutinized.
  Fixed via `scripts/flatten_openapi_allof.py` against
  `api-specs/dereferenced/deref-workflows.v1.yaml`; must be re-applied if
  this spec is ever re-bundled from a newer upstream revision.
- **`definition` is a raw JSON string, not a typed block.** Its `steps`
  sub-property is `additionalProperties: true` - a free-form map keyed by
  step name, where each step's own shape varies by its `type`
  (`action`/`approval`/`success`/etc, with further nested
  attribute expressions). Excluded from codegen (`schema.ignores`) and
  hand-added as a `jsontypes.Normalized` JSON-string `CustomType`, giving
  semantic (not textual) equality so whitespace/key-ordering differences
  don't produce false diffs.
- **The entire `trigger` block is hand-written, not just `attributes`.**
  `trigger.attributes` is an `anyOf` across 3 shapes (`EVENT`/`EXTERNAL`/
  `SCHEDULED`) keyed by the sibling `trigger.type`. Unlike `transform_v1`'s
  top-level `attributes` (which could simply be added as an extra struct
  field), `trigger` is a nested `single_nested` block whose generated Go
  value type has no supported way to gain an extra hand-added field after
  codegen - so the *whole* `trigger` block (`type`, `display_name`,
  `attributes`) is `schema.ignores`'d and hand-written in full (schema and
  model), following `segment_v1`'s `visibility_criteria` precedent for a
  fully hand-rolled nested block.
- **Update uses a full `PUT`**, not `PATCH`/JSON Patch, mirroring
  `transform_v1`'s identical choice - every field a workflow exposes is
  mutable via `PUT` per the API's own documentation, so there was no need
  for JSON Patch's more complex per-field diffing.
- **Enabled workflows cannot be deleted.** The live API rejects `DELETE` on
  an `enabled = true` workflow; disable it first (`enabled = false`, then
  `apply`) before destroying.
- **Phase B (live `terraform plan`/`apply`/`destroy` against a real sandbox
  tenant) is complete for this pilot.** A full create/plan(no-drift)/destroy
  cycle was confirmed against a real tenant using `test/workflow/main.tf`.
  One real bug was found and fixed in the process: the API enriches
  `trigger.attributes` on read with extra null-valued keys not present in
  the original request (e.g. an `EVENT` trigger's response includes
  `"integrationId": null`, a key that's only meaningful for `SCHEDULED`
  triggers), which previously caused a "provider produced inconsistent
  result after apply" error on `Create`/`Update`. The provider now strips
  null-valued keys from `trigger.attributes` before storing it in state.
