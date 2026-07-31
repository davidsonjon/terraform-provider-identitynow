# golang-sdk Upstream Issues Log

This is a **living catalog** (not a strict append-only journal like the
paired `.knowledge.md` file) of every upstream defect, gap, or notable
inconsistency found in `github.com/sailpoint-oss/golang-sdk` while building
this provider. Each entry's `Status` should be kept current as new evidence
appears (e.g. a version bump that fixes it, or a filed/resolved upstream
issue) — do not delete an entry when it's resolved, mark it `Resolved` and
say how/when. New entries should also get a matching dated entry in
`identitynow-terraform-provider-developer.knowledge.md` at time of discovery
(this file is the durable index; the knowledge file is the narrative record
of how it was found/fixed).

Scope: this file is specifically about defects/gaps in the **Go SDK package
itself** (`github.com/sailpoint-oss/golang-sdk/v2` and its `api_beta`/etc.
subpackages) — struct typing, missing methods, inconsistent shapes across
sibling types, and the SDK's internal structure/constraints. It is NOT for:
OpenAPI spec/generator_config issues in *this* repo's own codegen pipeline
(those belong in the `tfplugingen-openapi-troubleshooter`/
`tfplugingen-openapi-type-reviewer` knowledge files), or for genuine
IdentityNow API behavior quirks that have nothing to do with the SDK's Go
code (those belong in the main knowledge file's dated entries and/or the
relevant resource's package doc).

## Issue Catalog

### 1. `ProvisioningConfigManagedResourceRefsInner.Type/Id/Name` mistyped as `map[string]interface{}` instead of `string`
- **Status:** Open (upstream). Workaround shipped and stable.
- **Package/type:** `api_beta.ProvisioningConfigManagedResourceRefsInner` (file `model_provisioning_config_managed_resource_refs_inner.go`)
- **Affected versions confirmed via direct source inspection:** `v2.5.1` (this repo's original pin), `v2.7.81`, `v2.7.106` (latest as of 2026-07-24/25) — a version bump alone does **not** fix it.
- **Impact:** Any `ServiceDeskIntegrationDto` whose `provisioningConfig.managedResourceRefs[]` is non-empty fails to decode — breaks `Read`/`Import`/data-source lookup for any real service desk integration with populated managed resource refs. Newly-created resources (which start with empty `managedResourceRefs` in this provider's hand-written CRUD) are unaffected.
- **Root cause:** the field should be a plain `string` per the actual API response shape, but the SDK's OpenAPI-codegen'd struct types it as `map[string]interface{}`, causing `encoding/json` unmarshal to fail on real string values.
- **Workaround (shipped):** `internal/provider/service_desk_integration_v1/sdk_fallback.go` — `decodeServiceDeskIntegrationFallback` re-decodes the raw HTTP response body using locally-defined structs with the field correctly typed as `string`, bypassing the SDK's broken `UnmarshalJSON`. Wired in via `withManagedResourceRefsFallback` at all 4 call sites that decode a `ServiceDeskIntegrationDto` (resource Create/Read/Update, data source Read). Detection is a targeted `err.Error()` substring match (`isManagedResourceRefsTypeBug`) so unrelated errors are never masked.
- **Filed upstream?** No — user explicitly decided (2026-07-24) not to file a `sailpoint-oss/golang-sdk` GitHub issue. The workaround is this provider's sole remediation.
- **Revisit trigger:** if a future `golang-sdk` release is confirmed (via direct source inspection, not changelog alone) to fix this typing, `isManagedResourceRefsTypeBug`'s string-match guard will simply stop matching and the fallback path goes dormant harmlessly — safe to leave in place, but worth removing once confirmed fixed to reduce dead code.
- **Discovered:** 2026-07-24, live-tenant validation of `service_desk_integration_v1` against a real pre-existing object. Full repro documented in that day's knowledge entries.

### 2. `ServiceDeskIntegrationDto` has no declared `Id` field
- **Status:** Open (upstream, low severity). Workaround shipped and stable.
- **Package/type:** `api_beta.ServiceDeskIntegrationDto`
- **Impact:** The `/service-desk-integrations/v1` REST API returns an `id` in every response, but the SDK struct has no `Id` field to receive it — only reachable via the struct's `AdditionalProperties["id"]` map (the SDK's generated catch-all for undeclared JSON fields).
- **Workaround (shipped):** a small `dtoID()` helper in `resource_service_desk_integration.go`/`datasource_service_desk_integration.go` that type-asserts `dto.AdditionalProperties["id"].(string)`.
- **Filed upstream?** No.
- **Discovered:** 2026-07-24, `service_desk_integration_v1` pilot implementation.

### 3. `JsonPatchOperation.Value` is a shared oneOf wrapper type, not a plain `interface{}`
- **Status:** Not a defect — a deliberate (if awkward) SDK design choice. Documented here because it's a recurring integration cost, not a one-off.
- **Package/type:** `api_beta.JsonPatchOperation.Value`, typed `*UpdateMultiHostSourcesRequestInnerValue` (a oneOf wrapper shared across every JSON-Patch-based update endpoint in the SDK, despite the type name referencing one specific endpoint).
- **Impact:** every hand-written `Update()` that builds an RFC 6902 JSON Patch body (`service_desk_integration_v1`, `role_v1`) must construct each patch value via the wrapper's `StringAs...`/`BoolAs...`/`ArrayOfArrayInnerAs...` convenience constructors instead of assigning a plain Go value — easy to miss on a new target, and the constructor names don't obviously map to their purpose from the name alone (`ArrayOfArrayInnerAsUpdateMultiHostSourcesRequestInnerValue` for a plain list-of-objects patch value, for example).
- **Workaround/pattern (shipped):** `roleJSONPatchReplace`/`ammJSONPatchReplace`-style small wrapper helpers (see `resource_role.go`, `resource_access_model_metadata_attribute.go`) that hide the verbose constructor name behind a one-line call; array-of-object patch values go through a `roleSliceToArrayInner`/`ammSliceToArrayInner`-style struct→`map[string]interface{}`→`api_beta.ArrayInner{MapmapOfStringAny: &m}` round-trip.
- **Filed upstream?** No — not considered a bug, just an integration cost worth documenting so future targets don't rediscover the pattern from scratch.
- **Discovered:** 2026-07-24, `service_desk_integration_v1` Update() implementation; reused unmodified on `role_v1` and `access_model_metadata_attribute_v1`.

### 4. Inconsistent `Name` field typing across structurally-identical "ref" DTOs
- **Status:** Open (upstream inconsistency). Workaround shipped and stable.
- **Package/types:** `api_beta.AdditionalOwnerRef.Name` and `api_beta.EntitlementRef.Name` are typed `NullableString`, while sibling/structurally-identical reference DTOs in the very same SDK version/package — `api_beta.AccessProfileRef.Name`, `api_beta.DimensionRef.Name`, `api_beta.OwnerReference.Name` — are plain `*string`. All five are simple `{id, name, type}`-shaped reference objects with no principled reason to differ.
- **Impact:** breaks `tfplugingen-framework`'s `associated_external_type` mapping for the two `NullableString` outliers — the generated `ToApi_beta...Dto()`/`FromApi_beta...Dto()` converters (which assume a plain `*string`) fail to compile against a `NullableString` field (`cannot use v.Name.ValueStringPointer() ... as api_beta.NullableString value`).
- **Workaround (shipped):** leave `entitlements`/`additional_owners` **unmapped** (schema-native generated Value types) in `role_v1`'s generator config, and hand-write the DTO conversion in `roleModelToDto`/`roleDtoToModel` instead, constructing `api_beta.EntitlementRef{..., Name: *api_beta.NewNullableString(...)}` manually.
- **Filed upstream?** No.
- **Discovered:** 2026-07-24, `role` pipeline type-linking pass (documented as codegen pitfall #3 in that day's knowledge entry, since promoted into `tfplugingen-openapi-type-reviewer.agent.md`'s Workflow).
- **Standing guidance:** any future `associated_external_type` candidate should have its exact field types (not just field *names*) diffed against every structurally-similar sibling DTO in the same SDK package before mapping — don't assume two "ref"-shaped DTOs share field types just because they look identical in the OpenAPI schema.

### 5. `Role.AdditionalOwners`/`Role.PrivilegeLevel` absent from the pinned `v2.5.1` release
- **Status:** Resolved by version bump — **not** a persistent defect, included here only for contrast with the other entries (a reminder that not every "field missing" surprise is a permanent SDK bug).
- **Package/type:** `api_beta.Role`
- **Impact:** blocked the `role_v1` pipeline's type-linking step entirely until resolved.
- **Fix:** upgraded `github.com/sailpoint-oss/golang-sdk/v2` from the pinned `v2.5.1` to `v2.7.106` (latest at the time). Confirmed zero regressions via full `go build`/`vet`/`test`/`golangci-lint`/`make plan TARGET=service_desk_integration` re-run after the bump.
- **Discovered/resolved:** 2026-07-24, `role` pipeline.

### 6. `DeleteAccessModelMetadataAttribute`/`DeleteAccessModelMetadataAttributeValue` methods do not exist — spec omission, not a real API limitation
- **Status:** Resolved (workaround shipped) — root cause is a **published OpenAPI spec gap**, not a golang-sdk code defect per se, but recorded here because the practical symptom (generated SDK has no delete method to call) is identical to a real SDK gap and the remediation lives in this provider's SDK-adjacent code.
- **Package/type:** `api_beta.AccessModelMetadataAPIService` — has `Create`/`Get`/`Update`(PATCH)/`List` for both attributes and values, but no `Delete*` method for either.
- **Impact:** initially (2026-07-25) misdiagnosed as "no delete capability exists at all in the API," leading to a warning-only no-op `Delete()` design and permanence-caveat documentation. **Corrected 2026-07-26**: the user supplied a captured browser network call showing `DELETE /beta/access-model-metadata/attributes/{key}` succeeding from the IdentityNow Admin UI itself — the operation is real, it's simply missing from SailPoint's published spec (both the legacy `beta` and per-service `v1` specs omit it), so the SDK — generated from that spec — never got a method for it.
- **Workaround (shipped):** `internal/provider/access_model_metadata_attribute_v1/resource_access_model_metadata_attribute_delete.go` — a hand-rolled raw HTTP `DELETE` call built entirely from the SDK's own **exported** configuration/auth surface (`api_beta.Configuration.BaseURL`/`ClientId`/`ClientSecret`/`TokenURL`/`Token`/`HTTPClient` are all exported fields), replicating the same request shape and OAuth2 client-credentials/token-caching pattern the SDK's own unexported `prepareRequest`/`getAccessToken` use internally. Deliberately not a vendor fork — just an exported-surface extension.
- **Filed upstream?** No (spec gap, not a golang-sdk code issue — if filed, should go against `sailpoint-oss/api-specs`, not `golang-sdk`).
- **Revisit trigger:** if SailPoint ever publishes this operation in the spec and a future `golang-sdk` release generates a real `DeleteAccessModelMetadataAttribute` method, switch back to the generated call and delete the hand-rolled helper.
- **Discovered:** 2026-07-25 (misdiagnosed), corrected 2026-07-26.
- **Standing guidance (see also the agent's "Surface apparent missing-functionality findings" Enforcement Rule, added 2026-07-26 because of this exact case):** a missing method on a generated `*APIService` is evidence the *published spec* omits the operation, not evidence the *real API* doesn't support it — verify against real API/UI traffic or SailPoint's written docs before concluding permanence.

## Remediation-strategy notes (not per-issue, but relevant to all of them)

- **A full vendor-replace shim is impractical for single-field fixes.** `golang-sdk`'s root `client.go` unconditionally imports and wires every versioned API group (`api_beta`, `api_v3`, `api_v2024`, `api_v2025`, `api_v2026`, `api_generic`, `api_nerm`, `api_nerm_v2025`) into one `APIClient` struct with no build-tag/module separation — a local `replace` fork aimed at fixing one struct in `api_beta` would still need to vendor (approximately) the *entire* upstream module tree to compile (measured ~167M in the local module cache, 2026-07-24). This is why every issue above was remediated with a targeted, in-repo workaround (raw-JSON fallback decode, hand-rolled HTTP call, unmapped-field hand-conversion) rather than a vendored/patched SDK fork. Reconsider this tradeoff only if a *third* independent SDK defect needs the same kind of low-level struct patch that these targeted workarounds can't reach.
- **The SDK's exported `Configuration` surface is usable for hand-rolled calls.** `APIClient.GetConfig()` returns a `*Configuration` with exported `BaseURL`/`ClientId`/`ClientSecret`/`TokenURL`/`Token`/`HTTPClient` fields — sufficient to build and authenticate a raw HTTP call for any operation the generated client doesn't expose (see issue #6's workaround), without needing to fork or vendor anything.

### 7. `SourcesAPIService.Delete` (not `DeleteSource`) is a naming outlier among SDK "delete" methods
- **Status:** Open (cosmetic, not a functional defect) — recorded because it's a real trap for future contributors grepping for `DeleteSource`.
- **Package/type:** `api_beta.SourcesAPIService` — its delete operation is generated as plain `Delete(ctx, id)`, unlike most other `*APIService` types in this SDK which consistently name their delete method `Delete<Resource>` (e.g. `DeleteGovernanceGroup`, `DeleteRole`, `DeleteAccessProfile`).
- **Impact:** minor — a `grep -n "func.*DeleteSource"` or similarly-named search turns up nothing and could lead a contributor to (wrongly) conclude the method doesn't exist. Also, `Delete` returns 3 values (`api_beta.Delete202Response`, `*http.Response`, `error`), not the simpler 2-value `(*http.Response, error)` shape several other resources' Delete calls use — the 202 response carries a `{Type, Id, Name}` reference to the async account-removal task IdentityNow kicks off before the source itself is deleted.
- **Workaround:** none needed beyond awareness — `sources_v1`'s `resource_source.go` calls `client.Beta.SourcesAPI.Delete(ctx, id).Execute()` and discards the `Delete202Response` return value (fire-and-forget, consistent with every other `_v1` pilot's Delete() convention — the async account-removal task is not polled to completion).
- **Filed upstream?** No (cosmetic naming inconsistency, low priority).
- **Discovered:** 2026-07-28, `sources` pipeline.

### 8. Vendored SDK operator-precedence bug breaks any generated method with an optional `*os.File` param
- **Status:** Open (upstream). Workaround shipped and stable.
- **Package/type:** `golang-sdk/v2@v2.7.106`'s `api_beta/client.go` `prepareRequest` (~line 539): `(hasMultipartPrefix && len(formParams)>0) || len(formFiles)>0` — an operator-precedence bug means any generated method with an optional `*os.File` parameter (e.g. `ImportEntitlements`) sends `Content-Type: multipart/form-data` with no boundary and an empty body when no file is supplied.
- **Impact:** the live API deterministically rejects the malformed request with `HTTP 500` whenever such a method is called with a `nil`/omitted file argument — not specific to `ImportEntitlements`, any SDK method sharing this `prepareRequest` code path with an optional-file param is potentially affected (not yet individually confirmed for `ImportAccounts`/`ImportUncorrelatedAccounts`).
- **Workaround (shipped):** always pass a real (even throwaway/empty) `*os.File` rather than `nil` when calling an affected method — confirmed working for `ImportEntitlements` in `entitlement_v1`.
- **Filed upstream?** No.
- **Revisit trigger:** if a future `golang-sdk` release fixes the precedence bug, the throwaway-file workaround becomes unnecessary but remains harmless — safe to leave in place, worth removing once confirmed fixed.
- **Discovered:** 2026-07-30/31, `entitlement_v1`'s `ImportEntitlements` integration (see the matching knowledge.md entry for full repro).


### 9. `GetConnectorRuleList`'s generated request builder has no pagination/filter methods despite the spec declaring them
- **Status:** Open (upstream). No workaround shipped — the affected data source's scope was reduced instead (see below).
- **Package/type:** `api_beta.ConnectorRuleManagementAPIService.GetConnectorRuleList` / `api_beta.ApiGetConnectorRuleListRequest` — confirmed via direct source inspection the request type has **zero** builder methods beyond `ctx`/`Execute()` (no `Limit`/`Offset`/`Count`), even though the OpenAPI spec for `GET /connector-rules/v1` declares `limit`/`offset`/`count` query parameters.
- **Impact:** `identitynow_connector_rules_v1` (the plural data source) has no filtering/pagination support at all — it always returns every connector rule in the tenant on every read, unlike every other plural data source in this repo. Documented explicitly in `datasource_connector_rules.go`'s package doc and the resource's own docs/example so practitioners aren't surprised by the lack of `limit`/`filters` arguments.
- **Workaround:** none — the data source's schema simply omits the attributes that would map to the missing builder methods; there is no raw-HTTP fallback for this one since the missing capability is convenience/scale, not a functional blocker (small-to-medium tenants are unaffected).
- **Filed upstream?** No.
- **Discovered:** 2026-07-31, `connector_rule_v1` pipeline.
