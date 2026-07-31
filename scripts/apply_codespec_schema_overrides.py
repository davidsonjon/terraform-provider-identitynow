#!/usr/bin/env python3
"""Apply durable, checked-in schema-shape edits (e.g. required-ness fixes) to
a freshly generated openapi_code_spec_<target>.json (or _v1.json) file.

Why this exists
----------------
Some upstream OpenAPI specs omit a `required` array on a POST/PATCH request
body even though the live API actually rejects the request if that field is
missing (confirmed on `governance_group_v1`'s `owner` field: the spec marks
it fully optional, but `POST /workgroups/v1` returns a 400 "Required field
\"owner\" was missing or empty." if it's absent). tfplugingen-openapi has no
way to know this from the spec alone, so it generates the attribute as
`computed_optional` and terraform-plugin-framework silently allows omitting
it, deferring the failure to a live apply-time API error instead of a local
plan-time validation error.

Since this drifts from the spec itself (which this repo does not own/edit -
see api-specs/README.md), the fix belongs in a checked-in, replayable
post-processing config, mirroring apply_codespec_type_mappings.py's pattern,
so it survives a future `make gen-api-v1` re-run against a refreshed spec.

Config schema (generator_config/schema_overrides_<target>.yml)
-------------------------------------------------------------
required_overrides:
  # Dotted attribute path within a resource/data_source schema (see
  # --dump-paths on apply_codespec_type_mappings.py to discover the exact
  # dotted path names for a freshly generated code spec).
  - path: "owner"
    scope: resource   # one of: resource, data_source, both (default: resource)
    required: true    # sets computed_optional_required to "required"

strip_defaults:
  # Removes a static `default` block tfplugingen-openapi derived from the
  # OpenAPI spec's own `default:`/`example:` keyword on a Computed(-optional)
  # attribute. Appropriate when that attribute is genuinely server-computed
  # (its real value depends on live server-side state, e.g. a health-check
  # result or a status enum) rather than a stable client-side default -
  # forcing a static default on such a field makes Terraform plan a specific
  # known value for an unconfigured attribute, which then triggers a
  # "Provider produced inconsistent result after apply" error the moment the
  # live value differs from that default (confirmed on `sources_v1`'s
  # `healthy`/`category` fields, whose live values are almost never their
  # spec-declared defaults of `false`/`null` for a freshly created source).
  # Removing the default leaves the attribute planning as Unknown when
  # unconfigured, which is always consistent with whatever the live value
  # turns out to be.
  - path: "healthy"
    scope: resource

computed_only_overrides:
  # Forces an attribute to Computed-only (no Optional), for a field confirmed
  # live to be silently ignored by the API when the practitioner sets it -
  # every create attempt comes back with a different value than configured,
  # which is a permanent (not just first-apply) "Provider produced
  # inconsistent result after apply" error as long as the attribute stays
  # Optional. Confirmed on `sources_v1`'s `category`: every existing real
  # source in the tenant has `category = null` regardless of connector type,
  # and a live `terraform apply` setting `category = "CredentialProvider"`
  # still came back `null` from the API. Unlike `strip_defaults` (which just
  # removes a misleading static default so the attribute plans as Unknown),
  # this additionally removes the ability to configure the attribute at all,
  # since configuring it can never actually succeed.
  - path: "category"
    scope: resource

drop_attributes:
  # Removes an attribute entirely (both its schema definition and its
  # generated model/converter code, once regenerated), for attributes that
  # tfplugingen-openapi synthesizes from a *path parameter* rather than a
  # true request/response body property, and which duplicate a
  # same-valued/same-purpose attribute already present in the body schema.
  # `schema.ignores` in generator_config_<target>.yml only filters real
  # body-schema properties - it does NOT reach these path-param-derived
  # attributes, so they survive even when listed there. Confirmed on
  # `identity_profile_v1`: the read/update/delete path uses
  # `{identity-profile-id}` (unlike most other _v1 targets' plain `{id}`),
  # which tfplugingen-openapi doesn't correlate with the response body's own
  # "id" property - it generates both a normal "id" attribute (from the
  # body schema) and a redundant "identityprofileid" attribute (from the
  # path parameter name) with no way to `ignores:` it away.
  - path: "identityprofileid"
    scope: both

Usage
-----
    python3 scripts/apply_codespec_schema_overrides.py --target governance_group_v1
        # reads   openapi_code_spec/openapi_code_spec_governance_group_v1.json
        # reads   generator_config/schema_overrides_governance_group_v1.yml
        # writes  openapi_code_spec/openapi_code_spec_governance_group_v1.json (in place)

Run this AFTER apply_codespec_type_mappings.py (if both apply to a target)
and BEFORE `make gen-framework-api-v1`, since gen-api-v1 regenerates the code
spec JSON from scratch each time and both post-processing steps are applied
on top of that fresh output, not merged with each other.

This script is intentionally dependency-light (stdlib json + PyYAML only)
and vendor-agnostic: it knows nothing about SailPoint specifically, only the
tfplugingen-openapi code spec JSON shape (resources[]/data_sources[]
.schema.attributes[], with single_nested/list_nested nesting).
"""
import argparse
import json
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("PyYAML is required: pip install pyyaml", file=sys.stderr)
    sys.exit(1)

