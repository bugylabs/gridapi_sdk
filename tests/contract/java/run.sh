#!/usr/bin/env bash
# Compile the Java SDK + contract test with plain javac and run the test.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
BUILD="$(mktemp -d)"
trap 'rm -rf "$BUILD"' EXIT

mkdir -p "$BUILD/classes"

javac -encoding UTF-8 -d "$BUILD/classes" \
  $(find "$ROOT/packages/java/src/main/java" -name '*.java' | sort) \
  "$ROOT/examples/java/Quickstart.java" \
  "$HERE/ContractTest.java"

java -cp "$BUILD/classes" -Dgridapi.root="$ROOT" ContractTest
