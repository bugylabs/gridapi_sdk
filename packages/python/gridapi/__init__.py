"""Official Python SDK for the GridAPI public REST API.

Two keys are required at construction: an account key (``gp_live_...``) from
``GRIDAPI_API_KEY`` and a project key (``gapi_live_...``) from
``GRIDAPI_PROJECT_KEY``. Only the account key is sent as the Bearer token. The
project key never leaves this process: the public REST API has no project-key
header, so the SDK uses it only as a local scope guard alongside ``verify()``.

A project slug is a routing/authorization target, never a credential.

Server-side only: this SDK sends the account key in an Authorization header. Do
not ship it in a browser bundle, mobile app, or any other client-side code.

Contract: openapi/openapi.yaml in the gridapi_sdk monorepo. Standard library
only; no third-party dependencies.
"""

from __future__ import annotations

import json
import os
import random
import re
import socket
import time
import urllib.error
import urllib.parse
import urllib.request
from email.utils import parsedate_to_datetime

__all__ = [
    "GridApiClient",
    "GridAPI",
    "GridApiError",
    "ConfigurationError",
    "AuthenticationError",
    "AuthorizationError",
    "ProjectNotFoundError",
    "TableNotFoundError",
    "RecordNotFoundError",
    "ValidationError",
    "RateLimitError",
    "QuotaExceededError",
    "UpstreamProviderError",
    "NetworkError",
    "TimeoutError",
    "redact",
]

_BEARER_RE = re.compile(r"Bearer\s+\S+", re.IGNORECASE)
_KEY_RE = re.compile(r"(gp_live_|gapi_live_)[A-Za-z0-9_-]+")
_MAX_BACKOFF_SECONDS = 30.0


def redact(text, secret=None):
    """Strip credential-looking text before it reaches a log or a user.

    Redacts Bearer tokens, GridAPI key prefixes, and the exact configured key
    when one is supplied. Keeps exception messages and reprs safe to print.
    """
    if not isinstance(text, str) or not text:
        return text
    out = _BEARER_RE.sub("Bearer [REDACTED]", text)
    out = _KEY_RE.sub(r"\1[REDACTED]", out)
    if secret and isinstance(secret, str):
        out = out.replace(secret, "[REDACTED]")
    return out


class GridApiError(Exception):
    """Base class for every error raised by the SDK.

    Carries ``status``, ``code``, ``body`` and ``retryable``. Messages, body and
    code are redacted so neither ``str(error)`` nor ``repr(error)`` leaks a key.
    """

    default_retryable = False

    def __init__(self, message, *, status=None, code=None, body=None,
                 retryable=None, secret=None):
        safe = redact(message, secret)
        super().__init__(safe)
        self.status = status
        self.code = redact(code, secret) if isinstance(code, str) else code
        self.body = redact(body, secret) if isinstance(body, str) else body
        self.retryable = self.default_retryable if retryable is None else bool(retryable)

    @property
    def name(self):
        """Error class name, mirroring the JavaScript SDK's ``error.name``."""
        return type(self).__name__

    def __repr__(self):
        return (
            f"{self.name}(message={str(self)!r}, status={self.status!r}, "
            f"code={self.code!r}, retryable={self.retryable!r})"
        )


class ConfigurationError(GridApiError):
    """A required key is missing or malformed; raised before any request."""


class AuthenticationError(GridApiError):
    """401: missing, invalid, or expired API key."""


class AuthorizationError(GridApiError):
    """403: the key is valid but may not access this project."""


class ProjectNotFoundError(GridApiError):
    """404: the project is not authorized or does not exist."""


class TableNotFoundError(GridApiError):
    """404: the sheet/endpoint does not exist."""


class RecordNotFoundError(GridApiError):
    """404: the requested record does not exist."""


class ValidationError(GridApiError):
    """400-422: invalid payload, unknown column, or bad request."""


class RateLimitError(GridApiError):
    """429: transient rate limit. Retryable."""

    default_retryable = True


class QuotaExceededError(GridApiError):
    """429 SUBSCRIPTION_LIMIT_REACHED: quota exhausted. Terminal, not retried."""


