# Execution plan (working)

Source of truth for the API: `gridapi_server` working tree at
`/Users/rohan3155/Desktop/lab/micro-saas/gridapi_server` (uncommitted changes
present — read-only; never modified).

## Contract facts (verified from server source)
- Fastify, single hardcoded version `/api/v1`; no OpenAPI spec exists in server.
- Public data API: `GET|POST|PUT|DELETE /api/v1/{projectSlug}/{endpoint}` and
  `GET|PATCH|PUT|DELETE /api/v1/{projectSlug}/{endpoint}/{id}`. POST not allowed on `/:id`.
- Query on list: `filter` (`col:value`, comma separated), `limit` (1..100, default 100),
  `page` (offset only when both page+limit present). `search`/`sort`/`order` are
  accepted but IGNORED for live Google Sheets — SDKs must not claim sorting.
- `x-filter` header drives collection PUT/DELETE.
- Discovery (Bearer key, no quota): `GET /api/v1/me`, `/me/projects`, `/me/usage`,
  `/me/projects/{projectRef}/docs`. Key-free: `/api/v1/capabilities`,
  `/api/v1/public-docs/{slug}/{endpoint}`.
- Auth: `Authorization: Bearer <key>`. Key prefixes `gp_live_` (account) and
  `gapi_live_` (legacy project). Project slug is authorization target, never a credential.
- Success statuses are always 200. List `{data,meta:{total,limit,offset}}`;
  single GET bare record; POST `{inserted,records}`; PATCH/PUT `{updated,record}`;
  DELETE `{deleted,id}`.
- Errors: middleware `{error,code}`; data-plane catch strips `code` → `{error}`.
  Quota 429 `{code:"SUBSCRIPTION_LIMIT_REACHED",message}`. Rate limit = plugin
  defaults with `Retry-After` headers, no custom body.
- Rate limits/sec by plan: free 1, indie 5, startup 20, business 50.

## Method mapping (no invented endpoints)
get→GET collection; getById→GET /:id; post→POST collection; patch→PATCH /:id;
put→PUT /:id; delete→DELETE /:id; query→GET with filter/page/limit;
count→GET with limit=1 returning meta.total; listProjects→/me/projects;
listTables→sheets from /me/projects; getSchema→/me/projects/{ref}/docs.

## Status
- [x] Discovery of server contract and client docs architecture
- [x] Repository scaffold, LICENSE, SECURITY, CONTRIBUTING, .gitignore
- [x] openapi/openapi.yaml + validation script (14 operations, 8 paths)
- [x] 6 SDK implementations + package metadata + per-language README
- [x] shared contract tests + mock backend (per-SDK suites)
- [x] examples, docs manifest, independent-tag CI workflows, root README
- [x] gridapi_client docs: SDK section rendered from sync'd manifest + stale auth text corrected
- [x] builds/tests/typechecks: all 6 SDK suites pass; contract validator OK; gridapi_client tsc --noEmit and npm run build pass
