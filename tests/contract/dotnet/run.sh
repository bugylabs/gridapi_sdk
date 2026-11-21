#!/usr/bin/env bash
# Runs the .NET SDK contract tests against the shared mock server.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT"
dotnet run --project tests/contract/dotnet/GridApiContractTests.csproj
