# GridAPI SDKs

**Source Available | Proprietary | Non-Commercial Use Only**

Copyright (c) 2026 BugyLabs. All Rights Reserved.

Six official, server-side SDKs for the GridAPI public REST API. The source is
publicly visible; it is **not** open source. Personal, educational, research
and non-commercial evaluation use is permitted. Commercial use requires a
separate written agreement with BugyLabs. See [LICENSE](LICENSE).

- One backend contract: [`openapi/openapi.yaml`](openapi/openapi.yaml) (canonical) and generated [`openapi/openapi.json`](openapi/openapi.json).
- One auth model: `Authorization: Bearer <GRIDAPI_API_KEY>`. The key is account-level; a project slug is a routing/authorization target, never a credential.
- Server-side authorization: the server resolves the caller's role per project; SDKs never make authorization decisions.
- Six independent packages and release processes in one repository.

## Packages

| Language | Version | Install | Registry | Source & docs |
| --- | --- | --- | --- | --- |
| TypeScript / JavaScript | 1.0.0 | `npm install @gridapi/sdk` | [npm](https://www.npmjs.com/package/@gridapi/sdk) | [README](packages/javascript/README.md) · [source](packages/javascript) |
| Python | 1.0.0 | `pip install gridapi-sdk` | [PyPI](https://pypi.org/project/gridapi-sdk/) | [README](packages/python/README.md) · [source](packages/python) |
| Java | 1.0.0 | `io.gridapi:gridapi-sdk:1.0.0` | [Maven Central](https://central.sonatype.com/artifact/io.gridapi/gridapi-sdk) | [README](packages/java/README.md) · [source](packages/java) |
| .NET / C# | 1.0.0 | `dotnet add package GridAPI` | [NuGet](https://www.nuget.org/packages/GridAPI/) | [README](packages/dotnet/README.md) · [source](packages/dotnet) |
| Go | 1.0.0 | `go get github.com/bugylabs/gridapi-sdk` | [Go modules](https://pkg.go.dev/github.com/bugylabs/gridapi-sdk) | [README](packages/go/README.md) · [source](packages/go) |
| Rust | 1.0.0 | `gridapi-sdk = "1.0"` | [crates.io](https://crates.io/crates/gridapi-sdk) | [README](packages/rust/README.md) · [source](packages/rust) |

Registry names were checked for availability on 2026-11-21. **Nothing has been
published yet.** Exact owner setup and blockers are tracked in
[DEPLOYMENT_STATUS.md](DEPLOYMENT_STATUS.md) and
[AUDIT_REPORT.md](AUDIT_REPORT.md).

Supported runtimes: Node.js >= 18 (npm trusted publishing requires the runner
to use Node >= 22.14 and npm >= 11.5.1), Python >= 3.8, Java 21+, .NET 10,
Go 1.22+, Rust 1.80+ (edition 2021). See [COMPATIBILITY.md](docs/COMPATIBILITY.md).

## Common API surface

All six expose the same operations with idiomatic names: `get`/`query`,
`getById`, `post`, `patch`, `put`, `delete`, `count`, `listProjects`,
`listTables`, `getSchema`, `getMe`, `getUsage`. Filtering is exact equality
(`col:value` or an object/map); pagination is `page` + `limit` (clamped 1–100).
Sorting and text search are deliberately not exposed: the backing Google Sheets
provider ignores them. Quota exhaustion (429 `SUBSCRIPTION_LIMIT_REACHED`) is
terminal; transient 429s are retried only for safe methods with `Retry-After`.

## Quick start (TypeScript)

```ts
import { GridApiClient } from "@gridapi/sdk";

const grid = new GridApiClient({ apiKey: process.env.GRIDAPI_API_KEY! });
const { data, meta } = await grid.get("acme", "users", {
  filter: { status: "active" },
  limit: 10,
});
console.log(data, meta.total);
```

## Verify locally

```bash
python3 scripts/validate_contract.py          # contract/manifest/SDK drift + leaked keys
python3 scripts/check_examples.py             # example syntax and placeholder checks
node tests/contract/mock-server.mjs           # shared in-memory contract backend
python3 scripts/run-all-tests.py              # every SDK with an installed toolchain
```

Each contract suite starts the shared mock server on its own port; the mock is a
test fixture, not part of the public API. No live account is required.

## Release process

Each SDK has its own version and its own tag: `javascript-vX.Y.Z`, `python-vX.Y.Z`,
`java-vX.Y.Z`, `dotnet-vX.Y.Z`, `rust-vX.Y.Z`, and `vX.Y.Z` for the root Go
module. A tag or `workflow_dispatch` run triggers only that SDK's
`publish-*.yml`; shared contract changes run all checks in `ci.yml`. Publishing
uses GitHub Environments and OIDC trusted publishing where the registry supports
it. See [RELEASES.md](RELEASES.md) for the exact owner actions.

## Layout

```
openapi/         canonical contract (YAML source of truth, generated JSON)
docs/            sdk-manifest.json, compatibility matrix, execution notes
packages/        one independently versioned SDK per language
examples/        runnable examples (environment variables only)
tests/contract/  shared mock backend + per-SDK contract suites
scripts/         contract validator, example checks, client docs sync
.github/workflows/  independent CI and six publish workflows
```

## Contributing and support

See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md),
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and [SUPPORT.md](SUPPORT.md). Report
issues at <https://github.com/bugylabs/gridapi-sdk/issues>; do not report
security issues publicly.

## License

**BugyLabs Proprietary Non-Commercial License. Source Available | Proprietary |
Non-Commercial Use Only.** Not MIT, Apache, BSD, ISC, GPL or any open-source
license. Commercial licensing requires a separate written agreement with
BugyLabs. The custom license should be reviewed by legal counsel before first
publication. Third-party dependencies keep their own licenses.
