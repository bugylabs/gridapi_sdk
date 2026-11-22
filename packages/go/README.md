# GridAPI Go SDK

> Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

The official, standard-library-only Go client for the [GridAPI public REST API](../../openapi/openapi.yaml). It uses only `net/http` and `encoding/json` — no third-party dependencies — and exposes the same operations as the other GridAPI SDKs from one contract.

**Server-side only.** Two keys are required: `GRIDAPI_API_KEY` (`gp_live_...`) is the account-level credential sent as `Authorization: Bearer ...`, and `GRIDAPI_PROJECT_KEY` (`gapi_live_...`) is a local-only scope guard. The public REST API has **no project-key header**, so the project key is never transmitted, logged, or echoed — it is validated locally at construction. A project slug is a routing/authorization target, never a credential. The server resolves your role per project; this SDK checks membership locally as a convenience, not as a security boundary. Never ship either key in a browser bundle, mobile app, or other client-side code.

## Requirements

- Go 1.22 or newer.
- A GridAPI account, an account API key (`gp_live_...`), and a project key (`gapi_live_...`).

## Install

```sh
go get github.com/bugylabs/gridapi-sdk
```

The module root is `github.com/bugylabs/gridapi-sdk` (one monorepo, tagged `v1.0.0`); the SDK package lives at `packages/go`:

```go
import gridapi "github.com/bugylabs/gridapi-sdk/packages/go"
```

## Features / supported operations

Every operation is a method on `*gridapi.Client` and takes a `context.Context`:

| Operation | Method | Description |
| --- | --- | --- |
| List records | `Get` | Read a page of records from a sheet |
| List records (alias) | `Query` | Identical to `Get` |
| Get by ID | `GetByID` | Read one record by its primary key |
| Create | `Post` | Insert one record or a batch (never retried) |
| Patch | `Patch` | Merge changes into one record |
| Replace | `Put` | Replace one record |
| Delete | `Delete` | Delete one record |
| Count | `Count` | Count records matching a filter |
| Projects | `ListProjects` | List authorized projects and their sheets |
| Tables | `ListTables` | List the sheets of a project |
| Schema | `GetSchema` | Read a sheet's fields and primary key |
| Identity | `GetMe` | Read authenticated-account information |
| Usage | `GetUsage` | Read subscription API-call usage |

Also included: typed, branchable errors; redacted error messages; context deadlines; per-attempt timeouts; retries with exponential backoff, jitter, and `Retry-After` support; exact-match filtering and pagination; a `Verify` scope check run by every data method. `Client` is safe for concurrent use.

## Authentication

Both keys are required. Set the account key in `GRIDAPI_API_KEY` and the project key in `GRIDAPI_PROJECT_KEY`:

```sh
export GRIDAPI_API_KEY=gp_live_...
export GRIDAPI_PROJECT_KEY=gapi_live_...
```

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account key (`gp_live_...`), sent as `Authorization: Bearer ...`. |
| `GRIDAPI_PROJECT_KEY` | yes | — | Project key (`gapi_live_...`). Local scope guard only: never transmitted. |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API root; override for a proxy or test server. |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-attempt request timeout in milliseconds. |

`NewClient` returns `*ConfigurationError` if either key is missing, has the wrong prefix, or has an empty suffix. Key values never appear in error messages.

Environment variables are read by `NewClient` and can be overridden with options:

```go
client, err := gridapi.NewClient(
    gridapi.WithAPIKey("gp_live_..."),
    gridapi.WithProjectKey("gapi_live_..."),
    gridapi.WithBaseURL("https://api.gridapi.in"),
    gridapi.WithTimeout(10*time.Second),
)
```

The project key is **not** sent on any request. The public REST API has no project-key header; the local check is a convenience guard, not a security boundary — the server still resolves your role per project.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    gridapi "github.com/bugylabs/gridapi-sdk/packages/go"
)