REPO_ROOT = Path(__file__).resolve().parent.parent
CODE_SPEC_DIR = REPO_ROOT / "openapi_code_spec"
GENERATOR_CONFIG_DIR = REPO_ROOT / "generator_config"


def iter_attributes(node, name_path=""):
    """Yield (dotted_name_path, attribute_dict) for every named attribute
    dict found anywhere under `node`, recursing into single_nested/list_nested
    children."""
    if isinstance(node, dict):
        current_path = name_path
        if isinstance(node.get("name"), str):
            current_path = f"{name_path}.{node['name']}" if name_path else node["name"]
            yield current_path, node
        for value in node.values():
            yield from iter_attributes(value, current_path)
    elif isinstance(node, list):
        for item in node:
            yield from iter_attributes(item, name_path)


def find_attribute(schema_root, dotted_path):
    matches = [attr for path, attr in iter_attributes(schema_root.get("attributes", [])) if path == dotted_path]
    return matches


def type_key(attr):
    """Return the key of `attr` that holds the type-specific block
    (e.g. 'string', 'single_nested', 'list_nested') - the sibling of
    'name' that carries 'computed_optional_required'."""
    for key in ("string", "bool", "int64", "float64", "number", "list", "map", "set", "single_nested", "list_nested", "set_nested", "map_nested"):
        if key in attr:
            return key
    return None


def apply_override(entry, override, kind):
    schema = entry.get("schema", {})
    matches = find_attribute(schema, override["path"])
    if not matches:
        return 0
    applied = 0
    new_value = "required" if override.get("required", True) else "computed_optional"
    for attr in matches:
        key = type_key(attr)
        if key is None:
            print(f"WARNING: '{override['path']}' has no recognized type block - cannot set required-ness", file=sys.stderr)
            continue
        attr[key]["computed_optional_required"] = new_value
        applied += 1
    return applied


def apply_computed_only(entry, override, kind):
    """Force an attribute to Computed-only (drop Optional entirely), for
    fields confirmed live to be silently ignored by the API when set by the
    practitioner (e.g. `sources_v1`'s `category`, which every live create
    attempt proves the server never persists - it always comes back null
    regardless of what was configured, causing a permanent "Provider produced
    inconsistent result after apply" error as long as the attribute is
    Optional)."""
    schema = entry.get("schema", {})
    matches = find_attribute(schema, override["path"])
    if not matches:
        return 0
    applied = 0
    for attr in matches:
        key = type_key(attr)
        if key is None:
            print(f"WARNING: '{override['path']}' has no recognized type block - cannot force computed-only", file=sys.stderr)
            continue
        attr[key]["computed_optional_required"] = "computed"
        applied += 1
    return applied


def apply_strip_default(entry, override, kind):
    schema = entry.get("schema", {})
    matches = find_attribute(schema, override["path"])
    if not matches:
        return 0
    applied = 0
    for attr in matches:
        key = type_key(attr)
        if key is None:
            print(f"WARNING: '{override['path']}' has no recognized type block - cannot strip default", file=sys.stderr)
            continue
        if "default" in attr[key]:
            del attr[key]["default"]
            applied += 1
    return applied


