#!/usr/bin/env python3
"""Fetch and categorize PR review feedback.

Usage:
    fetch-pr-feedback.py [--pr PR_NUMBER]

If --pr is not specified, uses the PR for the current branch.

Output: JSON to stdout with categorized feedback.

Categories (using LOGAF scale):
  - high: Must address before merge (h:, blocker, changes requested)
  - medium: Should address (m:, standard feedback)
  - low: Optional suggestions (l:, nit, style)
  - bot: Informational automated comments (Codecov, Dependabot, etc.)
  - resolved: Already resolved threads

Bot classification:
  - Review bots (Sentry, Warden, Cursor, Bugbot, etc.) provide actionable
    code feedback. Their comments are categorized by content into
    high/medium/low with a ``review_bot: true`` flag.
  - Info bots (Codecov, Dependabot, Renovate, etc.) post status reports
    and are placed in the ``bot`` bucket for silent skipping.

Slipway-specific behavior:

  - Uses a plain shebang; the shared Slipway Python contract targets 3.8+.
  - Runs a fail-fast ``gh``/credentials preflight that exits 2 with an
    actionable message when ``gh`` is missing or unauthenticated.

No live API calls are made by the preflight itself. Credentials are
read from ``GH_TOKEN`` / ``GITHUB_TOKEN`` or the existing ``gh`` login.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys


REVIEW_BOT_PATTERNS = [
    r"(?i)^sentry",
    r"(?i)^warden",
    r"(?i)^cursor",
    r"(?i)^bugbot",
    r"(?i)^seer",
    r"(?i)^copilot",
    r"(?i)^codex",
    r"(?i)^claude",
    r"(?i)^codeql",
]

INFO_BOT_PATTERNS = [
    r"(?i)^codecov",
    r"(?i)^dependabot",
    r"(?i)^renovate",
    r"(?i)^github-actions",
    r"(?i)^mergify",
    r"(?i)^semantic-release",
    r"(?i)^sonarcloud",
    r"(?i)^snyk",
    r"(?i)bot$",
    r"(?i)\[bot\]$",
]


def _die(code, message):
    sys.stderr.write(message.rstrip() + "\n")
    sys.exit(code)


def preflight():
    if shutil.which("gh") is None:
        _die(2, "fetch-pr-feedback: gh CLI not found on PATH; install gh or set PATH")
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
        _die(2, "fetch-pr-feedback: gh auth status failed: {0}".format(exc))
    if result.returncode != 0:
        _die(
            2,
            "fetch-pr-feedback: gh not authenticated; "
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


def get_repo_info():
    result = run_gh(["repo", "view", "--json", "owner,name"])
    if result:
        return result.get("owner", {}).get("login"), result.get("name")
    return None


def get_pr_info(pr_number):
    args = [
        "pr",
        "view",
        "--json",
        "number,url,headRefName,author,reviews,reviewDecision",
    ]
    if pr_number:
        args.insert(2, str(pr_number))
    return run_gh(args)


def is_review_bot(username):
    return any(re.search(p, username) for p in REVIEW_BOT_PATTERNS)


def is_info_bot(username):
    return any(re.search(p, username) for p in INFO_BOT_PATTERNS)


def is_bot(username):
    return is_review_bot(username) or is_info_bot(username)


def get_review_comments(owner, repo, pr_number):
    result = run_gh(
        [
            "api",
            "repos/{0}/{1}/pulls/{2}/comments".format(owner, repo, pr_number),
            "--paginate",
        ]
    )
    return result if isinstance(result, list) else []


def get_issue_comments(owner, repo, pr_number):
    result = run_gh(
        [
            "api",
            "repos/{0}/{1}/issues/{2}/comments".format(owner, repo, pr_number),
            "--paginate",
        ]
    )
    return result if isinstance(result, list) else []


def get_review_threads(owner, repo, pr_number):
    query = (
        "query($owner: String!, $repo: String!, $pr: Int!) {\n"
        "  repository(owner: $owner, name: $repo) {\n"
        "    pullRequest(number: $pr) {\n"
        "      reviewThreads(first: 100) {\n"
        "        nodes {\n"
        "          id isResolved isOutdated path line\n"
        "          comments(first: 10) {\n"
        "            nodes { id body author { login } createdAt }\n"
        "          }\n"
        "        }\n"
        "      }\n"
        "    }\n"
        "  }\n"
        "}\n"
    )
    try:
        result = subprocess.run(
            [
                "gh",
                "api",
                "graphql",
                "-f",
                "query={0}".format(query),
                "-F",
                "owner={0}".format(owner),
                "-F",
                "repo={0}".format(repo),
                "-F",
                "pr={0}".format(pr_number),
            ],
            capture_output=True,
            text=True,
            check=True,
        )
        data = json.loads(result.stdout)
        threads = (
            data.get("data", {})
            .get("repository", {})
            .get("pullRequest", {})
            .get("reviewThreads", {})
            .get("nodes", [])
        )
        return threads
    except (subprocess.CalledProcessError, json.JSONDecodeError):
        return []


def detect_logaf(body):
    logaf_patterns = [
        (r"^\s*(?:h:|h\s*:|high:|\[h\])", "high"),
        (r"^\s*(?:m:|m\s*:|medium:|\[m\])", "medium"),
        (r"^\s*(?:l:|l\s*:|low:|\[l\])", "low"),
    ]
    for pattern, level in logaf_patterns:
        if re.search(pattern, body, re.IGNORECASE):
            return level
    return None


def categorize_comment(comment, body):
    author = comment.get("author", {}).get("login", "") or comment.get("user", {}).get(
        "login", ""
    )
    if is_info_bot(author) and not is_review_bot(author):
        return "bot"
    logaf_level = detect_logaf(body)
    if logaf_level:
        return logaf_level
    high_patterns = [
        r"(?i)must\s+(fix|change|update|address)",
        r"(?i)this\s+(is\s+)?(wrong|incorrect|broken|buggy)",
        r"(?i)security\s+(issue|vulnerability|concern)",
        r"(?i)will\s+(break|cause|fail)",
        r"(?i)critical",
        r"(?i)blocker",
    ]
    for pattern in high_patterns:
        if re.search(pattern, body):
            return "high"
    low_patterns = [
        r"(?i)nit[:\s]",
        r"(?i)nitpick",
        r"(?i)suggestion[:\s]",
        r"(?i)consider\s+",
        r"(?i)could\s+(also\s+)?",
        r"(?i)might\s+(want\s+to|be\s+better)",
        r"(?i)optional[:\s]",
        r"(?i)minor[:\s]",
        r"(?i)style[:\s]",
        r"(?i)prefer\s+",
        r"(?i)what\s+do\s+you\s+think",
        r"(?i)up\s+to\s+you",
        r"(?i)take\s+it\s+or\s+leave",
        r"(?i)fwiw",
    ]
    for pattern in low_patterns:
        if re.search(pattern, body):
            return "low"
    return "medium"


def extract_feedback_item(
    body,
    author,
    path=None,
    line=None,
    url=None,
    is_resolved=False,
    is_outdated=False,
    review_bot=False,
    thread_id=None,
):
    summary = body[:200] + "..." if len(body) > 200 else body
    summary = summary.replace("\n", " ").strip()
    item = {"author": author, "body": summary, "full_body": body}
    if path:
        item["path"] = path
    if line:
        item["line"] = line
    if url:
        item["url"] = url
    if is_resolved:
        item["resolved"] = True
    if is_outdated:
        item["outdated"] = True
    if review_bot:
        item["review_bot"] = True
    if thread_id:
        item["thread_id"] = thread_id
    return item


def main():
    parser = argparse.ArgumentParser(description="Fetch and categorize PR feedback")
    parser.add_argument(
        "--pr", type=int, help="PR number (defaults to current branch PR)"
    )
    args = parser.parse_args()

    preflight()

    repo_info = get_repo_info()
    if not repo_info:
        print(json.dumps({"error": "Could not determine repository"}))
        sys.exit(1)
    owner, repo = repo_info

    pr_info = get_pr_info(args.pr)
    if not pr_info:
        print(json.dumps({"error": "No PR found for current branch"}))
        sys.exit(1)

    pr_number = pr_info["number"]
    pr_author = pr_info.get("author", {}).get("login", "")
    review_decision = pr_info.get("reviewDecision", "")

    feedback = {"high": [], "medium": [], "low": [], "bot": [], "resolved": []}

    reviews = pr_info.get("reviews", [])
    for review in reviews:
        if review.get("state") == "CHANGES_REQUESTED":
            author = review.get("author", {}).get("login", "")
            body = review.get("body", "")
            if body and author != pr_author:
                item = extract_feedback_item(body, author)
                item["type"] = "changes_requested"
                feedback["high"].append(item)

    threads = get_review_threads(owner, repo, pr_number)
    seen_thread_ids = set()

    for thread in threads:
        if not thread.get("comments", {}).get("nodes"):
            continue
        first_comment = thread["comments"]["nodes"][0]
        author = first_comment.get("author", {}).get("login", "")
        body = first_comment.get("body", "")
        if author == pr_author:
            continue
        if not body or len(body.strip()) < 3:
            continue
        is_resolved = thread.get("isResolved", False)
        is_outdated = thread.get("isOutdated", False)
        thread_id = thread.get("id")
        item = extract_feedback_item(
            body=body,
            author=author,
            path=thread.get("path"),
            line=thread.get("line"),
            is_resolved=is_resolved,
            is_outdated=is_outdated,
            thread_id=thread_id,
        )
        if thread_id:
            seen_thread_ids.add(thread_id)
        if is_resolved:
            feedback["resolved"].append(item)
        elif is_review_bot(author):
            category = categorize_comment(first_comment, body)
            item["review_bot"] = True
            feedback[category].append(item)
        elif is_info_bot(author):
            feedback["bot"].append(item)
        else:
            category = categorize_comment(first_comment, body)
            feedback[category].append(item)

    issue_comments = get_issue_comments(owner, repo, pr_number)
    for comment in issue_comments:
        author = comment.get("user", {}).get("login", "")
        body = comment.get("body", "")
        if author == pr_author:
            continue
        if not body or len(body.strip()) < 3:
            continue
        item = extract_feedback_item(
            body=body, author=author, url=comment.get("html_url")
        )
        if is_review_bot(author):
            category = categorize_comment(comment, body)
            item["review_bot"] = True
            feedback[category].append(item)
        elif is_info_bot(author):
            feedback["bot"].append(item)
        else:
            category = categorize_comment(comment, body)
            feedback[category].append(item)

    review_bot_count = sum(
        1
        for bucket in ("high", "medium", "low")
        for item in feedback[bucket]
        if item.get("review_bot")
    )

    output = {
        "pr": {
            "number": pr_number,
            "url": pr_info.get("url", ""),
            "author": pr_author,
            "review_decision": review_decision,
        },
        "summary": {
            "high": len(feedback["high"]),
            "medium": len(feedback["medium"]),
            "low": len(feedback["low"]),
            "bot_comments": len(feedback["bot"]),
            "resolved": len(feedback["resolved"]),
            "review_bot_feedback": review_bot_count,
            "needs_attention": len(feedback["high"]) + len(feedback["medium"]),
        },
        "feedback": feedback,
    }

    if feedback["high"]:
        output["action_required"] = "Address high-priority feedback before merge"
    elif feedback["medium"]:
        output["action_required"] = "Address medium-priority feedback"
    elif feedback["low"]:
        output["action_required"] = (
            "Review low-priority suggestions - ask user which to address"
        )
    else:
        output["action_required"] = None

    print(json.dumps(output, indent=2))


if __name__ == "__main__":
    main()
