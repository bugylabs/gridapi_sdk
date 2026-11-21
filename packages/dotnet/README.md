# GridAPI .NET SDK

Official server-side .NET SDK for the [GridAPI public REST API](../../openapi/openapi.yaml). One account-level API key authenticates every project you own or have membership in; the project slug is a routing target, not a credential.

> **Server-side only.** Never ship your API key in a browser bundle, desktop/mobile client, or any other code a user can inspect.

## Install

```sh
dotnet add package GridAPI --version 0.1.0
```

Or reference `packages/dotnet/GridAPI/GridAPI.csproj` directly.

Requires .NET 10.0 or later. Uses only the base class library (`System.Net.Http`, `System.Text.Json`) — no runtime dependencies.

## Quickstart

```csharp
using GridAPI;

using var client = new GridApiClient(); // reads the environment variables below

var projects = await client.ListProjectsAsync();
foreach (var project in projects)
    Console.WriteLine($"{project.Slug}: {project.Sheets.Count} tables");

var users = await client.GetAsync("acme", "users", new QueryOptions { Filter = "status:active", Limit = 10 });
Console.WriteLine($"Total active users: {users.Meta.Total}");

var active = await client.QueryAsync("acme", "users", new QueryOptions { Filter = "status:active" });
var total = await client.CountAsync("acme", "users", "status:active");

await client.PostAsync("acme", "users", new GridRecord { ["name"] = "Ann", ["status"] = "active" });
await client.PatchAsync("acme", "users", "1", new GridRecord { ["status"] = "inactive" });
await client.PutAsync("acme", "users", "1", new GridRecord { ["id"] = "1", ["name"] = "Ann" });
await client.DeleteAsync("acme", "users", "1");
```

Discovery is available too: `ListProjectsAsync`, `ListTablesAsync(project)`, `GetSchemaAsync(project, table)`, `GetMeAsync`, and `GetUsageAsync`. All methods have an `Async` suffix and accept a `CancellationToken`.

## Configuration

| Environment variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key. The constructor throws `AuthenticationError` when missing. |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL, including an optional path prefix. |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-attempt request timeout in milliseconds. |

All three can be overridden per client with `GridApiOptions`:

```csharp
using var client = new GridApiClient(new GridApiOptions
{
    ApiKey = keyFromYourSecretStore,
    BaseUrl = "https://api.gridapi.in",
    TimeoutMs = 10_000,
    MaxRetries = 2,        // transient failures on GET/DELETE/PUT (+ PATCH with RetryWrites)
    RetryDelayMs = 500,    // exponential backoff with jitter
    RetryWrites = false,   // POST is never retried
});
```

## Errors

All exceptions derive from `GridApiError` and expose `Status`, `Code`, `Body`, and `Retryable`. Messages and bodies have credentials redacted. The hierarchy mirrors the JavaScript SDK: `AuthenticationError`, `AuthorizationError`, `ProjectNotFoundError`, `TableNotFoundError`, `RecordNotFoundError`, `ValidationError`, `RateLimitError`, `QuotaExceededError` (terminal), `UpstreamProviderError`, `NetworkError`, and `TimeoutError`.

```csharp
try
{
    var row = await client.GetByIdAsync("acme", "users", "1");
}
catch (GridApiError ex)
{
    Console.WriteLine($"{ex.GetType().Name}: status={ex.Status} code={ex.Code} retryable={ex.Retryable}");
}
```

## Development

```sh
dotnet build packages/dotnet/GridAPI/GridAPI.csproj
bash tests/contract/dotnet/run.sh
```
