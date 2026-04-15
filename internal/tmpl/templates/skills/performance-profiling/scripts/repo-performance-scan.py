#!/usr/bin/env python3
"""Repo-level performance scan (stdlib only).

This is **not** a process or binary profiler. It inspects a directory
tree for static indicators that correlate with runtime cost: large
files, high dependency counts, and bundle-like build output. Use it as
a triage step before attaching a real profiler — see
`references/profiling-recipes.md` for the runtime tools.

Lifted and renamed from
`alirezarezvani/performance-profiler/scripts/performance_profiler.py`
(Wave-2 PR-2). The contract is narrowed to a repo scan on purpose; do
not add process, binary, flamegraph, or load-test orchestration flags.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Dict, Iterable, List, Tuple

EXT_WEIGHTS = {
    ".js": 1.0,
    ".jsx": 1.0,
    ".ts": 1.0,
    ".tsx": 1.0,
    ".css": 0.7,
    ".map": 2.0,
}

SKIP_DIRS = {
    ".git",
    "node_modules",
    ".next",
    "dist",
    "build",
    "coverage",
    "__pycache__",
    ".venv",
    "venv",
    "target",
}


def iter_files(root: Path) -> Iterable[Path]:
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for filename in filenames:
            path = Path(dirpath) / filename
            if path.is_file():
                yield path


def get_large_files(root: Path, threshold_bytes: int) -> List[Tuple[str, int]]:
    large: List[Tuple[str, int]] = []
    for file_path in iter_files(root):
        try:
            size = file_path.stat().st_size
        except OSError:
            continue
        if size >= threshold_bytes:
            large.append((str(file_path.relative_to(root)), size))
    return sorted(large, key=lambda item: (-item[1], item[0]))


def count_dependencies(root: Path) -> Dict[str, int]:
    counts = {"node_dependencies": 0, "python_dependencies": 0, "go_dependencies": 0}

    package_json = root / "package.json"
    if package_json.exists():
        try:
            data = json.loads(package_json.read_text(encoding="utf-8"))
            deps = data.get("dependencies", {}) or {}
            dev_deps = data.get("devDependencies", {}) or {}
            counts["node_dependencies"] = len(deps) + len(dev_deps)
        except (OSError, ValueError):
            pass

    requirements = root / "requirements.txt"
    if requirements.exists():
        try:
            lines = [
                ln.strip()
                for ln in requirements.read_text(
                    encoding="utf-8", errors="ignore"
                ).splitlines()
            ]
            counts["python_dependencies"] = sum(
                1 for ln in lines if ln and not ln.startswith("#")
            )
        except OSError:
            pass

    go_mod = root / "go.mod"
    if go_mod.exists():
        try:
            lines = go_mod.read_text(encoding="utf-8", errors="ignore").splitlines()
        except OSError:
            lines = []
        in_require_block = False
        go_count = 0
        for ln in lines:
            s = ln.strip()
            if s.startswith("require ("):
                in_require_block = True
                continue
            if in_require_block and s == ")":
                in_require_block = False
                continue
            if in_require_block and s and not s.startswith("//"):
                go_count += 1
            elif s.startswith("require ") and not s.endswith("("):
                go_count += 1
        counts["go_dependencies"] = go_count

    return counts


def bundle_indicators(root: Path) -> Dict[str, object]:
    build_dirs: List[str] = []
    for d in ("dist", "build", ".next", "out"):
        if (root / d).exists():
            build_dirs.append(d)

    bundle_files = 0
    weight = 0.0
    for path in iter_files(root):
        ext = path.suffix.lower()
        if ext in EXT_WEIGHTS:
            bundle_files += 1
            try:
                size_kb = path.stat().st_size / 1024.0
            except OSError:
                continue
            weight += size_kb * EXT_WEIGHTS[ext]

    return {
        "build_dirs_present": build_dirs,
        "bundle_like_files": bundle_files,
        "estimated_bundle_weight_kb": round(weight, 2),
    }


def format_size(num_bytes: int) -> str:
    units = ("B", "KB", "MB", "GB")
    value = float(num_bytes)
    for unit in units:
        if value < 1024.0 or unit == units[-1]:
            return f"{value:.1f}{unit}"
        value /= 1024.0
    return f"{num_bytes}B"


def build_report(root: Path, threshold_bytes: int) -> Dict[str, object]:
    return {
        "root": str(root),
        "large_file_threshold_bytes": threshold_bytes,
        "large_files": get_large_files(root, threshold_bytes),
        "dependency_counts": count_dependencies(root),
        "bundle_indicators": bundle_indicators(root),
    }


def print_text(report: Dict[str, object]) -> None:
    print("Repo Performance Scan")
    print(f"Root: {report['root']}")
    threshold = int(report["large_file_threshold_bytes"])
    print(f"Large-file threshold: {format_size(threshold)}")
    print("")

    dep_counts = report["dependency_counts"]
    print("Dependency Counts")
    print(f"- Node: {dep_counts['node_dependencies']}")
    print(f"- Python: {dep_counts['python_dependencies']}")
    print(f"- Go: {dep_counts['go_dependencies']}")
    print("")

    bundle = report["bundle_indicators"]
    build_dirs = bundle["build_dirs_present"] or ["none"]
    print("Bundle Indicators")
    print(f"- Build directories present: {', '.join(build_dirs)}")
    print(f"- Bundle-like files: {bundle['bundle_like_files']}")
    print(
        f"- Estimated weighted bundle size: {bundle['estimated_bundle_weight_kb']} KB"
    )
    print("")

    print("Large Files")
    large_files = report["large_files"]
    if not large_files:
        print("- None above threshold")
        return
    for rel_path, size in large_files[:20]:
        print(f"- {rel_path}: {format_size(size)}")


def parse_args(argv: List[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="repo-performance-scan.py",
        description=(
            "Scan a repository for static performance risk indicators. "
            "This tool does not attach a profiler or run the application."
        ),
    )
    parser.add_argument("path", help="Directory to scan")
    parser.add_argument(
        "--large-file-threshold-kb",
        type=int,
        default=512,
        help="Threshold in KB for reporting large files (default: 512)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit JSON output instead of text",
    )
    return parser.parse_args(argv)


def main(argv: List[str]) -> int:
    args = parse_args(argv)
    root = Path(args.path).expanduser().resolve()
    if not root.exists() or not root.is_dir():
        print(f"error: path is not a directory: {root}", file=sys.stderr)
        return 2

    threshold = max(1, args.large_file_threshold_kb) * 1024
    report = build_report(root, threshold)

    if args.json:
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        print_text(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
