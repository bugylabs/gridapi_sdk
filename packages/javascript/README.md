# @gridapi/sdk

Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

Official TypeScript/JavaScript SDK for the GridAPI public REST API — version 1.0.0.

## About GridAPI

GridAPI exposes a project's spreadsheet-backed tables as a simple REST API. One **account-level API key** authenticates every project your account owns or has membership in; the project slug in a URL is a **routing/authorization target, never a credential**. This SDK is a thin, typed client over that API. It is **server-side only** — never ship the key in a browser bundle or a mobile app.

## Features and supported operations

- One client, typed methods, and structured errors for every call.
- Exact-match filtering and `page`/`limit` pagination.
- Safe automatic retries with `Retry-After` support; writes are never duplicated accidentally.
- Configurable timeouts, base URL, and retry policy.
- Account discovery (`getMe`, `getUsage`, `listProjects`, `listTables`, `getSchema`) and full record CRUD.

Supported methods on `GridApiClient` (aliases: `GridAPI`, and the default export):

| Method | Purpose |
| --- | --- |
| `get` | List records with optional filter/pagination |
| `getById` | Fetch one record by primary key |
| `query` | Alias for `get` (same signature and behavior) |
| `count` | Number of records matching a filter |
| `post` | Insert one record or an array of records |
| `patch` | Partially update a record |
| `put` | Replace a record |
| `delete` | Delete a record |
| `listProjects` | List projects visible to the account |
| `listTables` | List tables/sheets in a project |
| `getSchema` | Get a table's field schema |
| `getMe` | Get authenticated account details |
| `getUsage` | Get API usage/quota details |

