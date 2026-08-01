#!/usr/bin/env python3
"""One-shot helper to migrate a provider subpackage from golang-sdk/v2's
`api_beta` package to golang-sdk/v3's per-service packages.

Usage:
    python3 scripts/migrate_v3.py <dir> <primary_v3_service> [extra_v3_service ...]

It rewrites, across every *.go file under <dir>:
  * root `sailpoint "...golang-sdk/v2"`  -> `...golang-sdk/v3`
  * `"...golang-sdk/v2/api_beta"`         -> `"...golang-sdk/v3/<primary_service>"`
  * `.Beta.XxxAPI`                        -> `.XxxAPI`
  * API-scoped method calls `XxxAPI.Foo(` -> `XxxAPI.FooV1(` (per METHOD_MAP)

Type-prefix rewriting (`api_beta.` -> `<service>.`) is only done automatically
for single-service packages (no extra services); multi-service packages are
left for manual/targeted follow-up and this script only does the safe swaps.
"""
import os
import re
import sys

# Method rename map: (APIService field name) -> {old_method: new_method}
# Nearly everything is just <Old>V1; the two exceptions are explicit.
_V1 = "V1"
SPECIAL = {
    ("ConnectorRuleManagementAPI", "UpdateConnectorRule"): "PutConnectorRuleV1",
    ("SourcesAPI", "Delete"): "DeleteSourceV1",
}

# Global type-token renames applied everywhere (v2 name -> v3 name). These are
# SDK-wide renames independent of which service package the token lives in, so
# they are safe substring replacements that preserve any `<pkg>.` prefix.
GLOBAL_TYPE_RENAMES = {
    # The shared JSON-Patch oneOf value wrapper was cleaned up in v3: the
    # misleadingly-named UpdateMultiHostSourcesRequestInnerValue became
    # JsonPatchOperationValue (and its StringAs.../BoolAs.../etc constructors
    # follow the same rename).
    "UpdateMultiHostSourcesRequestInnerValue": "JsonPatchOperationValue",
}


def method_renames(text):
    # Find all `XxxAPI.<Method>` selectors (possibly across whitespace/newline)
    # and rename to the versioned form.
    pat = re.compile(r"\b([A-Za-z0-9_]+API)\s*\.\s*([A-Z][A-Za-z0-9_]+)\b")

    def repl(m):
        api, meth = m.group(1), m.group(2)
        if meth.endswith(("V1", "V2", "V2024", "V2025", "V2026")):
            return m.group(0)
        if (api, meth) in SPECIAL:
            new = SPECIAL[(api, meth)]
        else:
            new = meth + _V1
        # preserve original inner spacing
        return m.group(0)[: m.group(0).index(meth)] + new
    return pat.sub(repl, text)


def migrate_file(path, primary, single_service):
    with open(path) as f:
        txt = f.read()
    orig = txt

    txt = txt.replace(
        'sailpoint "github.com/sailpoint-oss/golang-sdk/v2"',
        'sailpoint "github.com/sailpoint-oss/golang-sdk/v3"',
    )
    txt = txt.replace(
        '"github.com/sailpoint-oss/golang-sdk/v2/api_beta"',
        f'"github.com/sailpoint-oss/golang-sdk/v3/{primary}"',
    )
    # .Beta.XxxAPI -> .XxxAPI
    txt = re.sub(r"\.Beta\.([A-Za-z0-9_]+API)", r".\1", txt)

    if single_service:
        # api_beta.X -> <primary_pkg>.X  (primary pkg's Go package name is the
        # last path element)
        pkgname = primary.split("/")[-1]
        txt = re.sub(r"\bapi_beta\.", pkgname + ".", txt)

    txt = method_renames(txt)

    for old, new in GLOBAL_TYPE_RENAMES.items():
        txt = txt.replace(old, new)

    if txt != orig:
        with open(path, "w") as f:
            f.write(txt)
        return True
    return False


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(2)
    d = sys.argv[1]
    primary = sys.argv[2]
    extras = sys.argv[3:]
    single = not extras
    changed = []
    for root, _, files in os.walk(d):
        for fn in files:
            if fn.endswith(".go"):
                p = os.path.join(root, fn)
                if migrate_file(p, primary, single):
                    changed.append(p)
    for c in changed:
        print("changed:", c)


if __name__ == "__main__":
    main()