class UpstreamProviderError(GridApiError):
    """502+: an upstream provider (e.g. Google Sheets) failed. Retryable."""

    default_retryable = True


class NetworkError(GridApiError):
    """Transport failure with no HTTP response. Retryable."""

    default_retryable = True


class TimeoutError(GridApiError):
    """Request exceeded the configured timeout. Retryable."""

    default_retryable = True


def _parse_body(text):
    if not text:
        return None
    try:
        return json.loads(text)
    except ValueError:
        return text


def _as_message(parsed, fallback):
    if isinstance(parsed, dict):
        code = parsed.get("code") if isinstance(parsed.get("code"), str) else None
        raw_message = parsed.get("error")
        if not isinstance(raw_message, str):
            raw_message = parsed.get("message")
        message = raw_message if isinstance(raw_message, str) else fallback
        return message, code
    if isinstance(parsed, str) and parsed:
        return parsed, None
    return fallback, None


def _map_http_error(status, code, message, body, secret=None):
    opts = dict(status=status, code=code, body=body, secret=secret)
    if status == 401:
        return AuthenticationError(message, **opts)
    if status == 403:
        return AuthorizationError(message, **opts)
    if status == 404:
        if code == "SHEET_NOT_FOUND" or re.search(r"sheet|table", message, re.I):
            return TableNotFoundError(message, **opts)
        if code == "NOT_FOUND" or re.search(r"record", message, re.I):
            return RecordNotFoundError(message, **opts)
        return ProjectNotFoundError(message, **opts)
    if 400 <= status <= 422:
        return ValidationError(message, **opts)
    if status == 429:
        if code == "SUBSCRIPTION_LIMIT_REACHED":
            return QuotaExceededError(message, **opts)
        return RateLimitError(message, **opts)
    if status >= 502:
        return UpstreamProviderError(message, **opts)
    return GridApiError(message, retryable=status >= 500, **opts)


def _retry_delay_seconds(retry_after, base_ms, attempt):
    """Retry-After (seconds or HTTP date) wins; else capped exponential + jitter."""
    if retry_after:
        try:
            return max(0.0, float(retry_after))
        except (TypeError, ValueError):
            pass
        try:
            when = parsedate_to_datetime(retry_after)
            if when is not None:
                return max(0.0, when.timestamp() - time.time())
        except (TypeError, ValueError, OverflowError):
            pass
    base_seconds = base_ms / 1000.0
    backoff = min(_MAX_BACKOFF_SECONDS, base_seconds * (2 ** attempt))
    return backoff + random.random() * (base_seconds / 2)


def _segment(value):
    """URL-encode one path segment, including any embedded slashes."""
    return urllib.parse.quote(str(value), safe="")


def _required_key(value, prefix, env_name, option_name):
    """Return a valid key or raise ConfigurationError without echoing its value."""
    text = "" if value is None else str(value).strip()
    if not text:
        raise ConfigurationError(
            f"GridAPI: missing {env_name}. Set it or pass {option_name}."
        )
    if not text.startswith(prefix) or len(text) == len(prefix):
        raise ConfigurationError(
            f"GridAPI: {env_name} must be a {prefix}... key."
        )
    return text


