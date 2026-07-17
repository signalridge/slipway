#!/usr/bin/env python3
"""Deterministic host-side GitHub publication reconciliation fault harness.

This performs no network access and is not live GitHub (G) evidence. It tests the
host publication policy around exact approved draft/final body digests, one
create and one confirmed final-body PATCH, ambiguous writes, partial relations,
index delay, sticky duplicate observations, and the rule that neither mutation
is blindly retried.
"""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import sys
import uuid

ALLOWED_STATUS = {"created", "matched", "failed", "ambiguous"}
LEVEL_MARKER = "<!-- slipway-level: change/v2 -->"
OPERATION_PREFIX = "<!-- slipway-publication-operation: "
ITEM_PREFIX = "<!-- slipway-publication-item: "


def require_uuid(value: object, field: str) -> str:
    if not isinstance(value, str):
        raise AssertionError(f"{field} must be a UUID string")
    parsed = uuid.UUID(value)
    if str(parsed) != value:
        raise AssertionError(f"{field} must use canonical lowercase UUID text")
    return value


def framed_revision(*fields: str) -> str:
    digest = hashlib.sha256()
    for field in fields:
        encoded = field.encode("utf-8")
        digest.update(len(encoded).to_bytes(8, byteorder="big"))
        digest.update(encoded)
    return "sha256:" + digest.hexdigest()


def approved_body(operation_id: str, item_id: str) -> tuple[str, str, str, str]:
    definitions = [
        ("outcome", "outcome", "Outcome", "Exercise deterministic publication reconciliation."),
        ("requirements", "requirements", "Requirements", "Never blindly retry an ambiguous create."),
        ("acceptance-examples", "acceptance_examples", "Acceptance examples", "Zero, one, and multiple marker matches are classified."),
        ("constraints", "constraints", "Constraints", "No network or credentials."),
        ("non-goals", "non_goals", "Non-goals", "This is not live GitHub evidence."),
    ]
    sections: list[dict[str, object]] = []
    for index, (key, role, title, text) in enumerate(definitions, start=1):
        comment_body = f"<!-- slipway-section:v1 key={key} -->\n# {title}\n\n{text}\n"
        assert LEVEL_MARKER not in comment_body
        sections.append(
            {
                "key": key,
                "role": role,
                "title": title,
                "comment_node_id": f"IC_{item_id}_{key}",
                "comment_database_id": index,
                "body_sha256": framed_revision("slipway-comment-body/v1", comment_body),
            }
        )
    manifest = {"manifest_version": 2, "profile": "change/v2", "sections": sections}
    operation_marker = f"{OPERATION_PREFIX}{operation_id} -->"
    item_marker = f"{ITEM_PREFIX}{item_id} -->"
    draft_body = operation_marker + "\n" + item_marker + "\n"
    assert draft_body.splitlines() == [operation_marker, item_marker]
    assert LEVEL_MARKER not in draft_body and "slipway-manifest" not in draft_body

    final_body = "\n".join(
        [
            LEVEL_MARKER,
            "",
            "```slipway-manifest",
            json.dumps(manifest, ensure_ascii=False, separators=(",", ":"), sort_keys=True),
            "```",
            "",
            operation_marker,
            item_marker,
            "",
        ]
    )
    lines = final_body.splitlines()
    manifest_end = lines.index("```")
    assert lines[0] == LEVEL_MARKER
    assert lines[manifest_end + 2 :] == [operation_marker, item_marker]
    return (
        draft_body,
        "sha256:" + hashlib.sha256(draft_body.encode("utf-8")).hexdigest(),
        final_body,
        "sha256:" + hashlib.sha256(final_body.encode("utf-8")).hexdigest(),
    )