func main() {
    client, err := gridapi.NewClient() // reads GRIDAPI_API_KEY and GRIDAPI_PROJECT_KEY
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    res, err := client.Get(ctx, "acme", "users", &gridapi.QueryOptions{Limit: 10, Page: 1})
    if err != nil {
        log.Fatal(err)
    }
    for _, row := range res.Data {
        fmt.Println(row["id"], row["name"])
    }
    fmt.Println("total:", res.Meta.Total)
}
```

`Record` is `map[string]any`; values are normally strings, and JSON `null` decodes to `nil`.

## Initialization

```go
client, err := gridapi.NewClient()
```

`NewClient` reads the environment, applies any `Option`, and returns `(*Client, error)`. Options: `WithAPIKey`, `WithProjectKey`, `WithBaseURL`, `WithTimeout`, `WithMaxRetries`, `WithRetryDelay`, `WithRetryWrites`, `WithHTTPClient`. Reuse one client for the lifetime of your process; it is concurrency-safe.

## Get records

```go
res, err := client.Get(ctx, "acme", "users", &gridapi.QueryOptions{
    Filter: map[string]string{"status": "active"},
    Limit:  25,
    Page:   1,
})
if err != nil {
    log.Fatal(err)
}
// res.Data is []gridapi.Record; res.Meta has Total, Limit, Offset, CacheHit.
```

`Query(ctx, project, table, params)` is an alias for `Get` with the same options.

## Get by ID

```go
rec, err := client.GetByID(ctx, "acme", "users", "usr_123")
if err != nil {
    log.Fatal(err) // *RecordNotFoundError on a missing row
}
fmt.Println((*rec)["name"])
```

`GetByID` returns `*Record` (a pointer to the row map).

## POST (create)

```go
created, err := client.Post(ctx, "acme", "users", gridapi.Record{
    "id": "usr_124", "name": "Ada", "status": "active",
})
// created.Inserted, created.Records
```

Pass a `[]gridapi.Record` to insert a batch. POST is **never retried** to avoid duplicate inserts.

## PATCH (partial update)

```go
updated, err := client.Patch(ctx, "acme", "users", "usr_124", gridapi.Record{
    "status": "inactive",
})
// updated.Updated, updated.Record
```

PATCH retries only when the client is created with `gridapi.WithRetryWrites(true)`.

## PUT (replace)

```go
replaced, err := client.Put(ctx, "acme", "users", "usr_124", gridapi.Record{
    "id": "usr_124", "name": "Ada", "status": "active",
})
// replaced.Updated, replaced.Record
```

## DELETE

```go
deleted, err := client.Delete(ctx, "acme", "users", "usr_124")
// deleted.Deleted, deleted.ID
```

## Filtering and pagination

`QueryOptions.Filter` is exact equality only. It accepts either a `map[string]string` or a `"col:value,col2:value2"` string:

```go
&gridapi.QueryOptions{Filter: map[string]string{"status": "active", "role": "admin"}, Limit: 50, Page: 2}
// equivalent to:
&gridapi.QueryOptions{Filter: "status:active,role:admin", Limit: 50, Page: 2}
```

`Page` is 1-based and only takes effect when `Limit` is also set.

**Sorting and search are not supported.** The backing Google Sheets provider ignores them, so this SDK does not expose options for them.

## Count

```go
n, err := client.Count(ctx, "acme", "users", map[string]string{"status": "active"})
if err != nil {
    log.Fatal(err)
}
fmt.Println("active users:", n)
```

The `filter` argument takes the same forms as `QueryOptions.Filter`; pass `nil` to count all rows.

## Discovery

```go
projects, err := client.ListProjects(ctx)          // []gridapi.Project
sheets, err := client.ListTables(ctx, "acme")      // []gridapi.Sheet
schema, err := client.GetSchema(ctx, "acme", "users") // *gridapi.TableSchema

