# Release runbook

One repository, six independent packages, six independent release processes.
Never publish one SDK because another changed.

## Versioning

Each package version lives in its own file:

| SDK | Version source | Tag pattern |
| --- | --- | --- |
| JavaScript | `packages/javascript/package.json` | `javascript-vX.Y.Z` |
| Python | `packages/python/pyproject.toml` | `python-vX.Y.Z` |
| Java | `packages/java/pom.xml` | `java-vX.Y.Z` |
| .NET | `packages/dotnet/GridAPI/GridAPI.csproj` | `dotnet-vX.Y.Z` |
| Go | `go.mod` (module path) + tag | `vX.Y.Z` |
| Rust | `packages/rust/Cargo.toml` | `rust-vX.Y.Z` |

Bump only the SDK being released. Go is released from a plain root-module tag;
no `go/vX.Y.Z` prefix is needed because the module is at the repository root.
Never move, delete or reuse a published tag or version.

## Manual release (workflow_dispatch)

1. Confirm the change is merged to `main` and `ci.yml` passed for that SDK.
2. Bump the version in the file above and update `CHANGELOG.md`; commit and push.
3. In GitHub Actions, run the matching `publish-*.yml` with `version` set to the
   new version and `release_type` `stable` or `prerelease`.
4. The workflow validates the input against the package file, runs tests/build,
   publishes only to that SDK's registry, verifies publication, and creates a
   GitHub release. If verification fails, investigate before re-running.

## Tag-based release

Push the SDK-specific tag (for example `git tag python-v1.0.1 &&
git push origin python-v1.0.1`). The matching workflow runs the same validation
and verification. Generic `vX.Y.Z` tags are reserved for the Go module.

## Shared changes

Changes under `openapi/`, `docs/sdk-manifest.json`, `scripts/` or the shared mock
trigger every SDK's CI job because they can affect all packages. They do **not**
trigger any publish workflow: version bumps and tags are explicit.

## Preconditions

Repositories, environments, trusted publishers, credentials and namespace
ownership are documented in [DEPLOYMENT_STATUS.md](DEPLOYMENT_STATUS.md).
Publication is not claimed until the registry verification steps pass.
