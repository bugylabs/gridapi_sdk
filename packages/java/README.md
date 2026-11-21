# GridAPI Java SDK

Official Java client for the GridAPI public REST API. Requires **Java 21+** and
has **no runtime dependencies**.

> **Server-side only.** The key is an account-level credential: keep it on a
> trusted server and never bundle it into a browser, mobile app, or other
> client-facing artifact.

## Install (Maven)

```xml
<dependency>
  <groupId>io.gridapi</groupId>
  <artifactId>gridapi-sdk</artifactId>
  <version>0.1.0</version>
</dependency>
```

The artifact targets Java 21 (`maven.compiler.release=21`).

## Configuration

The client reads these environment variables at construction time:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level API key, sent as `Authorization: Bearer <key>` |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API base URL |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Connect and request timeout in milliseconds |

Every value can be overridden with the builder:

```java
GridApiClient client = GridApiClient.builder()
    .apiKey(System.getenv("GRIDAPI_API_KEY"))
    .baseUrl("http://localhost:4000")
    .timeoutMs(10_000)
    .maxRetries(2)
    .retryDelayMs(500)
    .retryWrites(false)   // PATCH retries are opt-in; POST is never retried
    .build();
```

## Quickstart

```java
import io.gridapi.*;
import java.util.Map;

GridApiClient client = new GridApiClient(); // reads env vars

ListResponse<GridRecord> page = client.get("acme", "users",
    new QueryOptions("status:active", 1, 10));
for (GridRecord row : page.data()) {
    System.out.println(row.get("name"));
}

CreateResponse<GridRecord> created = client.post("acme", "users",
    Map.of("name", "Dee", "status", "active"));

UpdateResponse<GridRecord> updated = client.patch("acme", "users",
    created.records().get(0).get("id"), Map.of("status", "active"));

int total = client.count("acme", "users", "status:active");
client.delete("acme", "users", updated.record().get("id"));
```

Discovery helpers: `listProjects()`, `listTables(project)`, `getSchema(project,
table)`, `getMe()`, and `getUsage()`.

## Errors and retries

All failures throw `GridApiError` (or a subclass such as
`AuthenticationError`, `QuotaExceededError`, or `RateLimitError`) carrying
`getStatus()`, `getCode()`, `getBody()`, and `isRetryable()`. API keys and
`Bearer` tokens are redacted from every error message.

Transient failures (429 quota-excluded, 5xx, timeouts, network errors) are
retried on GET, DELETE, and PUT; PATCH is retried only with `retryWrites(true)`.
POST is never retried. Retries honor `Retry-After` (seconds or an HTTP date)
and otherwise use exponential backoff with jitter.

## Build

No build tool is required: `javac -d out $(find src/main/java -name '*.java')`.
The included `pom.xml` is for publishing to Maven Central only.
