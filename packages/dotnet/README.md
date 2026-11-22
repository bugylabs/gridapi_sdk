# GridAPI .NET SDK

Official server-side .NET SDK for the [GridAPI public REST API](../../openapi/openapi.yaml). It is a thin, dependency-free wrapper over `System.Net.Http` and `System.Text.Json`: two required keys — an account key that authenticates the request and a project key that guards local scope — and every operation is exposed as an `Async` method that accepts a `CancellationToken`.

Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

> **Server-side only.** The SDK sends only the account key in an `Authorization: Bearer` header. The **project key is never transmitted, logged or echoed**: the public REST API has no project-key header, so the SDK uses it strictly as a local scope guard. Never ship either key in a browser bundle, mobile/desktop app, or any code an end user can inspect.

Repository: https://github.com/bugylabs/gridapi-sdk

## Features and supported operations

| Operation | Method | HTTP |
| --- | --- | --- |
| List/get records (filter, paginate) | `GetAsync`, `QueryAsync` | `GET /api/v1/{project}/{table}` |
| Get one record by id | `GetByIdAsync` | `GET /api/v1/{project}/{table}/{id}` |
| Insert one or many records | `PostAsync` | `POST /api/v1/{project}/{table}` |
| Update a record by id | `PatchAsync` | `PATCH /api/v1/{project}/{table}/{id}` |
| Replace a record by id | `PutAsync` | `PUT /api/v1/{project}/{table}/{id}` |
| Delete a record by id | `DeleteAsync` | `DELETE /api/v1/{project}/{table}/{id}` |
| Count matching records | `CountAsync` | (uses `GetAsync` with `limit=1`) |
| List projects | `ListProjectsAsync` | `GET /api/v1/me/projects` |
| List tables in a project | `ListTablesAsync` | (from `ListProjectsAsync`) |
| Get a table schema | `GetSchemaAsync` | `GET /api/v1/me/projects/{project}/docs` |
| Get authenticated account info | `GetMeAsync` | `GET /api/v1/me` |
| Get quota/usage info | `GetUsageAsync` | `GET /api/v1/me/usage` |
| Verify keys and project scope | `VerifyAsync` | `GET /api/v1/me`, `GET /api/v1/me/projects` |

Every method has an `Async` suffix and an optional trailing `CancellationToken cancellationToken = default`. Generic overloads (`GetAsync<T>`, `GetByIdAsync<T>`, `PostAsync<T>`, `PatchAsync<T>`, `PutAsync<T>`, `QueryAsync<T>`) let you map rows to your own class deriving from `GridRecord` with a parameterless constructor.

## Requirements

- .NET 10.0 or later (`net10.0`). No third-party runtime dependencies.

## Install

```sh
dotnet add package GridAPI
```

Or reference the project directly from a checkout of this monorepo:

```sh
dotnet add reference packages/dotnet/GridAPI/GridAPI.csproj
```

## Quick start

```csharp
using GridAPI;

using var client = new GridApiClient(); // reads GRIDAPI_API_KEY and GRIDAPI_PROJECT_KEY from the environment

await client.VerifyAsync("acme"); // optional, but fails fast on bad keys or scope

var projects = await client.ListProjectsAsync();
foreach (var project in projects)
    Console.WriteLine($"{project.Slug}: {project.Sheets.Count} tables");

var page = await client.GetAsync("acme", "users", new QueryOptions { Limit = 10 });
Console.WriteLine($"{page.Data.Count} of {page.Meta.Total} users on this page");
```

## Authentication

Authentication uses **two required keys**, both validated at construction (missing, empty, wrong-prefix, or empty-suffix values throw `ConfigurationException`; key values are never included in the error):

- `GRIDAPI_API_KEY` (`gp_live_...`, or `GridApiOptions.ApiKey`) — the account key. This is the **only** key sent, as `Authorization: Bearer <account key>`.
- `GRIDAPI_PROJECT_KEY` (`gapi_live_...`, or `GridApiOptions.ProjectKey`) — the project key. It is a **local scope guard only**: the public REST API has no project-key header, so the SDK never transmits it. It is never logged or echoed.

