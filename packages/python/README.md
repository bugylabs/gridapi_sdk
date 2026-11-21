# GridAPI Python SDK

Official, dependency-free Python client for the GridAPI public REST API. Uses
only the standard library (`urllib.request`), so there is nothing to install
alongside it.

> **Server-side only.** The SDK puts your account-level API key in an
> `Authorization: Bearer ...` header. Never ship the key in a browser bundle,
> mobile app, or any other client-side code. The project slug is a routing
> target, not a credential.

## Install

From PyPI:

```sh
pip install gridapi-sdk
```

From a checkout of this monorepo:

```sh
pip install -e ./packages/python
```

Requires Python 3.8+. No third-party runtime packages (an editable install needs
setuptools available to pip as build tooling; alternatively, put `packages/python`
on your `PYTHONPATH` and skip installation entirely).

## Quickstart

```python
from gridapi import GridApiClient

client = GridApiClient()  # reads credentials from the environment

# List, filter, paginate
page = client.get("acme", "users", {"filter": {"status": "active"}, "limit": 10, "page": 1})
for row in page["data"]:
    print(row["id"], row["name"])
print("total:", page["meta"]["total"])

# One record, create, update, delete
row = client.get_by_id("acme", "users", "1")
created = client.post("acme", "users", {"name": "Dee", "status": "active"})
client.patch("acme", "users", created["records"][0]["id"], {"status": "invited"})
client.put("acme", "users", created["records"][0]["id"], {"name": "Dee", "status": "active"})
client.delete("acme", "users", created["records"][0]["id"])

# Count without fetching every row
total = client.count("acme", "users", filter={"status": "active"})
```

Discovery reads (no quota cost):

```python
client.list_projects()                       # authorized projects
client.list_tables("acme")                   # sheets for a project
client.get_schema("acme", "users")           # endpoint schema
client.get_me()                              # account + subscription info
client.get_usage()                           # usage counters
```

`query(project, table, params)` is an alias for `get`. Filtering is exact-match
only; sorting and text search are intentionally not implemented because the
server ignores them.

## Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key (missing key raises `AuthenticationError`) |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL, e.g. a local mock server |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-request timeout in milliseconds |

Every value can be overridden in the constructor:

```python
client = GridApiClient(
    api_key="...",
    base_url="https://api.gridapi.in",
    timeout_ms=30000,
    max_retries=2,
    retry_delay_ms=500,
    retry_writes=False,
)
```

One key authenticates every authorized project.

## Errors

All errors derive from `GridApiError` and carry `name`, `status`, `code`, `body`
and `retryable`. Messages and reprs are redacted, so printing an exception is
safe.

| Error | When |
| --- | --- |
| `AuthenticationError` | 401, or missing API key |
| `AuthorizationError` | 403 — key not allowed for this project |
| `ProjectNotFoundError` | 404 — unknown/unauthorized project |
| `TableNotFoundError` | 404 — unknown sheet/endpoint |
| `RecordNotFoundError` | 404 — unknown record |
| `ValidationError` | 400–422 — bad payload or unknown column |
| `RateLimitError` | 429 — transient rate limit (retried) |
| `QuotaExceededError` | 429 `SUBSCRIPTION_LIMIT_REACHED` — terminal, never retried |
| `UpstreamProviderError` | 502+ — upstream provider failure (retried) |
| `NetworkError` | transport failure (retried) |
| `TimeoutError` | request timeout (retried) |

```python
from gridapi import GridApiClient, QuotaExceededError, GridApiError

try:
    client.get("acme", "users")
except QuotaExceededError:
    print("subscription limit reached — do not retry")
except GridApiError as err:
    print(err.name, err.status, err.retryable, str(err))
```

Retries apply to `GET`, `DELETE` and `PUT`; `PATCH` only with
`retry_writes=True`; `POST` never. Retry-After (integer seconds or HTTP date)
is honored, otherwise capped exponential backoff with jitter is used.

## Contract tests

```sh
python3 -m unittest discover -s tests/contract/python -v
```
