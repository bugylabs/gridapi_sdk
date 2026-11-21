# gridapi-sdk (Rust)

Official Rust SDK for the [GridAPI](https://gridapi.in) public REST API.

> **Server-side only.** The API key is an account-level secret. Never ship it in
> a browser bundle, mobile app, or any other client that reaches end users.

## Install

```sh
cargo add gridapi-sdk
```

Or add it to `Cargo.toml`:

```toml
[dependencies]
gridapi-sdk = "0.1"
```

## Quick start

```rust
use gridapi_sdk::{Client, GridRecord, QueryOptions};

fn main() -> Result<(), gridapi_sdk::GridError> {
    // Reads GRIDAPI_API_KEY from the environment (required).
    let client = Client::from_env()?;

    let page = client.get(
        "acme",
        "users",
        &QueryOptions {
            filter: Some("status:active".into()),
            page: Some(1),
            limit: Some(10),
            ..Default::default()
        },
    )?;
    println!("{} of {} rows", page.data.len(), page.meta.total);

    let mut row = GridRecord::new();
    row.insert("name".into(), Some("Ann".into()));
    let created = client.post("acme", "users", &[row])?;
    println!("inserted {}", created.inserted);
    Ok(())
}
```

Use `ClientBuilder` for explicit configuration instead of environment variables:

```rust
use gridapi_sdk::ClientBuilder;

let client = ClientBuilder::new()
    .api_key("account-key-here")
    .base_url("http://127.0.0.1:4000")
    .timeout_ms(5_000)
    .max_retries(3)
    .retry_writes(false)
    .build()?;
```

A runnable example lives under [`examples/rust`](../../examples/rust) in this
repository.

## Environment variables

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `GRIDAPI_API_KEY` | yes | — | Account-level key, sent as `Authorization: Bearer <key>` |
| `GRIDAPI_BASE_URL` | no | `https://api.gridapi.in` | API root; trailing slashes are trimmed |
| `GRIDAPI_TIMEOUT_MS` | no | `30000` | Per-request timeout in milliseconds |

`ClientBuilder` overrides take precedence over the environment. The project
slug passed to data methods is a routing target, not a credential.

## API

| Method | Description |
| --- | --- |
| `get` / `query` | List records with `filter`, `page`, `limit` |
| `count` | Count via `limit=1` and `meta.total` |
| `get_by_id` | Read one record by primary key |
| `post` | Insert records (never retried) |
| `patch` / `put` | Update a record |
| `delete` | Delete a record |
| `list_projects` / `list_tables` / `get_schema` | Key-authenticated discovery |
| `get_me` / `get_usage` | Account and quota reads |

Sorting and text search are intentionally not exposed: the live Google Sheets
data source ignores them.

## Errors and retries

All fallible calls return `Result<T, GridError>`. `GridError` mirrors the
TypeScript SDK taxonomy (`Authentication`, `Authorization`, `ProjectNotFound`,
`TableNotFound`, `RecordNotFound`, `Validation`, `RateLimit`, `QuotaExceeded`,
`Upstream`, `Network`, `Timeout`) and exposes `status()`, `code()`,
`message()` and `retryable()`. Messages are redacted, so keys never appear in
logs or error output.

Transient failures (429 without a quota code, 5xx, network/timeout) are retried
on `GET`, `DELETE` and `PUT`. `POST` is never retried; `PATCH` is retried only
with `retry_writes(true)`. `Retry-After` is honoured as seconds or an HTTP date,
otherwise exponential backoff with jitter is used. The default is 2 retries
with a 500 ms base delay.

```rust
use gridapi_sdk::{GridError, QueryOptions};

match client.get("missing", "users", &QueryOptions::default()) {
    Ok(page) => println!("{}", page.data.len()),
    Err(GridError::ProjectNotFound(e)) => eprintln!("bad project: {e}"),
    Err(GridError::RateLimit(e)) => eprintln!("retryable: {}", e.retryable()),
    Err(e) => eprintln!("{e}"),
}
```

## License

MIT
