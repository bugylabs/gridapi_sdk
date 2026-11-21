#!/usr/bin/env python3
"""Runnable GridAPI Python SDK example.

Reads credentials from the environment only — no keys in source::

    export GRIDAPI_API_KEY="..."           # account-level key
    export GRIDAPI_BASE_URL="https://api.gridapi.in"   # optional
    python3 examples/python/quickstart.py

Against the local contract mock server instead:

    GRIDAPI_API_KEY=gp_live_testkey \\
    GRIDAPI_BASE_URL=http://127.0.0.1:8787 \\
    python3 examples/python/quickstart.py
"""

import os
import sys
from pathlib import Path

# Allow running straight from a checkout without installing the package.
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "packages" / "python"))

from gridapi import GridApiClient, GridApiError  # noqa: E402


def main():
    client = GridApiClient()  # GRIDAPI_API_KEY / BASE_URL / TIMEOUT_MS

    me = client.get_me()
    print("authenticated as account:", me.get("account", {}).get("id"))

    projects = client.list_projects()
    print("projects:", [p["slug"] for p in projects])

    project = "acme"
    tables = [t["endpoint"] for t in client.list_tables(project)]
    print("tables for", project, ":", tables)

    # Filter + paginate a collection.
    page = client.get(project, "users", {"filter": {"status": "active"}, "limit": 2})
    print("active users:", [r["id"] for r in page["data"]], "of", page["meta"]["total"])
    print("count:", client.count(project, "users", filter={"status": "active"}))

    # One record, then create → patch → put → delete a throwaway record.
    first = client.get_by_id(project, "users", "1")
    print("first record:", first)

    created = client.post(
        project, "users", {"name": "Example", "status": "active", "team": "core"}
    )
    new_id = created["records"][0]["id"]
    print("inserted:", created["inserted"], "id:", new_id)

    print("patched:", client.patch(project, "users", new_id, {"status": "invited"})["updated"])
    print("replaced:", client.put(
        project, "users", new_id, {"name": "Example", "status": "active", "team": "core"}
    )["updated"])
    print("deleted:", client.delete(project, "users", new_id)["deleted"])

    print("schema:", client.get_schema(project, "users")["endpoint"])
    print("usage:", client.get_usage()["totalApiCallsUsed"])


if __name__ == "__main__":
    try:
        main()
    except GridApiError as err:
        # err is safe to print: keys are redacted from message and repr.
        print(f"{err.name} (status={err.status}, retryable={err.retryable}): {err}")
        sys.exit(1)
