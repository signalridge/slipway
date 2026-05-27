#!/usr/bin/env python3
"""Fetch PR CI checks and extract relevant failure snippets.

Usage:
    fetch-pr-checks.py [--pr PR_NUMBER]

If --pr is not specified, uses the PR for the current branch.

Output: JSON to stdout with structured check data.

This helper stays Python because log snippet extraction is regex-heavy and
easier to keep readable there. Slipway-specific behavior:

  - Uses a plain shebang; Slipway's shared Python contract targets 3.8+.
  - Runs a fail-fast preflight that checks ``gh`` is on PATH and credentials
    are loaded (``gh auth status``), surfacing a structured credential error
    instead of silent ``gh`` failures.

No live API calls are made by the preflight itself. Credentials are
read from ``GH_TOKEN`` / ``GITHUB_TOKEN`` or the existing ``gh``
login; supply one of them before invocation.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys


def _die(code, message):
    sys.stderr.write(message.rstrip() + "\n")
    sys.exit(code)


def preflight():
    if shutil.which("gh") is None:
        _die(2, "fetch-pr-checks: gh CLI not found on PATH; install gh or set PATH")
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if token:
        return
    try:
        result = subprocess.run(
            ["gh", "auth", "status"],
            capture_output=True,
            text=True,
            timeout=15,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        _die(2, "fetch-pr-checks: gh auth status failed: {0}".format(exc))
    if result.returncode != 0:
        _die(
            2,
            "fetch-pr-checks: gh not authenticated; "
            "set GH_TOKEN or run `gh auth login` before invoking this helper",
        )


def run_gh(args):
    try:
        result = subprocess.run(
            ["gh"] + args,
            capture_output=True,
            text=True,
            check=True,
        )
        return json.loads(result.stdout) if result.stdout.strip() else None
    except subprocess.CalledProcessError as e:
        sys.stderr.write("Error running gh {0}: {1}\n".format(" ".join(args), e.stderr))
        return None
    except json.JSONDecodeError:
        return None


def get_pr_info(pr_number):
    args = ["pr", "view", "--json", "number,url,headRefName,baseRefName"]
    if pr_number:
        args.insert(2, str(pr_number))
    return run_gh(args)


def get_checks(pr_number):
    args = ["gh", "pr", "checks"]
    if pr_number:
        args.append(str(pr_number))
    try:
        result = subprocess.run(args, capture_output=True, text=True)
        if not result.stdout.strip():
            return []
        checks = []
        for line in result.stdout.strip().split("\n"):
            if not line.strip():
                continue
            parts = line.split("\t")
            if len(parts) >= 2:
                checks.append(
                    {
                        "name": parts[0].strip(),
                        "bucket": parts[1].strip(),
                        "link": parts[3].strip() if len(parts) > 3 else "",
                        "workflow": "",
                    }
                )
        return checks
    except Exception:
        return []


def get_failed_runs(branch):
    result = run_gh(
        [
            "run",
            "list",
            "--branch",
            branch,
            "--limit",
            "10",
            "--json",
            "databaseId,name,status,conclusion,headSha",
        ]
    )
    if not isinstance(result, list):
        return []
    return [r for r in result if r.get("conclusion") == "failure"]


def extract_failure_snippet(log_text, max_lines=50):
    lines = log_text.split("\n")
    failure_patterns = [
        r"error[:\s]",
        r"failed[:\s]",
        r"failure[:\s]",
        r"traceback",
        r"exception",
        r"assert(ion)?.*failed",
        r"FAILED",
        r"panic:",
        r"fatal:",
        r"npm ERR!",
        r"yarn error",
        r"ModuleNotFoundError",
        r"ImportError",
        r"SyntaxError",
        r"TypeError",
        r"ValueError",
        r"KeyError",
        r"AttributeError",
        r"NameError",
        r"IndentationError",
        r"===.*FAILURES.*===",
        r"___.*___",
    ]
    combined_pattern = "|".join(failure_patterns)
    failure_indices = []
    for i, line in enumerate(lines):
        if re.search(combined_pattern, line, re.IGNORECASE):
            failure_indices.append(i)
    if not failure_indices:
        return "\n".join(lines[-max_lines:])
    first_failure = failure_indices[0]
    start = max(0, first_failure - 5)
    end = min(len(lines), first_failure + max_lines - 5)
    snippet_lines = lines[start:end]
    remaining_failures = [i for i in failure_indices if i >= end]
    if remaining_failures:
        snippet_lines.append(
            "\n... ({0} more error(s) follow)".format(len(remaining_failures))
        )
    return "\n".join(snippet_lines)


def get_run_logs(run_id):
    try:
        result = subprocess.run(
            ["gh", "run", "view", str(run_id), "--log-failed"],
            capture_output=True,
            text=True,
            timeout=60,
        )
        return result.stdout if result.stdout else result.stderr
    except subprocess.TimeoutExpired:
        return None
    except subprocess.CalledProcessError:
        return None


def main():
    parser = argparse.ArgumentParser(
        description="Fetch PR CI checks with failure snippets"
    )
    parser.add_argument(
        "--pr", type=int, help="PR number (defaults to current branch PR)"
    )
    args = parser.parse_args()

    preflight()

    pr_info = get_pr_info(args.pr)
    if not pr_info:
        print(json.dumps({"error": "No PR found for current branch"}))
        sys.exit(1)

    pr_number = pr_info["number"]
    branch = pr_info["headRefName"]

    checks = get_checks(pr_number)
    processed_checks = []
    failed_runs = None

    for check in checks:
        processed = {
            "name": check.get("name", "unknown"),
            "status": check.get("bucket", check.get("state", "unknown")),
            "link": check.get("link", ""),
            "workflow": check.get("workflow", ""),
        }
        if processed["status"] == "fail":
            if failed_runs is None:
                failed_runs = get_failed_runs(branch)
            workflow_name = processed["workflow"] or processed["name"]
            matching_run = next(
                (r for r in failed_runs if workflow_name in r.get("name", "")),
                None,
            )
            if matching_run:
                logs = get_run_logs(matching_run["databaseId"])
                if logs:
                    processed["log_snippet"] = extract_failure_snippet(logs)
                    processed["run_id"] = matching_run["databaseId"]
        processed_checks.append(processed)

    output = {
        "pr": {
            "number": pr_number,
            "url": pr_info.get("url", ""),
            "branch": branch,
            "base": pr_info.get("baseRefName", ""),
        },
        "summary": {
            "total": len(processed_checks),
            "passed": sum(1 for c in processed_checks if c["status"] == "pass"),
            "failed": sum(1 for c in processed_checks if c["status"] == "fail"),
            "pending": sum(1 for c in processed_checks if c["status"] == "pending"),
            "skipped": sum(
                1 for c in processed_checks if c["status"] in ("skipping", "cancel")
            ),
        },
        "checks": processed_checks,
    }

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
