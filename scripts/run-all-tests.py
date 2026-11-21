#!/usr/bin/env python3
"""Run every SDK contract suite whose toolchain is installed."""
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# (label, executable needed, command, working directory)
SUITES = [
    ("javascript", "node", ["npm", "test"], ROOT / "packages/javascript"),
    ("python", "python3", [sys.executable, "-m", "unittest", "discover", "-s", "tests/contract/python", "-v"], ROOT),
    ("java", "javac", ["bash", "tests/contract/java/run.sh"], ROOT),
    ("dotnet", "dotnet", ["bash", "tests/contract/dotnet/run.sh"], ROOT),
    ("go", "go", ["go", "test", "./tests/contract/go", "-v"], ROOT),
    ("rust", "cargo", ["cargo", "test", "--manifest-path", "packages/rust/Cargo.toml"], ROOT),
]

def main():
    failed = []
    for label, needs, cmd, cwd in SUITES:
        if not shutil.which(needs):
            print(f"SKIP {label}: {needs} not installed")
            continue
        print(f"\n=== {label} ===", flush=True)
        result = subprocess.run(cmd, cwd=cwd)
        if result.returncode != 0:
            failed.append(label)
    if failed:
        print(f"\nFAILED: {', '.join(failed)}")
        return 1
    print("\nAll available SDK suites passed.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
