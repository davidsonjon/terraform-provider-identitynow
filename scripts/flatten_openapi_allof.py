#!/usr/bin/env python3
"""Flattens simple, all-object `allOf` compositions in a bundled+dereferenced
per-service v1 OpenAPI spec (api-specs/dereferenced/deref-<service>.v1.yaml)
in place, so `tfplugingen-openapi generate` can map the resulting schema
("schema composition is currently not supported" is a hard limitation of
that tool, not something this provider's generator_config can work around).

This is a checked-in generalization of an ad hoc `/tmp` script written
independently for several targets in a row (transform, governance_group,
sources, identity_profile - see the dated entries in
.github/agents/identitynow-terraform-provider-developer.knowledge.md) -
promoted here per that knowledge file's own "if a 5th target hits this same
shape, promote the script" guardrail note.

What it does, recursively, for every dict containing an `allOf` key whose
value is a list:
  - If every member of that list is itself a mapping that is either
    `type: object` (or has no explicit `type` but declares `properties`,
    matching how dereferenced $refs commonly look) - i.e. every member is a
    genuine object schema, not a scalar/enum/oneOf shape - the members are
    merged into a single flat object:
      * `properties` are unioned (later members win on key collision; a
        warning is printed for any collision, since that's usually a sign
        the flatten target needs a closer look, not treated as fatal).
      * `required` arrays are unioned (order-preserving, de-duplicated).
      * Any other keys each member carries at the object level (`nullable`,
        `description`, `title`, `additionalProperties`, `default`, etc.) are
        copied in, first-seen-wins, then the *original* dict's own sibling
        keys (anything already alongside the `allOf` key itself, e.g. a
        modifier-only `nullable: true` next to `allOf: [...]`) always take
        precedence over anything copied from the members.
      * The `allOf` key itself is removed and replaced with the merged
        object's keys directly on the original dict.
  - If any member isn't a mergeable object schema (e.g. a `type: string,
    enum: [...]` variant, or a member using `oneOf`/`anyOf` itself), the
    `allOf` is left completely untouched and a warning is printed - this
    script intentionally only handles the narrow "all-object-typed allOf"
    case; a whole-response `oneOf`-style composition needs a human decision,
    not an automatic merge.

Usage:
    python3 scripts/flatten_openapi_allof.py api-specs/dereferenced/deref-<service>.v1.yaml

Rewrites the file in place. Re-run `make gen-api-v1` afterward to confirm the
"schema composition is currently not supported" warnings are gone; if any
remain, inspect the surviving warnings' `oas_line_number` - they are the
genuinely out-of-scope compositions this script correctly declined to touch.
"""

import sys

import yaml

# Schema-structural keys: if a member declares any of these, it's making a
# real type/shape assertion (not just annotating), so it must go through
# _is_mergeable_object's stricter "plain object schema" check instead of
# being treated as metadata-only.
_SCHEMA_STRUCTURAL_KEYS = (
    "type", "properties", "additionalProperties", "required", "items",
    "enum", "oneOf", "anyOf", "allOf", "$ref", "format", "pattern",
    "minimum", "maximum", "minLength", "maxLength", "minItems", "maxItems",
    "uniqueItems", "multipleOf",
)


def _is_mergeable_object(member):
    """A member is safe to flatten if it's a mapping shaped like a plain
    object schema: either explicit `type: object`, or no `type` at all but
    at least one object-ish key (`properties`, `additionalProperties`,
    `required`) - which is how a dereferenced `$ref` to a base object schema
    commonly looks once bundled. Anything else (scalars, enums, oneOf/anyOf
    members) is not mergeable and forces the whole allOf to be left alone.
    """
    if not isinstance(member, dict):
        return False
    declared_type = member.get("type")
    if declared_type is not None:
        return declared_type == "object"
    return any(k in member for k in ("properties", "additionalProperties", "required"))


def _is_metadata_only(member):
    """A member is safe to flatten (as a no-op merge target) if it carries
    no schema-structural keys at all - just annotation-style keys like
    `description`, `nullable`, `example`, `title`, `deprecated`, `default`,
    etc. This is the common `allOf: [<$ref-object>, {nullable: true}]`
    pattern OpenAPI tooling emits for an "optional/nullable ref" (first seen
    on `entitlement_v1`'s `privilegeLevel` - see the dated knowledge.md
    entry). `_merge_allof` already copies these keys onto the merged object
    generically; this predicate just lets `_walk` recognize such a member as
    mergeable instead of bailing out the whole allOf.
    """
    if not isinstance(member, dict):
        return False
    return not any(k in _SCHEMA_STRUCTURAL_KEYS for k in member)


def _merge_allof(members, original_siblings):
    merged: dict = {"type": "object"}
    properties: dict = {}
    required: list = []
    seen_required = set()

    for member in members:
        for key, val in member.get("properties", {}).items():
            if key in properties and properties[key] != val:
                print(f"    WARN: property {key!r} redefined across allOf members - using the later definition", file=sys.stderr)
            properties[key] = val
        for req in member.get("required", []):
            if req not in seen_required:
                seen_required.add(req)
                required.append(req)
        for key, val in member.items():
            if key in ("properties", "required", "type"):
                continue
            merged.setdefault(key, val)

    if properties:
        merged["properties"] = properties
    if required:
        merged["required"] = required

    # Sibling keys already present alongside the original `allOf` key (e.g. a
    # modifier-only `nullable: true`) always win over anything copied from
    # the members themselves.
    for key, val in original_siblings.items():
        if key == "allOf":
            continue
        merged[key] = val

    return merged


def _walk(node, path="$"):
    """Recursively walks a loaded YAML structure, flattening any mergeable
    `allOf` in place. Returns nothing - mutates dicts/lists it's given
    directly, exactly like the ad hoc predecessor scripts did.
    """
    if isinstance(node, dict):
        allof = node.get("allOf")
        if isinstance(allof, list) and allof:
            if all(_is_mergeable_object(m) or _is_metadata_only(m) for m in allof):
                print(f"  Flattening allOf at {path} ({len(allof)} members)")
                merged = _merge_allof(allof, node)
                node.clear()
                node.update(merged)
            else:
                print(f"  WARN: leaving non-object-only allOf untouched at {path} (not all members are plain object schemas)", file=sys.stderr)
        for key, val in list(node.items()):
            _walk(val, f"{path}.{key}")
    elif isinstance(node, list):
        for i, item in enumerate(node):
            _walk(item, f"{path}[{i}]")


def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <path/to/deref-spec.v1.yaml>", file=sys.stderr)
        sys.exit(1)

    spec_path = sys.argv[1]
    with open(spec_path, "r") as f:
        spec = yaml.safe_load(f)

    _walk(spec)

    with open(spec_path, "w") as f:
        yaml.safe_dump(spec, f, sort_keys=False, default_flow_style=False, allow_unicode=True)

    print(f"Rewrote {spec_path} with any mergeable allOf compositions flattened.")


if __name__ == "__main__":
    main()