`VerifyAsync(project)` checks `/api/v1/me` for authentication, then `/api/v1/me/projects` to confirm the project matches its slug, id, or name. The account check is cached per client and verified projects are remembered. Every data method (`GetAsync`, `PostAsync`, `PatchAsync`, `PutAsync`, `DeleteAsync`, `CountAsync`, …) calls `VerifyAsync` for its project first; discovery methods (`ListProjectsAsync`, `ListTablesAsync`, `GetSchemaAsync`, `GetMeAsync`, `GetUsageAsync`) serve the check itself and never re-trigger it.

The **project slug** is not a credential. It is a routing/authorization target: the server checks whether the key's account is authorized for that project. Keep both keys server-side and store them in a secret manager, never in source control or client code.

Error messages and response bodies are passed through `GridApiError`, which redacts Bearer tokens and `gp_live_`/`gapi_live_` keys before they reach a log or user (`GridApiRedaction.Redact`).

## Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key (`gp_live_...`); sent as Bearer token. |
| `GRIDAPI_PROJECT_KEY` | yes | — | Project key (`gapi_live_...`); local scope guard, never transmitted. |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL, including an optional path prefix. |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-attempt request timeout in milliseconds. |

All four can be overridden per client through `GridApiOptions`, along with retry settings:

| `GridApiOptions` property | Type | Default | Purpose |
| --- | --- | --- | --- |
| `ApiKey` | `string?` | `GRIDAPI_API_KEY` | Explicit account key; overrides the environment. |
| `ProjectKey` | `string?` | `GRIDAPI_PROJECT_KEY` | Explicit project key; overrides the environment. Never transmitted. |
| `BaseUrl` | `string?` | `GRIDAPI_BASE_URL` or `https://api.gridapi.in` | Base URL; a trailing `/` is trimmed. |
| `TimeoutMs` | `int?` | `GRIDAPI_TIMEOUT_MS` or `30000` | Per-attempt timeout in ms. |
| `MaxRetries` | `int?` | `2` | Retries for transient failures on safe methods. |
| `RetryDelayMs` | `int?` | `500` | Base backoff in ms (exponential with jitter). |
| `RetryWrites` | `bool?` | `false` | Allow retries of non-idempotent `PATCH`; `POST` is never retried. |

## Initialization

```csharp
using GridAPI;

// From the environment (both GRIDAPI_API_KEY and GRIDAPI_PROJECT_KEY required):
using var client = new GridApiClient();

// Or configure explicitly:
using var configured = new GridApiClient(new GridApiOptions
{
    ApiKey = accountKeyFromYourSecretStore,
    ProjectKey = projectKeyFromYourSecretStore,
    BaseUrl = "https://api.gridapi.in",
    TimeoutMs = 10_000,
    MaxRetries = 2,
    RetryDelayMs = 500,
    RetryWrites = false,
});
```

`GridApiClient` implements `IDisposable`; dispose it (or use `using`) to release the underlying `HttpClient`.

## GET records

`GetAsync` returns a `ListResponse<GridRecord>` with `Data` (a list of rows) and `Meta` (`Total`, `Limit`, `Offset`, `CacheHit`). `QueryAsync` is an alias with the same signature.

```csharp
var page = await client.GetAsync("acme", "users", new QueryOptions
{
    Filter = "status:active",
    Page = 1,
    Limit = 10,
});

foreach (GridRecord row in page.Data)
    Console.WriteLine($"{row["id"]}: {row["name"]}");

Console.WriteLine($"total={page.Meta.Total}, cacheHit={page.Meta.CacheHit}");

// Strongly typed rows:
var typed = await client.GetAsync<MyRow>("acme", "users");
```

## GET by ID

`GetByIdAsync` returns a single `GridRecord` (or your `T`).

```csharp
GridRecord row = await client.GetByIdAsync("acme", "users", "42");
Console.WriteLine(row["name"]);
```

## POST

`PostAsync` inserts one `GridRecord` or an `IEnumerable<GridRecord>` and returns `CreateResponse<T>` with `Inserted` and `Records`.

```csharp
var created = await client.PostAsync("acme", "users", new GridRecord
{
    ["name"] = "Ann",
    ["status"] = "active",
});
Console.WriteLine($"inserted={created.Inserted}, id={created.Records[0]["id"]}");

// Bulk insert:
var batch = await client.PostAsync("acme", "users", new[]
{
    new GridRecord { ["name"] = "Bo" },
    new GridRecord { ["name"] = "Cy" },
});
```

