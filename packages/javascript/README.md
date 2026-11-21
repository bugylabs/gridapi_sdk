# @gridapi/sdk

Official TypeScript/JavaScript SDK for the GridAPI public REST API.

Install:

```bash
npm install @gridapi/sdk
```

Quickstart (server-side only — the key is account-level and must never ship to a browser):

```ts
import { GridApiClient } from "@gridapi/sdk";

const api = new GridApiClient({ apiKey: process.env.GRIDAPI_API_KEY });
const users = await api.get("acme", "users", { filter: "status:active", limit: 50 });
const created = await api.post("acme", "users", { name: "Ann" });
await api.patch("acme", "users", created.records[0].id, { status: "invited" });
```

Environment: `GRIDAPI_API_KEY` (required), `GRIDAPI_BASE_URL` (default `https://api.gridapi.in`), `GRIDAPI_TIMEOUT_MS` (default `30000`).

Operations: `get`, `getById`, `post`, `patch`, `put`, `delete`, `query`, `count`, `listProjects`, `listTables`, `getSchema`, `getMe`, `getUsage`. Filtering is exact equality (`col:value` or `{ col: "value" }`); pagination is `page` + `limit` (server clamps 1–100). Sorting and text search are not available because the server ignores them.

Errors are typed (`AuthenticationError`, `AuthorizationError`, `ProjectNotFoundError`, `TableNotFoundError`, `RecordNotFoundError`, `ValidationError`, `RateLimitError`, `QuotaExceededError`, `UpstreamProviderError`, `NetworkError`, `TimeoutError`) and messages are redacted. Safe methods retry transient 429/5xx/network failures with `Retry-After`; quota exhaustion is terminal; `POST` is never retried.

Build: `npm run build`. Test: `npm test` (starts the shared contract mock server itself).
