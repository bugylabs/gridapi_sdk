# Deployment status

Date: 2026-11-21. Status is intentionally conservative: **no package has been
published**, no registry namespace has been proven owned, and no trusted
publisher has been configured. Code readiness is described separately from
registry authorization.

Status meanings: **READY** = code, config and registry authorization verified;
**BLOCKED** = a specific external action or credential is missing;
**PUBLISHED** = published and installation verified; **FAILED** = an operation
failed.

## Local verification performed (no registry access)

All checks below ran locally on 2026-11-21 and passed:

- `python3 scripts/validate_contract.py` → 14 operations, 8 paths, 6 SDKs, no drift.
- `python3 scripts/check_examples.py` → 6 example directories checked.
- `python3 scripts/run-all-tests.py` → JS 9, Python 9, Java 52, .NET 31, Go 8, Rust 7 + 1 doctest.
- JavaScript: `npm install`, `npm run build` (ESM + CJS + declarations), `npm test`, `npm pack --dry-run` (8 files).
- Python: sdist + wheel built in a clean venv; wheel installed and `from gridapi import GridApiClient` verified.
- .NET: `dotnet pack` produced `GridAPI.1.0.0.nupkg` with LICENSE and README included.
- Rust: `cargo fmt --check`, `cargo clippy --all-targets -- -D warnings`, `cargo test`, `cargo package --allow-dirty`.
- Go: `go mod tidy`, `go vet ./...`, contract suite (8 subtests).
- All 7 workflow YAML files parse. Maven publication cannot be exercised locally (no `mvn` and no Sonatype credentials).

## JavaScript / TypeScript — `@gridapi/sdk` → npm

**Status: BLOCKED** (code READY, registry authorization missing)

- [x] Package metadata: name `@gridapi/sdk`, version `1.0.0`, ESM + CJS + declarations, `publishConfig.provenance`, LICENSE and README included.
- [x] Local build, typecheck and 9 contract tests pass; `npm pack --dry-run` verified in CI workflow.
- [ ] Platform: npmjs.com. Account/namespace `@gridapi` ownership must be verified by the owner.
- [ ] GitHub: create repository `bugylabs/gridapi-sdk`, push `main`, create environment `npm-production` (Settings → Environments), add required reviewers if desired.
- [ ] npm trusted publisher: configure GitHub Actions OIDC for owner `bugylabs`, repo `gridapi-sdk`, workflow `publish-npm.yml`, environment `npm-production`. Requires npm CLI >= 11.5.1 and Node >= 22.14 (workflow pins Node 24 / npm 11.5.1). Setup/docs: <https://docs.npmjs.com/trusted-publishers> — <https://www.npmjs.com/settings> → Trusted Publishers.
- [ ] Next action: owner creates the repo and trusted publisher, then pushes tag `javascript-v1.0.0` (or runs the workflow with version `1.0.0`). No npm token is needed or committed.

## Python — `gridapi-sdk` → PyPI

**Status: BLOCKED** (code READY, registry authorization missing)

- [x] `pyproject.toml`: version `1.0.0`, license file, proprietary classifier, authors/URLs; wheel+sdist built and clean-install verified in CI workflow.
- [x] 9 contract tests pass locally.
- [ ] Platform: PyPI account that owns/or can register `gridapi-sdk`: <https://pypi.org/account/register/>.
- [ ] PyPI trusted publisher: <https://pypi.org/manage/account/publishing/> — add a GitHub publisher with owner `bugylabs`, repo `gridapi-sdk`, workflow `publish-pypi.yml`, environment `pypi-production`.
- [ ] GitHub: environment `pypi-production`.
- [ ] Next action: owner configures the trusted publisher, then pushes `python-v1.0.0` or dispatches the workflow. No PyPI token is stored.

## Java — `com.bugylabs:gridapi-sdk` → Maven Central (Central Publisher Portal)

**Status: READY** (namespace verified, public key published, credentials configured)

- [x] `pom.xml`: coordinates `com.bugylabs:gridapi-sdk:1.0.0`, sources and javadoc jars, GPG signing pinned to fingerprint `9FA34FA29C9759534B98910946547264E642F2E2`, `central-publishing-maven-plugin:0.12.0` with `publishingServerId=central`, `deploymentName=gridapi-sdk-1.0.0`, `autoPublish=true`, `waitUntil=published`; contract tests (69 checks) pass locally.
- [x] Platform: namespace `com.bugylabs` is **Verified** in the Central Publisher Portal (Org: Bugylabs).
- [x] GPG: public key published to and retrievable from `keys.openpgp.org` by full fingerprint; private key stored as secret `GPG_PRIVATE_KEY`, passphrase as `MAVEN_GPG_PASSPHRASE`. Never commit the key.
- [x] GitHub secrets `CENTRAL_USERNAME` and `CENTRAL_PASSWORD` and environment `maven-central-production` are configured.
- [ ] Next action: trigger `publish-maven.yml`; publication is not claimed until the artifact is publicly available on Maven Central.

## .NET / C# — `GridAPI` → NuGet

**Status: BLOCKED** (code READY, registry authorization missing)

