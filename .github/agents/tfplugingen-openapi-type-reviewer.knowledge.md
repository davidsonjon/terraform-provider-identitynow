# tfplugingen-openapi Type Reviewer Knowledge

## Entry Template
For a type-mapping review entry (specific target reviewed):
- Date: YYYY-MM-DD
- Target: <target>
- Path(s): JSON path(s) updated
- Mapping: associated_external_type/custom_type value added
- Evidence: SDK symbol or in-repo example
- Validation: command and outcome
- Follow-up: uncertain mappings left unchanged

For a review/authoring-pass entry (no specific target being mapped):
- Date: YYYY-MM-DD
- Task: review/cleanup/authoring scope of the pass
- Summary: what was checked and what (if anything) was changed
- Validation: how the change was confirmed correct
Use whichever template fits the entry - do not force a review pass into the Target/Path(s)/Mapping fields, or vice versa.

## Entries
- Date: 2026-05-30
- Target: role
- Path(s): resource.role.* nested blocks including owner, additional_owners, access_profiles, entitlements, membership, access_request_config, revocation_request_config, dimension_refs, access_model_metadata
- Mapping: added 13 `associated_external_type` mappings; no `custom_type` added
- Evidence: SDK v2.7.81 api_beta model types (`Role`, `OwnerReference`, `AdditionalOwnerRef`, `AccessProfileRef`, `EntitlementRef`, `RoleMembershipSelector`, `RoleMembershipIdentity`, `RequestabilityForRole`, `RevocabilityForRole`, `ApprovalSchemeForRole`, `AccessDuration`, `DimensionRef`, `AttributeDTOList`) and in-repo style from `openapi_code_spec_with_types.json`
- Validation: `jq . openapi_code_spec/openapi_code_spec_role.json` succeeded; mapping count now 13
- Follow-up: membership criteria recursive blocks (`criteria`/`key` levels) not mapped because generated shape uses multiple level-specific DTOs and requires additional verification before safe binding

- Date: 2026-05-30
- Target: role
- Path(s): resource.role.owner, resource.role.additional_owners[], resource.role.access_profiles[], resource.role.entitlements[], resource.role.dimension_refs[], resource.role.access_request_config.max_permitted_access_duration
- Mapping: retained 6 framework-compatible `associated_external_type` mappings after rerun; removed deeper nested parent mappings that broke framework generation
- Evidence: `make gen-framework-api TARGET=role` failed when parent mappings covered list/single-nested unsupported paths (`access_model_metadata.attributes`, `access_request_config.approval_schemes`, `membership.criteria`, `revocation_request_config.approval_schemes`) and succeeded after trimming to leaf/reference-compatible mappings
- Validation: `make gen-framework-api TARGET=role` succeeded; final mapping count 6
- Follow-up: avoid parent `associated_external_type` on blocks containing unsupported nested list/single structures until framework gains deeper to/from support

- Date: 2026-07-24
- Target: transform
- Path(s): none - reviewed `openapi_code_spec/openapi_code_spec_transform_v1.json` in full and found zero candidate blocks for `associated_external_type`. The entire generated schema (both resource and data source) is plain scalars: `name`/`type` (string, with `stringvalidator.OneOf`/`LengthBetween`), `id` (string), `internal` (bool, with a `booldefault.StaticBool(false)` default). The one field that *would* normally need a mapping decision - `attributes`, a discriminated union across ~35 "type" values with genuinely arbitrary-depth children - was excluded from codegen entirely (`generator_config_transform_v1.yml`'s `schema.ignores: [attributes]`) per the 2026-07-24 dynamic-attributes-pattern-research decision (hand-added as a `jsontypes.Normalized` JSON-string `CustomType` instead of any generated/mapped block), so it never reached this review step at all.
- Mapping: none added.
- Evidence: n/a (no mapping made) - confirmed via `jq` inspection that no `single_nested`/`list_nested` blocks exist anywhere in the code spec to consider.
- Validation: `python3 -m json.tool openapi_code_spec/openapi_code_spec_transform_v1.json` visually confirmed the flat attribute list (4 attributes, all scalar) for both `resources[0]` and `datasources[0]`; `make gen-framework-api-v1 TARGET=transform` succeeded on the first attempt with zero framework-generation errors, consistent with there being no mapping-related risk surface at all.
- Follow-up: **this is the expected/normal outcome whenever a target's only "interesting" field is a schema.ignores'd dynamic-shape attribute (like `attributes` here, or `connectorAttributes` for the still-unimplemented `source` target)** - do not skip running this review step just because a target looks "simple" pre-review; the symbol-collision scan (Workflow step 2a) and the "confirm zero candidates" finding are both still meaningful, reportable outcomes, not a no-op to be silently assumed.

- Date: 2026-07-25
- Task: review/cleanup pass across all agents/prompts
- Summary: No unused/orphaned agent, knowledge, or prompt files found in `.github/agents/`/`.github/prompts/` - every agent has exactly one matching knowledge file and one matching prompt, and all referenced `make` targets exist. Found and fixed internal drift instead: this agent's own Mission/Workflow text only referenced `openapi_code_spec/openapi_code_spec_<target>.json` (legacy naming) even though its own "In-repo pattern example" reference already pointed at `openapi_code_spec_service_desk_integration_v1.json` (per-service v1 naming) - inconsistent. Added explicit legacy-vs-v1 pipeline branching to the Mission, argument-hint, Workflow step 1 (file location/generation command), and step 4 (validation command), matching the same fix applied to the paired `tfplugingen-openapi-runner.prompt.md`/`tfplugingen-openapi-type-review.prompt.md`.
- Validation: re-parsed YAML front matter (clean); confirmed both `openapi_code_spec_service_desk_integration_v1.json` and `openapi_code_spec_transform_v1.json` exist on disk as the in-repo v1-naming precedents this agent's text now correctly describes.
