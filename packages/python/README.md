# GridAPI Python SDK

Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

`gridapi-sdk` 1.0.0 is the Python client for the GridAPI public REST API. It is a
small, synchronous wrapper over `urllib.request` from the Python standard
library: no third-party runtime dependencies, no async runtime, and no code
generation. One account-level API key authenticates every project authorized
for that account.

> **Server-side only.** This SDK sends the API key in an
> `Authorization: Bearer ...` header. Use it from a backend, a script, or a
> server-side job. Never embed the key in a browser bundle, mobile app, or any
> other client-side code. A project slug is a routing/authorization target,
> not a credential.

## Features and supported operations

The client is `GridApiClient` (also exported as the alias `GridAPI`). Every
public method is listed below.

| Method | Operation |
| --- | --- |
| `get(project, table, params=None)` | List/filter/paginate records |
| `get_by_id(project, table, id)` | Fetch one record by primary key |
| `post(project, table, record)` | Insert one record or a list of records |
| `patch(project, table, id, changes)` | Partially update a record |
| `put(project, table, id, record)` | Replace a record |
| `delete(project, table, id)` | Delete a record |
| `query(project, table, params=None)` | Alias for `get` |
| `count(project, table, filter=None)` | Count matching records (returns an integer) |
| `list_projects()` | List projects authorized by the key |
| `list_tables(project)` | List sheets/tables for a project |
| `get_schema(project, table)` | Read a table's schema |
| `get_me()` | Account, authentication, and subscription info |
| `get_usage()` | Account API usage counters |

Filtering is exact-match only. Pagination uses `page` and `limit`. Sorting and
text search are deliberately not exposed: the backing Google Sheets data source
ignores `sort`, `order`, and `search` even though the HTTP layer accepts them
for backwards compatibility.

Discovery reads (`list_projects`, `list_tables`, `get_schema`, `get_me`,
`get_usage`) consume no quota. Listing records consumes one quota unit per call
(which is why `count` also costs one).

## Installation

Install the distribution by name:

```sh
pip install gridapi-sdk
```

This checkout does not build or publish a wheel. To work from a clone of the
monorepo, install the source package in editable mode from the repository root:

```sh
pip install -e ./packages/python
```

Or skip installation entirely and put the package on `PYTHONPATH`:

```sh
PYTHONPATH=packages/python python3 your_script.py
```

## Requirements

- Python 3.8 or newer (`requires-python = ">=3.8"` in `packages/python/pyproject.toml`).
- Python standard library only; there are no runtime dependencies.

## Quick start

```python
from gridapi import GridApiClient

client = GridApiClient()  # reads GRIDAPI_API_KEY from the environment

# Discovery: which projects and tables can this key reach?
for project in client.list_projects():
    print(project["slug"])

tables = client.list_tables("acme")
print([table["endpoint"] for table in tables])

# Read a page of filtered records.
page = client.get("acme", "users", {"filter": {"status": "active"}, "limit": 10, "page": 1})
for row in page["data"]:
    print(row["id"], row["name"])
print("total matching:", page["meta"]["total"])

# Read one record, then create / update / delete a throwaway record.
one = client.get_by_id("acme", "users", "1")
created = client.post("acme", "users", {"name": "Dee", "status": "active", "team": "core"})
new_id = created["records"][0]["id"]
client.patch("acme", "users", new_id, {"status": "invited"})
client.put("acme", "users", new_id, {"name": "Dee", "status": "active", "team": "core"})
client.delete("acme", "users", new_id)

print("matching count:", client.count("acme", "users", filter={"status": "active"}))
```

A runnable version lives in the repository at
[`../../examples/python/quickstart.py`](../../examples/python/quickstart.py). It
reads credentials from the environment and contains no keys in source.

## Authentication

Set `GRIDAPI_API_KEY` to your account-level key before constructing the client:

```sh
export GRIDAPI_API_KEY="gp_live_..."       # account-level key; keep it secret
```

The key is account-level: one key works for every project that account is
authorized to use. The project slug you pass as the first argument to record
methods (for example `"acme"`) is only a routing/authorization target. It is
never a credential, and the SDK never makes authorization decisions — the
server resolves the caller's role per project.

The SDK sends `Authorization: Bearer <key>` on every request. Constructing a
client with no key available raises `AuthenticationError`. Error messages,
`code`, `body`, and `repr` are redacted, so an exception is safe to log.

Use this SDK only in server-side code. Do not ship the key to a browser, a
mobile app, or a client-side bundle.

## Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key; missing key raises `AuthenticationError` |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL, e.g. a local mock server |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-request timeout in milliseconds |

Every value can also be passed to the constructor, which takes precedence over
the environment.

## Initialization

