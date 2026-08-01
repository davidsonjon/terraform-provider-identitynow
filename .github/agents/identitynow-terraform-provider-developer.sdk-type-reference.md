# IdentityNow golang-sdk Type-Shape Reference

Append-only-ish catalog (edits to correct a stale entry are fine; don't
delete a historically-useful entry without a note) of confirmed field shapes
for `github.com/sailpoint-oss/golang-sdk/v2` structs used by this provider's
type-linking step (`tfplugingen-openapi-type-reviewer`'s review, applied via
`scripts/apply_codespec_type_mappings.py`). Exists to replace repeated
`go list -m` + `grep`/`view` derivation of the same struct shapes across
sessions with a single durable, checked-in source — per the
[terraform-provider-developer](terraform-provider-developer.agent.md) base
agent's "prefer generation/config over repeated dynamic derivation"
principle.

**How to resolve the SDK source location** (never hardcode an absolute
path — it won't exist on another machine/session):
```
go list -m -f '{{.Dir}}' github.com/sailpoint-oss/golang-sdk/v2
```
All entries below were confirmed against `v2.7.106` unless noted otherwise.
Re-confirm (`grep -A15 "^type <Name> struct" "$(go list -m -f '{{.Dir}}' ...)/api_beta/model_<snake_name>.go"`)
after any SDK version bump before trusting a stale entry for a new target.

## Entry Template
```
### <TypeName>
- **File**: api_beta/model_<snake_case_name>.go
- **Confirmed against**: v2.x.x (date)
- **Shape**: <plain-English summary + field table or excerpt>
- **Notes**: <Nullable vs pointer quirks, recursion, shared-type usage, oddities>
- **Used by targets**: <role_v1, access_profile_v1, ...>
```

---

### OwnerReference
- **File**: `api_beta/model_owner_reference.go`
- **Confirmed against**: v2.7.106 (2026-07-27)
- **Shape**: `Type *string`, `Id *string`, `Name *string` (all plain pointers, not `NullableString`), plus `AdditionalProperties map[string]interface{}`.
- **Notes**: Simple leaf ref type — no nested blocks. Safe to `associated_external_type` map directly.
- **Used by targets**: `access_profile_v1` (`owner`), `role_v1` (`owner`).

### EntitlementRef
- **File**: `api_beta/model_entitlement_ref.go`
- **Shape**: `Type *string`, `Id *string`, `Name NullableString` (note: `Name` here is `NullableString`, unlike `OwnerReference.Name` which is a plain `*string` — don't assume consistency across similarly-named ref types).
- **Notes**: `NullableString` (a distinct SDK wrapper type, not a plain pointer) requires `.Get()`/`.Set()`/`NewNullableString(...)` rather than direct pointer dereference/assignment.
- **Used by targets**: (reserved for future entitlement-related targets; not yet used by role_v1/access_profile_v1 directly).

### AdditionalOwnerRef
- **File**: `api_beta/model_additional_owner_ref.go`
- **Shape**: `Type *string`, `Id *string`, `Name NullableString`.
- **Notes**: Same `NullableString` quirk as `EntitlementRef.Name`. **Confirmed 2026-07-28: do not attempt to map this via `associated_external_type`** — tried it against `access_profile_v1`'s `additional_owners` field (a full regenerate-then-revert round trip: backed up the generated Go + code spec, added the mapping, ran `make gen-framework-api-v1`, hit the exact predicted compile error `cannot use v.Name.ValueStringPointer() ... as api_beta.NullableString value`, then restored from backup), which is exactly the type-reviewer's documented "NullableString incompatibility" pitfall. The existing hand-written field-by-field conversion in `resource_access_profile.go`/`access_profile_data_source.go` (manually building `[]api_beta.AdditionalOwnerRef` from the schema-native `AdditionalOwnersValue` struct, one field at a time) is the correct, intentional design for this field — not a gap to close, unless a future SDK version changes `Name` to a plain `*string`.
- **Used by targets**: `access_profile_v1` (`additional_owners`, list_nested — schema-native, hand-converted, deliberately NOT `associated_external_type`-mapped; see note above).

### AccessProfileSourceRef
- **File**: `api_beta/model_access_profile_source_ref.go`
- **Shape**: `Id *string`, `Type *string`, `Name *string` — all plain pointers.
- **Used by targets**: `access_profile_v1` (`source`).

### AccessProfileApprovalScheme
- **File**: `api_beta/model_access_profile_approval_scheme.go`
- **Shape**: `ApproverType *string`, `ApproverId NullableString`.
- **Notes**: The `ApproverType` field's description text is copy-pasted from a shared type and mentions "Owner of the associated Access Profile **or Role**" even in access-profile-only contexts — a genuine upstream spec/doc artifact (SailPoint's own description text is shared across Role and Access Profile approval schemes), not a bug in this provider or a hallucination. Confirmed 2026-07-27 while reviewing `access_profile_v1` docs.
- **Used by targets**: `access_profile_v1` (`access_request_config.approval_schemes`, `revocation_request_config.revocation_approval_schemes`), `role_v1` (equivalent fields).

### AccessDuration
- **File**: `api_beta/model_access_duration.go`
- **Shape**: `Value *int32`, `TimeUnit *string`.
- **Used by targets**: `access_profile_v1` (`access_request_config.max_permitted_access_duration`).

### AttributeDTOList / AttributeDTO / AttributeValueDTO
- **Files**: `api_beta/model_attribute_dto_list.go`, `model_attribute_dto.go`, `model_attribute_value_dto.go`
- **Shape**: `AttributeDTOList{ Attributes []AttributeDTO }`. `AttributeDTO{ Key, Name, Multiselect *bool, Status, Type, ObjectTypes []string, Description *string, Values []AttributeValueDTO }`. `AttributeValueDTO{ Value, Name, Status *string }`.
- **Notes**: This is the `access_model_metadata` shape shared across access-profile/role — see the `AccessProfileApprovalScheme` note above re: shared "Role" language; same root cause here (SailPoint's `access_model_metadata` schema is genuinely shared/reused verbatim across both Role and Access Profile in the upstream spec, not something this provider invented). Only `AttributeValueDTO` (the innermost leaf) is `associated_external_type`-mapped in practice — the outer `AttributeDTOList`/`AttributeDTO` levels contain further nested `list_nested` children (`Values`), so per the type-reviewer's leaf-only-mapping rule they are NOT mapped themselves.
- **Used by targets**: `access_profile_v1`, `role_v1` (`access_model_metadata.attributes.values` → `*api_beta.AttributeValueDTO`).

### JsonPatchOperation / UpdateMultiHostSourcesRequestInnerValue / ArrayInner
- **Files**: `api_beta/model_json_patch_operation.go`, `model_update_multi_host_sources_request_inner_value.go`, `model_array_inner.go`
- **Shape**: `JsonPatchOperation{ Op string, Path string, Value *UpdateMultiHostSourcesRequestInnerValue }`. `UpdateMultiHostSourcesRequestInnerValue` is a **oneOf wrapper struct** with mutually-exclusive fields `ArrayOfArrayInner *[]ArrayInner`, `Bool *bool`, `Int32 *int32`, `MapmapOfStringAny *map[string]interface{}`, `String *string` (only one should be set at a time). `ArrayInner` is itself a smaller oneOf wrapper (`Int32`, `MapmapOfStringAny`, `String` — no bool, no nested array).
- **Notes**: Despite its "MultiHostSources"-specific name, this generic oneOf wrapper is reused by the SDK for **any** JSON-Patch `Value` field across services — the name is a generator artifact from whichever endpoint's spec happened to define the oneOf first, not a signal that it's multi-host-source-specific. Use its `.StringAs...(...)`/`MapmapOfStringAnyAs...(...)`/etc. convenience constructors (generated alongside the struct) rather than raw field assignment, to get correct marshaling.
- **Used by targets**: any hand-written CRUD using `JsonPatchOperation` for an `Update`, e.g. `service_desk_integration_v1`.

### ProvisioningCriteriaLevel1 / Level2 / Level3 / ProvisioningCriteriaOperation
- **Files**: `api_beta/model_provisioning_criteria_level1.go`, `_level2.go`, `_level3.go`, `model_provisioning_criteria_operation.go`
- **Shape**: A 3-level fixed-depth recursive tree (not infinitely recursive): `Level1.Children []Level2`, `Level2.Children []Level3`, but **`Level3.Children` is `NullableString`, not `[]Level4`** — the recursion is capped at exactly 3 levels by construction (matches the spec's own comment: "A maximum of three levels of criteria are supported, including leaf nodes"). `Operation *ProvisioningCriteriaOperation` (a `string`-based enum type, not a struct) is present at all 3 levels; `Attribute`/`Value` are both `NullableString` at all 3 levels.
- **Notes**: Don't assume `Level3.Children` continues the same list-of-next-level pattern — it's a genuinely different type (`NullableString`), presumably a spec/generator quirk for representing "no further children" at the deepest level rather than an actual usable string field. Treat it as effectively unused/ignorable when mapping.
- **Used by targets**: not yet used by any current target's `associated_external_type` mappings (captured here proactively since it's a distinctive recursive shape likely to recur for a future provisioning-criteria-bearing target, e.g. `source`).

### WorkgroupDto
- **File**: `api_beta/model_workgroup_dto.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: `Owner *WorkgroupDtoOwner`, `Id *string`, `Name *string`, `Description *string`, `MemberCount *int64`, `ConnectionCount *int64`, `Created *SailPointTime`, `Modified *SailPointTime`, plus `AdditionalProperties map[string]interface{}`.
- **Notes**: Unlike `api_beta.ServiceDeskIntegrationDto` (sdk-issues.md entry #2), `Id` is a real, properly-typed declared field here - no `AdditionalProperties["id"]` workaround needed. `Created`/`Modified` are genuine typed `*SailPointTime` (embeds `time.Time`) fields, read back unconditionally on every response - use `.Format(time.RFC3339)`, matching the existing pattern in `access_profile_v1`/`role_v1` (NOT `service_desk_integration_v1`, which hardcodes both to nil since its DTO has no typed Created/Modified fields at all - don't assume that hardcoded-nil pattern generalizes).
- **Used by targets**: `governance_group_v1`.

### WorkgroupDtoOwner
- **File**: `api_beta/model_workgroup_dto_owner.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: `Type *string`, `Id *string`, `Name *string`, `DisplayName *string`, `EmailAddress *string` - all plain pointers, plus `AdditionalProperties map[string]interface{}`.
- **Notes**: A leaf ref-like type similar in shape to `OwnerReference`/`AccessProfileSourceRef`, but with two extra read-only fields (`DisplayName`/`EmailAddress`) not present on those. No `NullableString` outliers on `Name` (unlike `AdditionalOwnerRef`/`EntitlementRef.Name`) - safe to `associated_external_type` map directly as a leaf `single_nested` block.
- **Used by targets**: `governance_group_v1` (`owner`).

### Source
- **File**: `api_beta/model_source.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: `Id *string`, `Name string`, `Description *string`, `Owner NullableSourceOwner` (required in the schema but wrapped Nullable in the SDK - `.Get()`/`.Set()`), `Cluster *MultiHostIntegrationsCluster`, `AccountCorrelationConfig *MultiHostSourcesAccountCorrelationConfig`, `AccountCorrelationRule *MultiHostSourcesAccountCorrelationRule`, `ManagerCorrelationMapping *ManagerCorrelationMapping`, `ManagerCorrelationRule *MultiHostSourcesManagerCorrelationRule`, `BeforeProvisioningRule *MultiHostSourcesBeforeProvisioningRule`, `Schemas []MultiHostSourcesSchemasInner`, `PasswordPolicies []MultiHostSourcesPasswordPoliciesInner`, `Type *string`, `Connector string` (required), `ConnectorClass *string`, `ConnectorAttributes map[string]interface{}` (dynamic/connector-type-discriminated - hand-modeled as `jsontypes.Normalized`, not mapped), `DeleteThreshold *int32`, `Features []string`, `ManagementWorkgroup *MultiHostIntegrationsManagementWorkgroup`, `Authoritative *bool`, `Category NullableString`, `CredentialProviderEnabled *bool`, `ConnectorId *string`, `ConnectorName *string`, `ConnectorImplementationId *string`, `ConnectionType *string`, `Created *SailPointTime`, `Modified *SailPointTime`, `Status *string`, `Since *string`, `Healthy *bool`, plus `AdditionalProperties map[string]interface{}`.
- **Notes**: `Name`/`Connector` are the only two plain (non-pointer, non-Nullable) required scalar fields at the top level - everything else is a pointer or a Nullable wrapper, even several fields the OpenAPI schema marks `required` at the top level (`owner` is `required` in the schema but still `NullableSourceOwner` in the SDK). Don't assume top-level schema `required`-ness predicts SDK field shape - it doesn't, consistently, across this SDK.
- **Used by targets**: `sources_v1`.

### MultiHostIntegrationsCluster (rejected as a type mapping - hand-converted instead)
- **File**: `api_beta/model_multi_host_integrations_cluster.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: `Id string`, `Name string`, `Type string` - all **plain, non-pointer, always-required** (no pointer, no Nullable wrapper) fields, plus `AdditionalProperties map[string]interface{}`.
- **Notes**: This is a *new* variant of the NullableString-incompatibility failure class (sdk-issues.md #4): here the incompatibility is caused by the nested object's own OpenAPI `required: [id, name, type]` list making the SDK codegen emit non-pointer fields, which breaks `tfplugingen-framework`'s generated `ToApi_beta.../FromApi_beta...` converter template (it universally assumes pointer semantics, calling `.ValueStringPointer()`/`types.StringPointerValue(...)`). **Do not attempt to `associated_external_type` map this type** - hand-convert instead (see `sources_v1`'s `clusterFromAPI`/`datasourceClusterFromAPI` helpers, built via the generated `NewClusterValueNull()`/`NewClusterValue(attributeTypes, map[string]attr.Value)` constructors). Before finalizing any future nested-object type mapping, check the nested schema's own `required` list, not just whether the top-level/sibling fields are Nullable - both are independent triggers for this same failure class.
- **Used by targets**: `sources_v1` (schema-native `ClusterValue`, hand-converted - not `associated_external_type` mapped).

### SourceOwner / MultiHostSourcesAccountCorrelationConfig / MultiHostSourcesAccountCorrelationRule / ManagerCorrelationMapping / MultiHostSourcesManagerCorrelationRule / MultiHostSourcesBeforeProvisioningRule / MultiHostIntegrationsManagementWorkgroup / MultiHostSourcesSchemasInner / MultiHostSourcesPasswordPoliciesInner
- **Files**: `api_beta/model_source_owner.go`, `model_multi_host_sources_account_correlation_config.go`, `model_multi_host_sources_account_correlation_rule.go`, `model_manager_correlation_mapping.go`, `model_multi_host_sources_manager_correlation_rule.go`, `model_multi_host_sources_before_provisioning_rule.go`, `model_multi_host_integrations_management_workgroup.go`, `model_multi_host_sources_schemas_inner.go`, `model_multi_host_sources_password_policies_inner.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: All 9 are leaf-only, plain-pointer-field structs (no further nested structs, no NullableString outliers, no non-pointer required-field traps like `MultiHostIntegrationsCluster` above) - each safe to `associated_external_type` map directly. `MultiHostSourcesBeforeProvisioningRule`/`MultiHostSourcesManagerCorrelationRule`/`MultiHostSourcesAccountCorrelationRule` are all `{Type *string, Id *string, Name *string}` - structurally identical to each other and to `service_desk_integration_v1`'s own distinct `BeforeProvisioningRuleDto`, but each is a genuinely separate Go type; do not conflate them when mapping a new target.
- **Used by targets**: `sources_v1` (`owner`, `account_correlation_config`, `account_correlation_rule`, `manager_correlation_mapping`, `manager_correlation_rule`, `before_provisioning_rule`, `management_workgroup`, `schemas`, `password_policies`).

### Delete202Response
- **File**: `api_beta/model_delete202_response.go`
- **Confirmed against**: v2.7.106 (2026-07-28)
- **Shape**: `Type *string`, `Id *string`, `Name *string` - a plain task-reference DTO returned by `DELETE /sources/v1/{id}`'s `202 Accepted` response, referencing the async account-removal task IdentityNow queues before the source itself is deleted.
- **Notes**: Not polled to completion by `sources_v1`'s `Delete()` - fire-and-forget, matching every other `_v1` pilot's Delete() convention.
- **Used by targets**: `sources_v1` (return value discarded).

### ProvisioningPolicyDto
- **File**: `api_beta/model_provisioning_policy_dto.go`
- **Confirmed against**: v2.7.106 (2026-08-01)
- **Shape**: `Name NullableString` (required in the schema but Nullable in the SDK), `Description *string`, `UsageType *UsageType` (string-based enum type), `Fields []FieldDetailsDto`, plus `AdditionalProperties map[string]interface{}`. **No native `id` field at all.**
- **Notes**: Because there's no real `id`, `source_provisioning_policy_v1`'s Terraform `id` is a hand-synthesized `<source_id>/<usage_type>` composite (matching `application_access_association_v1`'s convention), not anything derived from this DTO. `Name`'s `NullableString` typing despite being schema-required is consistent with this SDK's general pattern of top-level `required` not predicting field shape (see `Source`'s entry above).
- **Used by targets**: `source_provisioning_policy_v1`.

### FieldDetailsDto (excluded from codegen - hand-modeled as `jsontypes.Normalized`)
- **File**: `api_beta/model_field_details_dto.go`
- **Confirmed against**: v2.7.106 (2026-08-01)
- **Shape**: Includes its own `Transform map[string]interface{}` (a discriminated union keyed by a sibling `type`, identical in kind to `transform_v1`'s top-level `attributes`) and `Attributes map[string]interface{}` (transform-specific parameters, also dynamic).
- **Notes**: Because *each element* of the `fields` array has its own dynamic sub-shape, the entire array (not just one field) was excluded via `schema.ignores` and hand-added as a single `jsontypes.Normalized` JSON string on the parent resource, per the established dynamic/discriminated-union pattern - not mapped to this Go type at all.
- **Used by targets**: `source_provisioning_policy_v1` (not directly - superseded by the JSON-string `fields` attribute).

### Schema (source schema)
- **File**: `api_beta/model_schema.go`
- **Confirmed against**: v2.7.106 (2026-08-01)
- **Shape**: `Id *string`, `Name *string`, `NativeObjectType *string`, `IdentityAttribute *string`, `DisplayAttribute *string`, `HierarchyAttribute NullableString`, `IncludePermissions *bool`, `Features []string`, `Configuration map[string]interface{}` (dynamic, hand-modeled as `jsontypes.Normalized`), `Attributes []AttributeDefinition`, `Created *SailPointTime` (plain pointer, NOT Nullable), `Modified NullableTime`.
- **Notes**: `Created`/`Modified` have asymmetric shapes (`*SailPointTime` vs `NullableTime`) despite both being similarly-named timestamp fields - don't assume sibling timestamp fields share the same wrapper convention. `HierarchyAttribute` is the only plain string-ish field that's `NullableString` here; the rest are plain pointers.
- **Used by targets**: `source_schema_v1`.

### AttributeDefinition / AttributeDefinitionSchema
- **Files**: `api_beta/model_attribute_definition.go`, `api_beta/model_attribute_definition_schema.go`
- **Confirmed against**: v2.7.106 (2026-08-01)
- **Shape**: `AttributeDefinition`: `Name *string`, `NativeName NullableString`, `Type *AttributeDefinitionType` (string-based enum), `Schema NullableAttributeDefinitionSchema`, `Description *string`, `IsMulti *bool`, `IsEntitlement *bool`, `IsGroup *bool`. `AttributeDefinitionSchema`: `Type *string`, `Id *string`, `Name *string` - all plain pointers, no Nullable outliers, safe to map directly if ever needed as a standalone type.
- **Notes**: `AttributeDefinition.Schema` (a pointer to another Schema, used e.g. by an account schema's `memberOf`-style attribute referencing a group schema) is `NullableAttributeDefinitionSchema`, requiring `.Get()`/`.Set()` handling exactly like other `Nullable<Type>` wrappers in this SDK - both `AttributeDefinition` and its nested `AttributeDefinitionSchema` were hand-converted (not `associated_external_type` mapped) since `tfplugingen-framework`'s generated converter template doesn't handle the `NullableAttributeDefinitionSchema` wrapper automatically, consistent with every other `Nullable*`-typed nested-object field found in this SDK so far.
- **Used by targets**: `source_schema_v1` (`attributes` list-nested attribute).
