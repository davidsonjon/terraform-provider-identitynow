#!/usr/bin/env python3
"""Apply durable, checked-in type-linking edits to a freshly generated
openapi_code_spec_<target>.json (or _v1.json) file.

Why this exists
----------------
Step 3/4 of the codegen pipeline ("type linking" / the
tfplugingen-openapi-type-reviewer's review) used to be applied as a one-off
inline Python snippet run directly in an agent session and then discarded.
That made the edits non-reproducible: after any future `make gen-api`/
`make gen-api-v1` re-run (e.g. following an upstream spec refresh), someone
had to re-derive and re-apply the same associated_external_type mappings and
symbol-collision renames from memory or a stale chat transcript.

This script makes that step a named, replayable, diffable, checked-in
pipeline stage instead: the mappings live in
generator_config/type_mappings_<target>.yml, and this script applies them to
the freshly generated code spec JSON idempotently.

Config schema (generator_config/type_mappings_<target>.yml)
-------------------------------------------------------------
external_type_mappings:
  # Dotted attribute path within a resource/data_source schema (see
  # `--dump-paths` below to discover the exact dotted path names for a
  # freshly generated code spec), mapped to the associated_external_type
  # block that tfplugingen-framework expects.
  - path: "access_model_metadata.attributes.values"
    scope: both        # one of: resource, data_source, both (default: both)
    import_path: "github.com/sailpoint-oss/golang-sdk/v2/api_beta"
    type: "*api_beta.AttributeValueDTO"
  - path: "owner"
    scope: both
    import_path: "github.com/sailpoint-oss/golang-sdk/v2/api_beta"
    type: "*api_beta.OwnerReference"

symbol_renames: []
  # Reserved for future use: a list of {from, to} literal string renames
  # applied across the whole code-spec JSON text, for resolving Go symbol
  # collisions between multiple mapped types that share a generated name.
  # Not yet needed for any target as of 2026-07-27 (documented for role_v1
  # and access_profile_v1's manual passes but not yet used programmatically
  # here); keep the schema stable so a future target can add entries without
  # a script change.

Usage
-----
    python3 scripts/apply_codespec_type_mappings.py --target access_profile_v1
        # reads   openapi_code_spec/openapi_code_spec_access_profile_v1.json
        # reads   generator_config/type_mappings_access_profile_v1.yml
        # writes  openapi_code_spec/openapi_code_spec_access_profile_v1.json (in place)

    python3 scripts/apply_codespec_type_mappings.py --target access_profile_v1 --dump-paths
        # prints every attribute's dotted name path (for authoring/confirming
        # a type_mappings_<target>.yml file against a freshly generated spec)
        # without applying any changes.

This script is intentionally dependency-light (stdlib json + PyYAML only,
both already used elsewhere in this repo's tooling) and vendor-agnostic: it
knows nothing about SailPoint specifically, only the tfplugingen-openapi code
spec JSON shape (resources[]/data_sources[].schema.attributes[], with
single_nested/list_nested nesting).
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
    children. `node` is expected to be a resources[]/data_sources[] entry's
    schema.attributes list, or a nested attributes list within one."""
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
    """Find the single attribute dict matching dotted_path within a
    resources[]/data_sources[] entry's `schema` block. Raises if zero or
    more than one match is found (ambiguous paths are a config bug)."""
    matches = [attr for path, attr in iter_attributes(schema_root.get("attributes", [])) if path == dotted_path]
    return matches


def nested_container(attr):
    """associated_external_type is set on the nested-block container, not the
    attribute wrapper itself: for a single_nested attribute it's a sibling of
    that block's own `attributes` key; for a list_nested attribute it's a
    sibling of `nested_object`'s `attributes` key. Returns the dict to set
    associated_external_type on, or None if `attr` isn't a nestable block."""
    if "single_nested" in attr:
        return attr["single_nested"]
    if "list_nested" in attr and "nested_object" in attr["list_nested"]:
        return attr["list_nested"]["nested_object"]
    return None


def apply_mapping(entry, mapping, kind):
    schema = entry.get("schema", {})
    matches = find_attribute(schema, mapping["path"])
    if not matches:
        return 0
    ext_type = {
        "import": {"path": mapping["import_path"]},
        "type": mapping["type"],
    }
    applied = 0
    for attr in matches:
        container = nested_container(attr)
        if container is None:
            print(f"WARNING: '{mapping['path']}' is not a single_nested/list_nested block - cannot attach associated_external_type", file=sys.stderr)
            continue
        container["associated_external_type"] = ext_type
        applied += 1
    return applied


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--target", required=True, help="Target name matching openapi_code_spec_<target>.json and generator_config/type_mappings_<target>.yml (include the _v1 suffix if applicable)")
    parser.add_argument("--dump-paths", action="store_true", help="Print every attribute's dotted name path instead of applying mappings")
    parser.add_argument("--code-spec", help="Override path to the code spec JSON (default: openapi_code_spec/openapi_code_spec_<target>.json)")
    parser.add_argument("--config", help="Override path to the type mappings YAML (default: generator_config/type_mappings_<target>.yml)")
    args = parser.parse_args()

    code_spec_path = Path(args.code_spec) if args.code_spec else CODE_SPEC_DIR / f"openapi_code_spec_{args.target}.json"
    if not code_spec_path.exists():
        print(f"Code spec not found: {code_spec_path}", file=sys.stderr)
        sys.exit(1)

    with open(code_spec_path) as f:
        code_spec = json.load(f)

    if args.dump_paths:
        for kind in ("resources", "datasources"):
            for entry in code_spec.get(kind, []):
                schema = entry.get("schema", {})
                for path, attr in iter_attributes(schema.get("attributes", [])):
                    container = nested_container(attr)
                    tag = " [mapped]" if container and "associated_external_type" in container else ""
                    print(f"{kind}.{entry.get('name', '?')}.{path}{tag}")
        return

    config_path = Path(args.config) if args.config else GENERATOR_CONFIG_DIR / f"type_mappings_{args.target}.yml"
    if not config_path.exists():
        print(f"No type_mappings config at {config_path} - nothing to apply (this is expected for targets with zero mapping candidates, e.g. transform_v1).", file=sys.stderr)
        return

    with open(config_path) as f:
        config = yaml.safe_load(f) or {}

    total_applied = 0
    for mapping in config.get("external_type_mappings", []):
        scope = mapping.get("scope", "both")
        kinds = {"resource": ["resources"], "data_source": ["datasources"], "both": ["resources", "datasources"]}[scope]
        for kind in kinds:
            for entry in code_spec.get(kind, []):
                applied = apply_mapping(entry, mapping, kind)
                total_applied += applied
                if applied == 0:
                    print(f"WARNING: path '{mapping['path']}' not found in {kind}/{entry.get('name', '?')} - config may be stale", file=sys.stderr)

    if config.get("symbol_renames"):
        raw = json.dumps(code_spec)
        for rename in config["symbol_renames"]:
            raw = raw.replace(rename["from"], rename["to"])
        code_spec = json.loads(raw)

    with open(code_spec_path, "w") as f:
        json.dump(code_spec, f, indent=2)
        f.write("\n")

    print(f"Applied {total_applied} type mapping(s) to {code_spec_path}")


if __name__ == "__main__":
    main()