def validated_marker_matches(
    raw_matches: object,
    operation_id: str,
    item_id: str,
    draft_sha256: str,
    final_sha256: str,
    repository_id: str,
    repository_url: str,
    observed_identities: dict[str, object],
) -> list[dict[str, object]]:
    if not isinstance(raw_matches, list):
        raise AssertionError("call-log matches must be a list")
    matches: list[dict[str, object]] = []
    current_urls: set[str] = set()
    current_issue_ids: set[str] = set()
    issue_url_prefix = repository_url + "/issues/"
    for match in raw_matches:
        if not isinstance(match, dict) or set(match) != {
            "operation_id",
            "item_id",
            "repository_id",
            "issue_id",
            "issue_number",
            "url",
            "body_sha256",
        }:
            raise AssertionError("marker match has an unexpected shape")
        if match["operation_id"] != operation_id or match["item_id"] != item_id:
            raise AssertionError("marker match escaped the approved operation/item")
        if match["repository_id"] != repository_id:
            raise AssertionError("marker match escaped the approved repository identity")
        issue_id = match["issue_id"]
        issue_number = match["issue_number"]
        url = match["url"]
        if not isinstance(issue_id, str) or not issue_id:
            raise AssertionError("marker match issue identity is invalid")
        if not isinstance(issue_number, int) or isinstance(issue_number, bool) or issue_number <= 0:
            raise AssertionError("marker match issue number is invalid")
        if not isinstance(url, str) or url != issue_url_prefix + str(issue_number):
            raise AssertionError("marker match URL differs from the approved repository/Issue identity")
        if url in current_urls or issue_id in current_issue_ids:
            raise AssertionError("marker readback repeats one Issue identity")
        current_urls.add(url)
        current_issue_ids.add(issue_id)
        identity = (repository_id, issue_id, issue_number, url)
        prior_for_url = observed_identities.setdefault("url:" + url, identity)
        prior_for_id = observed_identities.setdefault("id:" + issue_id, identity)
        if prior_for_url != identity or prior_for_id != identity:
            raise AssertionError("marker match Issue identity changed between readbacks")
        if match["body_sha256"] not in {draft_sha256, final_sha256}:
            raise AssertionError("marker match body differs from both approved publication phases")
        matches.append(match)
    return matches


