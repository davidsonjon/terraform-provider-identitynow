# api-specs (this project's own derived spec outputs only)

This directory is **git-tracked**, but it is **not** a copy of the upstream
[`sailpoint-oss/api-specs`](https://github.com/sailpoint-oss/api-specs)
repository. It only holds OpenAPI spec artifacts that this project has
produced or modified itself as part of the codegen pipeline, e.g. the
bundled+dereferenced per-service specs under `dereferenced/deref-<service>.v1.yaml`
(the output of `make bundle-spec-v1`).

## Where the raw upstream specs live

Raw/unmodified upstream specs (`idn/apis/<service>/openapi.yaml`, the legacy
monolithic `dereferenced/deref-sailpoint-api.<version>.yaml` files, etc.) are
**not** stored in this repo at all. They are read directly, at codegen time,
from a separately git-managed local clone of `sailpoint-oss/api-specs` via the
`API_SPECS_SOURCE` GNUmakefile variable. It defaults to a sibling directory
next to this repo checkout (`../api-specs`) as a portable convenience
convention — **not** a hardcoded path tied to any one contributor's machine.
If your clone lives elsewhere, override per-invocation:
`make <target> API_SPECS_SOURCE=/path/to/your/clone`. Any target that
actually needs this (`bundle-spec-v1`, `gen-api`) fails fast with a clear,
actionable error (via the `check-api-specs-source` prerequisite) if the
resolved path doesn't exist, rather than silently passing a broken path
downstream — run `make check-api-specs-source` on its own to check your
setup without triggering a real codegen run.

This keeps this repo free of the ~126M+ upstream spec tree while still
versioning the small set of derived artifacts this project actually owns and
has reconciled/patched for the codegen tools to consume. See
`.github/agents/identitynow-terraform-provider-developer.agent.md` for the
full pipeline this feeds into.
