#!/usr/bin/env python3
"""Fetch unread GitHub review-requested notifications for open PRs.

Usage:
    fetch-review-requests.py --teams TEAM1,TEAM2 [--org ORG]

Arguments:
    --org     GitHub organization slug (default: getsentry)
    --teams   Comma-separated team slugs to filter by

Output: JSON to stdout with matching PRs.

Lifted from ``getsentry/gh-review-requests/scripts/fetch_review_requests.py``
(Wave-3 PR-2). Narrowings vs upstream:

  - Interpreter floor narrowed from ``requires-python = ">=3.12"`` (PEP
    723 inline metadata, intended for ``uv run``) to the shared Slipway
    Python contract (3.8+). Set-type annotations were rewritten to the
    quoted typing form so py_compile accepts the file on 3.8.
  - Replaced ``uv run`` invocation with a direct ``python3`` shebang so
    this helper matches the other Wave-3 scripts.
  - Added a fail-fast ``gh``/credentials preflight that exits 2 with an
    actionable message when ``gh`` is missing or unauthenticated.

Credentials come from ``GH_TOKEN`` / ``GITHUB_TOKEN`` or the existing
``gh`` login.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys


def _die(code, message):
    sys.stderr.write(message.rstrip() + "\n")
    sys.exit(code)


def preflight():
    if shutil.which("gh") is None:
        _die(
            2, "fetch-review-requests: gh CLI not found on PATH; install gh or set PATH"
        )
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
        _die(2, "fetch-review-requests: gh auth status failed: {0}".format(exc))
    if result.returncode != 0:
        _die(
            2,
            "fetch-review-requests: gh not authenticated; "
            "set GH_TOKEN or run `gh auth login` before invoking this helper",
        )


def gh(path, paginate=False):
    cmd = ["gh", "api", path]
    if paginate:
        cmd.append("--paginate")
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0 or not result.stdout:
        sys.stderr.write(
            "Error running gh {0}: {1}\n".format(" ".join(cmd), result.stderr)
        )
        return [] if paginate else {}
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        return [] if paginate else {}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--org", default="getsentry")
    parser.add_argument("--teams", required=True, help="Comma-separated team slugs")
    args = parser.parse_args()

    preflight()

    team_slugs = [t.strip() for t in args.teams.split(",")]

    members = set()  # type: ignore[var-annotated]
    team_display_names = {}
    for slug in team_slugs:
        data = gh("orgs/{0}/teams/{1}/members".format(args.org, slug), paginate=True)
        for m in data:
            members.add(m["login"])
        team_data = gh("orgs/{0}/teams/{1}".format(args.org, slug))
        team_display_names[slug] = team_data.get("name", slug)

    all_notifs = gh("notifications", paginate=True)
    review_notifs = [
        n
        for n in all_notifs
        if n.get("reason") == "review_requested" and n.get("unread")
    ]

    prs = []
    for n in review_notifs:
        url = n["subject"]["url"]
        repo_path = url.replace("https://api.github.com/repos/", "")
        repo = repo_path.rsplit("/pulls/", 1)[0]
        pr_num = repo_path.rsplit("/", 1)[-1]
        html_url = "https://github.com/{0}/pull/{1}".format(repo, pr_num)

        pr_data = gh("repos/{0}/pulls/{1}".format(repo, pr_num))
        if not pr_data:
            continue
        if pr_data.get("merged_at") or pr_data.get("state") == "closed":
            continue

        author = pr_data["user"]["login"]
        reviewers_data = gh(
            "repos/{0}/pulls/{1}/requested_reviewers".format(repo, pr_num)
        )
        requested_team_names = [t["slug"] for t in reviewers_data.get("teams", [])]
        matching_teams = [
            t
            for t in requested_team_names
            if any(slug.lower() == t.lower() for slug in team_slugs)
        ]

        by_team_member = author in members
        review_from_team = len(matching_teams) > 0

        if not (by_team_member or review_from_team):
            continue

        reasons = []
        if review_from_team:
            reasons.append(
                "review requested from: {0}".format(", ".join(matching_teams))
            )
        if by_team_member:
            reasons.append("opened by: {0}".format(author))

        prs.append(
            {
                "notification_id": n["id"],
                "title": n["subject"]["title"],
                "url": html_url,
                "repo": repo,
                "pr_number": int(pr_num),
                "author": author,
                "reasons": reasons,
            }
        )

    print(json.dumps({"total": len(prs), "prs": prs}, indent=2))


if __name__ == "__main__":
    main()