def validated_matches(
    call_log: object,
    operation_id: str,
    item_id: str,
    draft_sha256: str,
    final_sha256: str,
    repository_id: str,
    repository_url: str,
) -> dict[str, object]:
    if not isinstance(call_log, list) or not call_log:
        raise AssertionError("call_log must be a non-empty sequence")
    sequences = [
        entry.get("sequence") if isinstance(entry, dict) else None for entry in call_log
    ]
    if any(
        not isinstance(sequence, int) or isinstance(sequence, bool) or sequence <= 0
        for sequence in sequences
    ) or sequences != sorted(sequences):
        raise AssertionError("call_log sequence values must be positive and strictly increasing")
    if len(set(sequences)) != len(sequences):
        raise AssertionError("call_log sequence values must be unique")
    create_entries = [
        entry
        for entry in call_log
        if isinstance(entry, dict) and entry.get("operation") == "create"
    ]
    create_attempts = len(create_entries)
    blind_retry = create_attempts > 1
    if create_attempts != 1:
        raise AssertionError(
            f"create attempts={create_attempts} (blind_retry={str(blind_retry).lower()}); "
            "publication create must execute exactly once"
        )
    patch_entries = [
        entry
        for entry in call_log
        if isinstance(entry, dict) and entry.get("operation") == "patch_final"
    ]
    if len(patch_entries) > 1:
        raise AssertionError(
            f"final patch attempts={len(patch_entries)} (blind_retry=true); "
            "the confirmed final-body mutation must not be blindly retried"
        )
    if not isinstance(call_log[0], dict) or call_log[0].get("operation") != "create":
        raise AssertionError("call_log must begin with create")
    create = call_log[0]
    if set(create) != {
        "sequence",
        "operation",
        "body_phase",
        "body_sha256",
        "result",
        "returned_url",
    }:
        raise AssertionError("create call has an unexpected shape")
    if create["body_phase"] != "draft" or create["body_sha256"] != draft_sha256:
        raise AssertionError("create must use the exact approved receipt-only Change draft body")
    request_result = create["result"]
    if request_result not in {"success", "timeout", "partial", "ambiguous"}:
        raise AssertionError("create call has an unsupported result")
    returned_url = create["returned_url"]
    issue_url_prefix = repository_url + "/issues/"
    if returned_url is not None and (
        not isinstance(returned_url, str)
        or not returned_url.startswith(issue_url_prefix)
        or not returned_url.removeprefix(issue_url_prefix).isdigit()
        or int(returned_url.removeprefix(issue_url_prefix)) <= 0
    ):
        raise AssertionError("create returned URL escaped the approved repository")

    polls = 0
    first_match_poll: int | None = None
    latest_poll_matches: list[dict[str, object]] | None = None
    final_matches: list[dict[str, object]] | None = None
    final_patch: dict[str, object] | None = None
    observed_identities: dict[str, object] = {}
    observed_issue_ids: set[str] = set()
    saw_multiple_matches = False
    for index, entry in enumerate(call_log[1:], start=1):
        if not isinstance(entry, dict):
            raise AssertionError("call_log entries must be objects")
        operation = entry.get("operation")
        if operation == "poll":
            if final_patch is not None:
                raise AssertionError("poll cannot follow the final-body patch")
            if final_matches is not None:
                raise AssertionError("poll cannot follow final_readback")
            if set(entry) != {"sequence", "operation", "matches"}:
                raise AssertionError("poll call has an unexpected shape")
            polls += 1
            current = validated_marker_matches(
                entry["matches"],
                operation_id,
                item_id,
                draft_sha256,
                final_sha256,
                repository_id,
                repository_url,
                observed_identities,
            )
            if any(match["body_sha256"] != draft_sha256 for match in current):
                raise AssertionError("pre-patch reconciliation observed a non-draft body")
            observed_issue_ids.update(str(match["issue_id"]) for match in current)
            latest_poll_matches = current
            saw_multiple_matches = (
                saw_multiple_matches or len(current) > 1 or len(observed_issue_ids) > 1
            )
            if current and first_match_poll is None:
                first_match_poll = polls
            continue
        if operation == "patch_final":
            if set(entry) != {"sequence", "operation", "target_url", "body_sha256", "result"}:
                raise AssertionError("final patch call has an unexpected shape")
            if final_patch is not None:
                raise AssertionError("final patch attempts=2 (blind_retry=true)")
            if final_matches is not None:
                raise AssertionError("final patch cannot follow final_readback")
            if latest_poll_matches is None or len(latest_poll_matches) != 1:
                raise AssertionError("final patch requires one exact reconciled draft target")
            if saw_multiple_matches:
                raise AssertionError("final patch is forbidden after multiple marker matches")
            target_url = entry["target_url"]
            if target_url != latest_poll_matches[0]["url"]:
                raise AssertionError("final patch target differs from the reconciled draft")
            if entry["body_sha256"] != final_sha256:
                raise AssertionError("final patch body differs from the approved manifested Change")
            if entry["result"] not in {"success", "timeout", "partial", "ambiguous", "error"}:
                raise AssertionError("final patch has an unsupported result")
            final_patch = entry
            continue
        if operation == "final_readback":
            if set(entry) != {"sequence", "operation", "matches"}:
                raise AssertionError("final_readback has an unexpected shape")
            if index != len(call_log) - 1 or final_matches is not None:
                raise AssertionError("call_log must end with exactly one final_readback")
            final_matches = validated_marker_matches(
                entry["matches"],
                operation_id,
                item_id,
                draft_sha256,
                final_sha256,
                repository_id,
                repository_url,
                observed_identities,
            )
            if final_patch is None and any(
                match["body_sha256"] == final_sha256 for match in final_matches
            ):
                raise AssertionError("final body appeared without the confirmed final-body patch")
            observed_issue_ids.update(str(match["issue_id"]) for match in final_matches)
            saw_multiple_matches = (
                saw_multiple_matches
                or len(final_matches) > 1
                or len(observed_issue_ids) > 1
            )
            continue
        raise AssertionError(f"unsupported call_log operation: {operation!r}")
    if polls == 0:
        raise AssertionError("call_log must include at least one reconciliation poll")
    if final_matches is None:
        raise AssertionError("call_log must end with final_readback convergence evidence")
    if len(final_matches) == 1 and not saw_multiple_matches and final_patch is None:
        raise AssertionError("one reconciled draft requires exactly one confirmed final-body patch")
    return {
        "request_result": request_result,
        "returned_url": returned_url,
        "create_attempts": create_attempts,
        "blind_retry": blind_retry,
        "polls": polls,
        "first_match_poll": first_match_poll,
        "final_matches": final_matches,
        "final_patch_attempts": len(patch_entries),
        "final_patch_result": None if final_patch is None else final_patch["result"],
        "final_patch_target": None if final_patch is None else final_patch["target_url"],
        "saw_multiple_matches": saw_multiple_matches,
    }


