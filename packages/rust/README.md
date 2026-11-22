# gridapi-sdk (Rust)

Official Rust SDK for the [GridAPI](https://gridapi.in) public REST API. It is a
small synchronous client over the same HTTP contract every GridAPI SDK shares,
with typed request/response models, structured errors, environment-based
configuration, and method-aware retries.

Source Available | Proprietary | Non-Commercial Use Only — Copyright (c) 2026 BugyLabs. All Rights Reserved. Personal, educational, research and non-commercial evaluation use only. Commercial use requires a separate written agreement with BugyLabs. See ../../LICENSE.

> **Server-side only.** Both keys are secrets. Never embed them in a browser
> bundle, mobile app, or any other client that ships to end users. Keep them in
> environment variables on your server.

The crate identifier used in Rust source is `gridapi_sdk` (underscores); the
dependency name in `Cargo.toml` is `gridapi-sdk` (hyphens).

## Features and supported operations

- **Records:** list/query (`get`, `query`), read one (`get_by_id`), insert
  (`post`), partial update (`patch`), replace (`put`), delete (`delete`), and
  count (`count`).
- **Discovery:** `list_projects`, `list_tables`, `get_schema`, `get_me`,
  `get_usage` (discovery reads; no data quota cost).
- **Filtering and pagination** via `QueryOptions`. Sorting and search are
  deliberately not exposed because the backing Sheets data source ignores them.
- **Structured errors** via `GridError`, with status/code/message accessors and
  a `retryable()` hint.
- **Safe retries** with `Retry-After` support and exponential backoff; writes
  are retried only when explicitly allowed.
- **Redaction** of keys and bearer tokens in error messages (`redact`).
- **Two-key model:** an account key authenticates requests; a separate project
  key is a local scope guard. The project slug is a routing/authorization
  target, never a credential.
- **Project verification:** data methods call `verify(project)` first; the
  account check and verified project names are cached per client.

## Install

Add the dependency to your `Cargo.toml` (canonical crate name retained):

```toml
[dependencies]
gridapi-sdk = "1.0"
```

Then use it as `gridapi_sdk`:

```rust
use gridapi_sdk::Client;
```

This crate is distributed as source in this repository. Access to it does not
grant commercial rights; see [License](#license).

## Requirements

- Rust **edition 2021** (the crate's `Cargo.toml` declares `edition = "2021"`).
- A stable Rust toolchain (the crate builds on stable; no nightly features).

## Authentication and project scope

Two keys are required at build time:

- `GRIDAPI_API_KEY` (`gp_live_...`) — the **account key**. It is the only value
  sent to the server, as `Authorization: Bearer <account key>`.
- `GRIDAPI_PROJECT_KEY` (`gapi_live_...`) — the **project key**. It is a local
  scope guard only: the public REST API has no project-key header, so this SDK
  never transmits, logs or echoes it. It is validated and then discarded.

Both must be present, use the correct prefix and have a non-empty suffix,
otherwise `build()` returns `GridError::Configuration`. That error contains no
key values. A **project slug** (for example `acme`) is a routing/authorization
target, not a credential. Keys are **server-side only**: do not ship them to a
browser or mobile client, commit them, or print them in logs.

## Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | Yes | — | Account key (`gp_live_...`), sent as `Bearer`. |
| `GRIDAPI_PROJECT_KEY` | Yes | — | Project key (`gapi_live_...`), local scope guard only; never transmitted. |
| `GRIDAPI_BASE_URL` | No | `https://api.gridapi.in` | API root; trailing slashes are removed. |
| `GRIDAPI_TIMEOUT_MS` | No | `30000` | Per-request timeout in milliseconds. |

Values passed to `ClientBuilder` take precedence over the environment. Missing,
empty or malformed keys are `GridError::Configuration` errors at build time.

## Initialization

`Client::from_env()` builds a client from the four environment variables.
Reuse one client across requests — it owns a pooled HTTP agent and the
verification cache.

```rust
use gridapi_sdk::Client;

fn main() -> Result<(), gridapi_sdk::GridError> {
    let client = Client::from_env()?;
    Ok(())
}
```

For explicit configuration, use `ClientBuilder`:

```rust
use gridapi_sdk::ClientBuilder;

fn main() -> Result<(), gridapi_sdk::GridError> {
    let client = ClientBuilder::new()
        .api_key("gp_live_...")
        .project_key("gapi_live_...")
        .base_url("https://api.gridapi.in")
        .timeout_ms(10_000)
        .max_retries(3)
        .retry_delay_ms(500)
        .retry_writes(false)
        .build()?;
    Ok(())
}
```

Builder defaults: `max_retries = 2`, `retry_delay_ms = 500`,
`retry_writes = false`; unset key/base URL/timeout fall back to the environment.

## Project verification

Every data method (`get`, `query`, `get_by_id`, `post`, `patch`, `put`,
`delete`, `count`) calls `verify(Some(project))` before its request. You can
also call it yourself:

```rust
client.verify(None)?;             // authenticate the account once
client.verify(Some("acme"))?;     // confirm the project is in scope
```

`verify` calls `GET /api/v1/me` (a non-true `authenticated` field yields
`GridError::Authentication`), then, for a named project, `GET
/api/v1/me/projects` and matches the slug, id or name. An empty project list
yields `GridError::Authorization`; a non-empty list without a match yields
`GridError::ProjectNotFound`. The account check and each verified project are
cached per client. Discovery methods (`list_projects`, `list_tables`,
`get_schema`, `get_me`, `get_usage`) deliberately do not verify, so there is no
recursion. Verification is local: it does not use or send the project key.

## Quick start

```rust
use gridapi_sdk::{Client, GridRecord, QueryOptions};

fn main() -> Result<(), gridapi_sdk::GridError> {
    // Reads GRIDAPI_API_KEY, GRIDAPI_PROJECT_KEY, GRIDAPI_BASE_URL and
    // GRIDAPI_TIMEOUT_MS from the env. Data methods verify the project first.
    let client = Client::from_env()?;
    client.verify(Some("acme"))?;

    let options = QueryOptions {
        filter: Some("status:active".into()),
        limit: Some(10),
        ..Default::default()
    };
    let page = client.get("acme", "users", &options)?;
    println!("{} of {} rows", page.data.len(), page.meta.total);

    let mut row = GridRecord::new();
    row.insert("name".into(), Some("Ann".into()));
    row.insert("email".into(), Some("ann@example.com".into()));
    let created = client.post("acme", "users", &[row])?;
    println!("inserted {} record(s)", created.inserted);

    Ok(())
}
```

## GET records (list / query)

`get` lists records; `query` is an alias for the same call. Both return a
`ListResponse` with `data: Vec<GridRecord>` and `meta: ListMeta`
(`total`, `limit`, `offset`, optional `cache_hit`).

```rust
use gridapi_sdk::{Client, QueryOptions};

let client = Client::from_env()?;
let options = QueryOptions {
    filter: Some("status:active".into()),
    page: Some(1),
    limit: Some(25),
    ..Default::default()
};
let page = client.get("acme", "users", &options)?;
for row in &page.data {
    println!("{:?}", row);
}
println!("page {} of {} total", options.page.unwrap_or(1), page.meta.total);
// Equivalent:
let same = client.query("acme", "users", &options)?;
```

## GET by ID

`get_by_id` retrieves a single record by its primary key and returns a
`GridRecord`.

```rust
let row = client.get_by_id("acme", "users", "usr_1001")?;
println!("{:?}", row.get("name"));
```

## POST (insert)

`post` inserts one or more `GridRecord` values and returns a `CreateResponse`
(`inserted: u64`, `records: Vec<GridRecord>`). It is a write and is **never
retried**.

```rust
use gridapi_sdk::GridRecord;

let mut a = GridRecord::new();
a.insert("name".into(), Some("Ann".into()));
let mut b = GridRecord::new();
b.insert("name".into(), Some("Ben".into()));

let created = client.post("acme", "users", &[a, b])?;
println!("inserted {} record(s)", created.inserted);
```

## PATCH (partial update)

`patch` updates selected columns of one record and returns an `UpdateResponse`
(`updated: u64`, `record: GridRecord`). It is retried only when
`retry_writes(true)` is set on the builder.

```rust
use gridapi_sdk::GridRecord;

let mut changes = GridRecord::new();
changes.insert("status".into(), Some("inactive".into()));
let result = client.patch("acme", "users", "usr_1001", &changes)?;
println!("updated {} record(s)", result.updated);
```

## PUT (replace)

`put` replaces a whole record and returns an `UpdateResponse`. Unlike `PATCH`,
`PUT` is treated as idempotent and is eligible for safe retries.

```rust
use gridapi_sdk::GridRecord;

let mut replacement = GridRecord::new();
replacement.insert("name".into(), Some("Ann".into()));
replacement.insert("email".into(), Some("ann@example.com".into()));
let result = client.put("acme", "users", "usr_1001", &replacement)?;
println!("updated {} record(s)", result.updated);
```

## DELETE

`delete` removes a record by primary key and returns a `DeleteResponse`
(`deleted: u64`, `id: String`). It is safe and eligible for retries.

```rust
let result = client.delete("acme", "users", "usr_1001")?;
println!("deleted {} record(s): {}", result.deleted, result.id);
```

## Filtering and pagination

`QueryOptions` has exactly three fields:

- `filter: Option<Filter>` — an exact-match filter string of the form
  `"col:value,col2:value2"` (`Filter` is a type alias for `String`).
- `page: Option<u32>` — 1-based page number; applied by the server only when
  `limit` is set.
- `limit: Option<u32>` — page size, clamped to 1–100 by the server.

**Sorting and search are not supported.** The backing Sheets data source
ignores them, so the SDK does not pretend to expose them. Use `filter`, `page`,
and `limit` only.

```rust
use gridapi_sdk::QueryOptions;

let opts = QueryOptions {
    filter: Some("status:active,team:eng".into()),
    page: Some(2),
    limit: Some(50),
};
let page = client.get("acme", "users", &opts)?;
```

## Count

`count` runs a list request with `limit = 1` and returns `meta.total` as a
`u64`. Pass `None` to count all rows, or a filter string to count matches.

```rust
let all = client.count("acme", "users", None)?;
let active = client.count("acme", "users", Some("status:active"))?;
println!("{active} of {all} rows are active");
```

## Discovery

Discovery reads do not consume data quota:

```rust
for project in client.list_projects()? {
    println!("{} ({})", project.name, project.slug);
}

for sheet in client.list_tables("acme")? {
    println!("{} -> {:?}", sheet.endpoint, sheet.primary_key);
}

let schema = client.get_schema("acme", "users")?;
for field in &schema.fields {
    println!("{}: {:?}", field.name, field.field_type);
}

let me = client.get_me()?;
println!("authenticated: {}", me.authenticated);

let usage = client.get_usage()?;
println!("used: {:?}, left: {:?}", usage.total_api_calls_used, usage.total_api_calls_left);
```

## Error handling

All fallible calls return `Result<T, GridError>`. `GridError` is an enum whose
variants each carry an `ErrorDetail` (`status: Option<u16>`,
`code: Option<String>`, redacted `message: String`):

| Variant | Typical cause |
| --- | --- |
| `Configuration` | missing or malformed account/project key at build time; no key values in the message. |
| `Authentication` | 401 — invalid key, or `/me` reports `authenticated: false`. |
| `Authorization` | 403 — account sees no projects, or key not allowed for the operation. |
| `ProjectNotFound` | 404 — unknown project slug, or local verification found no match. |
| `TableNotFound` | 404 — unknown sheet/table. |
| `RecordNotFound` | 404 — unknown record ID. |
| `Validation` | 400–422 — bad filter, payload, or parameters. |
| `RateLimit` | 429 — transient rate limit (retryable). |
| `QuotaExceeded` | 429 with `SUBSCRIPTION_LIMIT_REACHED` (terminal). |
| `Upstream` | 5xx or a response decode failure (retryable for 5xx). |
| `Network` | transport failure (retryable). |
| `Timeout` | request timeout (retryable). |

`GridError` exposes `status()`, `code()`, `message()`, and `retryable()`.
`retryable()` is `true` for `RateLimit`, `Upstream`, `Network`, and `Timeout`
only; clients still respect HTTP-method idempotency before retrying.

```rust
use gridapi_sdk::{redact, GridError, QueryOptions};

match client.get("missing", "users", &QueryOptions::default()) {
    Ok(page) => println!("{} rows", page.data.len()),
    Err(e @ GridError::ProjectNotFound(_)) => eprintln!("bad project: {}", e.message()),
    Err(e @ GridError::RateLimit(_)) => eprintln!("retryable: {}", e.retryable()),
    Err(e) => eprintln!("status={:?} code={:?}: {}", e.status(), e.code(), e.message()),
}

// Redact a key or bearer token before it reaches a log line.
println!("{}", redact("Authorization: Bearer gp_live_secretvalue"));
```

## Timeouts and retries

- `timeout_ms` (env `GRIDAPI_TIMEOUT_MS`, default `30000`) sets a per-request
  timeout. Timeouts surface as `GridError::Timeout`.
- `max_retries` (default `2`) is the number of retries **after** the first
  attempt; `retry_delay_ms` (default `500`) is the base backoff.
- Retries apply to **safe methods only**: `GET`, `DELETE`, and `PUT`.
- `POST` is **never** retried.
- `PATCH` is retried only when the builder sets `retry_writes(true)`.
- Only errors whose `retryable()` is `true` are retried. `QuotaExceeded`
  (`SUBSCRIPTION_LIMIT_REACHED`) is **terminal** and is never retried, even
  though it arrives as a 429.
- A `Retry-After` response header is honoured as integer seconds or an HTTP
  date; otherwise exponential backoff with small jitter (capped at 30 s) is
  used.

```rust
use gridapi_sdk::ClientBuilder;

let client = ClientBuilder::new()
    .api_key("gp_live_...")
    .project_key("gapi_live_...")
    .max_retries(3)
    .retry_delay_ms(250)
    .retry_writes(true) // opt in to PATCH retries; POST is still never retried
    .build()?;
```

## API reference

The canonical HTTP contract is [`openapi/openapi.yaml`](../../openapi/openapi.yaml)
(a JSON copy lives at `../../openapi/openapi.json`). The SDK never invents
endpoints; every method above maps to a path defined there.

## Examples

A runnable example lives at `../../examples/rust/src/main.rs`. Configure it
with `GRIDAPI_API_KEY` and `GRIDAPI_PROJECT_KEY` (both required), then from that
directory run `cargo run`.

## Contributing

See [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md). The Rust package owns its
own toolchain; from this directory run:

```sh
cargo build
cargo test
```

Shared contract scenarios live in `tests/contract.rs` and exercise a local mock
server; add a case there when changing shared HTTP behavior.

## License

Source Available | Proprietary | Non-Commercial Use Only. Copyright (c) 2026
BugyLabs. All Rights Reserved. Personal, educational, research, and
non-commercial evaluation use only; commercial use requires a separate written
agreement with BugyLabs. This is not open-source software and grants no
open-source rights. See [`../../LICENSE`](../../LICENSE) for the full terms.

## Support and issues

Repository: <https://github.com/bugylabs/gridapi-sdk>

- Bug reports and feature requests: <https://github.com/bugylabs/gridapi-sdk/issues>
- Security vulnerabilities: do **not** open a public issue; email
  `security@gridapi.in` (see [`../../SECURITY.md`](../../SECURITY.md)).
- Commercial licensing: contact BugyLabs (see [`../../LICENSE`](../../LICENSE)).
