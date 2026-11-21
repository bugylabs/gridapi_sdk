# GridAPI Java SDK

Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

Official Java client for the [GridAPI public REST API](../../openapi/openapi.yaml). One account-level API key authenticates every project you own or have membership in; the project slug is a routing/authorization target, never a credential. The SDK is server-side only and has no runtime dependencies beyond the JDK.

> **Server-side only.** The key is an account-level secret. Keep it on a trusted server and never bundle it into a browser, mobile app, or any other client-facing artifact. The server resolves the caller's role per project; this SDK never makes authorization decisions.

## Requirements

- **Java 21+** (`java.net.http` client; `maven.compiler.release=21`).
- No third-party runtime dependencies — only the JDK.

## Install

Maven:

```xml
<dependency>
  <groupId>io.gridapi</groupId>
  <artifactId>gridapi-sdk</artifactId>
  <version>1.0.0</version>
</dependency>
```

Gradle:

```groovy
implementation 'io.gridapi:gridapi-sdk:1.0.0'
```

## Features / supported operations

All operations share one account-level credential and the same request path contract:

| Operation | Method(s) | Result |
| --- | --- | --- |
| List records | `get(project, table)`, `get(project, table, QueryOptions)` | `ListResponse<GridRecord>` |
| Query (alias of `get`) | `query(project, table)`, `query(project, table, QueryOptions)` | `ListResponse<GridRecord>` |
| Get by ID | `getById(project, table, id)` | `GridRecord` |
| Create | `post(project, table, ...)` | `CreateResponse<GridRecord>` |
| Partial update | `patch(project, table, id, ...)` | `UpdateResponse<GridRecord>` |
| Replace | `put(project, table, id, ...)` | `UpdateResponse<GridRecord>` |
| Delete | `delete(project, table, id)` | `DeleteResponse` |
| Count | `count(project, table)`, `count(project, table, filter)`, `count(project, table, QueryOptions)` | `int` |
| Discovery | `listProjects()`, `listTables(project)`, `getSchema(project, table)`, `getMe()`, `getUsage()` | see below |

Filtering is exact equality; pagination is `page` + `limit`. Sorting and search are deliberately **not** exposed — the backing Google Sheets provider ignores them.

## Authentication

Set `GRIDAPI_API_KEY` to your account-level key. Every request sends it as `Authorization: Bearer <key>`. The key authenticates the account; the project slug passed to each call only selects which authorized project to route to. A valid key does not grant access to a project you are not a member of — the server returns `AuthorizationError` (403) or `ProjectNotFoundError` (404) in that case.

Never commit the key, log it, or place it in a client-side artifact. SDK errors redact `Bearer` values and `gp_live_`/`gapi_live_` key prefixes before they can reach a log.

## Environment variables

The no-argument constructor reads these once, at construction time:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Connect and request timeout in milliseconds |

Every value can be overridden with the builder; the builder also owns retry settings that have no env var:

```java
GridApiClient client = GridApiClient.builder()
    .apiKey(System.getenv("GRIDAPI_API_KEY"))
    .baseUrl("https://api.gridapi.in")
    .timeoutMs(10_000)
    .maxRetries(2)        // default 2
    .retryDelayMs(500)    // default 500 ms base for exponential backoff
    .retryWrites(false)   // default false; opt in to PATCH retries only
    .build();
```

Blank/negative timeout values fall back to `30000`; negative retry counts and delays are clamped to `0`.

## Initialization

Three constructors are available:

```java
GridApiClient fromEnv = new GridApiClient();                        // GRIDAPI_* env vars
GridApiClient withKey = new GridApiClient("gp_live_...");           // env for the rest
GridApiClient explicit = new GridApiClient("gp_live_...", "https://api.gridapi.in", 10_000);
GridApiClient built = GridApiClient.builder().apiKey("gp_live_...").build();
```

A missing or blank key throws `GridApiError.AuthenticationError` **at construction**, before any network call.

## Quick start

```java
import io.gridapi.CreateResponse;
import io.gridapi.GridApiClient;
import io.gridapi.GridRecord;
import io.gridapi.ListResponse;
import io.gridapi.QueryOptions;

import java.util.Map;

public final class Quickstart {
    public static void main(String[] args) {
        GridApiClient client = new GridApiClient(); // reads GRIDAPI_* env vars

        ListResponse<GridRecord> users = client.get("acme", "users",
            new QueryOptions("status:active", 1, 10));
        System.out.println("total active users: " + users.meta().total());
        for (GridRecord row : users.data()) {
            System.out.println(row.get("id") + " -> " + row.get("name"));
        }

        int allUsers = client.count("acme", "users");
        CreateResponse<GridRecord> created = client.post("acme", "users",
            Map.of("name", "New User", "status", "active"));
        System.out.println("inserted: " + created.inserted());

        client.delete("acme", "users", created.records().get(0).get("id"));
    }
}
```

