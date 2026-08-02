# IdentityNow Terraform Provider Developer Knowledge

Use this file to reinforce the lead provider-developer agent over time. Append only — do not rewrite or delete prior entries.

> Entries before 2026-08-02 predate the golang-sdk v2 -> v3 migration; treat any `v2`/`api_beta`
> package reference in older entries (here and in the archive) as historical only.

## Entry Template
- Date: YYYY-MM-DD
- Task type: pipeline | review | author-agent | author-prompt | author-skill
- Target/Scope: <target> or <files touched>
- Summary: what was done
- Delegated to: specialist agent(s) invoked, if any
- Validation: command(s) run and outcome
- Guardrail update: what changed (or "none") in agent/prompt/knowledge conventions

## Distilled Guardrails

Terse, still-current rules distilled from every `Guardrail update:` line in the
chronological log (this file plus the archive). "none"/superseded entries are
omitted; see the dated entries for full context and evidence.

**SDK-version note:** the 2026-08-02 golang-sdk v2 -> v3 migration supersedes several
earlier v2-specific guardrails — notably the Service Desk Integration
`managedResourceRefs` `map[string]interface{}` SDK bug (fixed in v3; the
`sdk_fallback.go` workaround is now dormant dead code) and the v2-only absence of
`ProvisioningPolicyV2` bindings (now published in v3). Treat any `v2`/`api_beta`
package reference in older guardrails/entries as historical.

### Agent / prompt authoring
- Every new agent ships with its paired `.knowledge.md` and `.prompt.md` in the same change.
- Never hand-edit generated `*_gen.go` files — fix the spec, generator config, or a
  `scripts/apply_codespec_*` config instead. Hand-written CRUD wrappers (`resource_*.go`,
  `datasource_*.go`, `*_planmodifiers.go`, `sdk_fallback.go`) ARE hand-maintained and exempt.
- Re-verify factual/structural claims against the repo, not session memory; for `review` tasks
  re-derive claims fresh (prefer invoking the paired `.prompt.md` for genuine statelessness).
- Keep `sdk-issues.md` current: check it before assuming a new SDK problem is novel, and
  add/update an entry whenever a new `golang-sdk` defect/gap is found.

### Security / credentials
- Never persist secrets and never print/cat/view/log `test/**/main.tf` or `test/.env` values.
- Live sandbox credentials live only in a gitignored `test/.env`
  (`SAIL_BASE_URL`/`SAIL_CLIENT_ID`/`SAIL_CLIENT_SECRET`, sourced per command); each `main.tf`
  provider block stays empty. This is the preferred pattern — it removes any need to view a `main.tf`.
- When extracting values from `test/**/main.tf`, never use bare `\s`/`\d` regex classes with
  `sed -E` on macOS/BSD (they silently fail and leak the whole line); use POSIX bracket classes
  and sanity-check by length/shape, never by printing.

### Codegen pipeline & schema overrides
- Two pipelines coexist: legacy monolithic vs. preferred per-service `_v1`
  (`make bundle-spec-v1`/`gen-api-v1`/`gen-framework-api-v1`; `TARGET`=underscore name,
  `SERVICE`=dash folder). Resource/data-source keys must match `^[a-z_][a-z0-9_]*$`.
- A target's regen is all-or-nothing across its full replayable chain (`gen-api` ->
  `apply_codespec_schema_overrides` -> `apply_codespec_type_mappings` -> `gen-framework`);
  running a subset silently drops overrides. Use a clean generated-docs diff as the tripwire.
