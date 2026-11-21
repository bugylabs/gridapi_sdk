#!/usr/bin/env python3
"""Check every SDK example: no embedded keys, valid syntax, buildable locally.

Stdlib only. Toolchains that are not installed are reported as SKIP, never as
pass. This is a release-gate helper, not a replacement for contract tests.
"""
import ast
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LANGS = ["javascript", "python", "java", "dotnet", "go", "rust"]
KEY_RE = re.compile(r"(gp_live_|gapi_live_)[A-Za-z0-9_-]{6,}|YOUR_API_KEY")


def run(cmd):
    try:
        return subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True, timeout=180)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return subprocess.CompletedProcess(cmd, 1, "", str(exc))


def main():
    problems, skipped = [], []
    for lang in LANGS:
        directory = ROOT / "examples" / lang
        files = [p for p in directory.rglob("*") if p.is_file()] if directory.exists() else []
        if not files:
            problems.append(f"{lang}: no examples found")
            continue
        for path in files:
            if KEY_RE.search(path.read_text(errors="ignore")):
                problems.append(f"{path.relative_to(ROOT)}: embedded credential or placeholder")
        if lang == "javascript":
            for path in directory.glob("*.mjs"):
                r = run(["node", "--check", str(path)])
                if r.returncode:
                    problems.append(f"{path.relative_to(ROOT)}: {r.stderr.strip()}")
        elif lang == "python":
            for path in directory.rglob("*.py"):
                try:
                    ast.parse(path.read_text())
                except SyntaxError as exc:
                    problems.append(f"{path.relative_to(ROOT)}: {exc}")
        elif lang == "java":
            if shutil.which("javac"):
                sources = [str(p) for p in directory.rglob("*.java")]
                r = run(["javac", "-sourcepath", str(ROOT / "packages/java/src/main/java"), "-d", str(ROOT / "target" / "example-classes"), *sources])
                if r.returncode:
                    problems.append(f"{lang}: javac failed: {r.stderr.strip()[:400]}")
            else:
                skipped.append("java (javac not installed)")
        elif lang == "dotnet":
            if shutil.which("dotnet"):
                r = run(["dotnet", "build", str(directory), "--nologo"])
                if r.returncode:
                    problems.append(f"{lang}: dotnet build failed: {r.stderr.strip()[:400]}")
            else:
                skipped.append("dotnet (not installed)")
        elif lang == "go":
            if shutil.which("go"):
                r = run(["go", "build", "-o", str(Path(tempfile.gettempdir()) / "gridapi-example-check"), "./examples/go"])
                if r.returncode:
                    problems.append(f"{lang}: go build failed: {r.stderr.strip()[:400]}")
            else:
                skipped.append("go (not installed)")
        elif lang == "rust":
            if shutil.which("cargo"):
                r = run(["cargo", "check", "--manifest-path", str(directory / "Cargo.toml")])
                if r.returncode:
                    problems.append(f"{lang}: cargo check failed: {r.stderr.strip()[:400]}")
            else:
                skipped.append("rust (cargo not installed)")
    if skipped:
        print("SKIP: " + ", ".join(skipped))
    if problems:
        print("FAIL: example checks")
        for item in problems:
            print("  - " + item)
        return 1
    print("OK: 6 SDK example directories checked")
    return 0


if __name__ == "__main__":
    sys.exit(main())
