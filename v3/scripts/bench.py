#!/usr/bin/env python3
"""Tiny local benchmark: pysh vs xargs vs GNU parallel.

This is a development helper, not a claim of universal performance. It uses
trivial commands so the numbers mostly measure scheduling and process-start
overhead. Run it on the machine and task shape you care about.
"""
from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
import tempfile
import time


def timed(cmd: list[str], cwd: str, stdin_path: str | None = None) -> float:
    stdin = open(stdin_path, "rb") if stdin_path else subprocess.DEVNULL
    try:
        start = time.perf_counter()
        proc = subprocess.run(cmd, cwd=cwd, stdin=stdin,
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        elapsed = time.perf_counter() - start
        if proc.returncode != 0:
            raise RuntimeError(f"{cmd[0]} exited {proc.returncode}")
        return elapsed
    finally:
        if stdin_path:
            stdin.close()


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--tasks", type=int, default=500)
    ap.add_argument("--jobs", type=int, default=8)
    ap.add_argument("--pysh", default=os.path.join("dist", "pysh"))
    args = ap.parse_args()

    pysh = os.path.abspath(args.pysh)
    if not os.path.isfile(pysh):
        print(f"missing {pysh}; run ./scripts/build.sh first", file=sys.stderr)
        return 2

    tmp = tempfile.mkdtemp(prefix="pysh-bench-")
    try:
        tasks = os.path.join(tmp, "tasks.dat")
        with open(tasks, "w", encoding="utf-8") as f:
            for _ in range(args.tasks):
                f.write("true\n")

        rows: list[tuple[str, float]] = []
        rows.append(("pysh", timed(
            [pysh, "run", "-i", tasks, "-c", str(args.jobs),
             "--workdir", os.path.join(tmp, ".pysh"), "--quiet"], tmp)))

        if shutil.which("xargs"):
            rows.append(("xargs -P", timed(
                ["xargs", "-P", str(args.jobs), "-I{}", "sh", "-c", "{}"], tmp, tasks)))

        if shutil.which("parallel"):
            rows.append(("GNU parallel", timed(
                ["parallel", "-j", str(args.jobs)], tmp, tasks)))

        print(f"\n{args.tasks} trivial tasks, -j {args.jobs}, cwd={tmp}\n")
        print("| Tool | Wall time | Tasks/sec |")
        print("|---|---:|---:|")
        for name, seconds in rows:
            print(f"| {name} | {seconds * 1000:.0f} ms | {args.tasks / seconds:.0f} |")
        return 0
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
