#!/usr/bin/env python3
"""Validate that the SDKs stay inside the canonical API contract.

Stdlib only. Reads openapi/openapi.json (generated from the canonical YAML)
and docs/sdk-manifest.json, then checks every SDK source for drift:
unknown endpoints, invented operations, or embedded credentials.
"""
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
HTTP = {"get", "post", "put", "patch", "delete"}


def fail(msg):
    print(f"FAIL: {msg}")
    return 1


def main():
    errors = []
    spec = json.loads((ROOT / "openapi" / "openapi.json").read_text())
    manifest = json.loads((ROOT / "docs" / "sdk-manifest.json").read_text())

    operations = {}
    paths = set()
    for path, item in spec.get("paths", {}).items():
        paths.add(path)
        for method, op in item.items():
            if method.lower() in HTTP:
                operations[op["operationId"]] = (method.upper(), path)

    # 1. Every operation declared by the SDK manifest exists in the contract.
    for op in manifest["operations"]:
        oid = op["operationId"]
        if oid not in operations:
            errors.append(f"operation {oid!r} is referenced by SDKs but absent from OpenAPI")
            continue
        method, path = operations[oid]
        if method != op["http"] or path != op["path"]:
            errors.append(
                f"operation {oid!r} drift: manifest {op['http']} {op['path']} vs contract {method} {path}"
            )

    # 2. Every contract operation used by SDKs must be documented with responses.
    for oid, (_, _) in operations.items():
        item = next(i for p in spec["paths"].values() for m, i in p.items()
                    if m.lower() in HTTP and i.get("operationId") == oid)
        if not item.get("responses"):
            errors.append(f"operation {oid!r} has no documented responses")

    def normalize(p):
        return re.sub(r"\{[^}]+\}", "{}", p)

    allowed = {normalize(p) for p in paths}
    path_re = re.compile(r"""['"`](/api/v1/[^'"`\s$]*)['"`]""")

    # 3. Scan SDK sources: no unknown endpoints, no embedded credentials.
    for lang in manifest["languages"]:
        src = ROOT / lang["sourceFile"]
        if not src.exists():
            errors.append(f"{lang['id']}: missing source {lang['sourceFile']}")
            continue
        text = src.read_text()
        for literal in path_re.findall(text):
            if normalize(literal) not in allowed:
                errors.append(f"{lang['id']}: endpoint not in contract: {literal}")
        for m in re.finditer(r"gp_live_[A-Za-z0-9_-]{12,}", text):
            errors.append(f"{lang['id']}: possible embedded account key in source")
        if "YOUR_API_KEY" in text:
            errors.append(f"{lang['id']}: placeholder key leaked into SDK source")

    # 4. Every language must have an example and a per-package README.
    for lang in manifest["languages"]:
        ex = ROOT / "examples" / lang["id"]
        if not any(ex.glob("*")):
            errors.append(f"{lang['id']}: no example under examples/{lang['id']}")
        pkg = ROOT / "packages" / lang["id"]
        if not any(pkg.glob("README.md")):
            errors.append(f"{lang['id']}: missing package README")

    # 5. Release metadata: one version, proprietary license, no private refs.
    release_files = {
        "packages/javascript/package.json": ('"version": "1.0.0"', '"license": "SEE LICENSE IN LICENSE"'),
        "packages/python/pyproject.toml": ('version = "1.0.0"', 'license = { file = "LICENSE" }'),
        "packages/java/pom.xml": ("<version>1.0.0</version>", "BugyLabs Proprietary"),
        "packages/dotnet/GridAPI/GridAPI.csproj": ("<Version>1.0.0</Version>", "<PackageLicenseFile>"),
        "packages/rust/Cargo.toml": ('version = "1.0.0"', 'license-file = "LICENSE"'),
    }
    for rel, needles in release_files.items():
        text = (ROOT / rel).read_text()
        for needle in needles:
            if needle not in text:
                errors.append(f"{rel}: missing release metadata {needle!r}")
        if re.search(r"(MIT|Apache-2\.0|BSD-3-Clause)\b", text):
            errors.append(f"{rel}: permissive license metadata still present")
    if "module github.com/bugylabs/gridapi-sdk" not in (ROOT / "go.mod").read_text():
        errors.append("go.mod: module path is not the canonical repository")
    for lang in manifest["languages"]:
        readme = ROOT / "packages" / lang["id"] / "README.md"
        text = readme.read_text() if readme.exists() else ""
        if "Copyright (c) 2026 BugyLabs. All Rights Reserved." not in text:
            errors.append(f"{lang['id']}: README missing proprietary license notice")
    private_refs = re.compile(r"/Users/|gridapi_server|github\.com/gridapi/gridapi_sdk")
    for path in ROOT.rglob("*"):
        rel = path.relative_to(ROOT)
        if not path.is_file() or any(part in {".git", "node_modules", "dist", "target", "obj", "bin"} for part in rel.parts):
            continue
        # The audit report quotes the legacy paths deliberately; this file defines the pattern.
        if rel.as_posix() in {"AUDIT_REPORT.md", "scripts/validate_contract.py"}:
            continue
        if path.suffix not in {".md", ".json", ".yaml", ".yml", ".toml", ".xml", ".csproj", ".py", ".ts", ".mjs", ".go", ".rs", ".java", ".cs", ".mod"}:
            continue
        if private_refs.search(path.read_text(errors="ignore")):
            errors.append(f"{rel}: private/legacy repository reference")

    if errors:
        for e in errors:
            print(f"  - {e}")
        return fail(f"{len(errors)} contract violation(s)")
    print(
        f"OK: {len(operations)} operations, {len(paths)} paths, "
        f"{len(manifest['languages'])} SDKs — no drift detected"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