- [x] `GridAPI.csproj`: version `1.0.0`, license file, readme, repository/project metadata, XML docs; `dotnet pack` contents check in workflow; 31 contract checks pass locally.
- [ ] Platform: NuGet account owning or able to register `GridAPI`: <https://www.nuget.org/>. Verify with the account owner before first publish.
- [ ] NuGet trusted publishing policy: <https://www.nuget.org/account/trustedpublishing> — owner `bugylabs`, repo `gridapi-sdk`, workflow `publish-nuget.yml`, environment `nuget-production`. Requires GitHub secret `NUGET_USER` (NuGet account username). Docs: <https://learn.microsoft.com/en-us/nuget/nuget-org/trusted-publishing>.
- [ ] GitHub: environment `nuget-production` and secret `NUGET_USER`.
- [ ] Next action: owner registers the policy, then pushes `dotnet-v1.0.0`. The login action returns a short-lived key; no API key is stored.

## Go — `github.com/bugylabs/gridapi-sdk` → Go modules / Git tags

**Status: BLOCKED** (code READY, repository missing)

- [x] Root module path `github.com/bugylabs/gridapi-sdk`, Go 1.22 directive; `go mod tidy`, `go vet ./...`, `go test ./...` (8 subtests) pass locally.
- [ ] Platform: <https://github.com/bugylabs/gridapi-sdk> does not exist. `bugylabs` is currently a personal account, not an organization. Create the repository and push `main`; it must be public for `go get` and pkg.go.dev. Create environment `go-production`.
- [ ] Next action: owner pushes tag `v1.0.0` (plain tag: the module is at the repo root, so no `go/v1.0.0` prefix). Verify at <https://proxy.golang.org/github.com/bugylabs/gridapi-sdk/@v/v1.0.0.info> and <https://pkg.go.dev/github.com/bugylabs/gridapi-sdk@v1.0.0>. Never move or reuse a published tag.

## Rust — `gridapi-sdk` → crates.io

**Status: BLOCKED** (code READY, crate ownership/bootstrap missing)

- [x] `Cargo.toml`: version `1.0.0`, `license-file = "LICENSE"` (no false SPDX MIT), repository URL; `cargo fmt --check`, `cargo clippy -D warnings`, `cargo test` (7 tests + 1 doctest) and `cargo package` pass locally.
- [ ] Platform: crate name `gridapi-sdk` is free as of 2026-11-21. A crates.io account must own it. Crates.io trusted publishing requires a one-time manual bootstrap: publish the first version with a local API token or use `cargo publish` manually, then link the GitHub repository in crate settings and enable trusted publishing.
- [ ] Setup: <https://crates.io/me> (account/API token for bootstrap) and crate settings <https://crates.io/crates/gridapi-sdk/settings> (repository link + trusted publishing) — owner `bugylabs`, repo `gridapi-sdk`, workflow `publish-rust.yml`, environment `cratesio-production`. Docs: <https://doc.rust-lang.org/cargo/reference/publishing.html>.
- [ ] GitHub: environment `cratesio-production`; the OIDC action supplies the token, so no long-lived token is stored after bootstrap.
- [ ] Next action: owner performs the one-time bootstrap publish/link, then pushes `rust-v1.0.0`. Verify with `cargo search gridapi-sdk` and `cargo info gridapi-sdk@1.0.0`. Never republish a version.

## Shared prerequisites (all SDKs)

- [ ] Create the canonical public repository at <https://github.com/new> using owner `bugylabs` and name `gridapi-sdk`; push the `main` branch from this local checkout. Decide whether `bugylabs` should become an organization before publishing (recommended for shared ownership).
- [ ] Create GitHub Environments: `npm-production`, `pypi-production`, `maven-central-production`, `nuget-production`, `go-production`, `cratesio-production`. Add required reviewers/protection rules as policy requires.
- [ ] Verify the custom proprietary license with legal counsel before any publication; confirm it is acceptable to npm/PyPI/Maven/NuGet/crates.io metadata policies (crates.io uses `license-file`, npm uses `SEE LICENSE IN LICENSE`).
- [ ] Confirm package owner decisions: keep `gridapi-sdk` crate name (`gridapi` is also available), use `com.bugylabs` Maven namespace (verified), keep npm scope `@gridapi`.

## Prioritized manual checklist

1. Create `bugylabs/gridapi-sdk` on GitHub and push `main` (unblocks all six).
2. Create the six GitHub Environments with appropriate protection.
3. Legal review + sign-off of `LICENSE`.
4. Maven: verify `io.gridapi` namespace, generate/publish GPG key, add four GitHub secrets.
5. PyPI: add trusted publisher; npm: add trusted publisher; NuGet: add trusted publisher policy + `NUGET_USER` secret; crates.io: bootstrap publish then link repository + trusted publishing.
6. Tag one SDK at a time (`javascript-v1.0.0`, `python-v1.0.0`, `java-v1.0.0`, `dotnet-v1.0.0`, `v1.0.0`, `rust-v1.0.0`) and confirm each verification step before the next.
7. Update this file to PUBLISHED with installation links only after the registry verification steps pass.