def classify_relationship(raw: object, case_name: str) -> dict[str, object]:
    if not isinstance(raw, dict) or set(raw) != {"name", "call_log"}:
        raise AssertionError(f"{case_name}: relationship shape differs")
    name = raw["name"]
    call_log = raw["call_log"]
    if not isinstance(name, str) or not name:
        raise AssertionError(f"{case_name}: relationship name must be non-empty")
    if not isinstance(call_log, list) or len(call_log) < 2:
        raise AssertionError(f"{case_name}.{name}: relationship call_log is incomplete")
    sequences = [
        entry.get("sequence") if isinstance(entry, dict) else None for entry in call_log
    ]
    if any(
        not isinstance(sequence, int) or isinstance(sequence, bool) or sequence <= 0
        for sequence in sequences
    ) or sequences != sorted(sequences) or len(set(sequences)) != len(sequences):
        raise AssertionError(f"{case_name}.{name}: sequence values must be unique and increasing")

    mutation_entries = [
        entry
        for entry in call_log
        if isinstance(entry, dict) and entry.get("operation") == "mutate"
    ]
    if len(mutation_entries) != 1 or call_log[0] is not mutation_entries[0]:
        raise AssertionError(f"{case_name}.{name}: relationship mutation must execute exactly once and first")
    mutation = mutation_entries[0]
    if set(mutation) != {"sequence", "operation", "result", "returned_id"}:
        raise AssertionError(f"{case_name}.{name}: relationship mutation shape differs")
    if mutation["result"] not in {"success", "timeout", "error", "ambiguous"}:
        raise AssertionError(f"{case_name}.{name}: unsupported relationship mutation result")
    returned_id = mutation["returned_id"]
    if returned_id is not None and (not isinstance(returned_id, str) or not returned_id):
        raise AssertionError(f"{case_name}.{name}: returned relationship id is invalid")

    readbacks = call_log[1:]
    if any(
        not isinstance(entry, dict)
        or set(entry) != {"sequence", "operation", "matches"}
        or entry.get("operation") != "readback"
        or not isinstance(entry.get("matches"), list)
        or any(not isinstance(match, str) or not match for match in entry["matches"])
        for entry in readbacks
    ):
        raise AssertionError(f"{case_name}.{name}: relationship readback shape differs")
    matches = readbacks[-1]["matches"]
    observed_relationship_ids = {
        match for entry in readbacks for match in entry["matches"]
    }
    multiple_observed = len(observed_relationship_ids) > 1
    if multiple_observed:
        status = "ambiguous"
    elif len(matches) == 1:
        if returned_id is not None and returned_id != matches[0]:
            status = "ambiguous"
        elif mutation["result"] == "success" and returned_id is not None:
            status = "created"
        else:
            status = "matched"
    else:
        status = "failed"

    return {
        "name": name,
        "status": status,
        "mutation_attempts": len(mutation_entries),
        "readbacks": len(readbacks),
        "historical_multiple_matches": multiple_observed,
    }


