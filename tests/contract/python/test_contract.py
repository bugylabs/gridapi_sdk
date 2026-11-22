"""Contract tests for the Python SDK against the shared mock server.

Run from the gridapi_sdk root::

    python3 -m unittest discover -s tests/contract/python -v

The mock server (tests/contract/mock-server.mjs) is started as a subprocess on
a free localhost port and is always terminated in tearDownClass.
"""

import os
import socket
import subprocess
import sys
import time
import unittest
import urllib.request
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "packages" / "python"))

from gridapi import (  # noqa: E402
    AuthenticationError,
    AuthorizationError,
    ConfigurationError,
    GridApiClient,
    ProjectNotFoundError,
    QuotaExceededError,
    RateLimitError,
    RecordNotFoundError,
    TableNotFoundError,
    TimeoutError,
    ValidationError,
    redact,
)

# Public test fixtures defined in the shared mock server, not real credentials.
TEST_KEY = "gp_live_testkey"
OTHER_KEY = "gp_live_otheraccount"
TEST_PROJECT_KEY = "gapi_live_testproject"


def _free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


class GridApiContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        port = _free_port()
        cls.base_url = f"http://127.0.0.1:{port}"
        cls._server = subprocess.Popen(
            ["node", str(ROOT / "tests" / "contract" / "mock-server.mjs"), str(port)],
            cwd=str(ROOT),
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        deadline = time.time() + 10
        ready = False
        while time.time() < deadline and cls._server.poll() is None:
            try:
                with urllib.request.urlopen(
                    cls.base_url + "/api/v1/capabilities", timeout=0.5
                ) as response:
                    ready = response.status == 200
            except OSError:
                ready = False
            if ready:
                break
            time.sleep(0.05)
        if not ready:
            cls._stop_server()
            raise RuntimeError("mock server did not become ready on " + cls.base_url)
        cls.client = GridApiClient(
            api_key=TEST_KEY, project_key=TEST_PROJECT_KEY, base_url=cls.base_url
        )

    @classmethod
    def _stop_server(cls):
        server = getattr(cls, "_server", None)
        if server is not None and server.poll() is None:
            server.terminate()
            try:
                server.wait(timeout=5)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)
        if server is not None and server.stderr is not None:
            server.stderr.close()

    @classmethod
    def tearDownClass(cls):
        cls._stop_server()
        super().tearDownClass()

    def fault_client(self, name):
        return GridApiClient(
            api_key=TEST_KEY, project_key=TEST_PROJECT_KEY,
            base_url=f"{self.base_url}/fault/{name}",
        )

    def test_list_filter_and_pagination(self):
        page = self.client.get("acme", "users")
        self.assertEqual(page["meta"]["total"], 3)
        self.assertEqual([r["id"] for r in page["data"]], ["1", "2", "3"])

        filtered = self.client.query("acme", "users", {"filter": {"status": "active"}})
        self.assertEqual([r["id"] for r in filtered["data"]], ["1", "2"])
        self.assertEqual(filtered["meta"]["total"], 2)

        paged = self.client.query(
            "acme", "users", {"filter": "team:growth", "limit": 1, "page": 2}
        )
        self.assertEqual([r["id"] for r in paged["data"]], ["3"])
        self.assertEqual(
            paged["meta"], {"total": 2, "limit": 1, "offset": 1}
        )

    def test_get_by_id(self):
        self.assertEqual(self.client.get_by_id("acme", "users", "2")["name"], "Ben")

    def test_post_patch_put_delete(self):
        created = self.client.post(
            "acme", "users", {"name": "Zed", "status": "temp", "team": "core"}
        )
        self.assertEqual(created["inserted"], 1)
        new_id = created["records"][0]["id"]
        self.assertTrue(new_id.startswith("n"))

        patched = self.client.patch("acme", "users", new_id, {"status": "invited"})
        self.assertEqual(patched["updated"], 1)
        self.assertEqual(patched["record"]["status"], "invited")

        replaced = self.client.put(
            "acme", "users", new_id, {"name": "Zed", "status": "active", "team": "core"}
        )
        self.assertEqual(replaced["updated"], 1)
        self.assertEqual(replaced["record"]["status"], "active")

        deleted = self.client.delete("acme", "users", new_id)
        self.assertEqual(deleted, {"deleted": 1, "id": new_id})
        with self.assertRaises(RecordNotFoundError):
            self.client.get_by_id("acme", "users", new_id)

    def test_count(self):
        self.assertEqual(self.client.count("acme", "users"), 3)
        self.assertEqual(
            self.client.count("acme", "users", filter={"status": "active"}), 2
        )

    def test_error_mapping(self):
        bad_key = GridApiClient(
            api_key="gp_live_wrongkey123", project_key=TEST_PROJECT_KEY,
            base_url=self.base_url,
        )
        with self.assertRaises(AuthenticationError):
            bad_key.get_me()
        with patch.dict(os.environ, {"GRIDAPI_API_KEY": "",
                                     "GRIDAPI_PROJECT_KEY": TEST_PROJECT_KEY}):
            with self.assertRaises(ConfigurationError):
                GridApiClient(base_url=self.base_url)

        other = GridApiClient(
            api_key=OTHER_KEY, project_key=TEST_PROJECT_KEY, base_url=self.base_url
        )
        with self.assertRaises(AuthorizationError):
            other.get("acme", "users")

        with self.assertRaises(ProjectNotFoundError):
            self.client.get("missing", "users")
        with self.assertRaises(TableNotFoundError):
            self.client.get("acme", "not_a_table")
        with self.assertRaises(RecordNotFoundError):
            self.client.get_by_id("acme", "users", "999")
        with self.assertRaises(ValidationError):
            self.client.post("acme", "users", {"__bad": "column"})

        quota = self.fault_client("quota")
        error = None
        try:
            quota.get("acme", "users")
        except QuotaExceededError as exc:
            error = exc
        self.assertIsNotNone(error)
        self.assertEqual(error.status, 429)
        self.assertEqual(error.code, "SUBSCRIPTION_LIMIT_REACHED")
        self.assertFalse(error.retryable)  # terminal

    def test_redaction(self):
        error = None
        try:
            self.fault_client("echo").get("acme", "users")
        except ValidationError as exc:
            error = exc
        self.assertIsNotNone(error)
        for rendered in (str(error), repr(error), str(error.args), error.body):
            self.assertNotIn(TEST_KEY, rendered)
            self.assertNotIn("Bearer", rendered)

        sanitized = redact(f"Bearer {TEST_KEY} and gapi_live_secret123", secret=TEST_KEY)
        self.assertNotIn(TEST_KEY, sanitized)
        self.assertNotIn("gapi_live_secret123", sanitized)
        self.assertIn("[REDACTED]", sanitized)

    def test_retries_and_no_post_retry(self):
        client = self.fault_client("rate")
        client.max_retries = 2
        client.retry_delay_ms = 1

        # First attempt is 429 (retryable), second is 200: transparent recovery.
        page = client.get("acme", "users")
        self.assertEqual(page["meta"]["total"], 3)

        # Next hit is 429 again; POST must fail immediately, never retried.
        with self.assertRaises(RateLimitError) as caught:
            client.post("acme", "users", {"name": "no-retry"})
        self.assertTrue(caught.exception.retryable)

    def test_timeout(self):
        client = self.fault_client("timeout")
        client.timeout_ms = 150
        client.max_retries = 0
        with self.assertRaises(TimeoutError):
            client.get("acme", "users")

    def test_missing_api_key(self):
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(ConfigurationError) as caught:
                GridApiClient(base_url=self.base_url)
        self.assertIn("GRIDAPI_API_KEY", str(caught.exception))

    def test_missing_project_key(self):
        with patch.dict(os.environ, {"GRIDAPI_API_KEY": TEST_KEY}, clear=True):
            with self.assertRaises(ConfigurationError) as caught:
                GridApiClient(base_url=self.base_url)
        self.assertIn("GRIDAPI_PROJECT_KEY", str(caught.exception))
        # An explicit api key does not rescue a missing project key.
        with patch.dict(os.environ, {}, clear=True):
            with self.assertRaises(ConfigurationError):
                GridApiClient(api_key=TEST_KEY, base_url=self.base_url)

    def test_malformed_prefixes(self):
        cases = [
            dict(api_key="gapi_live_wrongprefix", project_key=TEST_PROJECT_KEY),
            dict(api_key="gp_live_", project_key=TEST_PROJECT_KEY),
            dict(api_key=TEST_KEY, project_key="gp_live_wrongprefix"),
            dict(api_key=TEST_KEY, project_key="gapi_live_"),
            dict(api_key="not-a-key", project_key=TEST_PROJECT_KEY),
            dict(api_key=TEST_KEY, project_key="not-a-key"),
        ]
        for kwargs in cases:
            with self.subTest(**kwargs):
                with self.assertRaises(ConfigurationError):
                    GridApiClient(base_url=self.base_url, **kwargs)

    def test_verify_success_and_caching(self):
        client = GridApiClient(
            api_key=TEST_KEY, project_key=TEST_PROJECT_KEY, base_url=self.base_url
        )
        # Discovery calls must not trigger verification (no recursion).
        client.list_projects()
        client.get_me()
        self.assertFalse(client._account_verified)

        client.verify()  # account only
        self.assertTrue(client._account_verified)
        client.verify("acme")
        client.verify("p1")  # id
        client.verify("Acme")  # name
        self.assertEqual(client._verified_projects, {"acme", "p1", "Acme"})
        with patch.object(client, "list_projects", side_effect=AssertionError("recached")):
            client.verify("acme")  # served from the project cache

    def test_verify_unknown_project(self):
        with self.assertRaises(ProjectNotFoundError) as caught:
            self.client.verify("ghost")
        self.assertIn("ghost", str(caught.exception))

    def test_verify_forbidden_account(self):
        other = GridApiClient(
            api_key=OTHER_KEY, project_key=TEST_PROJECT_KEY, base_url=self.base_url
        )
        with self.assertRaises(AuthorizationError):
            other.verify("acme")
        with self.assertRaises(AuthorizationError):
            other.get("acme", "users")

    def test_no_error_message_contains_key_values(self):
        errors = []
        with patch.dict(os.environ, {}, clear=True):
            try:
                GridApiClient(base_url=self.base_url)
            except ConfigurationError as exc:
                errors.append(exc)
        try:
            GridApiClient(api_key=TEST_KEY, project_key="gp_live_wrongprefix",
                          base_url=self.base_url)
        except ConfigurationError as exc:
            errors.append(exc)
        try:
            GridApiClient(api_key="gp_live_wrongkey123",
                          project_key=TEST_PROJECT_KEY,
                          base_url=self.base_url).get_me()
        except AuthenticationError as exc:
            errors.append(exc)
        try:
            self.client.verify("ghost")
        except ProjectNotFoundError as exc:
            errors.append(exc)

        self.assertTrue(errors)
        for error in errors:
            for rendered in (str(error), repr(error), str(error.args), error.body or ""):
                for key in (TEST_KEY, OTHER_KEY, TEST_PROJECT_KEY):
                    self.assertNotIn(key, rendered)

    def test_discovery(self):
        projects = self.client.list_projects()
        self.assertTrue({"acme", "beta"}.issubset({p["slug"] for p in projects}))
        tables = self.client.list_tables("acme")
        self.assertEqual({"users", "empty"}, {t["endpoint"] for t in tables})
        with self.assertRaises(ProjectNotFoundError):
            self.client.list_tables("nope")

        schema = self.client.get_schema("acme", "users")
        self.assertEqual(schema["endpoint"], "users")
        self.assertEqual(schema["primaryKey"], "id")
        with self.assertRaises(TableNotFoundError):
            self.client.get_schema("acme", "nope")

        me = self.client.get_me()
        self.assertTrue(me["authenticated"])
        self.assertEqual(me["account"]["id"], "acct1")
        self.assertEqual(self.client.get_usage()["totalApiCallsUsed"], 10)


if __name__ == "__main__":
    unittest.main()