## GET records

`get` and `query` are aliases with identical signatures and behavior. Without options the server's default page is returned.

```java
ListResponse<GridRecord> all = client.get("acme", "users");
ListResponse<GridRecord> filtered = client.query("acme", "users",
    new QueryOptions("status:active", 1, 10));

all.meta().total();      // int — total matching rows, across all pages
all.meta().limit();      // int — page size actually applied
all.meta().offset();     // int — row offset of this page
all.meta().cacheHit();   // Boolean — may be null

for (GridRecord row : filtered.data()) {
    String name = row.get("name"); // column -> String value (null for empty cell)
}
```

## GET by ID

```java
GridRecord user = client.getById("acme", "users", "1");
System.out.println(user.get("email"));
```

`id` is typed `Object` and is sent as a URL-encoded path segment (`String.valueOf(id)`). An unknown id throws `GridApiError.RecordNotFoundError`.

## POST

Create one row from a `Map<String, String>` or a `GridRecord`, or several rows from a `List<Map<String, String>>`:

```java
CreateResponse<GridRecord> one = client.post("acme", "users",
    Map.of("name", "Dee", "status", "active"));

CreateResponse<GridRecord> many = client.post("acme", "users",
    java.util.List.of(
        Map.of("name", "Dee", "status", "active"),
        Map.of("name", "Ivo", "status", "active")));

CreateResponse<GridRecord> fromRecord = client.post("acme", "users",
    new GridRecord(Map.of("name", "Dee")));

one.inserted();                       // int
one.records().get(0).get("id");       // generated id returned by the server
```

`post` is **never retried**, because a replay could insert duplicate rows.

## PATCH

Partial update of the row identified by `id`:

```java
UpdateResponse<GridRecord> updated = client.patch("acme", "users", "2",
    Map.of("name", "Ben2"));

updated.updated();           // int — number of rows changed
updated.record().get("id");  // GridRecord with the resulting row
```

Retries are opt-in: set `.retryWrites(true)` on the builder to retry PATCH on transient failures. Off by default.

## PUT

Replace the row identified by `id`:

```java
UpdateResponse<GridRecord> replaced = client.put("acme", "users", "3",
    Map.of("name", "Cat2", "status", "active", "team", "growth"));

replaced.updated();
replaced.record().get("name");
```

`GridRecord` overloads exist for both `patch` and `put`: `patch(project, table, id, GridRecord)` and `put(project, table, id, GridRecord)`.

## DELETE

```java
DeleteResponse deleted = client.delete("acme", "users", "1");

deleted.deleted();  // int — number of rows removed
deleted.id();       // String — id the server matched
```

## Filtering and pagination

`QueryOptions` is a record `QueryOptions(String filter, Integer page, Integer limit)`. Use the constructor, `QueryOptions.of(filter, page, limit)`, or `QueryOptions.by(map, page, limit)` to build an exact-match filter:

```java
new QueryOptions("status:active,team:core", 1, 10);                     // filter string
QueryOptions.of("status:active", 2, 25);                               // same, static factory
QueryOptions.by(Map.of("status", "active", "team", "core"), 1, 10);    // -> "status:active,team:core"
```

- `filter` is comma-separated `key:value` pairs; matching is exact equality. A blank filter is ignored.
- `page` is 1-based; `limit` is the page size. Either may be `null` to leave the server default in place.
- `data()` holds the current page; `meta().total()` is the total number of matching rows.
- **Sorting and search are not supported** by the Sheets-backed API and are not exposed by this SDK. Do not rely on an implicit order.

## Count

`count` performs a one-row list read and returns `meta().total()`:

```java
int all = client.count("acme", "users");
int active = client.count("acme", "users", "status:active");
int paged = client.count("acme", "users", new QueryOptions("status:active", 1, 10));
```

Only the filter is used; `page` and `limit` values in `QueryOptions` do not change the count.

## Discovery