def classify_case(
    raw_case: object, operation_id: str, repository_id: str, repository_url: str
) -> dict[str, object]:
    if not isinstance(raw_case, dict):
        raise AssertionError("case must be an object")
    required = {
        "name",
        "item_id",
        "call_log",
        "relationships",
        "expected",
    }
    if set(raw_case) != required:
        raise AssertionError(f"case keys differ: {raw_case.get('name', '<unknown>')}")

    name = raw_case["name"]
    if not isinstance(name, str) or not name:
        raise AssertionError("case name must be non-empty")
    item_id = require_uuid(raw_case["item_id"], f"{name}.item_id")
    draft_body, draft_sha256, final_body, final_sha256 = approved_body(operation_id, item_id)
    trace = validated_matches(
        raw_case["call_log"],
        operation_id,
        item_id,
        draft_sha256,
        final_sha256,
        repository_id,
        repository_url,
    )
    request_result = trace["request_result"]
    returned_url = trace["returned_url"]
    matches = trace["final_matches"]
    assert isinstance(matches, list)

    saw_multiple_matches = bool(trace["saw_multiple_matches"])
    if saw_multiple_matches or len(matches) > 1:
        item_status = "ambiguous"
        canonical_url = None
        requires_new_confirmation = False
    elif len(matches) == 1:
        canonical_url = matches[0]["url"]
        patch_target = trace["final_patch_target"]
        if returned_url is not None and returned_url != canonical_url:
            item_status = "ambiguous"
            canonical_url = None
            requires_new_confirmation = False
        elif patch_target is not None and patch_target != canonical_url:
            item_status = "ambiguous"
            canonical_url = None
            requires_new_confirmation = False
        elif matches[0]["body_sha256"] != final_sha256:
            item_status = "failed"
            requires_new_confirmation = True
        elif (
            request_result in {"success", "partial"}
            and returned_url is not None
            and trace["final_patch_result"] == "success"
        ):
            item_status = "created"
            requires_new_confirmation = False
        else:
            item_status = "matched"
            requires_new_confirmation = False
    else:
        item_status = "failed"
        canonical_url = None
        requires_new_confirmation = True

    relationships = raw_case["relationships"]
    if not isinstance(relationships, list):
        raise AssertionError(f"{name}: relationships must be a list")
    relation_results = [classify_relationship(relation, name) for relation in relationships]

    ordered_entries = list(raw_case["call_log"])
    for relationship in relationships:
        ordered_entries.extend(relationship["call_log"])
    sequence_values = [entry["sequence"] for entry in ordered_entries]
    if sorted(sequence_values) != list(range(1, len(sequence_values) + 1)):
        raise AssertionError(f"{name}: global call sequence must be complete and unique")
    mutation_entries = [
        entry
        for entry in ordered_entries
        if entry["operation"] in {"create", "mutate", "patch_final"}
    ]
    final_patch_entries = [
        entry for entry in mutation_entries if entry["operation"] == "patch_final"
    ]
    final_patch_last_mutation: bool | None = None
    if final_patch_entries:
        final_patch_last_mutation = final_patch_entries[0]["sequence"] == max(
            entry["sequence"] for entry in mutation_entries
        )
        if not final_patch_last_mutation:
            raise AssertionError(f"{name}: final-body PATCH was not the last modeled mutation")

    result: dict[str, object] = {
        "name": name,
        "operation_id": operation_id,
        "item_id": item_id,
        "approved_draft_body_sha256": draft_sha256,
        "approved_final_body_sha256": final_sha256,
        "draft_marker_lines": draft_body.splitlines(),
        "final_marker_lines": [
            LEVEL_MARKER,
            f"{OPERATION_PREFIX}{operation_id} -->",
            f"{ITEM_PREFIX}{item_id} -->",
        ],
        "final_markers_follow_manifest": final_body.index("```") < final_body.index(OPERATION_PREFIX),
        "request_result": request_result,
        "create_attempts": trace["create_attempts"],
        "final_patch_attempts": trace["final_patch_attempts"],
        "final_patch_result": trace["final_patch_result"],
        "final_patch_last_modeled_mutation": final_patch_last_mutation,
        "blind_retry": trace["blind_retry"],
        "reconciliation_polls": trace["polls"],
        "first_match_poll": trace["first_match_poll"],
        "historical_multiple_matches": saw_multiple_matches,
        "convergence_readback": True,
        "marker_match_count": len(matches),
        "item_status": item_status,
        "canonical_url": canonical_url,
        "relationships": relation_results,
        "requires_new_confirmation": requires_new_confirmation,
    }

    expected = raw_case["expected"]
    if not isinstance(expected, dict) or set(expected) != {
        "item_status",
        "relationship_statuses",
        "requires_new_confirmation",
    }:
        raise AssertionError(f"{name}: expected shape differs")
    actual_relation_statuses = [relation["status"] for relation in relation_results]
    assert item_status == expected["item_status"], (name, result, expected)
    assert actual_relation_statuses == expected["relationship_statuses"], (
        name,
        result,
        expected,
    )
    assert requires_new_confirmation is expected["requires_new_confirmation"], (
        name,
        result,
        expected,
    )
    assert result["create_attempts"] == 1 and result["blind_retry"] is False
    assert trace["final_patch_attempts"] in {0, 1}
    assert item_status in ALLOWED_STATUS
    return result


