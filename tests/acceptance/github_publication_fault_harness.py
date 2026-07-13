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


def approved_body(operation_id: str, item_id: str) -> tuple[str, str]:
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
    body = "\n".join(
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
    lines = body.splitlines()
    assert lines[0] == LEVEL_MARKER
    assert lines[-2:] == [operation_marker, item_marker]
    return body, "sha256:" + hashlib.sha256(body.encode("utf-8")).hexdigest()


def validated_matches(
    observations: object, operation_id: str, item_id: str
) -> tuple[list[dict[str, str]], int]:
    if not isinstance(observations, list) or not observations:
        raise AssertionError("marker_observations must be a non-empty sequence")
    polls = 0
    final_matches: list[dict[str, str]] = []
    for raw_poll in observations:
        polls += 1
        if not isinstance(raw_poll, list):
            raise AssertionError("each marker observation must be a list")
        current: list[dict[str, str]] = []
        for match in raw_poll:
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
            current.append(match)
        final_matches = current
        if current:
            break
    return final_matches, polls


def classify_case(raw_case: object, operation_id: str) -> dict[str, object]:
    if not isinstance(raw_case, dict):
        raise AssertionError("case must be an object")
    required = {
        "name",
        "item_id",
        "request_result",
        "returned_url",
        "marker_observations",
        "relationships",
        "expected",
    }
    if set(raw_case) != required:
        raise AssertionError(f"case keys differ: {raw_case.get('name', '<unknown>')}")

    name = raw_case["name"]
    if not isinstance(name, str) or not name:
        raise AssertionError("case name must be non-empty")
    item_id = require_uuid(raw_case["item_id"], f"{name}.item_id")
    request_result = raw_case["request_result"]
    if request_result not in {"success", "timeout", "partial", "ambiguous"}:
        raise AssertionError(f"{name}: unsupported request result")

    body, body_sha256 = approved_body(operation_id, item_id)
    matches, polls = validated_matches(
        raw_case["marker_observations"], operation_id, item_id
    )
    returned_url = raw_case["returned_url"]
    if returned_url is not None and (
        not isinstance(returned_url, str)
        or not returned_url.startswith("https://github.com/")
    ):
        raise AssertionError(f"{name}: returned URL escaped github.com")

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
        "approved_body_sha256": body_sha256,
        "approved_marker_lines": [
            LEVEL_MARKER,
            f"{OPERATION_PREFIX}{operation_id} -->",
            f"{ITEM_PREFIX}{item_id} -->",
        ],
        "request_result": request_result,
        "create_attempts": 1,
        "blind_retry": False,
        "reconciliation_polls": polls,
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
    }:
        raise AssertionError("fixture top-level shape differs")
    if fixture["fixture_version"] != 1:
        raise AssertionError("unsupported fixture version")
    operation_id = require_uuid(fixture["operation_id"], "operation_id")
    if not isinstance(fixture["cases"], list) or not fixture["cases"]:
        raise AssertionError("fixture needs cases")
    cases = [classify_case(case, operation_id) for case in fixture["cases"]]
    observed = {case["item_status"] for case in cases}
    if observed != ALLOWED_STATUS:
        raise AssertionError(f"fixture must cover every status, observed {sorted(observed)}")
    return {
        "harness_version": 1,
        "evidence_class": "H/G-adjacent local fault harness; not live GitHub G",
        "network_used": False,
        "case_count": len(cases),
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