```python
from gridapi import GridApiClient

# Environment configuration (recommended).
client = GridApiClient()

# Explicit configuration.
client = GridApiClient(
    api_key="gp_live_...",
    base_url="https://api.gridapi.in",
    timeout_ms=30000,
    max_retries=2,
    retry_delay_ms=500,
    retry_writes=False,
)
```

Constructor signature:

```python
GridApiClient(api_key=None, base_url=None, timeout_ms=None,
              max_retries=2, retry_delay_ms=500, retry_writes=False)
```

`GridAPI` is an alias of `GridApiClient`; the two names are interchangeable.

## Get records

`get(project, table, params=None)` issues `GET /api/v1/{project}/{table}` and
returns `{"data": [...], "meta": {"total": ..., "limit": ..., "offset": ...}}`.

```python
page = client.get("acme", "users")  # first page, server default limit (100)
rows = page["data"]
print(rows[0]["name"], "of", page["meta"]["total"])
```

## Get by ID

`get_by_id(project, table, id)` returns the bare record object for the table's
configured primary key. An unknown record raises `RecordNotFoundError`.

```python
user = client.get_by_id("acme", "users", "1")
print(user["id"], user["name"])
```

## POST

`post(project, table, record)` accepts a single record dict or a list of them
and returns `{"inserted": n, "records": [...]}`. POST is a write and is never
retried automatically.

```python
result = client.post("acme", "users", {"name": "Dee", "status": "active"})
new_id = result["records"][0]["id"]

# Batch insert.
batch = client.post("acme", "users", [
    {"name": "Eli", "status": "active"},
    {"name": "Fay", "status": "invited"},
])
print(batch["inserted"])
```

## PATCH

`patch(project, table, id, changes)` partially updates one record and returns
`{"updated": n, "record": {...}}`.

```python
result = client.patch("acme", "users", "1", {"status": "invited"})
print(result["updated"], result["record"]["status"])
```

## PUT

`put(project, table, id, record)` replaces one record and returns
`{"updated": n, "record": {...}}`. PUT is treated as idempotent and is retried
on transient failures.

```python
result = client.put("acme", "users", "1", {
    "name": "Ann",
    "status": "active",
    "team": "growth",
})
print(result["record"])
```

## DELETE

`delete(project, table, id)` removes one record and returns
`{"deleted": n, "id": "..."}`. DELETE is idempotent and is retried on
transient failures.

```python
result = client.delete("acme", "users", "1")
print(result["deleted"], result["id"])
```

## Filtering and pagination

`filter` may be a dict (converted to `column:value` pairs) or an already-encoded
string. `limit` is a page size (the server clamps it to 1–100; default 100) and
`page` is 1-based. Offset paging applies only when both `page` and `limit` are
supplied, so pass both when you iterate pages. `query(...)` is an alias for
`get(...)` and accepts the same parameters.

```python
page = client.get("acme", "users", {
    "filter": {"status": "active", "team": "growth"},  # exact matches, AND-ed
    "limit": 25,
    "page": 2,
})
# Equivalent string form: "status:active,team:growth"
same = client.query("acme", "users", {"filter": "status:active,team:growth"})

total = page["meta"]["total"]
rows = page["data"]
```

Sorting and text search are intentionally not supported. The API accepts
`sort`, `order`, and `search` for backwards compatibility, but the live Google
Sheets data source ignores them, so the SDK does not expose them.

## Count

`count(project, table, filter=None)` returns the integer number of matching
records. There is no dedicated count endpoint: the SDK issues a list request
with `limit=1` and reads `meta.total`. It still consumes one quota unit.

```python
active = client.count("acme", "users", filter={"status": "active"})
invited = client.count("acme", "users", filter={"status": "invited"})
```

## Error handling

Every error raised by the SDK derives from `GridApiError` and carries `name`,
`status`, `code`, `body`, and `retryable`. Messages, `code`, and `body` are
redacted so `str(error)` and `repr(error)` are safe to log.

| Exception | When it is raised |
| --- | --- |
| `GridApiError` | Base class; also used for unmapped HTTP errors |
| `AuthenticationError` | Missing API key, or HTTP 401 (invalid/expired key) |
| `AuthorizationError` | HTTP 403 — key not allowed for this project |
| `ProjectNotFoundError` | HTTP 404 — project unknown or unauthorized |
| `TableNotFoundError` | HTTP 404 — sheet/endpoint does not exist |
| `RecordNotFoundError` | HTTP 404 — record does not exist |
| `ValidationError` | HTTP 400–422 — bad payload or unknown column |
| `RateLimitError` | HTTP 429 — transient rate limit (retryable) |
| `QuotaExceededError` | HTTP 429 `SUBSCRIPTION_LIMIT_REACHED` — subscription quota exhausted (terminal) |
| `UpstreamProviderError` | HTTP 502+ — upstream provider failure (retryable) |
| `NetworkError` | Transport failure with no HTTP response (retryable) |
| `TimeoutError` | Request exceeded `timeout_ms` (retryable) |