def run(fixture_path: Path) -> dict[str, object]:
    with fixture_path.open("r", encoding="utf-8") as stream:
        fixture = json.load(stream)
    if not isinstance(fixture, dict) or set(fixture) != {
        "fixture_version",
        "operation_id",
        "repository_id",
        "repository_url",
        "cases",
        "rejected_cases",
    }:
        raise AssertionError("fixture top-level shape differs")
    if fixture["fixture_version"] != 7:
        raise AssertionError("unsupported fixture version")
    operation_id = require_uuid(fixture["operation_id"], "operation_id")
    repository_id = fixture["repository_id"]
    repository_url = fixture["repository_url"]
    if not isinstance(repository_id, str) or not repository_id:
        raise AssertionError("repository_id must be non-empty")
    if not isinstance(repository_url, str) or not repository_url.startswith("https://github.com/"):
        raise AssertionError("repository_url must identify a github.com repository")
    repository_path = repository_url.removeprefix("https://github.com/").split("/")
    if len(repository_path) != 2 or not all(repository_path):
        raise AssertionError("repository_url must contain exactly one owner/repository path")
    if not isinstance(fixture["cases"], list) or not fixture["cases"]:
        raise AssertionError("fixture needs cases")
    cases = [
        classify_case(case, operation_id, repository_id, repository_url)
        for case in fixture["cases"]
    ]
    observed = {case["item_status"] for case in cases}
    if observed != ALLOWED_STATUS:
        raise AssertionError(f"fixture must cover every status, observed {sorted(observed)}")

    rejected_cases = fixture["rejected_cases"]
    if not isinstance(rejected_cases, list) or not rejected_cases:
        raise AssertionError("fixture needs rejected_cases")
    rejected_names: list[str] = []
    for rejected in rejected_cases:
        if not isinstance(rejected, dict) or set(rejected) != {
            "name",
            "item_id",
            "call_log",
            "expected_error",
        }:
            raise AssertionError("rejected case shape differs")
        name = rejected["name"]
        item_id = require_uuid(rejected["item_id"], f"{name}.item_id")
        _, draft_sha256, _, final_sha256 = approved_body(operation_id, item_id)
        expected_error = rejected["expected_error"]
        try:
            validated_matches(
                rejected["call_log"],
                operation_id,
                item_id,
                draft_sha256,
                final_sha256,
                repository_id,
                repository_url,
            )
        except AssertionError as error:
            if not isinstance(expected_error, str) or expected_error not in str(error):
                raise AssertionError(
                    f"{name}: got rejection {error!s}, want substring {expected_error!r}"
                ) from error
        else:
            raise AssertionError(f"{name}: rejected call log unexpectedly passed")
        rejected_names.append(str(name))

    return {
        "harness_version": 7,
        "evidence_class": "H/G-adjacent local fault harness; not live GitHub G",
        "network_used": False,
        "repository_id": repository_id,
        "repository_url": repository_url,
        "case_count": len(cases),
        "rejected_case_count": len(rejected_names),
        "rejected_cases": rejected_names,
        "statuses_covered": sorted(observed),
        "cases": cases,
    }


def main() -> int:
    default_fixture = Path(__file__).with_name("fixtures") / "github-publication-faults.json"
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixture", type=Path, default=default_fixture)
    args = parser.parse_args()
    try:
        report = run(args.fixture)
    except (AssertionError, OSError, ValueError, json.JSONDecodeError) as error:
        print(f"github publication fault harness failed: {error}", file=sys.stderr)
        return 1
    json.dump(report, sys.stdout, ensure_ascii=False, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
