# GridAPI Go SDK

Official, dependency-free Go client for the [GridAPI public REST API](../../openapi/openapi.yaml). Requires Go 1.22+; uses only the standard library (`net/http`, `encoding/json`).

> **Server-side only.** The API key is account-level and authenticates every project you have access to. Never embed it in a browser bundle, mobile app, or other client-side code.

## Install

```sh
go get github.com/gridapi/gridapi-go
```

The module root is `github.com/gridapi/gridapi-go`; the SDK package lives at `github.com/gridapi/gridapi-go/packages/go`:

```go
import gridapi "github.com/gridapi/gridapi-go/packages/go"
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	gridapi "github.com/gridapi/gridapi-go/packages/go"
)

func main() {
	client, err := gridapi.NewClient() // reads GRIDAPI_API_KEY from the environment
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	users, err := client.Get(ctx, "acme", "users", &gridapi.QueryOptions{
		Filter: map[string]string{"status": "active"},
		Limit:  10,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, row := range users.Data {
		fmt.Println(row["id"], row["name"])
	}
}
```

Run with:

```sh
export GRIDAPI_API_KEY=gp_live_...
go run .
```

See [`examples/go`](../../examples/go) for a runnable program.

## Configuration

Configuration comes from the environment and can be overridden with functional options:

| Environment variable     | Default                    | Purpose                                  |
| ------------------------ | -------------------------- | ---------------------------------------- |
| `GRIDAPI_API_KEY`        | — (required)               | Account-level key; sent as Bearer token. |
| `GRIDAPI_BASE_URL`       | `https://api.gridapi.in`   | API root.                                |
| `GRIDAPI_TIMEOUT_MS`     | `30000`                    | Per-attempt request timeout.             |

```go
client, err := gridapi.NewClient(
	gridapi.WithAPIKey("..."),
	gridapi.WithBaseURL("http://localhost:4000"),
	gridapi.WithTimeout(5*time.Second),
	gridapi.WithMaxRetries(2),       // default
	gridapi.WithRetryDelay(500*time.Millisecond),
	gridapi.WithRetryWrites(true),   // allow retrying PATCH; POST is never retried
	gridapi.WithHTTPClient(myClient),
)
```

`GRIDAPI_API_KEY` is required; `NewClient` returns an `AuthenticationError` when it is missing. A project slug is only a routing/authorization target, never a credential.

## Methods

| Method | Description |
| ------ | ----------- |
| `Get(ctx, project, table, *QueryOptions)` | List records with optional filter/page/limit. |
| `Query(ctx, project, table, *QueryOptions)` | Alias for `Get`. |
| `GetByID(ctx, project, table, id)` | Fetch one record by primary key. |
| `Post(ctx, project, table, records)` | Insert one `Record` or a `[]Record`. Never retried. |
| `Patch(ctx, project, table, id, changes)` | Partial update; retried only with `WithRetryWrites(true)`. |
| `Put(ctx, project, table, id, record)` | Full replacement. |
| `Delete(ctx, project, table, id)` | Delete one record. |
| `Count(ctx, project, table, filter)` | Count via `limit=1` and `meta.total`. |
| `ListProjects(ctx)` | All authorized projects. |
| `ListTables(ctx, project)` | Sheets derived from `/api/v1/me/projects`. |
| `GetSchema(ctx, project, table)` | Schema derived from `/api/v1/me/projects/{ref}/docs`. |
| `GetMe(ctx)` | Authenticated account summary. |
| `GetUsage(ctx)` | API call counters. |

`QueryOptions.Filter` accepts a `"col:value,col2:value2"` string or a `map[string]string`. `Page` is 1-based and applies only when `Limit` is set. Sorting and search are intentionally not exposed — the API ignores them for live data.

## Errors

Errors are typed and work with `errors.As`. Every typed error embeds `*gridapi.Error`, so `Status`, `Code`, `Message`, `Body`, and `Retryable` are promoted. Error strings (and all stored fields) are redacted, so they are safe to log.

```go
var (
	auth   *gridapi.AuthenticationError
	quota  *gridapi.QuotaExceededError
	rate   *gridapi.RateLimitError
	apiErr gridapi.APIError
)

switch {
case errors.As(err, &auth):
	// 401: missing or invalid key
case errors.As(err, &quota):
	// 429 SUBSCRIPTION_LIMIT_REACHED — terminal, never retried
case errors.As(err, &rate):
	// 429 without the quota code — retryable, honours Retry-After
case errors.As(err, &apiErr):
	fmt.Println(apiErr.StatusCode(), apiErr.ErrorCode(), apiErr.IsRetryable())
}
```

Full taxonomy: `AuthenticationError`, `AuthorizationError`, `ProjectNotFoundError`, `TableNotFoundError`, `RecordNotFoundError`, `ValidationError`, `RateLimitError`, `QuotaExceededError`, `UpstreamError`, `NetworkError`, `TimeoutError`. Unclassified responses return `*gridapi.Error`.

Transient failures are retried on GET, DELETE, PUT, and (opt-in) PATCH with exponential backoff plus jitter; `Retry-After` (seconds or HTTP date) takes precedence. POST is never retried. Each attempt is bounded by `GRIDAPI_TIMEOUT_MS` and by the caller's context.

## Contract tests

The contract tests start the shared mock server themselves. Once Go is installed, from the repository root:

```sh
go test ./tests/contract/go -v
```

## License

MIT
