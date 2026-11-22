# Public-release audit report

Date: 2026-11-21. Scope: `gridapi_sdk` only. No private backend source is
included, referenced by builds, or published from this repository.

## Repository state inspected

- Git: one local commit `6ff554cb0d4dadc8081849444734373a2ad2d40b`; 68 tracked
  files at audit time; no tags; no committed build artifacts.
- Structure: `openapi/`, `docs/`, `packages/{javascript,python,java,dotnet,go,rust}`,
  `examples/<lang>`, `tests/contract/`, `scripts/`, `.github/workflows/`.
- Implementation check: no TODOs or placeholder methods found. All six SDKs use
  the public REST API only; no private packages, internal services, or backend
  imports. SDK versions at audit: JS 0.1.0, Python 1.0.0, Java 0.1.0, .NET 0.1.0,
  Rust 0.1.0, Go unversioned. All are now 1.0.0.

## Findings and changes applied

| Area | Finding | Action |
| --- | --- | --- |
| License | MIT in LICENSE, all six package metadata files and OpenAPI info | Replaced root LICENSE with BugyLabs proprietary non-commercial license; copied into packages; updated npm (`SEE LICENSE IN LICENSE`), PyPI (`license = { file = "LICENSE" }` + proprietary classifier), Maven (license name/URL), NuGet (`PackageLicenseFile`), Rust (`license-file`) and OpenAPI metadata. |
| Repository URL | `github.com/gridapi/gridapi_sdk` in pom.xml, Cargo.toml, manifest | Changed to canonical `https://github.com/bugylabs/gridapi-sdk`. |
| Go module | `github.com/gridapi/gridapi-go` pointed at a different repo | Changed root module and all imports to `github.com/bugylabs/gridapi-sdk`; root module => plain `vX.Y.Z` tags (no `/go` prefix needed because the module is at the repository root). |
| Private references | Local `/Users/...` path and `gridapi_server` references in `docs/EXECUTION_PLAN.md` and `CONTRIBUTING.md` | Sanitized to public contract references. |
| Java publishing | No sources/javadoc jars, GPG signing or Central publisher plugin | Added source, javadoc, GPG and `central-publishing-maven-plugin:0.12.0` with `publishingServerId=central`. |
| .NET metadata | Missing repository/license/readme metadata | Added project URL, repository, tags, license file, readme packaging and XML docs. |
| Python metadata | No authors, classifiers or URLs | Added BugyLabs author, proprietary classifier and project URLs. |
| npm metadata | No repository/homepage/publishConfig | Added repository, homepage, issues, `publishConfig` with `provenance`. |
| CI | One combined `publish.yml`, no path filtering | Replaced with `ci.yml` + six independent publish workflows with per-SDK path filters and environments. |
| Docs | Stale per-project-key wording in generated client docs copy (fixed in prior phase) | `gridapi_client` now renders the two required credentials (`GRIDAPI_API_KEY` + `GRIDAPI_PROJECT_KEY`); that repo is outside this one. |

## Registry availability (live checks, 2026-11-21; 404 means the name is free)

| Registry | Name | Result |
| --- | --- | --- |
| npm | `@gridapi/sdk` | free |
| PyPI | `gridapi-sdk` | free |
| crates.io | `gridapi-sdk` (`gridapi` also free) | free |
| NuGet | `GridAPI` | free |
| Maven Central | `io.gridapi:gridapi-sdk` | no existing artifact found |
| GitHub | `bugylabs` account exists (user, not org); `bugylabs/gridapi-sdk` repo does not exist | owner must create the repository |

Name availability is **not** ownership. No package has been published, and no
trusted-publisher relationship or credential has been verified. See
[DEPLOYMENT_STATUS.md](DEPLOYMENT_STATUS.md) for the per-SDK blocker and exact
owner action.

## Security / public-release audit

- `.gitignore` excludes `node_modules/`, `dist/`, `target/`, `bin/`, `obj/`,
  Python caches, `.venv/`, `.env*`, `*.pem`, `*.key`, and compiled outputs.
- Git history contains no tags and no committed artifacts. `git ls-files`
  contains no key files, no example credential values and no internal URLs.
- Ignored `packages/dotnet/GridAPI/obj/` and similar local build folders contain
  absolute local paths from local builds; they are untracked and excluded from
  publication by the package manifests. No secret values were printed.
- The contract validator scans source files for embedded `gp_live_`/`gapi_live_`
  keys and the `YOUR_API_KEY` placeholder.
- Third-party dependency licenses remain their own: TypeScript (Apache-2.0),
  Rust `ureq`/`serde`/`serde_json` (MIT/Apache-2.0 dual), build plugins used only
  in CI. Verify them again in the dependency-lock step of each registry.

No destructive history rewrite was performed. If a future scan finds a leaked
credential in history, report it without printing the value and rotate first.

## Known limitations

- Maven, crates.io and registry publishing cannot be fully verified locally
  without accounts and credentials; workflow YAML was parsed locally, not run
  on GitHub.
- The Python wheel build is not available in this local environment
  (`setuptools` is absent); the release workflow installs `build` on the runner
  and verifies a clean install.
- Legal review of the custom license is required before publication.