`gridapi` also exports `redact(text, secret=None)`, the helper used to scrub
keys from text before logging.

```python
from gridapi import (
    GridApiClient,
    GridApiError,
    AuthenticationError,
    AuthorizationError,
    QuotaExceededError,
)

client = GridApiClient()

try:
    client.get("acme", "users")
except AuthenticationError:
    print("check GRIDAPI_API_KEY")
except AuthorizationError:
    print("this key may not access that project")
except QuotaExceededError:
    print("subscription limit reached — do not retry; wait or upgrade")
except GridApiError as err:
    # err.name mirrors the error class name; status/code/body/retryable are structured.
    print(err.name, err.status, err.code, err.retryable, str(err))
```

`QuotaExceededError` is terminal: never retry it in a loop. Check
`get_usage()["totalApiCallsLeft"]` before bulk work instead.

## Timeouts and retries

`timeout_ms` (default `30000`) is the per-attempt HTTP timeout. On a timeout,
transport failure, or retryable HTTP error, the SDK retries up to `max_retries`
times (default 2), sleeping `retry_delay_ms` (default 500 ms) as the base
backoff.

Retry eligibility depends on the method:

- `GET`, `DELETE`, and `PUT` are retried by default.
- `PATCH` is retried only when the client is constructed with `retry_writes=True`
  (off by default).
- `POST` is never retried automatically, because a retry could insert duplicates.

A `Retry-After` response header (integer seconds or an HTTP date) takes
precedence. Otherwise the SDK uses capped exponential backoff with jitter
(cap: 30 seconds). `QuotaExceededError` is never retried, regardless of method.
For writes that are not safe to repeat, prefer a unique business key and handle
duplicates at the application level.

## API reference

The canonical HTTP contract is [`openapi/openapi.yaml`](../../openapi/openapi.yaml)
in this monorepo (a generated `openapi/openapi.json` is also provided). All
methods above map directly to endpoints defined there; SDKs do not invent
endpoints.

## More examples

Paginate until exhausted:

```python
limit, page = 100, 1
while True:
    result = client.get("acme", "users", {
        "filter": {"status": "active"}, "limit": limit, "page": page,
    })
    for user in result["data"]:
        print(user["id"], user["name"])
    if len(result["data"]) < limit or page * limit >= result["meta"]["total"]:
        break
    page += 1
```

Create, verify, and clean up a record with error handling:

```python
from gridapi import GridApiClient, GridApiError, RecordNotFoundError

client = GridApiClient()
created = client.post("acme", "users", {"name": "Temp", "status": "active"})
record_id = created["records"][0]["id"]
try:
    try:
        row = client.get_by_id("acme", "users", record_id)
    except RecordNotFoundError:
        row = None
    print("visible before delete:", row is not None)
    print("deleted:", client.delete("acme", "users", record_id)["deleted"])
except GridApiError as err:
    print(f"{err.name} (status={err.status}, retryable={err.retryable}): {err}")
    raise
```

Check quota before a bulk import:

```python
usage = client.get_usage()
print("used:", usage["totalApiCallsUsed"], "left:", usage["totalApiCallsLeft"])
```

Point the SDK at a local mock server (used by the contract tests):

```python
client = GridApiClient(
    api_key="gp_live_testkey",  # public test fixture, not a real credential
    base_url="http://127.0.0.1:8787",
)
print(client.get_me()["authenticated"])
```

## Contributing

See [`CONTRIBUTING.md`](../../CONTRIBUTING.md) at the repository root. contract
first: an endpoint or parameter must exist in `openapi/openapi.yaml` before an
SDK exposes it. Never commit keys to source, fixtures, or logs. From the
repository root, run the shared contract suite and example checks:

```sh
python3 -m unittest discover -s tests/contract/python -v
python3 scripts/validate_contract.py
python3 scripts/check_examples.py
```

## License

Source Available | Proprietary | Non-Commercial Use Only. Copyright (c) 2026
BugyLabs. All Rights Reserved.

This is source-available, proprietary software. Personal, educational,
research, and non-commercial evaluation use is permitted under the terms in
[`../../LICENSE`](../../LICENSE). Commercial use requires a separate written
agreement with BugyLabs; contact `licensing@bugylabs.com`.

## Support and issues

Source and issue tracker: <https://github.com/bugylabs/gridapi-sdk>. For SDK
bugs or feature requests, open an issue there with the SDK version (`1.0.0`),
your Python version, and a minimal reproduction. Do not include live API keys
or customer data.

For security vulnerabilities, do **not** open a public issue — email
`security@gridapi.in` as described in [`SECURITY.md`](../../SECURITY.md).
