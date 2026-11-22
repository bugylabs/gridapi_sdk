# Security Policy

## Supported versions

Security fixes are applied to the latest published version of each SDK. See
`docs/COMPATIBILITY.md` for the SDK-to-API compatibility matrix.

## Reporting a vulnerability

Do **not** open a public issue for security vulnerabilities. Email
`security@gridapi.in` with:

- the affected SDK and version,
- a minimal reproduction,
- the impact and, if known, a suggested fix.

You will receive an acknowledgement within 72 hours. Do not include live API
keys, tokens, or customer data in a report.

## Credentials

- All SDKs require `GRIDAPI_API_KEY` (account key, `gp_live_...`) and
  `GRIDAPI_PROJECT_KEY` (project key, `gapi_live_...`) from the environment by
  default. Never commit a key, paste one into an example, or embed one in a
  browser/mobile bundle.
- Both keys are secrets. Only the account key is sent as the Bearer token; the
  project key is a local scope guard and is never transmitted or logged.
  A project slug (for example `sjnsks`) is a routing identifier, **not** a
  credential, and must never be treated as one.
- Release workflows use registry trusted publishing (OIDC) where supported;
  no registry credentials are stored in this repository.