class GridApiClient:
    """Synchronous GridAPI client built on ``urllib`` (standard library only).

    Both keys default to the environment and can be overridden per instance::

        client = GridApiClient()  # reads GRIDAPI_API_KEY / GRIDAPI_PROJECT_KEY
        client = GridApiClient(api_key="...", project_key="...")

    Only ``api_key`` is sent as the Bearer token. ``project_key`` is a local
    scope guard: the public REST API has no project-key header, so it is never
    transmitted, logged, or echoed.

    :param api_key: Account key (``gp_live_...``). Defaults to ``GRIDAPI_API_KEY`` (required).
    :param project_key: Project key (``gapi_live_...``). Defaults to ``GRIDAPI_PROJECT_KEY`` (required).
    :param base_url: Defaults to ``GRIDAPI_BASE_URL`` or https://api.gridapi.in.
    :param timeout_ms: Defaults to ``GRIDAPI_TIMEOUT_MS`` or 30000.
    :param max_retries: Retries for transient failures. Default 2.
    :param retry_delay_ms: Base backoff in milliseconds. Default 500.
    :param retry_writes: Retry non-idempotent PATCH. Default False. POST is never retried.
    """

    def __init__(self, api_key=None, base_url=None, timeout_ms=None,
                 max_retries=2, retry_delay_ms=500, retry_writes=False,
                 project_key=None):
        self.api_key = _required_key(
            api_key if api_key is not None else os.environ.get("GRIDAPI_API_KEY"),
            "gp_live_", "GRIDAPI_API_KEY", "api_key",
        )
        self.project_key = _required_key(
            project_key if project_key is not None
            else os.environ.get("GRIDAPI_PROJECT_KEY"),
            "gapi_live_", "GRIDAPI_PROJECT_KEY", "project_key",
        )
        self._account_verified = False
        self._verified_projects = set()
        self.base_url = (
            base_url if base_url is not None
            else os.environ.get("GRIDAPI_BASE_URL") or "https://api.gridapi.in"
        ).rstrip("/")

        raw_timeout = (
            timeout_ms if timeout_ms is not None
            else os.environ.get("GRIDAPI_TIMEOUT_MS")
        )
        try:
            parsed_timeout = int(raw_timeout) if raw_timeout else 0
        except (TypeError, ValueError):
            parsed_timeout = 0
        self.timeout_ms = parsed_timeout or 30000
        self.max_retries = max_retries
        self.retry_delay_ms = retry_delay_ms
        self.retry_writes = retry_writes

    def verify(self, project=None):
        """Validate the account, then confirm ``project`` is authorized.

        The account check and each verified project identifier are cached.
        Discovery calls (``get_me``, ``list_projects``) do not call this method,
        so verification never recurses into itself.

        :param project: Optional slug, id, or name to authorize.
        :raises AuthenticationError: the account key is not authenticated.
        :raises AuthorizationError: no projects are authorized for the account.
        :raises ProjectNotFoundError: the account has projects, but not this one.
        """
        if not self._account_verified:
            me = self.get_me() or {}
            if me.get("authenticated") is not True:
                raise AuthenticationError(
                    "GridAPI: account is not authenticated", secret=self.api_key
                )
            self._account_verified = True

        if project is None or project in self._verified_projects:
            return

        projects = self.list_projects()
        authorized = {
            value
            for item in projects
            for value in (item.get("slug"), item.get("id"), item.get("name"))
        }
        if project not in authorized:
            if not projects:
                raise AuthorizationError(
                    "GridAPI: no projects are authorized for this account and project key.",
                    secret=self.api_key,
                )
            raise ProjectNotFoundError(
                f"Project not found: {project}", secret=self.api_key
            )
        self._verified_projects.add(project)

    def _data_path(self, project, table, id=None):
        path = f"/api/v1/{_segment(project)}/{_segment(table)}"
        if id is not None:
            path += f"/{_segment(id)}"
        return path

    @staticmethod
    def _query_string(params):
        if not params:
            return ""
        pairs = {}
        if params.get("filter") is not None:
            filter_value = params["filter"]
            if isinstance(filter_value, dict):
                filter_value = ",".join(f"{k}:{v}" for k, v in filter_value.items())
            pairs["filter"] = filter_value
        if params.get("page") is not None:
            pairs["page"] = params["page"]
        if params.get("limit") is not None:
            pairs["limit"] = params["limit"]
        if not pairs:
            return ""
        return "?" + urllib.parse.urlencode(pairs)

    def _request(self, method, path, body=None):
        url = self.base_url + path
        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        }
        safe = (
            method in ("GET", "DELETE", "PUT")
            or (method == "PATCH" and self.retry_writes)
        )

        for attempt in range(self.max_retries + 1):
            request = urllib.request.Request(url, data=data, headers=headers, method=method)
            try:
                with urllib.request.urlopen(request, timeout=self.timeout_ms / 1000.0) as response:
                    return _parse_body(response.read().decode("utf-8", "replace"))
            except urllib.error.HTTPError as exc:
                try:
                    raw = exc.read().decode("utf-8", "replace") if exc.fp else ""
                finally:
                    exc.close()
                parsed = _parse_body(raw)
                message, code = _as_message(
                    parsed, f"GridAPI request failed with status {exc.code}"
                )
                error = _map_http_error(exc.code, code, message, raw, self.api_key)
                if safe and attempt < self.max_retries and error.retryable:
                    delay = _retry_delay_seconds(
                        exc.headers.get("Retry-After"), self.retry_delay_ms, attempt
                    )
                    time.sleep(delay)
                    continue
                raise error from None
            except (socket.timeout, TimeoutError) as exc:
                error = TimeoutError("GridAPI request timed out", secret=self.api_key)
                if safe and attempt < self.max_retries:
                    time.sleep(_retry_delay_seconds(None, self.retry_delay_ms, attempt))
                    continue
                raise error from None
            except urllib.error.URLError as exc:
                reason = exc.reason
                timed_out = isinstance(reason, socket.timeout)
                error = (
                    TimeoutError("GridAPI request timed out", secret=self.api_key)
                    if timed_out
                    else NetworkError(
                        f"GridAPI network error: {reason}", secret=self.api_key
                    )
                )
                if safe and attempt < self.max_retries:
                    time.sleep(_retry_delay_seconds(None, self.retry_delay_ms, attempt))
                    continue
                raise error from None

    def get(self, project, table, params=None):
        """List records: GET /api/v1/{project}/{endpoint}.

        ``params`` accepts ``filter`` (string or dict), ``page`` and ``limit``.
        Returns ``{"data": [...], "meta": {...}}``.
        """
        self.verify(project)
        return self._request(
            "GET", self._data_path(project, table) + self._query_string(params)
        )

    def get_by_id(self, project, table, id):
        """Fetch one record by primary key. Returns a bare record object."""
        self.verify(project)
        return self._request("GET", self._data_path(project, table, id))

    def post(self, project, table, record):
        """Insert one record or a list. Returns ``{"inserted": n, "records": [...]}``."""
        self.verify(project)
        return self._request("POST", self._data_path(project, table), record)

    def patch(self, project, table, id, changes):
        """Partially update a record. Returns ``{"updated": n, "record": {...}}``."""
        self.verify(project)
        return self._request("PATCH", self._data_path(project, table, id), changes)

    def put(self, project, table, id, record):
        """Replace a record. Returns ``{"updated": n, "record": {...}}``."""
        self.verify(project)
        return self._request("PUT", self._data_path(project, table, id), record)

    def delete(self, project, table, id):
        """Delete a record. Returns ``{"deleted": n, "id": "..."}``."""
        self.verify(project)
        return self._request("DELETE", self._data_path(project, table, id))

    def query(self, project, table, params=None):
        """Alias for :meth:`get`: filter and paginate a collection."""
        return self.get(project, table, params or {})

    def count(self, project, table, filter=None):
        """Count matching records via the list response's ``meta.total``.

        Uses ``limit=1`` because there is no dedicated count endpoint.
        """
        result = self.get(project, table, {"filter": filter, "limit": 1})
        return result["meta"]["total"]

    def list_projects(self):
        """List projects authorized by this key. Discovery reads do not consume quota."""
        result = self._request("GET", "/api/v1/me/projects")
        return (result or {}).get("projects", [])

    def list_tables(self, project):
        """Derive a project's tables from its sheets in :meth:`list_projects`."""
        found = next(
            (
                p for p in self.list_projects()
                if project in (p.get("slug"), p.get("id"), p.get("name"))
            ),
            None,
        )
        if found is None:
            raise ProjectNotFoundError(f"Project not found: {project}")
        return found.get("sheets", [])

    def get_schema(self, project, table):
        """Read a table's schema from the project's docs endpoint."""
        docs = self._request("GET", f"/api/v1/me/projects/{_segment(project)}/docs")
        for endpoint in (docs or {}).get("endpoints", []):
            if str(endpoint.get("endpoint", "")).lower() == str(table).lower():
                return endpoint
        raise TableNotFoundError(f"Table not found: {table}")

    def get_me(self):
        """Return account, authentication, and subscription information."""
        return self._request("GET", "/api/v1/me")

    def get_usage(self):
        """Return API call usage counters for the account."""
        return self._request("GET", "/api/v1/me/usage")


# Alias matching the public documentation examples.
GridAPI = GridApiClient
