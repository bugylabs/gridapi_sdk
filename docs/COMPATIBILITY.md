# SDK compatibility matrix

All six SDKs target the same public contract: [openapi/openapi.yaml](../openapi/openapi.yaml)
(OpenAPI 3.1.0, 14 operations, 8 paths). Versions are independent.

| SDK | Package | Version | Minimum runtime | Dependencies | HTTP stack | Release workflow |
| --- | --- | --- | --- | --- | --- | --- |
| TypeScript / JavaScript | `@gridapi/sdk` | 1.0.0 | Node.js 18+ (ESM + CJS, declaration files) | TypeScript (dev only) | Fetch / AbortController | `publish-npm.yml` |
| Python | `gridapi-sdk` | 1.0.0 | Python 3.8+ | standard library only | `urllib.request` | `publish-pypi.yml` |
| Java | `io.gridapi:gridapi-sdk` | 1.0.0 | Java 21 | JDK only | `java.net.http.HttpClient` | `publish-maven.yml` |
| .NET | `GridAPI` | 1.0.0 | .NET 10 | base class library only | `System.Net.Http.HttpClient` | `publish-nuget.yml` |
| Go | `github.com/bugylabs/gridapi-sdk` | 1.0.0 | Go 1.22+ | standard library only | `net/http` | `publish-go.yml` |
| Rust | `gridapi-sdk` | 1.0.0 | Rust 1.80+ (edition 2021) | `ureq` (rustls), `serde`, `serde_json` | `ureq` | `publish-rust.yml` |

## API coverage

| Operation | JS | Python | Java | .NET | Go | Rust |
| --- | --- | --- | --- | --- | --- | --- |
| list records / query | `get` / `query` | `get` / `query` | `get` / `query` | `Get` / `Query` | `Get` / `Query` | `get` / `query` |
| get by id | `getById` | `get_by_id` | `getById` | `GetById` | `GetById` | `get_by_id` |
| create | `post` | `post` | `post` | `Post` | `Post` | `post` |
| patch / put / delete | yes | yes | yes | yes | yes | yes |
| count (from `meta.total`) | yes | yes | yes | yes | yes | yes |
| project/table/schema discovery | yes | yes | yes | yes | yes | yes |
| account/usage reads | yes | yes | yes | yes | yes | yes |

## Deliberate limitations

- Sorting and text search are not implemented: the server accepts the query
  parameters but the Google Sheets provider ignores them.
- Filtering is exact equality only, as a `col:value` string/map or object.
- Pagination is `page` + `limit`; `limit` is clamped to 1–100 by the server.
- SDKs are server-side only. The account-level API key must never ship to
  browsers or mobile clients; project slugs are not credentials.
- Transient 429s and other safe retries apply to GET/DELETE/PUT; PATCH is
  opt-in; POST is never retried; quota 429 is terminal.

See each language README for concrete signatures.