`POST` is never retried because it is not idempotent.

## PATCH

`PatchAsync` applies partial changes to a record by id and returns `UpdateResponse<T>` with `Updated` and `Record`.

```csharp
var updated = await client.PatchAsync("acme", "users", "42", new GridRecord
{
    ["status"] = "inactive",
});
Console.WriteLine($"updated={updated.Updated}");
```

`PATCH` is retried only when `GridApiOptions.RetryWrites` is `true` (off by default).

## PUT

`PutAsync` replaces a record by id and returns `UpdateResponse<T>`. It is treated as idempotent and may be retried.

```csharp
var replaced = await client.PutAsync("acme", "users", "42", new GridRecord
{
    ["id"] = "42",
    ["name"] = "Ann",
    ["status"] = "active",
});
Console.WriteLine($"updated={replaced.Updated}");
```

## DELETE

`DeleteAsync` removes a record by id and returns a `DeleteResponse` with `Deleted` and `Id`. It may be retried.

```csharp
DeleteResponse deleted = await client.DeleteAsync("acme", "users", "42");
Console.WriteLine($"deleted={deleted.Deleted}, id={deleted.Id}");
```

## Filtering and pagination

`QueryOptions` carries three optional fields:

- `Filter` — exact-match filters as `column:value` pairs separated by commas, e.g. `"status:active,team:growth"`. It can also be built from a dictionary via `new QueryOptions(IDictionary<string, string>)`.
- `Page` — 1-based page number.
- `Limit` — page size; the server clamps it to 1–100 (default 100).

`Meta.Total` in the response is the number of matching rows before pagination.

```csharp
var q = new QueryOptions(new Dictionary<string, string>
{
    ["status"] = "active",
    ["team"] = "growth",
}) { Page = 2, Limit = 25 };

var result = await client.QueryAsync("acme", "users", q);
Console.WriteLine($"page 2 of {result.Meta.Total} rows");
```

**Sorting and text search are not supported.** The API accepts `sort`, `order`, and `search` only for backwards compatibility and ignores them for live Google Sheets data. Do not rely on them for ordering or search; `GridApiOptions`/`QueryOptions` deliberately do not expose them.

## Count

There is no dedicated count endpoint. `CountAsync` issues a `GetAsync` with `limit=1` and returns `Meta.Total` for the optional exact-match `filter`.

```csharp
int total = await client.CountAsync("acme", "users", "status:active");
Console.WriteLine($"active users: {total}");
```

## Discovery

```csharp
IReadOnlyList<ProjectInfo> projects = await client.ListProjectsAsync();
IReadOnlyList<SheetInfo> tables = await client.ListTablesAsync("acme");
TableSchema schema = await client.GetSchemaAsync("acme", "users");
AccountInfo me = await client.GetMeAsync();
UsageInfo usage = await client.GetUsageAsync();
```

Discovery calls do not re-trigger `VerifyAsync` — they are the discovery the check itself uses. `VerifyAsync("acme")` throws `AuthenticationError` if the account is not authenticated, `AuthorizationError` if the account has no authorized projects, and `ProjectNotFoundError` if the project is absent from a non-empty list. `ListTablesAsync` resolves a project by slug, id, or name and throws `ProjectNotFoundError` if it does not exist. `GetSchemaAsync` throws `TableNotFoundError` if the table is absent. `UsageInfo` is a `Dictionary<string, JsonElement>`; read values with `usage["totalApiCallsLeft"].GetInt32()`.

## Error handling

All SDK exceptions derive from `GridApiError : Exception` and expose `int? Status`, `string? Code`, `string? Body`, and `bool Retryable`. Messages and bodies have credentials redacted.

