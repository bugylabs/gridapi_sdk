# Changelog

All notable changes are recorded here. Each package keeps its own version;
this file records release dates and cross-SDK changes. Format: Keep a Changelog,
Semantic Versioning.

## [1.0.0] — 2026-11-21 (prepared, not published)

### Added

- Six independently versioned SDKs (TypeScript, Python, Java, .NET, Go, Rust)
  built from one OpenAPI 3.1 contract and one account-level API key.
- Shared in-memory mock backend and per-SDK contract suites covering CRUD,
  filters, pagination, discovery, error mapping, redaction, retries and timeouts.
- Independent CI and six publish workflows with OIDC trusted publishing where
  supported; per-SDK path filters and protected GitHub Environments.
- Public-release audit, per-SDK deployment status and release runbook.

### Changed

- Replaced MIT licensing throughout with the BugyLabs Proprietary Non-Commercial
  License; all package versions set to 1.0.0.
- Canonical repository/URL set to `github.com/bugylabs/gridapi-sdk`; Go module
  migrated to the repository root.

### Notes

- Nothing is published yet. See `DEPLOYMENT_STATUS.md` for blockers and exact
  owner actions.