```java
for (ProjectInfo project : client.listProjects()) {
    System.out.println(project.slug() + " " + project.role());
}
// listTables accepts a project slug, id, or name
for (SheetInfo sheet : client.listTables("acme")) {
    System.out.println(sheet.endpoint() + " " + sheet.primaryKey()
        + " " + sheet.allowedMethods());
}
TableSchema schema = client.getSchema("acme", "users");
System.out.println(schema.primaryKey());
for (TableSchema.Field field : schema.fields()) {
    System.out.println(field.name() + ": " + field.type());
}

AccountInfo me = client.getMe();
System.out.println(me.account().email());
UsageInfo usage = client.getUsage();
System.out.println(usage.totalApiCallsUsed() + " / " + usage.totalApiCallsLeft());
```

`listTables` throws `ProjectNotFoundError` for an unknown project; `getSchema` throws `TableNotFoundError` for an unknown endpoint.

## Error handling

Every failure throws `GridApiError` (an unchecked `RuntimeException`) or one of its nested subclasses:

| Class | Typical cause |
| --- | --- |
| `GridApiError.AuthenticationError` | Missing key at construction, or HTTP 401 |
| `GridApiError.AuthorizationError` | HTTP 403 — key is valid but lacks access to the project |
| `GridApiError.ProjectNotFoundError` | Unknown/unauthorized project (HTTP 404) |
| `GridApiError.TableNotFoundError` | Unknown sheet/endpoint (HTTP 404) |
| `GridApiError.RecordNotFoundError` | Unknown row id (HTTP 404) |
| `GridApiError.ValidationError` | HTTP 400/422, e.g. an unknown column |
| `GridApiError.RateLimitError` | HTTP 429 without the quota code |
| `GridApiError.QuotaExceededError` | HTTP 429 with code `SUBSCRIPTION_LIMIT_REACHED` — terminal |
| `GridApiError.UpstreamProviderError` | HTTP 5xx from the backing provider |
| `GridApiError.NetworkError` | Connection failure or interruption |
| `GridApiError.TimeoutError` | Request timed out |

```java
try {
    client.getById("acme", "users", "999");
} catch (GridApiError.RecordNotFoundError e) {
    // specific subtype
} catch (GridApiError e) {
    Integer status = e.getStatus();   // HTTP status, may be null
    String code = e.getCode();        // server code, may be null
    String body = e.getBody();        // response body, may be null
    boolean retryable = e.isRetryable();
    throw e;
}
```

Credentials are redacted from `getMessage()` and `getBody()`.

## Timeouts and retries

- `timeoutMs` (default `30000`) is applied to both the TCP connect and each HTTP request; a timeout surfaces as `TimeoutError`.
- `maxRetries` (default `2`) and `retryDelayMs` (default `500`) control retry count and base backoff (exponential with jitter, capped at 30 s).
- **Safe methods are retried by default:** GET, DELETE, and PUT. PATCH is retried only when the builder sets `.retryWrites(true)`. POST is **never** retried, even with `retryWrites(true)`.
- Transient failures retried: HTTP 429 (`RateLimitError`), 5xx (`UpstreamProviderError` / other 5xx), network errors, and timeouts.
- `Retry-After` is honored when present, as whole seconds or an RFC-1123 HTTP date; otherwise exponential backoff with jitter is used.
- `QuotaExceededError` (`SUBSCRIPTION_LIMIT_REACHED`) is **terminal**: it is never retried, regardless of method.

```java
GridApiClient noRetries = GridApiClient.builder()
    .apiKey(System.getenv("GRIDAPI_API_KEY"))
    .maxRetries(0)
    .timeoutMs(5_000)
    .build();
```

## API reference

The canonical contract is [`openapi/openapi.yaml`](../../openapi/openapi.yaml) (generated JSON alongside it in `openapi/openapi.json`). Endpoints, parameters, and response shapes are defined there; the SDK never invents them.

## Contributing

Contributions follow the repository [CONTRIBUTING.md](../../CONTRIBUTING.md). Contract changes belong in `openapi/openapi.yaml` first, and behavior changes must pass the shared Java contract suite:

```sh
bash tests/contract/java/run.sh
```

Run all SDK checks from the repository root with `python3 scripts/run-all-tests.py`.

## License

Proprietary — Source Available, Non-Commercial Use Only. Copyright (c) 2026 BugyLabs. See [LICENSE](../../LICENSE). Personal, educational, research, and non-commercial evaluation use only; commercial use requires a separate written agreement with BugyLabs.

## Support and issues

Report bugs, ask questions, and request features in the [issue tracker](https://github.com/bugylabs/gridapi-sdk/issues) at https://github.com/bugylabs/gridapi-sdk. For security reports, follow [SECURITY.md](../../SECURITY.md) rather than opening a public issue.