Record values are `string | null`. Filters are exact-equality matches only (see [Filtering and pagination](#filtering-and-pagination)).

## Installation

```bash
npm install @gridapi/sdk
```

The package ships ESM and CommonJS builds plus TypeScript declarations. To build from source instead, see [Contributing](#contributing).

## Requirements

- **Node.js >= 18** at runtime. The client uses the built-in `fetch`, `AbortController`, and `URLSearchParams` APIs.
- TypeScript is optional; declarations are included.

**Publishing note (maintainers only, not a consumer requirement):** releasing new versions is done in CI with npm OIDC/trusted publishing (`npm publish --provenance`), which requires a newer npm — use **npm >= 11.5.1** (CI runs Node 22). This has no bearing on applications that merely install and use the SDK; those only need Node >= 18.

## Quick start

```ts
import { GridApiClient } from "@gridapi/sdk";

// Reads GRIDAPI_API_KEY from the environment. Server-side only.
const api = new GridApiClient();

const account = await api.getMe();
console.log("Authenticated:", account.authenticated);

const { data, meta } = await api.get("acme", "users", {
  filter: "status:active",
  limit: 25,
});
console.log(`Active users: ${data.length} of ${meta.total}`);

const created = await api.post("acme", "users", { name: "Ada", status: "active" });
const id = created.records[0].id;
await api.patch("acme", "users", id, { status: "invited" });
```

The calls above were verified against the shared contract mock server (`tests/contract/mock-server.mjs`): `getMe`, `get`, `post`, `patch`, and `delete` each returned the documented response shapes.

## Authentication

Set `GRIDAPI_API_KEY` before constructing the client, or pass `apiKey` explicitly:

```ts
import { GridApiClient } from "@gridapi/sdk";

const api = new GridApiClient({ apiKey: process.env.GRIDAPI_API_KEY });
```

- The key is **account-level**. It authenticates your account, not a single project.
- The project slug (e.g. `"acme"` above) is a **routing/authorization identifier only**. It is never a credential and must never be treated as one.
- The client sends `Authorization: Bearer <key>` on every request.
- **Server-side only.** Do not embed the key in client-side JavaScript, mobile apps, or public repositories.
- Missing key: the constructor throws `AuthenticationError` synchronously. Invalid or unauthorized keys surface as `AuthenticationError` / `AuthorizationError` on the first call.

## Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | Yes | — | Account-level API key. Equivalent to `apiKey` in client options. |
| `GRIDAPI_BASE_URL` | No | `https://api.gridapi.in` | API base URL. Trailing slashes are stripped. Equivalent to `baseUrl`. |
| `GRIDAPI_TIMEOUT_MS` | No | `30000` | Per-attempt request timeout in milliseconds. Equivalent to `timeoutMs`. |

Explicit options take precedence over environment variables. `maxRetries`, `retryDelayMs`, and `retryWrites` are **options only** (no env vars).

## Client initialization

```ts
import { GridApiClient, GridAPI } from "@gridapi/sdk";
// GridAPI is an alias of GridApiClient; there is also a default export.

const api = new GridApiClient({
  apiKey: process.env.GRIDAPI_API_KEY,          // default: GRIDAPI_API_KEY
  baseUrl: "https://api.gridapi.in",           // default: GRIDAPI_BASE_URL
  timeoutMs: 30_000,                           // default: GRIDAPI_TIMEOUT_MS or 30000
  maxRetries: 2,                               // default: 2
  retryDelayMs: 500,                           // default: 500 ms base backoff
  retryWrites: false,                          // default: false (see Timeouts and retries)
});
```

## GET records

`get(project, table, params?)` returns `{ data, meta }`, where `meta` is `{ total, limit, offset, cacheHit? }`.

```ts
const page = await api.get("acme", "users", { filter: "status:active", page: 2, limit: 50 });
for (const row of page.data) console.log(row.id, row.name);
console.log(page.meta.total, page.meta.limit, page.meta.offset);
```

`query(project, table, params?)` has the same signature and is an alias for `get`.

## GET by ID

`getById(project, table, id)` returns the record object (`id` may be a string or number).

```ts
const user = await api.getById("acme", "users", "42");
console.log(user.name);
// Throws RecordNotFoundError if the id does not exist.
```

## POST

`post(project, table, record)` inserts a single record or an array of records and returns `{ inserted, records }`.

```ts
const created = await api.post("acme", "users", { name: "Ada", status: "active" });
console.log(created.inserted, created.records[0].id);

const many = await api.post("acme", "users", [
  { name: "Ben", status: "active" },
  { name: "Cy", status: "invited" },
]);
console.log(many.inserted); // 2
```

`POST` is **never retried** (see [Timeouts and retries](#timeouts-and-retries)). Invalid fields raise `ValidationError`.

## PATCH

`patch(project, table, id, changes)` partially updates a record and returns `{ updated, record }`.

```ts
const result = await api.patch("acme", "users", "42", { status: "invited" });
console.log(result.updated, result.record.status);
```

`PATCH` is retried only when the client is created with `retryWrites: true`.

## PUT

`put(project, table, id, record)` replaces a record and returns `{ updated, record }`.

```ts
const replaced = await api.put("acme", "users", "42", { name: "Ada L.", status: "active" });
console.log(replaced.record.name);
```

`PUT` is treated as a safe (idempotent) method and may be retried.

## DELETE

`delete(project, table, id)` returns `{ deleted, id }`.

```ts
const removed = await api.delete("acme", "users", "42");
console.log(removed.deleted, removed.id);
```

`DELETE` is safe and may be retried. A missing record raises `RecordNotFoundError`.

## Filtering and pagination

`QueryOptions` accepts:

- `filter` — `string | Record<string, string>`. Exact equality only:
  - string form: `"status:active,team:growth"`
  - object form: `{ status: "active", team: "growth" }`
- `page` — 1-based page number. Applied only when `limit` is also supplied.
- `limit` — page size, clamped by the server to 1–100.

```ts
const res = await api.get("acme", "users", {
  filter: { status: "active" },
  page: 1,
  limit: 50,
});
```

**Sorting and text search are NOT supported.** The backing Google Sheets provider ignores them, so the API does not implement them — do not pass `sort`, `order`, `search`, or `q` options expecting an effect. `count(filter?)` accepts the same filter forms.

## Count

`count(project, table, filter?)` returns a `number` (the total matching records). It reads `meta.total` from a limited list request; there is no separate count endpoint.

```ts
const active = await api.count("acme", "users", "status:active");
const invited = await api.count("acme", "users", { status: "invited" });
```

## Error handling

All failures are instances of `GridApiError`, which exposes `status?: number`, `code?: string`, `body?: string`, and `retryable: boolean`. Messages and bodies are automatically **redacted** via the exported `redact()` helper (Bearer tokens and live keys are stripped) before they reach logs or callers.

Exported error classes:

| Class | When |
| --- | --- |
| `GridApiError` | Base class; uncategorized failures (other 5xx, etc.) |
| `AuthenticationError` | Missing key, or HTTP 401 |
| `AuthorizationError` | HTTP 403 — key lacks access to the project |
| `ProjectNotFoundError` | HTTP 404 for an unknown project |
| `TableNotFoundError` | HTTP 404 for an unknown table/sheet |
| `RecordNotFoundError` | HTTP 404 for an unknown record id |
| `ValidationError` | HTTP 400/422 — bad or disallowed fields |
| `RateLimitError` | HTTP 429 — transient throttling (retryable) |
| `QuotaExceededError` | HTTP 429 with code `SUBSCRIPTION_LIMIT_REACHED` — terminal |
| `UpstreamProviderError` | HTTP >= 502 — backing provider failure (retryable) |
| `NetworkError` | Transport failure (retryable) |
| `TimeoutError` | Request exceeded `timeoutMs` (retryable) |

```ts
import { GridApiClient, GridApiError, RecordNotFoundError, ValidationError } from "@gridapi/sdk";

try {
  const user = await api.getById("acme", "users", "42");
} catch (err) {
  if (err instanceof RecordNotFoundError) {
    console.warn("No such record");
  } else if (err instanceof ValidationError) {
    console.error("Bad input:", err.status, err.code);
  } else if (err instanceof GridApiError) {
    // status, code, body and message are redacted and safe to log.
    console.error(`GridAPI ${err.status}: ${err.message} (retryable=${err.retryable})`);
  } else {
    throw err;
  }
}
```

## Timeouts and retries

- Each attempt uses `timeoutMs` (default 30000); an aborted request raises `TimeoutError`.
- **Safe methods retried:** `GET`, `DELETE`, `PUT`.
- **`PATCH` is opt-in:** retried only with `retryWrites: true` (default `false`), because it is non-idempotent.
- **`POST` is never retried**, so a new insert cannot be duplicated.
- Retries apply to transient failures: `RateLimitError`, `UpstreamProviderError`, `NetworkError`, `TimeoutError`, and other retryable 5xx responses.
- **`Retry-After` is honored** (numeric seconds or an HTTP date); otherwise exponential backoff with jitter is used, starting at `retryDelayMs` (default 500 ms) and capped at 30 s.
- `maxRetries` (default 2) is the number of **retries**, i.e. up to 3 total attempts.
- **Quota exhaustion is terminal:** `QuotaExceededError` (`SUBSCRIPTION_LIMIT_REACHED`) is never retried.

```ts
const api = new GridApiClient({
  apiKey: process.env.GRIDAPI_API_KEY,
  timeoutMs: 10_000,
  maxRetries: 3,
  retryDelayMs: 400,
  retryWrites: true, // allow PATCH retries
});
```

## API reference

The complete HTTP contract — every endpoint, parameter, response shape, and error code — is in [`openapi/openapi.yaml`](../../openapi/openapi.yaml) in this repository (also at [github.com/bugylabs/gridapi-sdk](https://github.com/bugylabs/gridapi-sdk/blob/main/openapi/openapi.yaml)). It is the single source of truth for all official SDKs; this client exposes no endpoint that is absent from it.

## Examples

Discovery and typed records:

```ts
import { GridApiClient, GridRecord } from "@gridapi/sdk";

const api = new GridApiClient();

// Discovery reads: no quota consumed by these calls.
const account = await api.getMe();                 // { authenticated, account?, subscription? }
const usage = await api.getUsage();                // { totalApiCallsUsed, totalApiCallsLeft, ... }
const projects = await api.listProjects();         // ProjectInfo[]
const tables = await api.listTables("acme");      // SheetInfo[]
const schema = await api.getSchema("acme", "users");

type User = GridRecord & { id: string; name: string; status: string };
const res = await api.get<User>("acme", "users", { filter: { status: "active" }, limit: 50 });
```

A read → update → verify flow:

```ts
const user = await api.getById("acme", "users", "42");
await api.patch("acme", "users", "42", { status: "invited" });
const updated = await api.getById("acme", "users", "42");
if (updated.status !== "invited") throw new Error("update did not stick");
```

More runnable code lives in [`examples/javascript/`](../../examples/javascript).

## Contributing

See [`CONTRIBUTING.md`](../../CONTRIBUTING.md) at the repository root. Key rules: the OpenAPI contract is authoritative, the key is never committed, and shared behavior changes need a contract-test case. From this package:

```bash
npm install
npm run build       # ESM + CJS + declarations
npm run typecheck   # noEmit
npm test            # starts tests/contract/mock-server.mjs itself
```

## License

Source Available | Proprietary | Non-Commercial Use Only. Personal, educational, research, and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See [`LICENSE`](../../LICENSE).

## Support and issues

Report bugs, request features, and ask questions at [github.com/bugylabs/gridapi-sdk/issues](https://github.com/bugylabs/gridapi-sdk/issues). Repository: [github.com/bugylabs/gridapi-sdk](https://github.com/bugylabs/gridapi-sdk).
