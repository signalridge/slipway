#!/usr/bin/env python3
"""Deterministic host-side GitHub publication reconciliation fault harness.

This performs no network access and is not live GitHub (G) evidence. It tests the
host publication policy around approved typed markers, ambiguous writes, partial
relations, index delay, and the rule that create is never blindly retried.
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
    raw_matches: object, operation_id: str, item_id: str
) -> list[dict[str, str]]:
    if not isinstance(raw_matches, list):
        raise AssertionError("call-log matches must be a list")
    matches: list[dict[str, str]] = []
    for match in raw_matches:
        if not isinstance(match, dict) or set(match) != {
            "operation_id",
            "item_id",
            "url",
        }:
            raise AssertionError("marker match has an unexpected shape")
        if match["operation_id"] != operation_id or match["item_id"] != item_id:
            raise AssertionError("marker match escaped the approved operation/item")
        url = match["url"]
        if not isinstance(url, str) or not url.startswith("https://github.com/"):
            raise AssertionError("fixture URL must remain on github.com")
        matches.append(match)
    return matches


def validated_matches(
    call_log: object, operation_id: str, item_id: str
) -> dict[str, object]:
    if not isinstance(call_log, list) or not call_log:
        raise AssertionError("call_log must be a non-empty sequence")
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
    if not isinstance(call_log[0], dict) or call_log[0].get("operation") != "create":
        raise AssertionError("call_log must begin with create")
    create = call_log[0]
    if set(create) != {"operation", "body_phase", "result", "returned_url"}:
        raise AssertionError("create call has an unexpected shape")
    if create["body_phase"] != "draft":
        raise AssertionError("create must use the receipt-only Change draft body")
    request_result = create["result"]
    if request_result not in {"success", "timeout", "partial", "ambiguous"}:
        raise AssertionError("create call has an unsupported result")
    returned_url = create["returned_url"]
    if returned_url is not None and (
        not isinstance(returned_url, str)
        or not returned_url.startswith("https://github.com/")
    ):
        raise AssertionError("create returned URL escaped github.com")

    polls = 0
    first_match_poll: int | None = None
    final_matches: list[dict[str, str]] | None = None
    for index, entry in enumerate(call_log[1:], start=1):
        if not isinstance(entry, dict):
            raise AssertionError("call_log entries must be objects")
        operation = entry.get("operation")
        if operation == "poll":
            if set(entry) != {"operation", "matches"}:
                raise AssertionError("poll call has an unexpected shape")
            if final_matches is not None:
                raise AssertionError("poll cannot follow final_readback")
            polls += 1
            current = validated_marker_matches(entry["matches"], operation_id, item_id)
            if current and first_match_poll is None:
                first_match_poll = polls
            continue
        if operation == "final_readback":
            if set(entry) != {"operation", "body_phase", "matches"}:
                raise AssertionError("final_readback has an unexpected shape")
            if entry["body_phase"] != "final":
                raise AssertionError("final_readback must observe the final manifested Change body")
            if index != len(call_log) - 1 or final_matches is not None:
                raise AssertionError("call_log must end with exactly one final_readback")
            final_matches = validated_marker_matches(entry["matches"], operation_id, item_id)
            continue
        raise AssertionError(f"unsupported call_log operation: {operation!r}")
    if polls == 0:
        raise AssertionError("call_log must include at least one reconciliation poll")
    if final_matches is None:
        raise AssertionError("call_log must end with final_readback convergence evidence")
    return {
        "request_result": request_result,
        "returned_url": returned_url,
        "create_attempts": create_attempts,
        "blind_retry": blind_retry,
        "polls": polls,
        "first_match_poll": first_match_poll,
        "final_matches": final_matches,
    }


def classify_case(raw_case: object, operation_id: str) -> dict[str, object]:
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
    trace = validated_matches(raw_case["call_log"], operation_id, item_id)
    request_result = trace["request_result"]
    returned_url = trace["returned_url"]
    matches = trace["final_matches"]
    assert isinstance(matches, list)

    if len(matches) > 1:
        item_status = "ambiguous"
        canonical_url = None
        requires_new_confirmation = False
    elif len(matches) == 1:
        canonical_url = matches[0]["url"]
        if returned_url is not None and returned_url != canonical_url:
            item_status = "ambiguous"
            canonical_url = None
        elif request_result in {"success", "partial"} and returned_url is not None:
            item_status = "created"
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
    relation_results: list[dict[str, str]] = []
    for relation in relationships:
        if not isinstance(relation, dict) or set(relation) != {"name", "status"}:
            raise AssertionError(f"{name}: relationship shape differs")
        if relation["status"] not in ALLOWED_STATUS:
            raise AssertionError(f"{name}: invalid relationship status")
        relation_results.append(
            {"name": str(relation["name"]), "status": str(relation["status"])}
        )

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
        "blind_retry": trace["blind_retry"],
        "reconciliation_polls": trace["polls"],
        "first_match_poll": trace["first_match_poll"],
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
    assert item_status in ALLOWED_STATUS
    return result


def run(fixture_path: Path) -> dict[str, object]:
    with fixture_path.open("r", encoding="utf-8") as stream:
        fixture = json.load(stream)
    if not isinstance(fixture, dict) or set(fixture) != {
        "fixture_version",
        "operation_id",
        "cases",
        "rejected_cases",
    }:
        raise AssertionError("fixture top-level shape differs")
    if fixture["fixture_version"] != 2:
        raise AssertionError("unsupported fixture version")
    operation_id = require_uuid(fixture["operation_id"], "operation_id")
    if not isinstance(fixture["cases"], list) or not fixture["cases"]:
        raise AssertionError("fixture needs cases")
    cases = [classify_case(case, operation_id) for case in fixture["cases"]]
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
        expected_error = rejected["expected_error"]
        try:
            validated_matches(rejected["call_log"], operation_id, item_id)
        except AssertionError as error:
            if not isinstance(expected_error, str) or expected_error not in str(error):
                raise AssertionError(
                    f"{name}: got rejection {error!s}, want substring {expected_error!r}"
                ) from error
        else:
            raise AssertionError(f"{name}: rejected call log unexpectedly passed")
        rejected_names.append(str(name))

    return {
        "harness_version": 2,
        "evidence_class": "H/G-adjacent local fault harness; not live GitHub G",
        "network_used": False,
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