def apply_drop_attribute(entry, override):
    """Removes every top-level attribute named override['path'] from
    entry['schema']['attributes'] (only top-level - dropping a nested
    single_nested/list_nested child isn't supported, since this is only
    intended for the whole-attribute path-param-duplicate case, not partial
    nested-shape edits)."""
    schema = entry.get("schema", {})
    attrs = schema.get("attributes", [])
    before = len(attrs)
    schema["attributes"] = [a for a in attrs if a.get("name") != override["path"]]
    return before - len(schema["attributes"])


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--target", required=True, help="Target name matching openapi_code_spec_<target>.json and generator_config/schema_overrides_<target>.yml (include the _v1 suffix if applicable)")
    parser.add_argument("--code-spec", help="Override path to the code spec JSON (default: openapi_code_spec/openapi_code_spec_<target>.json)")
    parser.add_argument("--config", help="Override path to the schema overrides YAML (default: generator_config/schema_overrides_<target>.yml)")
    args = parser.parse_args()

    code_spec_path = Path(args.code_spec) if args.code_spec else CODE_SPEC_DIR / f"openapi_code_spec_{args.target}.json"
    if not code_spec_path.exists():
        print(f"Code spec not found: {code_spec_path}", file=sys.stderr)
        sys.exit(1)

    with open(code_spec_path) as f:
        code_spec = json.load(f)

    config_path = Path(args.config) if args.config else GENERATOR_CONFIG_DIR / f"schema_overrides_{args.target}.yml"
    if not config_path.exists():
        print(f"No schema_overrides config at {config_path} - nothing to apply.", file=sys.stderr)
        return

    with open(config_path) as f:
        config = yaml.safe_load(f) or {}

    total_applied = 0
    for override in config.get("required_overrides", []):
        scope = override.get("scope", "resource")
        kinds = {"resource": ["resources"], "data_source": ["datasources"], "both": ["resources", "datasources"]}[scope]
        for kind in kinds:
            for entry in code_spec.get(kind, []):
                applied = apply_override(entry, override, kind)
                total_applied += applied
                if applied == 0:
                    print(f"WARNING: path '{override['path']}' not found in {kind}/{entry.get('name', '?')} - config may be stale", file=sys.stderr)

    for override in config.get("strip_defaults", []):
        scope = override.get("scope", "resource")
        kinds = {"resource": ["resources"], "data_source": ["datasources"], "both": ["resources", "datasources"]}[scope]
        for kind in kinds:
            for entry in code_spec.get(kind, []):
                applied = apply_strip_default(entry, override, kind)
                total_applied += applied
                if applied == 0:
                    print(f"WARNING: strip_defaults path '{override['path']}' not found (or had no default) in {kind}/{entry.get('name', '?')} - config may be stale", file=sys.stderr)

    for override in config.get("computed_only_overrides", []):
        scope = override.get("scope", "resource")
        kinds = {"resource": ["resources"], "data_source": ["datasources"], "both": ["resources", "datasources"]}[scope]
        for kind in kinds:
            for entry in code_spec.get(kind, []):
                applied = apply_computed_only(entry, override, kind)
                total_applied += applied
                if applied == 0:
                    print(f"WARNING: computed_only_overrides path '{override['path']}' not found in {kind}/{entry.get('name', '?')} - config may be stale", file=sys.stderr)

    for override in config.get("drop_attributes", []):
        scope = override.get("scope", "resource")
        kinds = {"resource": ["resources"], "data_source": ["datasources"], "both": ["resources", "datasources"]}[scope]
        for kind in kinds:
            for entry in code_spec.get(kind, []):
                applied = apply_drop_attribute(entry, override)
                total_applied += applied
                if applied == 0:
                    print(f"WARNING: drop_attributes path '{override['path']}' not found in {kind}/{entry.get('name', '?')} - config may be stale", file=sys.stderr)

    with open(code_spec_path, "w") as f:
        json.dump(code_spec, f, indent=2)
        f.write("\n")

    print(f"Applied {total_applied} schema override(s) to {code_spec_path}")


if __name__ == "__main__":
    main()