- Apply durable code-spec corrections via checked-in configs + `scripts/apply_codespec_schema_overrides.py`,
  never hand-edits. Override kinds: `required_overrides` (force required/optional, incl. dotted
  nested paths, when the live API requires a field the spec's `required` array omits),
  `strip_defaults` (spec's static `Default` doesn't match live API behavior), `computed_only_overrides`
  (API silently ignores/never persists a configured value).
- Flatten metadata-only / whole-response `allOf` wrappers with `scripts/flatten_openapi_allof.py`
  (structural match: all `allOf` members are `type: object`; also handles `{description, nullable}`-only members).
- A shared schema may carry zero `required:` markers (not just a missing `allOf` flatten) — check both.
- The code-spec JSON's plural key is `datasources`, not `data_sources` — grep the literal key.
- `api-specs/` is a small, git-tracked, disposable local reference synced from an external clone
  via `make sync-api-specs`; see `api-specs/README.md`.

### Type-linking
- `associated_external_type` mapping only affects to/from conversion methods — it never suppresses
  local `Type`/`Value` struct generation, so it cannot resolve a Go symbol-name collision (only
  `schema.ignores` + hand-write can).
- Mapping does NOT work for attributes nested inside `set_nested`/`list_nested` collections; for
  `list_nested`, nest `associated_external_type` inside `nested_object`. Leaf-only mapping rule applies.
- Reject a type mapping and hand-convert when the SDK field is `NullableString`, or when a nested
  attribute's own `required: [...]` list breaks the generated converter's pointer assumptions.
- Structurally identical but distinct ref DTOs must not be conflated in `type_mappings_*.yml`.

### Hand-written CRUD
- Every new resource/data-source must: set `resp.Schema.Description`/`MarkdownDescription`; ship a
  `_planmodifiers.go` from the start; call `util.SailpointErrorDetail` (not a per-package `errDetail`);
  and carry `tflog.Debug` at entry + `tflog.Info`/`tflog.Error` at the terminal point with structured
  (never string-interpolated) fields.
- Every new target ships `examples/` + `templates/` (with a "Known Limitations & Live Testing Notes"
  section) and a `templates/index.md.tmpl` subsection before it is documentation-complete.
  `make generate` = `fmt` + `docs`.
- An Optional+Computed attribute with no PlanModifier is reported Unknown (not Null) when omitted —
  guard with `!IsNull() && !IsUnknown()` before `.ValueString()`, treating Unknown like Null. Resolve
  any remaining `IsUnknown()` to a typed Null at the end of `dtoToModel`/`datasourceDtoToModel` for
  pass-through blocks (only a live `apply` catches a miss).
- For a pass-through / enriched JSON-blob attribute, use the fallback-preservation split: read the live
  value back only when the plan/state value is null/unknown; when the practitioner genuinely changes it,
  re-fetch live and merge (don't blindly resend a subset or skip the patch). Use `ImportStateVerifyIgnore`
  for that attribute and a bounded `CheckDestroy` retry for eventual consistency.
- When reusing a singular data source's generated schema as a nested object in a plural list data source,
  override `Required` lookup keys (e.g. `id`) to `Computed`.
- Before exposing pagination/filter attributes on a plural data source, confirm the SDK request builder
  actually implements them. Never expose an API's `count` list-query-param as a Terraform attribute
  (reserved root name).
- When a path parameter isn't literally `{id}`, expect two consequences: a duplicate attribute AND a
  missing auto-`Required` on the data source's `id` — check both.
- Recognized hand-written patterns: "join"/membership sub-resource (list + bulk-add + bulk-delete only,
  no single-item CRUD); read-only data source for a sub-resource with no write endpoint; async
  (202 + task-poll) delete; full-replace `PUT` vs. JSON-Patch update.
- Use whole-struct `.Equal()` cautiously for change-detection — it is unsafe when any sub-field is
  Computed-only (routinely Unknown at plan time); compare only the meaningfully-configurable sub-fields.

### Terraform framework / plan modifiers
- Never apply an object-level `UseStateForUnknown` to a `SingleNestedAttribute` containing nested
  Computed attributes — it breaks Create, not just Update. Only protect plain scalar leaf attributes this way.
- A field the API changes on every real Update (e.g. `modified`) cannot be made plan-stable by any
  mechanism (neither `UseStateForUnknown` nor `ignore_changes`); use `ExpectNonEmptyPlan: true` in tests /
  document the "(known after apply)" noise. Before protecting a `modified`-style field, confirm `dtoToModel`
  reads it from a live API field vs. a constant placeholder (a constant placeholder is safe to protect).
- There is no schema-level way to echo a server-injected value back into a `Required` or
  `Optional+Computed` attribute that has a known config value — either keep it pure Computed-only, or
  never write the server value into state.
- Use `ImportStateCheck` (not `ImportStateVerify`) for any resource whose `id` intentionally differs
  after import vs. after Create.

### Live / Phase B validation
- Phase A passing never implies Phase B passing — a live `apply` regularly surfaces bugs invisible to
  `plan`/`validate`/`lint`/`docs`/unit tests. Phase B is mandatory (not optional polish) for any resource
  that manages an association/sub-resource or triggers async backend jobs.
- For association/sub-resource resources, a clean `plan`/`apply` is insufficient — issue a direct,
  out-of-band API call confirming the parent/foreign object's own field state at each of Create/Update/Delete.
- A spec/schema YAML file existing on disk does NOT prove its endpoint is live — trace it to a real
  top-level `paths:` entry or confirm via live curl before designing a write path around it.
- "No delete (or other) operation in the spec/SDK" must be verified against real API/UI traffic before
  being treated as a permanent constraint; prefer a hand-rolled raw HTTP call built from the SDK's
  exported config/auth over a permanent no-op Delete, and reserve no-op-with-warning for
  independently-confirmed cases.
- When a filter/search-based data source lookup returns zero results in Phase B, check the target
  object's own indexing/lifecycle state via a direct GET before assuming a provider bug.
- For a no-delete target, `CheckDestroy` asserts the object still exists (200) after Terraform "destroy" —
  a 404 there signals an unexpected behavior change, not a pass.
- Never trust a delegated background agent's self-reported build/test/lint/live-test results —
  independently re-run them.

### Tooling / repo
- `.golangci.yml` (`make lint`) exclusions are scoped to generated code only; scaffold-first is required
  for new targets' hand-written CRUD.
- `make apply TARGET=<folder>` is a deliberately separate, higher-risk target from `make plan`.
- Any git worktree whose directory name isn't exactly `terraform-provider-<canonical-short-name>` must
  pass `--provider-name <short-name>` to `tfplugindocs` (the checked-in `make docs` doesn't), or docs
  generation fails outright — and this failure has masked a real schema-override regression before, so
  always use the docs diff as the cross-check.

### Open follow-ups (tracked, not yet done)
- `governance_group_v1`/`workflow_v1` `owner` is effectively required by the live API though the spec's
  `required` array omits it — surface a plan-time-adjacent error (validator/doc) rather than only failing
  on live `apply`.
- Promote the recurring hand-written-CRUD patterns (join-resource, read-only sub-resource data source,
  async-delete, static-default class, nested-object `.Equal()` pitfall, fallback-preservation,
  path-param-naming consequences, two-phase-Create-via-PATCH) into
  `terraform-provider-developer.agent.md`'s durable guidance.
- With v3's new `ProvisioningPolicyV2` bindings, lift `source_provisioning_policy_v1`'s v1-only
  composite-key limitation (needs live verification).

## Chronological Log

Entries before 2026-07-31 are archived in
`identitynow-terraform-provider-developer.knowledge-archive.md`. Only the most recent entries are
kept here to keep this file lean; append new dated entries below using the Entry Template above.

## 2026-07-31: segment_v1
- Date: 2026-07-31
- Task type: pipeline
- Target/Scope: `segment_v1` (resource + `identitynow_segment_v1`/`identitynow_segments_v1` data sources),
  `generator_config/generator_config_segment_v1.yml`, `generator_config/type_mappings_segment_v1.yml`,
  `internal/provider/segment_v1/*`, `internal/provider/segment_v1_resource_acc_test.go` (new),
  `internal/provider/provider.go`, examples/templates for all 3 surfaces, `test/segment/main.tf`.
- Summary: Standard CRUD resource (`/segments/v1` POST/GET/PATCH/DELETE, JSON-Patch update) modeling
  IdentityNow Segments. Two genuine `tfplugingen-framework` generator limitations were hit and solved, both
  durable lessons for future targets:
  1. **Repeated-attribute-name symbol collision, unfixable via type mapping**: the spec's
     `visibilityCriteria.expression.value` and `.expression.children.value` blocks are structurally identical
     and both literally named `value` at two different nesting depths. `tfplugingen-framework` keys its
     generated Go helper types (`ValueType`/`ValueValue`) only by attribute *name*, not full schema path, so
     this produced an unfixable "ValueType redeclared in this block" build failure - confirmed NOT fixable via
     `associated_external_type` (that only affects to/from conversion methods, not the underlying generated
     type name). Fix: `schema.ignores: [visibilityCriteria]` (the ORIGINAL CAMELCASE OpenAPI property name -
     using the Terraform snake_case name here silently does nothing, no error/warning, costing real debugging
     time) on the resource + both data sources, and hand-write the entire subtree in `resource_segment.go`,
     preserving the spec/reference-provider's exact attribute names rather than renaming to dodge the
     collision. The tree is deliberately capped at 2 levels (`visibility_criteria.expression` +
     `.expression.children`) per the API spec's own comment that a child's own `children` is "always null" -
     hand-written `visibilityChildModelToAPI` always calls `child.SetChildrenNil()`.
  2. **`associated_external_type` mapping silently doesn't work for attributes nested inside
     `set_nested`/`list_nested` collections**: mapping `owner` for the plural `segments` data source's list
     items left an unused `api_beta` import and failed to build (the generated `OwnerValue` had no conversion
     methods at all) - confirmed this only works for direct/non-collection-nested object attributes. The
     plural data source's `owner` is hand-converted instead; the mapping in `type_mappings_segment_v1.yml` is
     scoped to the resource + singular data source only.
  3. **Terraform reserves `count` as a root meta-argument name**: the API's own `count` query param (an
     X-Total-Count-header toggle) generated a root-level `count` attribute in the plural data source, which
     Terraform's schema validation rejects outright ("count is a reserved root attribute/block name"). Fixed
     the same way `governance_groups_v1`'s hand-written plural data source already avoided this (that
     precedent simply never exposed `count` at all): added `count` to `schema.ignores` for the `segments` data
     source and removed the corresponding `Count`/`.Count(...)` field and SDK call from the hand-written model
     - this is a durable convention now confirmed twice (once by precedent, once by fix): **never expose an
     API's `count` list-query-param as a Terraform data source attribute**.
  - **New bug class found only during live Phase B testing (not caught by any Phase A check)**: hand-patching
    a generated schema's nested `SetNestedAttribute.NestedObject.Attributes` map (to re-inject the
    hand-written `visibility_criteria` field ignored above) is not sufficient on its own - the `NestedObject`
    also carries a fixed `CustomType` (here, generated `SegmentsType`) computed at codegen time from the
    pre-patch attribute set, which takes precedence over the Attributes map when the framework computes
    `.Type()`. Without also clearing `NestedObject.CustomType = nil` after the patch, the object type silently
    omits the newly-added field, causing a runtime-only "Struct defines fields not found in object" conversion
    error on the plural data source's `Read` - reproducible only by actually invoking Read (a live `terraform
    plan`/`apply`), not by `go build`/`go vet`/unit tests. **New pattern: any time a generated
    `SetNestedAttribute`/`ListNestedAttribute`'s `NestedObject.Attributes` map is hand-patched after
    generation, also clear its `CustomType` field to `nil`** so the object type is derived dynamically from
    the patched Attributes instead of the stale generated type.
  - **Also found only during live Phase B testing**: `resource_segment.go` shipped with zero `PlanModifiers`
    at all (missing the `resource_<name>_planmodifiers.go` file every other hand-written resource in this repo
    has - role_v1, sources_v1, entitlement_v1, application_v1, governance_group_v1, identity_profile_v1,
    service_desk_integration_v1, transform_v1), causing permanent spurious "(known after apply)" diffs for
    `id`/`created` on every Update. Fixed by adding `resource_segment_planmodifiers.go` with
    `stringplanmodifier.UseStateForUnknown()` scoped narrowly to `id`/`created` only (matching `role_v1`'s
    precedent exactly: `modified` stays unpinned since it's genuinely volatile on every real Update, and
    `name`/`description`/`active`/`owner` all have real write support so must not be pinned). **New pattern:
    every future hand-written resource must include a `_planmodifiers.go` file from the start** (during
    initial hand-written-CRUD implementation, not discovered later via live-testing) - this should be checked
    explicitly as part of the hand-written-CRUD review checklist, not left to Phase B to catch.
- Delegated to: one background `general-purpose` agent for the initial hand-written CRUD + data sources
  (iterated once via `write_agent` mid-task after discovering, via a web fetch of the reference
  `davidsonjon/identitynow` provider's docs, that its singular data source supports lookup by `id` OR `name` -
  asked the agent to add this dual-lookup mode following `entitlement_v1`'s idiom, implemented via
  `datasourcevalidator.ExactlyOneOf` and clear zero/multiple-match error messages).
- Validation: Phase A (`gofmt -s -l .`/`go vet ./...`/`go build ./...`/`go test ./...`/`make lint`/`make
  docs`/`make tflint`/`make validate-examples` 33/33, all clean) and Phase B (live sandbox lifecycle in
  `test/segment/main.tf` against a dedicated fixture segment, not any real in-use segment: create with the
  2-level `visibility_criteria` tree, no-op idempotent plan, in-place update of `description`/`active`/a
  nested `visibility_criteria` leaf value, `terraform import`, `terraform destroy` - all clean after the two
  bugs above were fixed) both green. `go test ./internal/provider/segment_v1/...` (new
  `resource_segment_test.go`: modelToDTO/DTOToModel round-trips, patch-op diffing for scalar + nested
  `visibility_criteria` fields including add/replace/remove/no-op/unknown-plan branches,
  `segmentPatchRequestBody`, and the always-nil `children` invariant) and `TF_ACC=1 go test
  ./internal/provider/ -run TestAccSegmentV1Resource` (live; 3 steps: create+data-source-pairing checks,
  `ImportStateVerify` succeeds cleanly with no `ExpectNonEmptyPlan` needed - unlike `role_v1`, since
  segment_v1 has no pass-through-only nested blocks - and update) both pass.

## 2026-07-31: identity_v1, segment_access_v1, application_access_association_v1, entitlement_request_config_v1
- Date: 2026-07-31
- Task type: pipeline
- Target/Scope: `identity_v1` (read-only `identitynow_identity_v1`/`identitynow_identities_v1` data sources),
  `segment_access_v1` (new resource), `application_access_association_v1` (new resource),
  `entitlement_request_config_v1` (new resource), plus Phase B live testing for all four and a Phase B fixture
  bug fix in `entitlement_request_config_v1`.
- Summary:
  1. **`identity_v1`**: straightforward read-only data sources (singular lookup by exactly one of
     `id`/`alias`/`email_address`; plural list/filter). Live Phase B testing surfaced a **tenant-data gotcha,
     not a provider bug**: the first fixture identity picked (`00001078bd9c497a8122c6fc3f3571b1`, alias
     `A77M3879M`) had `lifecycleState`/`processingState` both null and a stale `lastRefresh` - a
     never-fully-indexed record - causing `alias eq`/filter-based lookups to return zero results even though
     the identity exists and its `id`-based lookup works. Confirmed via direct curl that the filter mechanism
     itself is correct by testing a second, properly-indexed identity (`00a62191d8e74828a702dacb79ddb657`,
     alias `M200082`) which resolved cleanly on all 3 lookup modes. **New pattern: when a filter/search-based
     data source lookup unexpectedly returns zero results in Phase B testing, check the target object's own
     indexing/lifecycle state via a direct API GET before assuming a provider bug** - a real IdentityNow
     tenant can contain identities that were never fully processed/indexed and are thus invisible to
     `eq`/filter search even though they're directly gettable by id.
  2. **`segment_access_v1`** (originally implemented and Phase-A-validated in a prior 2026-07-3x session):
     live Phase B testing invalidated the ENTIRE prior write-path design. The original Create/Update/Delete
     used `client.Generic.DefaultAPI.GenericPost` against `POST
     /beta/access-roles-change-segment-assignments`, a path/schema pair that exists as YAML files under
     `api-specs/idn/beta/paths|schemas/` but is **never referenced by any top-level `paths:` index anywhere in
     the api-specs checkout, including the bundled/dereferenced monolithic beta spec** - confirmed via direct
     curl (404 across `/beta/`, `/v3/`, `/v2025/`, and path-variant attempts with real ids). **Critical
     durable lesson: a path/schema YAML file existing in an `api-specs` checkout does NOT guarantee the
     endpoint is actually live/wired** - always verify via the top-level spec's `paths:` index (or, more
     reliably, a live curl test with real ids) before treating a "found via grep" spec fragment as
     authoritative, especially for anything reached only by filename pattern-matching rather than tracing an
     actual `$ref` chain from a real spec entrypoint. Reworked (via `write_agent` correction to the idle
     background agent) to use the real, confirmed-working mechanism: GET the Role
     (`client.Beta.RolesAPI.GetRole`) or AccessProfile (`client.Beta.AccessProfilesAPI.GetAccessProfile`) to
     read its current `Segments []string`, add/remove only this resource's own `segment_id` (preserving all
     other segment memberships - a Role/AP can belong to multiple segments), then PATCH `/segments` via
     JSON-Patch replace - mirroring `role_v1`'s/`access_profile_v1`'s own pre-existing `segments`-patching
     code exactly (`internal/provider/role_v1/resource_role.go`,
     `internal/provider/access_profile_v1/resource_access_profile.go`). This is an N+1-calls-per-apply design
     (one GET+PATCH pair per assignment), documented as a Known Limitation in the resource's docs template.
     Re-verified live: Create/Update (drop one of two assignments)/Delete all confirmed correct via direct API
     checks of the Role's/AccessProfile's actual `segments` field after each step.
  3. **`application_access_association_v1`**: Phase B fully passed on first live attempt (after fixing an
     unrelated fixture issue - `access_profile_v1`'s `entitlements` attribute is Required+non-empty unless
     `enabled=false`, so throwaway fixture access profiles used only to be referenced by id should set
     `enabled = false`). Create (union-merge with any live-but-untracked access profiles), Update (drop one of
     two managed access profiles, preserving anything attached out-of-band), and Delete (removes only this
     resource's own tracked ids, preserving everything else) semantics all confirmed correct via direct API
     verification of the Application's live `accessProfiles` list at each step.
  4. **`entitlement_request_config_v1`**: two bugs found and fixed during Phase B:
     - Field-name mismatch (test-fixture-writing friction, not a provider bug): the real generated schema
       field names are
       `request_comment_required`/`denial_comment_required`/`reauthorization_required`/`require_end_date` and
       `revocation_request_config.revocation_approval_schemes` - NOT the reference-provider's
       `comments_required`/`denial_comments_required` naming. The `revocation_approval_schemes` rename (from
       the API's actual `approvalSchemes` JSON field) happened during Phase 1 codegen specifically to avoid a
       Go type-name collision with the sibling `access_request_config.approval_schemes` block.
     - **Real provider bug**: `approver_id`/`approver_type` inside
       `approval_schemes`/`revocation_approval_schemes` list items are generated `Optional: true, Computed:
       true` with **no PlanModifier** (no `UseStateForUnknown`/Default). When a practitioner omits
       `approver_id` from a scheme block in config (fully valid - e.g. `{ approver_type = "MANAGER" }` alone),
       Terraform Core reports it as **Unknown** (not Null) during planning, since the framework can't
       statically know whether the provider computes a value. The original hand-written
       `accessApprovalSchemesModelToAPI`/`revocationApprovalSchemesModelToAPI` conversion functions treated
       `IsUnknown()` as a hard apply-time error ("Unknown approval_schemes.approver_id value"). **Fix**: since
       these fields are purely practitioner-supplied (the API never computes a server-side default for an
       omitted approver_id/approver_type), treat Unknown the same as Null (omit from the request body) rather
       than erroring - the subsequent GET/PUT-response-driven Read always repopulates the real value
       afterward, so this can never produce a "Provider produced inconsistent result after apply" error. **New
       pattern: any hand-written Optional+Computed list-item field with no PlanModifier must check both
       `!field.IsNull() && !field.IsUnknown()` before calling `.ValueString()`/similar in a model-to-API
       conversion function** - checking only `IsNull()` is an incomplete guard and will error (or silently
       misbehave) whenever a practitioner omits the field from config, which is the normal/expected case for
       an Optional attribute.
     - Live-verified Create, Update (added a second GOVERNANCE_GROUP-type approval scheme with a real
       workgroup id `007c4fda-c531-436b-8290-44b2789ee58c`, flipped 3 boolean flags), and Delete (confirmed
       state-only per the resource's own Delete implementation - does not touch the underlying entitlement or
       its live request-config). Noted a live-tenant-side quirk (not a provider bug): the GET request-config
       endpoint returns 404 shortly after a Terraform-state-only destroy even though Delete makes no API call
       - not investigated further since Delete's own state-only contract is already correctly implemented and
       documented.
- Delegated to: the `segment_access_v1` write-path rework was delegated via `write_agent` to the same idle
  background agent that originally implemented it (agent name `segment-access-v1`), with a detailed correction
  covering the invalidated endpoint finding and the exact GET+PATCH mechanism to use instead, citing
  `role_v1`'s/`access_profile_v1`'s own existing code as the pattern to mirror. All other Phase B fixes
  (approver_id/approver_type Unknown-handling, fixture corrections) were made directly by the primary session,
  not delegated.
- Validation: Phase A re-verified clean for all 4 (`gofmt -s -l .`/`go vet ./...`/`go build ./...`/`go test
  ./...`/`make lint`/`make docs`/`make tflint`/`make validate-examples` 38/38) - critically, the
  `segment_access_v1` rework was **independently re-run** by the primary session rather than trusting the
  background agent's self-report, per this repo's standing convention of never taking a delegated agent's
  validation claims at face value without re-running them directly. Phase B (live sandbox, via
  `test/identity/main.tf`, `test/segment_access/main.tf`, `test/application_access_association/main.tf`,
  `test/entitlement_request_config/main.tf`) fully green for all 4 after the fixes above, each verified via a
  combination of `terraform apply`/`plan`(no-drift)/`destroy` AND independent direct `curl` calls against the
  live tenant (not just trusting Terraform's own state) to confirm the underlying
  Role/AccessProfile/Application/Entitlement objects actually reflect the expected real-world change - this
  direct-API-cross-check step is what caught the `segment_access_v1` endpoint-doesn't-exist issue in the first
  place and is now the standing verification bar for any resource that manages a sub-resource/association
  rather than owning a full object lifecycle.
- Guardrail update: none to agent/prompt/knowledge conventions themselves this pass; the "verify a spec
  fragment is actually live before trusting it" and "always independently re-verify a delegated background
  agent's self-reported validation" lessons above are strong candidates for promotion into
  `terraform-provider-developer.agent.md`'s durable guardrails in a future pass, but were only added here as
  knowledge entries for now.

## 2026-07-31: Acting on independent (GPT-5.5) cross-model review feedback
- Date: 2026-07-31
- Task type: review
- Target/Scope: `identitynow-terraform-provider-developer.agent.md`, `.sdk-issues.md`,
  `identitynow-terraform-provider-developer.prompt.md`, and `copilot-instructions.md`'s IdentityNow-relevant
  sections.
- Summary: A background sub-agent was run explicitly on a different model (`gpt-5.5`) in read-only
  `task=review` mode to independently critique the agent/knowledge/prompt file set this session had just
  updated. Acted on its IdentityNow-specific findings: fixed stale `test/**/main.tf`-holds-credentials wording
  (repo migrated to `test/.env` mid-session; `main.tf` is now fixture-ids-only), rewrote the IdentityNow
  prompt's pipeline policy to match the agent (skip-bundling-if-cached, schema-overrides/type-mappings via
  scripts not hand-patching, full Phase A validation), added a read-only-review mode + matching Return format
  variant to the prompt, folded a formal schema-overrides/allOf-flattening step into pipeline step 4 (Type
  linking) rather than renumbering, trimmed the overly-dense Phase B step 10 bullet into sub-bullets (keeping
  only IdentityNow deltas, referencing the base agent for the rest), trimmed the "Tooling Gaps" section down
  to current-open-items-only (full resolved history already lives in this file's 2026-07-26/2026-07-28
  entries), and logged the `ImportEntitlements` multipart SDK bug as `.sdk-issues.md` issue #8 (previously
  only in this file's narrative, not the living index). See the base agent's matching 2026-07-31 knowledge
  entry for the full cross-file summary and the vendor-agnostic patterns generalized out of this repo's
  findings.
- Delegated to: the independent review itself was delegated to a `gpt-5.5`-model background agent (read-only);
  all resulting fixes were made directly by the primary session.
- Validation: YAML front matter re-parsed clean; `go build ./...`/`go vet ./...` clean; confirmed
  `.sdk-issues.md` issue #8's referenced `ImportEntitlements` bug matches this file's earlier 2026-07-3x
  `entitlement_v1` entry; confirmed no stale `test/README.md` references remain unfixed.
- Guardrail update: none new for this file specifically - all applicable guardrails were already added to the
  base agent per its own matching entry.

## 2026-07-31: connector_rule_v1
- Date: 2026-07-31
- Task type: pipeline
- Target/Scope: `connector_rule_v1` (new resource +
  `identitynow_connector_rule_v1`/`identitynow_connector_rules_v1` data sources) - an "extra" target with no
  counterpart in the reference `davidsonjon/identitynow` provider, wrapping `/connector-rules/v1`. Files:
  `internal/provider/connector_rule_v1/*`, `internal/provider/provider.go`, examples/docs for all 3 surfaces,
  `test/connector_rule/main.tf`.
- Summary: Standard bundle/flatten/codegen pipeline, then hand-written CRUD with
  `signature`/`source_code`/`input`/`output` left unmapped (per prior segment's decision) due to a
  `NullableString`-typed `Argument.Type` field incompatible with generated leaf-string conversion, requiring
  manual `ElementsAs`/`ListValueFrom` round-trips instead of generated To/FromApi converters. Two durable
  findings:
  1. **`GetConnectorRuleList`'s SDK request builder has zero pagination methods** despite the OpenAPI spec
     declaring `limit`/`offset`/`count` query params for `GET /connector-rules/v1` - confirmed via direct
     source inspection of `api_beta.ApiGetConnectorRuleListRequest` (only `ctx`/`Execute()`). The plural
     `identitynow_connector_rules_v1` data source therefore has no filtering/pagination support at all and
     always returns every connector rule in the tenant; documented in the data source's package doc and logged
     as `.sdk-issues.md` issue #9.
  2. **New, broadly-reusable Terraform framework constraint discovered via live Phase B testing, distinct from
     (and stricter than) the already-documented "Optional+Computed reports Unknown when omitted" pattern**:
     `CreateConnectorRule`/`UpdateConnectorRule` silently inject a server-computed `"sourceVersion"` key into
     the returned `attributes` object even when the practitioner's config sent `"{}"`. First fix attempt (kept
     `attributes` `Required`, let `connectorRuleResponseToModel` write the API's enriched value into state)
     failed with "Provider produced inconsistent result after apply" (a Required attribute's post-apply value
     must exactly equal the planned/configured value). Second fix attempt (`Optional: true, Computed: true` +
     a custom plan modifier proposing `types.StringUnknown()` whenever configured) failed with a *different*,
     harder error: **"Provider produced invalid plan: planned value cty.UnknownVal(cty.String) does not match
     config value cty.StringVal(...)"** - Terraform does not permit an Optional(+Computed) attribute to be
     planned Unknown when the practitioner's config sets a known, non-null value, full stop; only a pure
     Computed-only (non-Optional) attribute can diverge freely from config, or an Optional+Computed one whose
     config is null/unconfigured. **There is no schema-level way to let a provider freely echo back a
     server-enriched value for a Required or Optional+Computed attribute that has a known config value.**
     Final fix: kept `attributes` plain `Required` (no Computed, no custom plan modifier) and simply **never
     write the API's returned `attributes` into state** - `connectorRuleResponseToModel` always keeps the
     fallback (plan-on-Create/Update, prior-state-on-Read) value instead of `dto.Attributes`, sidestepping the
     consistency check entirely at the cost of not detecting out-of-band drift to that one field (documented
     as a `_v1` pilot limitation). The data source's `attributes` remains Computed-only (no config to conflict
     with) and is unaffected - it freely reflects the real API response.
  - Also hit and resolved a routine tainted-state replace during re-testing (an earlier failed apply attempt
    left the resource `tainted`; the next `terraform apply` correctly destroyed+recreated it, not a recurrence
    of the bug) - a reminder to check `terraform plan`'s actual action reason (`is tainted, so must be
    replaced`) before assuming a schema fix didn't work.
- Delegated to: none (all hand-written CRUD, schema fixes, and live debugging done directly by the primary
  session this pass).
- Validation: Phase A (`go build ./...`/`go vet ./...`/`gofmt -l .`/`go test ./...`/`make lint` 0 issues/`make
  tflint` clean/`make validate-examples` 41/41/`make docs` regenerated and reviewed) all green, re-run twice
  (once before Phase B, once after the final `attributes` fix to catch any regression from the plan-modifier
  rewrite). Phase B (live sandbox, `test/connector_rule/main.tf`): create, singular + plural data source
  reads, no-op `terraform plan` after apply, and `terraform destroy` all confirmed clean.
- Guardrail update: promoted the new "server-injected field can't be reflected back into a
  Required/Optional+Computed attribute with a known config value" constraint into
  `terraform-provider-developer.agent.md`'s stage 6 patterns list (see that file's matching entry) since it's
  a vendor-agnostic Terraform framework rule, not IdentityNow-specific; logged the `GetConnectorRuleList`
  missing-pagination gap as `.sdk-issues.md` issue #9.

- Date: 2026-07-31
- Summary: Implemented `workflow` as a new per-service v1 target end-to-end (resource + singular/plural data
  sources), Phase A only (no sandbox credentials available this session). (1) `make bundle-spec-v1
  TARGET=workflow SERVICE=workflows` succeeded cleanly against `api-specs/idn/apis/workflows/openapi.yaml`.
  (2) Confirmed the now-familiar whole-response-`allOf` pattern first seen on `transform_v1`: `workflows`'
  list/create/get/put/patch responses each wrap the base `Workflow` properties in a 2-member `allOf` with a
  `WorkflowBody`-shaped wrapper (7 occurrences total, including one inside the out-of-scope
  `workflow-executions/.../history-v2` response) - `scripts/flatten_openapi_allof.py` flattened all 7 cleanly
  on the first pass, no manual intervention needed (a good sign the promoted script generalizes well beyond
  its original `transform`/`governance_group`/`sources`/`identity_profile` motivating cases). (3) Authored
  `generator_config_workflow_v1.yml` with `schema.ignores: [definition, trigger]` (both resource and data
  source) - **a new variant of the dynamic-attributes pattern**: unlike `transform_v1`'s top-level
  `attributes` (which could simply be added back as an extra hand-written struct field once schema.ignores'd),
  workflow's dynamic field (`trigger.attributes`, an anyOf across EVENT/EXTERNAL/SCHEDULED shapes keyed by the
  sibling `trigger.type`) lives *inside* a nested `single_nested` block alongside two ordinary well-typed
  sibling fields (`trigger.type`, `trigger.displayName`). `tfplugingen-framework` generates one fixed Go value
  type per single_nested block at codegen time with no supported way to hand-insert an extra field into it
  afterward - so the **entire** `trigger` block, not just `attributes`, had to be `schema.ignores`'d and
  hand-written in full (schema + a plain `types.Object`-backed model), directly reusing `segment_v1`'s
  pre-existing `visibility_criteria` precedent for a fully hand-rolled nested block (confirmed via
  `types.ObjectValueFrom`/`.As(ctx, &model, basetypes.ObjectAsOptions{})` round-tripping cleanly). This is now
  the durable answer for any future target whose dynamic/discriminated field is nested one level deep rather
  than top-level. `definition` itself (the workflow's `{start, steps}` map, with `steps` being
  `additionalProperties: true`) stayed a simple top-level `jsontypes.Normalized` field exactly like
  `transform_v1`'s `attributes` - no new pattern needed there, since `api_beta.WorkflowDefinition` is a real
  (if leaf) SDK struct with working JSON marshal/unmarshal, so `definitionToAPI`/`definitionFromAPI` could
  decode/encode directly into it rather than needing a raw `map[string]interface{}` round-trip like
  `transform_v1` did. (4) `owner` was generated `computed_optional` (spec's `createWorkflowV1` request body
  only lists `name` as `required`), but `api_beta.CreateWorkflowRequest.Owner` is a plain non-pointer,
  non-omitempty `WorkflowBodyOwner` field in the SDK, and SailPoint's own Workflows docs describe every
  workflow as always having an owning identity - forced `owner` to `required` (resource scope only) via a new
  `schema_overrides_workflow_v1.yml` + `apply_codespec_schema_overrides.py`, mirroring `governance_group_v1`'s
  identical `owner`-required fix (not live-confirmed this session, but the SDK's own non-pointer-required
  field type is strong corroborating evidence, matching the `MultiHostIntegrationsCluster`
  non-pointer-required-triggers-a-fix precedent). (5) Type-linking review (done directly, no sub-agent
  invocation available) found 3 leaf-only mapping candidates - `owner`->`WorkflowBodyOwner`,
  `creator`->`WorkflowAllOfCreator`, `modified_by`->`WorkflowModifiedBy` (all 3 are structurally identical
  `{type,id,name}` all-plain-`*string` leaf refs per direct SDK source inspection, but 3 genuinely distinct Go
  types - captured in `type_mappings_workflow_v1.yml` with an explicit note not to conflate them, following
  the now-recurring "structurally identical but distinct ref DTOs" caution from `sources_v1`/`role_v1`).
  Applied cleanly via `apply_codespec_type_mappings.py`; `trigger` deliberately NOT in the mapping config
  since it was `schema.ignores`'d in full. (6) `gen-framework-api-v1` succeeded on the first attempt after
  applying both override configs. (7) Hand-wrote full CRUD (`resource_workflow.go`) against
  `api_beta.WorkflowsAPIService`
  (`CreateWorkflow(CreateWorkflowRequest)`/`GetWorkflow`/`PutWorkflow(WorkflowBody)` for a full-replace
  `Update` (matching `transform_v1`'s PUT-over-PATCH choice - every workflow field is mutable via PUT per the
  API's own docs, so JSON Patch's extra complexity wasn't needed)/`DeleteWorkflow`, with a Delete
  error-message addendum noting "enabled workflows cannot be deleted" per the spec's own description),
  `resource_workflow_planmodifiers.go` (hand-rolled `trigger` single_nested schema for both
  resource/data-source contexts, `UseStateForUnknown` on `id`/`created`/`modified`), `datasource_workflow.go`
  (singular, reusing `datasource_workflow`'s Go-distinct generated types), `datasource_workflows.go` (plural
  list against `ListWorkflows` with `filters`/`limit`/`offset`/`sorters`, `workflowsListMaxLimit = 250` per
  the spec's documented max, reusing the singular data source's hand-written model/converter - mirroring
  `governance_group_v1`'s plural-list precedent exactly, including its "a fully-known `filters` value invokes
  a live API call at plan time" caveat). Wired
  `workflow_v1.NewWorkflowResource`/`NewWorkflowDataSource`/`NewWorkflowsDataSource` into `provider.go` (18
  resources, 26 data sources total, both counts cross-checked by grepping the actual
  `Resources()`/`DataSources()` function bodies rather than hand-counted). (8) Deliberately scoped OUT of this
  pilot (documented in the package doc + resource template's "Known Limitations"): `PATCH /workflows/v1/{id}`
  (PUT already covers full-replace semantics), workflow test/execution endpoints (`POST .../test`, `GET/DELETE
  /workflow-executions/v1/*`, `GET .../executions` - transient, non-declarative operations), external-trigger
  invocation plumbing (`GET .../external/oauth-clients`, `POST .../execute/external/{id}`), and the read-only
  `/workflow-library/v1` action/trigger/operator catalogs (candidate future data sources, not required to
  manage a workflow itself). (9) Created `examples/{resources,data-sources}/identitynow_workflow(s)_v1/*.tf`
  (event + scheduled trigger examples), `templates/{resources,data-sources}/workflow(s)_v1.md.tmpl` with a
  full "Known Limitations & Live Testing Notes" section (explicitly noting Phase B is pending,
  schema/spec-reasoned only), `test/workflow/main.tf` (gitignored, reuses `test/governance_group`'s
  placeholder identity id fixture, includes both the singular resource/data-source lifecycle check and an
  explicit-filters plural-list block for later manual Phase B use), and updated `README.md`'s Scope counts
  (17->18 resources, 24->26 data sources) and Roadmap list (removed the now-implemented "Workflows" candidate,
  renumbered the rest) plus `templates/index.md.tmpl`'s resource/data-source index (`### Workflows` section,
  matching every other category's format) - `make docs` regenerated
  `docs/index.md`/`docs/resources/workflow_v1.md`/`docs/data-sources/workflow(s)_v1.md` cleanly on the first
  pass.
- Delegated to: none (no sub-agent invocation tooling available this session; type-linking/schema-override
  reviews done directly, following the same playbooks `tfplugingen-openapi-type-reviewer`/this agent's own
  pipeline steps document).
- Validation: Phase A only, all green - `go build ./...`, `go vet ./...`, `gofmt -l
  internal/provider/workflow_v1` (clean after one `gofmt -w` pass to fix initial struct-field alignment), `go
  test ./...` (all existing packages still pass, no test file for the new target itself - see below), `make
  lint` (0 issues), `make docs` (regenerated cleanly, template-driven "Known Limitations" sections rendered as
  expected), `make validate-examples` (44/44 passed, up from 41), `make tflint` (clean). **Phase B (live
  `terraform plan`/`apply` against a real sandbox tenant) is a pending follow-up** - no `test/.env`
  credentials were available this session; `test/workflow/main.tf` was authored and is ready for a future
  credentialed session to run `make plan TARGET=workflow` and a full apply/destroy lifecycle against, but
  nothing in it has been live-confirmed. No acceptance test file (`workflow_v1_resource_acc_test.go`) was
  written either, since every existing `*_resource_acc_test.go` in this repo was written only after a live
  Phase B pass confirmed the design - fabricating one now would violate the "never confirm what wasn't
  actually observed" rule; writing it is a natural next step once Phase B runs.
- Guardrail update: no new IdentityNow-specific Workflow guardrail needed beyond what's captured above (the
  "nested single_nested block with a hand-added dynamic field must be schema.ignores'd and hand-written in
  full, not just the dynamic sub-field" pattern is a direct reuse of `segment_v1`'s pre-existing
  `visibility_criteria` precedent, not a new mechanism) - worth a forward-reference the next time this shape
  recurs (a nested block containing one dynamic + one-or-more static sibling fields) that transform_v1's "add
  the field back as an extra struct field" trick only works for *top-level* dynamic fields, not nested ones.
## 2026-08-01: sod_policy_v1 (new pilot, dedicated worktree)
- Date: 2026-08-01
- Task type: pipeline
- Target/Scope: `sod_policy_v1` (new resource + `identitynow_sod_policy_v1`/`identitynow_sod_policies_v1` data
  sources), wrapping `/sod-policies/v1`. Implemented in a dedicated git worktree
  (`terraform-provider-identitynow-sodpolicy`, branch `feat/sod-policy-v1`) with no live sandbox credentials
  available, so Phase B was scoped out entirely and left pending. Files: `internal/provider/sod_policy_v1/*`,
  `internal/provider/provider.go`, examples/docs for all 3 surfaces,
  `generator_config/generator_config_sod_policy_v1.yml` + `type_mappings_sod_policy_v1.yml`,
  `test/sod_policy/main.tf` (gitignored, placeholder-id scaffolding only - never exercised).
- Summary:
  1. **Worktree-relative `API_SPECS_SOURCE` portability confirmed as a real, not just theoretical, concern**:
     this worktree's default `../api-specs` resolves relative to the worktree directory, not the main
     checkout, so every `API_SPECS_SOURCE`-dependent Makefile invocation needed an explicit absolute override
     for the one-time `make bundle-spec-v1` step (per the base agent's one-time-dependency design, this was
     the only step requiring it - all subsequent steps used the committed dereferenced spec).
  2. **A whole-operation-response `allOf` pattern (already documented for `transform`) recurred here at the
     individual-field level**: `conflictingAccessCriteria` is `allOf: [$ref, {nullable: true}]` at 3 schema
     sites (resource + both data sources) - `scripts/flatten_openapi_allof.py` handled this cleanly in one
     pass across the whole bundled spec (22 sites flattened total, including unused sub-endpoints like
     schedule/violation-report - harmless, since those aren't in this target's generator config scope).
  3. **A new two-field Go symbol collision class, structurally identical to `segment_v1`'s precedent but
     arising independently in two different ways in the same spec**: (a)
     `conflictingAccessCriteria.{leftCriteria,rightCriteria}.criteriaList` - two sibling branches reusing the
     identical leaf name `criteriaList` - and (b) top-level `ownerRef` vs.
     `violationOwnerAssignmentConfig.ownerRef` - same name at different tree depths. Both were
     `schema.ignores`'d and hand-written/hand-converted directly (mirroring `segment_v1`'s
     `visibility_criteria` fix), confirming this failure class (attribute-name-only keying, not full-path
     keying, in `tfplugingen-framework`'s generated Go helper types) is not a one-off but a recurring hazard
     worth checking proactively on any new target with sibling or multi-depth nested objects.
  4. **Confirmed (not merely inferred) that `associated_external_type` mapping does NOT suppress local
     Type/Value struct generation**: checked `segment_v1`'s already-mapped `owner` field and found it still
     generates a local `OwnerValue` struct (Terraform state needs an `attr.Value` implementation regardless of
     any SDK type mapping) - meaning type mapping alone can never resolve a naming collision; only
     `schema.ignores` + hand-write can. Worth stating explicitly for future targets tempted to reach for a
     mapping rename as a collision fix.
  5. **`NullableString` incompatibility (previously documented for other targets) reconfirmed as a second,
     independent reason** (beyond the naming collision) to exclude `violationOwnerAssignmentConfig` from
     codegen/type-mapping: both `ViolationOwnerAssignmentConfig.AssignmentRule` and its nested `OwnerRef.Type`
     are `NullableString` in the SDK, a shape the generated To/From converter templates cannot handle.
  6. **Update uses a genuine full-replace `PUT`** (like `transform_v1`/`connector_rule_v1`), not JSON-Patch
     (unlike `segment_v1`/`governance_group_v1`) - the sod-policies v1 API exposes a real, working `PUT
     /sod-policies/v1/{id}` per its own spec description ("This updates a specified SOD policy"), confirmed by
     direct spec inspection before committing to this design.
  7. **`terraform-plugin-docs`/`tfplugindocs` provider-name auto-detection breaks in a differently-named git
     worktree**: `tfplugindocs generate` derives the Terraform provider short name from the containing
     directory's `terraform-provider-<name>` prefix by default; running `make docs` unmodified inside
     `terraform-provider-identitynow-sodpolicy` caused it to look for a nonexistent
     `identitynow-sodpolicy_<resource>` schema and fail immediately with a template-execution error. Worked
     around by invoking `go tool tfplugindocs generate --provider-name identitynow --rendered-provider-name
     identitynow` directly (bypassing the checked-in `make docs` target, which doesn't pass `--provider-name`)
     rather than editing the shared Makefile for a worktree-specific quirk. **New pattern: any git worktree
     whose directory name doesn't exactly match `terraform-provider-<canonical-short-name>` must pass
     `--provider-name <canonical-short-name>` explicitly to `tfplugindocs` (or set it via a local Makefile
     override), or `make docs` will fail outright** - this is a worktree-naming hazard worth checking early in
     any future worktree-based session, not just an IdentityNow-specific one.
  8. **`terraform validate` (via `make validate-examples`) caught a real authoring mistake early**: the
     example/test HCL used `state = "ENABLED"`, but the generated schema's `stringvalidator.OneOf` only
     permits `"ENFORCED"`/`"NOT_ENFORCED"` (confirmed against the generated schema code, not guessed) - fixed
     across `examples/`, `templates/`, and `test/sod_policy/main.tf`. A reminder that `make validate-examples`
     is a real correctness check on authored example content, not just a smoke test of the pipeline itself.
  9. **Cross-reference maintained deliberately**: the resource's schema `MarkdownDescription` and its doc
     template both explicitly cross-reference the pre-existing read-only
     `identitynow_governance_group_connections_v1` data source's `SOD_POLICY` connection type (documented
     since before this session), completing the reverse-lookup relationship's documentation on both sides.
  10. **Intentionally scoped out of this CRUD resource**: the `schedule` (`GET`/`PUT
      /sod-policies/v1/{id}/schedule`) and evaluate/violation-report/violation-report-status endpoints - these
      model an asynchronous request/status-poll workflow, not declarative CRUD state, and are flagged in both
      the resource's package doc and `README.md`'s roadmap as a candidate for a purpose-built future
      resource/data source rather than being force-fit here.
- Delegated to: none (all steps run directly by the primary session; no sub-agent delegation this pass).
- Validation: Phase A only (no sandbox credentials available in this worktree) - `go build ./...`, `go vet
  ./...`, `go test ./...` (all pre-existing packages' tests still pass, no new unit tests added for this
  hand-written-only-conversion-logic target, consistent with most other `_v1` pilots that don't yet have
  dedicated `_test.go` files), `golangci-lint run`/`make lint` (0 issues), `make docs` (regenerated cleanly
  via the `--provider-name` workaround above; diff limited to the 3 new pages + `docs/index.md`'s new section,
  no unexpected changes elsewhere), `make validate-examples` (44/44 after the `ENABLED`->`ENFORCED` fix),
  `make tflint` (clean). Phase B (live `terraform plan`/`apply` against a real sandbox tenant) is explicitly
  **pending** - `test/sod_policy/main.tf` was authored with placeholder object ids as scaffolding only, never
  exercised, and is called out as pending in the final report per the task's own instructions.
- Guardrail update: added the `tfplugindocs --provider-name` worktree-naming workaround as a new item to check
  early in any future worktree-based pipeline task (documented here rather than promoted to the base agent,
  since the base agent's docs-generation guidance is already generic - the specific
  `--provider-name`/`--rendered-provider-name` flag names are `tfplugindocs`-tool-specific detail appropriate
  to keep at this level of specificity, not a vendor-agnostic principle needing promotion). No new SDK issue
  logged this pass - no genuine `golang-sdk` defect was found (the collisions/NullableString incompatibilities
  are `tfplugingen-framework`/spec-shape interactions, already covered by existing entries, not SDK bugs).
## 2026-08-01: source_provisioning_policy_v1, source_schema_v1
- Date: 2026-08-01
- Task type: pipeline
- Target/Scope: `source_provisioning_policy_v1` (new resource +
  `identitynow_source_provisioning_policy_v1`/`identitynow_source_provisioning_policies_v1` data sources) and
  `source_schema_v1` (new resource + `identitynow_source_schema_v1`/`identitynow_source_schemas_v1` data
  sources), both new sibling targets extending the existing `sources` service spec alongside `source_v1` -
  implementing the two sub-resources that `source_v1`'s own docs had explicitly deferred to "a future,
  separate resource/data source." Work done in an isolated git worktree
  (`terraform-provider-identitynow-sourceprov`, branch `feat/source-provisioning-policy-v1`) whose directory
  name differs from the main checkout's.
- Summary:
  1. **v1-vs-v2 provisioning-policy shape decision**: the task asked to prefer the newer
     `/sources/v2/{sourceId}/provisioning-policies/{id}` API (real-`id`-keyed CRUD, a cleaner Terraform fit)
     over the older `/sources/v1/{sourceId}/provisioning-policies/{usageType}` API (composite
     `sourceId`+`usageType` key, bulk-update-only PATCH) if v2's shape was clean enough. Confirmed via direct
     grep of the vendored `golang-sdk` (`api_beta.SourcesAPIService`, through `v2.7.106`) that it implements
     **zero v2 provisioning-policy bindings at all** - only v1's
     Create/Get/Update(PATCH)/Put/Delete/ListProvisioningPolicies. v2 additionally requires a mandatory
     `X-SailPoint-Experimental` header the SDK has no way to attach even if bindings existed. **Used v1**,
     with the composite `<source_id>/<usage_type>` id synthesized by hand (`ProvisioningPolicyDto` has no
     native `id` field) - a deliberate, fully-justified deviation from the task's stated preference, driven
     entirely by an SDK gap rather than a spec-quality judgment. Worth revisiting if/when the SDK adds v2
     bindings.
  2. **Source Schemas CSV-endpoint exclusion**: the `sources` service also exposes
     `sources-v1-by-id-schemas-accounts.yaml`/`sources-v1-by-id-schemas-entitlements.yaml` (CSV
     account/entitlement schema-template download/upload, `text/csv`/`multipart/form-data`,
     Delimited-File-source-only). Deliberately excluded from `source_schema_v1` - out of scope as
     file-transfer operations rather than structured JSON CRUD, a poor fit for a declarative resource and
     unsupportable by `tfplugingen-openapi`/`-framework` regardless. Only
     `sources-v1-by-sourceid-schemas(-by-schemaid).yaml`'s JSON CRUD (by `schemaId`) was covered.
  3. Both targets follow the same, by-now-standard structure: `fields` (provisioning policy, an array whose
     elements each nest a discriminated-union `transform`) and `configuration` (schema, a free-form object
     bag) are both `schema.ignores`'d in generator config and hand-added as `jsontypes.Normalized` JSON-string
     CustomTypes, matching `transform_v1`'s established pattern. `source_schema_v1` additionally hit the
     by-now-familiar "redundant computed id" shape: `tfplugingen-openapi` generates both a real `id` (from the
     Schema DTO body) and a synthesized `schema_id` (from the `{schemaId}` path param) that are always equal -
     both forced Computed-only via `computed_only_overrides` in `schema_overrides_source_schema_v1.yml`. Both
     plural list data sources (`..._policies_v1`/`..._schemas_v1`) are fully hand-written (not generated),
     reusing the singular data source's generated schema/model types, mirroring
     `role_v1`/`datasource_roles.go`'s established precedent - this pattern is now proven across at least 4
     targets (`role_v1`, `source_provisioning_policy_v1`, `source_schema_v1`, plus others).
  4. **Type-mapping review outcome: zero candidates for both targets** (matching `transform_v1`'s precedent) -
     once the dynamic-shape fields (`fields`/`configuration`) were excluded via `schema.ignores`, every
     remaining structural type (`AttributeDefinition`, `AttributeDefinitionSchema`, plain scalar leaves) was
     already a plain leaf list/object needing no `associated_external_type` linking or symbol-collision
     rename; no `type_mappings_<target>_v1.yml` files were created, and this "zero candidates" outcome was
     explicitly confirmed and documented rather than silently skipped.
  5. **Worktree-directory-name gotcha with `make docs`/tfplugindocs**: this worktree's directory is named
     `terraform-provider-identitynow-sourceprov` (not `terraform-provider-identitynow`), and `tfplugindocs
     generate --rendered-provider-name identitynow` (this repo's exact `make docs` invocation) derives the
     *schema-lookup* provider name from `--provider-dir`'s
     basename-after-stripping-`terraform-provider-`-prefix when `--provider-name` isn't also passed explicitly
     - so it looked for a `terraform-provider-identitynow-sourceprov_identitynow-sourceprov` schema entry and
     failed with `data source entitled "identitynow-sourceprov", or
     "identitynow-sourceprov_access_model_metadata_attribute_v1" does not exist`. Fixed for this session by
     invoking `go tool tfplugindocs generate --provider-name identitynow --rendered-provider-name identitynow`
     directly instead of `make docs` (which hardcodes only `--rendered-provider-name`). This is purely a
     consequence of using a differently-named git worktree directory, not a repo defect - do not "fix"
     `GNUmakefile` to hardcode `--provider-name identitynow` repo-wide without checking this doesn't break the
     main checkout's own (already-passing) `make docs` invocation; if this recurs across sessions, consider
     making `GNUmakefile`'s `docs:` target pass both flags unconditionally (harmless when the directory name
     already matches).
- Delegated to: none (all research, spec reading, hand-written CRUD, and validation done directly by the
  primary session this pass).
- Validation: Phase A only, per task instructions (no sandbox credentials available in this worktree) - `go
  build ./...`, `go vet ./...`, `go test ./...` (all packages pass/no test files), `make lint` (`golangci-lint
  run`, 0 issues), `make fmt` + `tfplugindocs generate --provider-name identitynow --rendered-provider-name
  identitynow` (docs regenerated cleanly, `git status` diff limited to expected new/updated files), `make
  validate-examples` (47/47 passed), `make tflint` (clean). Phase B (live `terraform plan`/`apply` against a
  real sandbox tenant) is an explicit pending follow-up - `test/source_provisioning_policy/main.tf` and
  `test/source_schema/main.tf` were created (gitignored) with placeholder source ids but never exercised.
- Guardrail update: none new to this file's own conventions - the "zero type-mapping candidates" and
  "hand-written plural list data source" patterns were already established precedents being followed, not
  newly discovered. The worktree-directory-name `make docs` gotcha is logged here as a one-off session note
  rather than a durable guardrail, since it's specific to running this pipeline from a non-standard-named
  worktree directory.

## 2026-08-02: golang-sdk v2 -> v3 whole-repo SDK migration
- Date: 2026-08-02
- Task type: pipeline (cross-cutting SDK major-version migration, not a single-target codegen run)
- Target/Scope: migrate the ENTIRE provider off `github.com/sailpoint-oss/golang-sdk/v2` (version-based
  packages: `api_v3`/`api_beta`/`api_cc`, though this repo only ever used `api_beta` + the root `sailpoint`
  client) onto `github.com/sailpoint-oss/golang-sdk/v3@v3.1.10` (per-service packages). ~21
  resource/data-source packages + all test files + shared `util`/`provider` bootstrap. Done in isolated
  worktree `terraform-provider-identitynow-v3migration`, branch `feat/golang-sdk-v3-migration`, 15 commits,
  static validation only (no sandbox creds).
- Summary:
  1. **v3 is a package-topology change, not primarily a behavior change.** v2's
     `client.Beta.<Xxx>API.<Method>` became v3's `client.<Xxx>API.<Method>` (the `.Beta.` accessor is gone;
     each service is a top-level field on the root `*sailpoint.APIClient`), and **almost every method name
     gained a `V1` suffix** (`GetRole`->`GetRoleV1`, `CreateWorkflow`->`CreateWorkflowV1`, etc.). Import paths
     changed from `.../v2/api_beta` to `.../v3/<service>` where `<service>` is the per-service package
     (`roles`, `sources`, `entitlements`, `segments`, `workflows`, `access_profiles`, `identity_profiles`,
     `task_management`, `service_desk_integration`, `governance_groups`, `sod_policies`, `transforms`,
     `connector_rule_management`, ...). A handful of methods renamed non-mechanically:
     `ConnectorRuleManagement.UpdateConnectorRule`->`PutConnectorRuleV1`,
     `SourcesAPI.Delete`->`DeleteSourceV1`. Mechanized the bulk of it with a throwaway `scripts/migrate_v3.py`
     (import swaps + `.Beta.` removal + method `+V1` + the
     `UpdateMultiHostSourcesRequestInnerValue`->`JsonPatchOperationValue` global rename); still required
     substantial hand-fixing per package for type-shape differences below.
  2. **Root client no longer exposes its config.** v2 code that reached `client.Beta.GetConfig()` for raw-HTTP
     fallbacks (e.g. `access_model_metadata_attribute_v1`'s hand-rolled DELETE) breaks: v3's root
     `*sailpoint.APIClient` has an unexported `cfg` and no getter, and config fields moved from
     `Configuration.{BaseURL,Token,ClientId,ClientSecret,TokenURL}` to
     `Configuration.ClientConfiguration.{...}` (`HTTPClient` stayed at `Configuration.HTTPClient`). Fix:
     `provider.go` now stashes the `*sailpoint.Configuration` at Configure() time and exposes it via a new
     `GetClientConfig()` method (alongside the existing `GetClient()`); packages needing raw HTTP take it
     through their `clientProvider` interface.
  3. **No shared error-DTO package in v3.** v2's `api_beta.ErrorResponseDto` (used by
     `util.SailpointErrorFromHTTPBody`) has no single canonical home in v3 - the identical `ErrorResponseDto`
     type is re-emitted into every per-service package. Picked one arbitrarily (`sources.ErrorResponseDto`,
     aliased `sperr`) for the shared `util` helper; harmless since the shape is identical everywhere.
  4. **Per-service type renames + Nullable/pointer shape drift are the real work.** Notable: sources'
     `MultiHost*`/`ManagerCorrelationMapping` family renamed to `Source*` and several fields flipped between
     `Nullable<T>` wrappers and plain `*T`; entitlements' API type is `EntitlementV2` (not `Entitlement`) with
     `Tags []string` + `PrivilegeLevel *EntitlementV2PrivilegeLevel` now DECLARED fields (were
     `AdditionalProperties` map keys in v2) and `ManuallyUpdatedFields` now a plain `map[string]interface{}`;
     roles/access_profiles `Owner` became `NullableOwnerReference` (must wrap via
     `roles.NewNullableOwnerReference(&roles.OwnerReference{...})`, ctors like `NewRole(name,
     NullableOwnerReference)`); segments has NO `JsonPatchOperation` type at all (`PatchSegmentV1` takes a raw
     `[]map[string]interface{}` RequestBody - hand-rolled a local `segmentJSONPatchOp` struct). Full
     field-shape catalog recorded in the sdk-type-reference file. **Because v2's
     declared-field-vs-AdditionalProperties choices differed from v3's, unit tests that seeded fixtures via
     `dto.AdditionalProperties[...]` had to switch to the new declared setters
     (`dto.SetTags(...)`/`dto.SetPrivilegeLevel(...)`), and this surfaced only at `go test` runtime, not at
     compile time** - a reminder that a clean `go build` is necessary but not sufficient for an SDK
     major-version bump; always run `go test ./...` too.
  5. **`ListWorkflowsV1` dropped ALL query params in v3** (no Filters/Sorters/Offset/Limit builder methods -
     v2 had them). Same regression class as the workflows-list gap already logged. Reworked
     `datasource_workflows.go` to warn when filters/sorters/offset are set (now unsupported server-side) and
     apply `limit` client-side by truncating. Logged as an SDK issue.
  6. **Two long-standing SDK issues are RESOLVED in v3, one NEWLY-enabled capability appeared** (all reflected
     in the sdk-issues file): (a) the Service Desk Integration `managedResourceRefs`
     `map[string]interface{}`-instead-of-`string` bug (issue #1) is FIXED - v3's type is
     `service_desk_integration.ServiceDeskSource` with `Type/Id/Name *string`, so
     `service_desk_integration_v1/sdk_fallback.go`'s workaround is now a dead (but harmless, retained) code
     path pending live-tenant verification before removal; (b) v3 now DOES publish
     `sources.SourcesAPIService.{Create,Get,Put,Delete}ProvisioningPolicyV2` (with an attachable
     `XSailPointExperimental` setter) - the exact bindings whose absence in v2 forced
     `source_provisioning_policy_v1` onto the composite-key v1 endpoints - so lifting that resource's v1-only
     limitation is now feasible as a tracked follow-up (deferred; needs live verification, out of scope for a
     static-only migration).
  7. **Test files have NO build tags in this repo**, so `*_resource_acc_test.go` (package `provider`) compile
     under `go vet`/`go test` and their imports count toward `go mod tidy` - ALL test files had to migrate
     before v2 could be removed from `go.mod`. Removed v2 only after `grep -rn 'sailpoint-oss/golang-sdk/v2'
     --include=*.go` showed zero remaining *imports* (two historical *comment* references intentionally kept).
  8. **The worktree-directory-name `make docs` gotcha (already logged twice before) bit again and this time
     masked a REAL regression.** `make docs` fails outright in a
     `terraform-provider-identitynow-v3migration`-named worktree (tfplugindocs infers provider short-name
     `identitynow-v3migration` from the dir). Bypassing with `go tool tfplugindocs generate --provider-name
     identitynow --rendered-provider-name identitynow` not only fixed the tool error but revealed a genuine
     one-file schema diff (`source_v1.md`: `category` had drifted from Read-Only back to Optional). Root
     cause: my `sources_v1` regen had run `gen-api-v1` + `apply_codespec_type_mappings.py` but SKIPPED
     `apply_codespec_schema_overrides.py`, silently dropping `schema_overrides_sources_v1.yml`'s
     `computed_only_overrides`/`strip_defaults` for `category`/`healthy`. **Lesson: when regenerating any
     target during an unrelated change, run the target's FULL replayable pipeline (gen-api -> schema_overrides
     -> type_mappings -> gen-framework), not a subset - and always use the docs regen (with the correct
     `--provider-name`) as the cross-check that a regen didn't silently drop a schema override.** Reapplied
     the full pipeline; `category` restored to Read-Only; docs then fully clean. Verified
     `identity_profile_v1` (the other regenerated target) was NOT similarly affected by reapplying its full
     pipeline and confirming zero diff.
- Delegated to: none (single primary session throughout; no sub-agent delegation).
- Validation: Phase A only (no sandbox credentials in this worktree, per task). Final state: `go build ./...`
  clean, `go vet ./...` clean, `go test ./...` = 61 packages ok / 0 failures, `make lint` (golangci-lint) 0
  issues, `make validate-examples` 53/53 passed, docs regenerate fully clean (zero diff) via the
  `--provider-name identitynow` invocation. `go.mod`/`go.sum` have ZERO `golang-sdk/v2` references; v3 is the
  sole SDK. Scratch clone `/tmp/sdk-v3-full` removed. Phase B (live `terraform plan`/`apply`) explicitly not
  run - no creds; the migration is a same-behavior SDK swap and should be smoke-tested against a sandbox
  before release.
- Guardrail update: added to the "regen discipline" lesson above - **a target's regen is all-or-nothing across
  its full script chain (gen-api -> apply_codespec_schema_overrides -> apply_codespec_type_mappings ->
  gen-framework); running only a subset silently drops overrides and is caught only by a clean docs diff.**
  Not promoted to the base agent (the specific script names are IdentityNow-repo-specific), but the general
  principle ("replay the whole replayable chain, not a subset, and use generated-docs diff as the regression
  tripwire") is worth keeping visible. No change needed to the worktree-`--provider-name` guidance (already
  documented).