| Exception | When |
| --- | --- |
| `ConfigurationException` | A key is missing, has the wrong prefix, or has an empty suffix (construction). |
| `AuthenticationError` | HTTP 401, or the account check reports `authenticated: false`. |
| `AuthorizationError` | HTTP 403, or `VerifyAsync` finds no authorized projects. |
| `ProjectNotFoundError` | Unknown project in `VerifyAsync`/a data method, HTTP 404 for a project, or unknown project in `ListTablesAsync`. |
| `TableNotFoundError` | HTTP 404 for a sheet/table (also thrown by `GetSchemaAsync`). |
| `RecordNotFoundError` | HTTP 404 for a single record. |
| `ValidationError` | HTTP 400–422 (bad filter, empty payload, invalid input). |
| `RateLimitError` | HTTP 429 short-window rate limit (`Retryable = true`). |
| `QuotaExceededError` | HTTP 429 with `code: SUBSCRIPTION_LIMIT_REACHED`; terminal (`Retryable = false`). |
| `UpstreamProviderError` | HTTP 502+ from the backing data provider. |
| `NetworkError` | `HttpRequestException` / `IOException`. |
| `TimeoutError` | A per-attempt timeout elapsed. |

```csharp
try
{
    var row = await client.GetByIdAsync("acme", "users", "42");
}
catch (RecordNotFoundError ex)
{
    Console.Error.WriteLine($"No such record (status {ex.Status}, code {ex.Code})");
}
catch (QuotaExceededError ex)
{
    Console.Error.WriteLine("Quota exhausted — stop calling until reset; retrying will not help.");
}
catch (GridApiError ex)
{
    Console.Error.WriteLine($"{ex.GetType().Name}: {ex.Message} (status {ex.Status}, code {ex.Code}, retryable {ex.Retryable})");
}
```

## Timeouts and retries

Each attempt uses `TimeoutMs` (default 30000) as a cancellation deadline; when it elapses the SDK raises `TimeoutError`. Retries apply only to **safe methods**: `GET`, `DELETE`, `PUT`, and `PATCH` only when `RetryWrites = true`. `POST` is never retried.

A retry happens on retryable errors (e.g. 5xx, `RateLimitError`, `NetworkError`, `TimeoutError`) while `attempt < MaxRetries`. The SDK honors a `Retry-After` response header when present; otherwise it uses exponential backoff from `RetryDelayMs` with jitter, capped at 30 seconds. `QuotaExceededError` is terminal and is never retried regardless of settings. Caller cancellation via the passed `CancellationToken` always stops immediately.

```csharp
using var client = new GridApiClient(new GridApiOptions
{
    MaxRetries = 3,        // safe methods only
    RetryDelayMs = 500,    // exponential backoff with jitter, honors Retry-After
    RetryWrites = false,   // set true only if your PATCH is safe to repeat
});
```

## API reference

The wire contract — endpoints, parameters, error codes, and schemas — is in [`openapi/openapi.yaml`](../../openapi/openapi.yaml).

## Realistic example

A small sync that pages through active users and deactivates stale ones. Configuration comes only from the environment.

```csharp
using GridAPI;

using var client = new GridApiClient();

var stale = await client.GetAsync("acme", "users", new QueryOptions
{
    Filter = "status:active",
    Limit = 100,
});

foreach (var row in stale.Data)
{
    if (row["lastSeen"] is { } seen && string.CompareOrdinal(seen, "2026-01-01") < 0)
    {
        await client.PatchAsync("acme", "users", row["id"]!, new GridRecord
        {
            ["status"] = "inactive",
        });
    }
}

Console.WriteLine($"Checked {stale.Meta.Total} active users");
```

A runnable version lives in [`examples/dotnet`](../../examples/dotnet). Run it with:

```sh
export GRIDAPI_API_KEY='gp_live_...'
export GRIDAPI_PROJECT_KEY='gapi_live_...'
dotnet run --project examples/dotnet
```

## Contributing

Issues and pull requests are welcome at https://github.com/bugylabs/gridapi-sdk . Build and verify changes locally:

```sh
dotnet build packages/dotnet/GridAPI/GridAPI.csproj
bash tests/contract/dotnet/run.sh
```

## License

Source Available. Proprietary. Non-commercial use only. Copyright (c) 2026 BugyLabs. All Rights Reserved. This is **not** open source and grants no open-source rights. Personal, educational, research, and non-commercial evaluation use is permitted; commercial use requires a separate written agreement with BugyLabs. See [`LICENSE`](../../LICENSE).

## Support / issues

For bugs, questions, and feature requests, open an issue at https://github.com/bugylabs/gridapi-sdk/issues .
