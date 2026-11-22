# Contributing

Thanks for helping improve the GridAPI SDKs.

## Layout

- `openapi/openapi.yaml` — the canonical API contract, generated/validated from
  the public OpenAPI contract in `openapi/openapi.yaml`. SDKs never
  invent endpoints.
- `packages/<language>/` — one independently versioned package per language.
- `tests/contract/` — language-agnostic scenarios every SDK must pass.
- `tests/integration/` — optional tests against a live GridAPI account.
- `docs/` — concept guides and the compatibility matrix. The public docs site
  (`gridapi_client`) consumes structured metadata from `docs/sdk-manifest.json`.

## Rules

1. **Contract first.** An endpoint, parameter, or response shape must exist in
   `openapi/openapi.yaml` (and in the backend) before an SDK exposes it.
2. **One auth model, two required keys.** Every SDK requires
   `GRIDAPI_API_KEY` (account key, sent as `Authorization: Bearer <key>`) and
   `GRIDAPI_PROJECT_KEY` (project key, used only as a local scope guard). Data
   operations verify the account via `/me` and check project membership via
   `/me/projects`. The project key is never transmitted; the project slug is a
   routing identifier, not a credential.
3. **No secrets in examples or CI.** Keys never appear in source, fixtures, or
   logs. Publishing uses registry trusted publishing (OIDC).
4. **Test both sides.** A change to shared behavior adds a contract-test case;
   a change touching real HTTP behavior adds an integration case.
5. **Independent releases.** Bump only the packages you changed, and update
   that package's CHANGELOG and the compatibility matrix.

## Running checks

Each package owns its own toolchain. From the repository root:

```sh
python3 scripts/validate_contract.py   # OpenAPI contract lint + drift checks
python3 scripts/check_examples.py      # examples compile / match the contract
```

See each package README for its build and test commands.

## Adding a language

Mirror an existing package: client core, typed models, structured errors,
environment configuration, retries, and the shared contract-test suite. Keep
the public method names idiomatic for the language while preserving the same
underlying HTTP calls.
