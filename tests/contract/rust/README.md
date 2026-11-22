# Rust contract tests

The Rust SDK's contract test lives in the Cargo package, next to the source it
exercises:

```
packages/rust/tests/contract.rs
```

It boots the shared mock server (`tests/contract/mock-server.mjs`) on a private
port for each test, waits for readiness, and kills the child process on drop.
It covers CRUD, filtering, pagination, count, error mapping, key redaction,
retries (including `Retry-After`, POST never retried, and 5xx), discovery,
two-key configuration errors, and project verification (account check, unknown
project, forbidden account, caching).

Every client in the tests supplies the mock project key `gapi_live_testproject`
alongside the account key `gp_live_testkey`; the project key is never sent.

Run it from the repository root:

```sh
cd packages/rust
cargo test --test contract
```

`node` must be on `PATH`. For the full crate checks, including doc tests:

```sh
cd packages/rust
cargo test
```