me, err := client.GetMe(ctx)       // *gridapi.Account (Authenticated, Account.Email, Account.Scope)
usage, err := client.GetUsage(ctx) // *gridapi.Usage (TotalAPICallsUsed, TotalAPICallsLeft)
```

## Verification and project scope

Every data method (`Get`, `Query`, `GetByID`, `Post`, `Patch`, `Put`, `Delete`, `Count`) calls `Verify` before its request. You can also call it directly for an early check:

```go
if err := client.Verify(ctx, "acme"); err != nil {
    log.Fatal(err)
}
```

`Verify(ctx, project)` first calls `GET /api/v1/me` once per client and returns `*AuthenticationError` if `authenticated` is not true. When `project` is non-empty it then calls `GET /api/v1/me/projects` and accepts a `slug`, `id`, or `name` match. An empty project list yields `*AuthorizationError`; a non-empty list without a match yields `*ProjectNotFoundError`. The account check and each verified project are cached, discovery calls never re-enter `Verify`, and the method is safe for concurrent use.

## Error handling

Every failing operation returns a typed error that embeds `*gridapi.Error` (fields `Status`, `Code`, `Message`, `Body`, `Retryable`). Messages and bodies are redacted, so they are safe to log. Match a category with `errors.As`:

```go
users, err := client.Get(ctx, "acme", "users", nil)
if err != nil {
    var (
        auth  *gridapi.AuthenticationError
        quota *gridapi.QuotaExceededError
        rate  *gridapi.RateLimitError
    )
    switch {
    case errors.As(err, &auth):
        log.Fatal("check GRIDAPI_API_KEY: ", err) // 401; local scope errors use ProjectNotFoundError/AuthorizationError
    case errors.As(err, &quota):
        log.Fatal("subscription quota exhausted: ", err) // 429, terminal
    case errors.As(err, &rate):
        log.Print("rate limited: ", err) // 429, transient
    default:
        var api gridapi.APIError
        if errors.As(err, &api) {
            log.Printf("GridAPI %d %s (retryable=%v)", api.StatusCode(), api.ErrorCode(), api.IsRetryable())
        } else {
            log.Printf("unexpected error: %v", err)
        }
        return
    }
}
_ = users
```

The full taxonomy is `ConfigurationError` (missing or malformed key, before any request), `AuthenticationError` (401), `AuthorizationError` (403), `ProjectNotFoundError`, `TableNotFoundError`, `RecordNotFoundError` (404 variants), `ValidationError` (400/422), `RateLimitError` and `QuotaExceededError` (429), `UpstreamError` (502+), `NetworkError`, `TimeoutError`. The `AuthorizationError` and `ProjectNotFoundError` raised by `Verify` for an empty or non-matching project list are client-side and have no HTTP status. Unclassified responses return `*gridapi.Error`. All of them satisfy the `gridapi.APIError` interface, so `errors.As(err, &api)` works for category-agnostic access to `StatusCode()`, `ErrorCode()`, and `IsRetryable()`.

## Timeouts and retries

- Each attempt is bounded by both your `context.Context` and the per-attempt timeout (`GRIDAPI_TIMEOUT_MS` / `WithTimeout`, default 30s). A cancelled or expired context yields `*TimeoutError`.
- Retries default to 2 after the first attempt (`WithMaxRetries`), with exponential backoff from 500ms (`WithRetryDelay`) plus jitter, capped at 30s. A `Retry-After` header (seconds or HTTP date) takes precedence.
- Safe methods are retried on transient errors: `Get`, `Query`, `GetByID`, `DELETE`, and `PUT`.
- `PATCH` is retried **only** with `WithRetryWrites(true)`.
- `POST` is **never** retried.
- `QuotaExceededError` (429 `SUBSCRIPTION_LIMIT_REACHED`) is terminal (`Retryable == false`) and is not retried. `RateLimitError` (429 without that code) is transient and is retried.
- Supply `WithHTTPClient` to use a custom transport (e.g. a proxy).

## API reference

The canonical contract is [`openapi/openapi.yaml`](../../openapi/openapi.yaml) (with a generated `openapi.json` alongside it). It is the source of truth for paths, request bodies, and error codes.

## Examples

A runnable program is at [`examples/go/main.go`](../../examples/go/main.go):

```sh
export GRIDAPI_API_KEY=gp_live_...
export GRIDAPI_PROJECT_KEY=gapi_live_...
go run ./examples/go <project> [table]
```

## Contract tests

Unit tests for key validation and `Verify` need no external services; contract tests start the shared mock server themselves. From the repository root, once Go and Node are installed:

```sh
go test ./packages/go/ ./tests/contract/go/ -v
```

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md). Please keep changes aligned with the shared contract in `openapi/openapi.yaml` and add/update contract tests for behavior changes.

## License

Source Available / Proprietary / Non-Commercial Use Only. Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research, and non-commercial evaluation use only; commercial use requires a separate written agreement with BugyLabs. This is **not** open source and is **not** licensed under MIT or any other open-source license. See [LICENSE](../../LICENSE).

## Support and issues

Repository: https://github.com/bugylabs/gridapi-sdk — open an issue there for bugs and questions. For security reports, see [SECURITY.md](../../SECURITY.md).
