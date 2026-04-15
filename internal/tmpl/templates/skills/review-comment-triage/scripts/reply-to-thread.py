#!/usr/bin/env python3
"""Reply to PR review threads.

Usage:
    reply-to-thread.py THREAD_ID BODY [THREAD_ID BODY ...] [--confirm]

Accepts one or more (thread_id, body) pairs as positional arguments.
Batches all replies into a single GraphQL mutation.

SAFETY INVARIANT (Wave-3 PR-2 blast-radius contract):
    This helper is write-capable. It DEFAULTS TO DRY-RUN: without
    ``--confirm`` it prints the intended GraphQL mutation to stderr and
    exits with a non-zero status. Posting happens only when ``--confirm``
    is supplied explicitly. Do not relax this default.

Lifted from ``getsentry/iterate-pr/scripts/reply_to_thread.py`` (Wave-3
PR-2). Narrowings vs upstream:

  - Shebang narrowed from PEP 723 ``requires-python = ">=3.9"`` to the
    shared Slipway Python contract (3.8+).
  - Added a mandatory ``--confirm`` gate: upstream posts immediately.
  - Added a fail-fast ``gh``/credentials preflight (only enforced when
    ``--confirm`` is set, so dry-run works offline).

Credentials come from ``GH_TOKEN`` / ``GITHUB_TOKEN`` or the existing
``gh`` login. Both must be absent for preflight to fail.
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
        _die(2, "reply-to-thread: gh CLI not found on PATH; install gh or set PATH")
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
        _die(2, "reply-to-thread: gh auth status failed: {0}".format(exc))
    if result.returncode != 0:
        _die(
            2,
            "reply-to-thread: gh not authenticated; "
            "set GH_TOKEN or run `gh auth login` before --confirm posting",
        )


def _normalize_body(body):
    normalized = body.replace("\\r\\n", "\\n").replace("\\n", "\n")
    lines = normalized.rstrip().split("\n")
    last_line = lines[-1] if lines else ""
    bot_signature_pattern = r"^\*[\u2014-]\s+.+\*$"
    if not re.match(bot_signature_pattern, last_line.strip()):
        if normalized and not normalized.endswith("\n"):
            normalized += "\n"
        if normalized and not normalized.endswith("\n\n"):
            normalized += "\n"
        normalized += "*\u2014 Claude Code*"
    return normalized


def build_mutation(pairs):
    mutations = []
    for i, (thread_id, body) in enumerate(pairs):
        escaped_thread_id = json.dumps(thread_id)
        escaped_body = json.dumps(_normalize_body(body))
        mutations.append(
            "  r{0}: addPullRequestReviewThreadReply(input: {{"
            "pullRequestReviewThreadId: {1}, body: {2}"
            "}}) {{ clientMutationId }}".format(i, escaped_thread_id, escaped_body)
        )
    return "mutation {\n" + "\n".join(mutations) + "\n}"


def reply_to_threads(pairs):
    query = build_mutation(pairs)
    try:
        result = subprocess.run(
            ["gh", "api", "graphql", "-f", "query={0}".format(query)],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode != 0:
            sys.stderr.write("GraphQL error: {0}\n".format(result.stderr))
            return [(tid, False) for tid, _ in pairs]
        try:
            response = json.loads(result.stdout)
        except (json.JSONDecodeError, TypeError):
            sys.stderr.write(
                "Failed to parse GraphQL response: {0}\n".format(result.stdout)
            )
            return [(tid, False) for tid, _ in pairs]
        data = response.get("data") or {}
        errors = response.get("errors") or []
        error_paths = set()
        for err in errors:
            for segment in err.get("path") or []:
                if isinstance(segment, str) and segment.startswith("r"):
                    error_paths.add(segment)
        operation_results = []
        for i, (tid, _) in enumerate(pairs):
            alias = "r{0}".format(i)
            if alias in error_paths or data.get(alias) is None:
                operation_results.append((tid, False))
            else:
                operation_results.append((tid, True))
        if any(not ok for _, ok in operation_results):
            failed = [tid for tid, ok in operation_results if not ok]
            sys.stderr.write(
                "GraphQL partial failure for threads: {0}\n".format(failed)
            )
        return operation_results
    except subprocess.TimeoutExpired:
        sys.stderr.write("Request timed out\n")
        return [(tid, False) for tid, _ in pairs]


def main():
    parser = argparse.ArgumentParser(
        description="Reply to PR review threads (default: dry-run; use --confirm to post)",
        usage="%(prog)s THREAD_ID BODY [THREAD_ID BODY ...] [--confirm]",
    )
    parser.add_argument(
        "--confirm",
        action="store_true",
        help="Actually post the reply. Without --confirm this is a dry-run.",
    )
    parser.add_argument("args", nargs="+", help="Alternating thread_id and body pairs")
    parsed = parser.parse_args()

    if len(parsed.args) % 2 != 0:
        _die(1, "Error: arguments must be (thread_id, body) pairs")

    pairs = []
    for i in range(0, len(parsed.args), 2):
        pairs.append((parsed.args[i], parsed.args[i + 1]))

    query = build_mutation(pairs)

    if not parsed.confirm:
        sys.stderr.write(
            "DRY-RUN: reply-to-thread would post the following GraphQL mutation.\n"
            "Pass --confirm to actually submit it.\n"
            "----- BEGIN DRY-RUN REQUEST -----\n"
        )
        sys.stderr.write(query + "\n")
        sys.stderr.write("----- END DRY-RUN REQUEST -----\n")
        sys.exit(3)

    preflight()

    results = reply_to_threads(pairs)
    success = all(ok for _, ok in results)
    by_thread = {}
    for tid, ok in results:
        by_thread.setdefault(tid, []).append(ok)

    output = {
        "replied": sum(1 for _, ok in results if ok),
        "failed": sum(1 for _, ok in results if not ok),
        "operations": [
            {"thread_id": tid, "status": "ok" if ok else "failed"}
            for tid, ok in results
        ],
        "threads": {
            tid: "ok" if all(statuses) else "failed"
            for tid, statuses in by_thread.items()
        },
    }
    print(json.dumps(output, indent=2))
    if not success:
        sys.exit(1)


if __name__ == "__main__":
    main()
