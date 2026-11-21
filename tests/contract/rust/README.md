# Rust contract tests

The Rust SDK's contract test lives in the Cargo package, next to the source it
exercises:

```
packages/rust/tests/contract.rs
```

It boots the shared mock server (`tests/contract/mock-server.mjs`) on a private
port for each test, waits for readiness, and kills the child process on drop.
It covers CRUD, filtering, pagination, count, error mapping, key redaction,
retries (including `Retry-After`, POST never retried, and 5xx), and discovery.

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
