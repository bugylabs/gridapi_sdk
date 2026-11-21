# GridAPI SDKs

Six official SDKs for the GridAPI public REST API, built from one contract and one account-level authentication model.

- One backend contract: [`openapi/openapi.yaml`](openapi/openapi.yaml) (canonical) and its generated `openapi/openapi.json`.
- One auth model: `Authorization: Bearer <GRIDAPI_API_KEY>`. The key is account-level; a project slug is a routing/authorization target, never a credential.
- Server-side authorization: the server resolves the caller's role per project; SDKs never make authorization decisions.
- Independent releases: each package holds its own version and is published from its own tag.

| Language | Package | Source |
| --- | --- | --- |
| TypeScript / JavaScript | `@gridapi/sdk` | `packages/javascript` |
| Python | `gridapi-sdk` | `packages/python` |
| Java | `io.gridapi:gridapi-sdk` | `packages/java` |
| .NET | `GridAPI` | `packages/dotnet` |
| Go | `github.com/gridapi/gridapi-go` | `packages/go` |
| Rust | `gridapi-sdk` | `packages/rust` |

All six expose the same operations: `get`, `getById`, `post`, `patch`, `put`, `delete`, `query`, `count`, `listProjects`, `listTables`, `getSchema`, `getMe`, `getUsage`. Filtering is exact equality; pagination uses `page` + `limit`. Sorting and search are not exposed because the backing Google Sheets provider ignores them.

## Verify

```bash
python3 scripts/validate_contract.py          # manifest/contract/SDK drift + leaked keys
node tests/contract/mock-server.mjs           # shared in-memory contract backend
python3 scripts/run-all-tests.py              # every SDK whose toolchain is installed
```

Each SDK's tests start the shared mock server on their own port, so they can run independently. The mock is a test fixture, not part of the public API.

Client documentation is rendered from `docs/sdk-manifest.json`; run `node scripts/sync-client-docs.mjs` after changing it to refresh the copy in `gridapi_client`.

## Layout

```
openapi/         canonical contract (YAML source of truth, generated JSON)
docs/            sdk-manifest.json — shared metadata for docs and validation
packages/        one independently versioned SDK per language
examples/        minimal runnable examples (env vars only)
tests/contract/  shared mock backend + per-SDK contract suites
scripts/         contract validator, client docs sync
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
