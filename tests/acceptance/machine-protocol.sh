#!/bin/sh
set -eu

umask 077

fail() {
  printf 'machine-protocol acceptance failed: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

require_command git
require_command python3

BIN=${SLIPWAY_BIN:-./slipway}
case "$BIN" in
  /*) ;;
  *) BIN=$(cd "$(dirname "$BIN")" && pwd)/$(basename "$BIN") ;;
esac
[ -x "$BIN" ] || fail "SLIPWAY_BIN is not executable: $BIN"

TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/slipway-machine-acceptance.XXXXXX")
cleanup() { rm -rf "$TMP_ROOT"; }
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
ERROR_STDOUT="$TMP_ROOT/error.stdout"
ERROR_STDERR="$TMP_ROOT/error.stderr"

new_repository() {
  repository=$1
  mkdir -p "$repository"
  git -C "$repository" init -q
  git -C "$repository" config user.email acceptance@example.invalid
  git -C "$repository" config user.name 'Slipway Acceptance'
  printf '# Acceptance repository\n' > "$repository/README.md"
  git -C "$repository" add README.md
  git -C "$repository" commit -qm initial
}

json_get() {
  python3 -I - "$1" "$2" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    value = json.load(stream)
for component in sys.argv[2].split("."):
    value = value[int(component)] if component.isdigit() else value[component]
if isinstance(value, bool):
    print("true" if value else "false")
elif value is None:
    print("")
else:
    print(value)
PY
}

assert_action() {
  python3 -I - "$1" "$2" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    envelope = json.load(stream)
required = {"contract_version", "run_id", "state", "action", "next"}
allowed = required | {
    "pause_reason", "summary", "suggested_actions", "pinned_source",
    "source_candidate", "resume_operation", "budget_applied",
}
assert required.issubset(envelope) and set(envelope).issubset(allowed), envelope
assert envelope["contract_version"] == 2, envelope
assert envelope["state"] == "active", envelope
assert envelope["next"]["operation"] == "action", envelope
data = envelope["action"]
expected_keys = {"contract_version", "run_id", "action_id", "kind", "goal", "brief", "context", "remaining_budget"}
assert set(data) == expected_keys, {"expected": sorted(expected_keys), "actual": sorted(data)}
assert type(data["contract_version"]) is int and data["contract_version"] == 2, data
assert isinstance(data["run_id"], str) and data["run_id"], data
assert envelope["run_id"] == data["run_id"], envelope
assert isinstance(data["action_id"], str) and data["action_id"], data
assert isinstance(data["kind"], str) and data["kind"] == sys.argv[2], data
assert isinstance(data["goal"], str) and data["goal"], data
assert isinstance(data["brief"], str) and data["brief"], data
assert isinstance(data["context"], str), data
assert type(data["remaining_budget"]) is int and data["remaining_budget"] >= 0, data
PY
}

assert_issue_action() {
  python3 -I - "$1" "$2" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    envelope = json.load(stream)
required = {"contract_version", "run_id", "state", "action", "next", "pinned_source"}
allowed = required | {
    "pause_reason", "summary", "suggested_actions", "source_candidate",
    "resume_operation", "budget_applied",
}
assert required.issubset(envelope) and set(envelope).issubset(allowed), envelope
assert envelope["contract_version"] == 2 and envelope["state"] == "active", envelope
assert envelope["next"]["operation"] == "action", envelope
assert isinstance(envelope["pinned_source"], dict), envelope
data = envelope["action"]
expected_keys = {
    "contract_version", "run_id", "action_id", "kind", "goal", "brief",
    "context", "source", "requirements", "remaining_budget",
}
assert set(data) == expected_keys, {"expected": sorted(expected_keys), "actual": sorted(data)}
assert data["contract_version"] == 2, data
assert envelope["run_id"] == data["run_id"], envelope
assert data["kind"] == sys.argv[2], data
assert set(data["source"]) == {
    "kind", "canonical_url", "issue_id", "source_revision", "manifest_revision",
    "requirements_revision",
}, data
assert data["source"]["kind"] == "change_issue", data
requirements = data["requirements"]
assert set(requirements) == {
    "requirements_revision", "sections", "required_for_action", "reader",
}, data
assert requirements["requirements_revision"] == data["source"]["requirements_revision"], data
keys = [section["key"] for section in requirements["sections"]]
assert requirements["required_for_action"] == keys, data
assert requirements["reader"]["operation"] == "read_material", data
assert requirements["reader"]["input"]["choices"] == keys, data
assert requirements["reader"]["base_argv"][:3] == ["slipway", "_machine", "material"], data
PY
}

assert_authorized_action() {
  python3 -I - "$1" "$2" "$3" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    envelope = json.load(stream)
required = {"contract_version", "run_id", "state", "action", "next"}
allowed = required | {
    "pause_reason", "summary", "suggested_actions", "pinned_source",
    "source_candidate", "resume_operation", "budget_applied",
}
assert required.issubset(envelope) and set(envelope).issubset(allowed), envelope
assert envelope["contract_version"] == 2 and envelope["state"] == "active", envelope
assert envelope["next"]["operation"] == "action", envelope
data = envelope["action"]
expected = {
    "contract_version", "run_id", "action_id", "kind", "goal", "brief",
    "context", "destructive_authorization", "remaining_budget",
}
assert set(data) == expected, data
assert envelope["run_id"] == data["run_id"], envelope
assert data["kind"] == "implement", data
authorization = data["destructive_authorization"]
assert set(authorization) == {
    "request_id", "originating_action_id", "scope_version", "scope_sha256",
    "targets", "impact", "confirmed_at",
}, authorization
assert authorization["originating_action_id"] == sys.argv[2], authorization
assert authorization["scope_sha256"] == sys.argv[3], authorization
assert authorization["scope_version"] == 1, authorization
assert isinstance(authorization["targets"], list) and authorization["targets"], authorization
PY
}

assert_state() {
  python3 -I - "$1" "$2" "${3:-}" "${4:-}" <<'PY'
import json
import os
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
required = {"contract_version", "run_id", "state", "next"}
allowed = required | {
    "pause_reason", "summary", "action", "suggested_actions",
    "pinned_source", "source_candidate", "resume_operation", "budget_applied",
}
assert required.issubset(data) and set(data).issubset(allowed), data
assert type(data["contract_version"]) is int and data["contract_version"] == 2, data
assert isinstance(data["run_id"], str) and data["run_id"], data
assert isinstance(data["state"], str) and data["state"] == sys.argv[2], data
if sys.argv[3]:
    assert data.get("pause_reason") == sys.argv[3], data
else:
    assert "pause_reason" not in data, data
next_value = data["next"]
assert set(next_value) == {"operation", "workspace_identity", "variants"}, next_value
assert next_value["workspace_identity"].startswith("sha256:") and len(next_value["workspace_identity"]) == 71, next_value
assert all(character in "0123456789abcdef" for character in next_value["workspace_identity"][7:]), next_value
assert isinstance(next_value["variants"], list), next_value
ids = []
workspace_roots = set()
for variant in next_value["variants"]:
    assert set(variant) == {"id", "base_argv", "inputs"}, variant
    assert isinstance(variant["id"], str) and variant["id"], variant
    assert isinstance(variant["base_argv"], list) and variant["base_argv"], variant
    assert isinstance(variant["inputs"], list), variant
    assert variant["base_argv"][0] == "slipway", variant
    roots = [index for index, value in enumerate(variant["base_argv"]) if value == "--root"]
    assert len(roots) == 1 and roots[0] + 1 < len(variant["base_argv"]), variant
    workspace_root = variant["base_argv"][roots[0] + 1]
    assert os.path.isabs(workspace_root), variant
    assert workspace_root != next_value["workspace_identity"], variant
    workspace_roots.add(workspace_root)
    serialized_argv = json.dumps(variant["base_argv"])
    assert "<answer>" not in serialized_argv and "<file>" not in serialized_argv and '"FILE"' not in serialized_argv, variant
    for input_value in variant["inputs"]:
        assert set(input_value).issubset({"name", "type", "flag", "required", "choices"}), input_value
        assert {"name", "type", "flag", "required"}.issubset(input_value), input_value
        assert input_value["type"] in {"string", "path", "enum", "digest"}, input_value
        if input_value["type"] == "enum":
            assert isinstance(input_value.get("choices"), list) and input_value["choices"], input_value
    ids.append(variant["id"])
if next_value["variants"]:
    assert len(workspace_roots) == 1, workspace_roots
if data["state"] == "ended":
    assert next_value["operation"] == "none" and next_value["variants"] == [], next_value
elif sys.argv[4]:
    assert sys.argv[4] in ids, {"expected": sys.argv[4], "actual": ids}
if "summary" in data:
    assert isinstance(data["summary"], str) and data["summary"], data
if data["state"] == "ended":
    assert isinstance(data.get("summary"), str) and data["summary"], data
if "suggested_actions" in data:
    assert isinstance(data["suggested_actions"], list), data
    for suggestion in data["suggested_actions"]:
        assert set(suggestion) == {"kind", "brief"}, suggestion
        assert isinstance(suggestion["kind"], str) and isinstance(suggestion["brief"], str) and suggestion["brief"], suggestion
if "action" in data:
    assert isinstance(data["action"], dict), data
    assert data["action"]["run_id"] == data["run_id"], data
if "pinned_source" in data:
    assert isinstance(data["pinned_source"], dict), data
if "source_candidate" in data:
    assert isinstance(data["source_candidate"], dict), data
if "resume_operation" in data:
    assert isinstance(data["resume_operation"], str) and data["resume_operation"], data
if "budget_applied" in data:
    assert type(data["budget_applied"]) is bool, data
PY
}

make_outcome() {
  python3 -I - "$1" "$2" "$3" "$4" "$5" "$6" <<'PY'
import json
import sys

path, action_id, action_kind, status, summary, encoded_extra = sys.argv[1:]
extra = json.loads(encoded_extra)
data = {
    "contract_version": 2,
    "action_id": action_id,
    "action_kind": action_kind,
    "status": status,
    "summary": summary,
    "observations": extra.pop("observations", []),
    "known_issues": extra.pop("known_issues", []),
    "suggested_actions": extra.pop("suggested_actions", []),
    "pause": None,
    "implementation": None,
    "review": None,
}
pause_reason = extra.pop("pause_reason", None)
if pause_reason is not None:
    data["pause"] = {
        "reason": pause_reason,
        "question": extra.pop("question", "One user decision is required."),
        "destructive_request": extra.pop("destructive_request", None),
    }
implementation_result = extra.pop("implementation_result", None)
if implementation_result is not None:
    data["implementation"] = {
        "result": implementation_result,
        "files_changed": extra.pop("files_changed", []),
        "activities": extra.pop("activities", []),
        "uncertainties": extra.pop("uncertainties", []),
        "attempts": extra.pop("attempts", 1),
    }
review_result = extra.pop("review_result", None)
if review_result is not None:
    data["review"] = {
        "result": review_result,
        "findings": extra.pop("findings", []),
        "uncertainties": extra.pop("uncertainties", []),
    }
assert not extra, extra
with open(path, "w", encoding="utf-8") as stream:
    json.dump(data, stream, separators=(",", ":"), sort_keys=True)
    stream.write("\n")
PY
}

expect_error() {
  expected_exit=$1
  expected_code=$2
  expected_variant=$3
  shift 3
  : > "$ERROR_STDOUT"
  : > "$ERROR_STDERR"
  actual_exit=0
  "$@" > "$ERROR_STDOUT" 2> "$ERROR_STDERR" || actual_exit=$?
  [ "$actual_exit" -eq "$expected_exit" ] || fail "expected exit $expected_exit for $expected_code, got $actual_exit"
  [ ! -s "$ERROR_STDOUT" ] || fail "error $expected_code wrote to stdout"
  python3 -I - "$ERROR_STDERR" "$expected_code" "$expected_exit" "$expected_variant" <<'PY'
import json
import os
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
required = {"contract_version", "code", "message", "next", "exit_code"}
assert required.issubset(data) and set(data).issubset(required | {"details"}), data
assert data["contract_version"] == 2, data
assert isinstance(data["code"], str) and data["code"] == sys.argv[2], data
assert type(data["exit_code"]) is int and data["exit_code"] == int(sys.argv[3]), data
assert isinstance(data["message"], str) and data["message"], data
next_value = data["next"]
assert set(next_value) == {"operation", "workspace_identity", "variants"}, next_value
assert next_value["workspace_identity"].startswith("sha256:") and len(next_value["workspace_identity"]) == 71, next_value
assert all(character in "0123456789abcdef" for character in next_value["workspace_identity"][7:]), next_value
assert isinstance(next_value["variants"], list), next_value
ids = [variant["id"] for variant in next_value["variants"]]
if sys.argv[4] == "none":
    assert next_value["operation"] == "none" and ids == [], next_value
else:
    assert sys.argv[4] in ids, {"expected": sys.argv[4], "actual": ids}
assert "next_command" not in data, data
if "details" in data:
    assert isinstance(data["details"], dict), data
PY
}

# Full lifecycle: queued clarify, idempotency, pause/answer, stop/resume,
# stale rejection, advisory review reporting, summarize, and terminal rejection.
REPO="$TMP_ROOT/lifecycle"
new_repository "$REPO"
EMPTY_STATUS="$TMP_ROOT/empty-status.json"
"$BIN" status --root "$REPO" --json > "$EMPTY_STATUS"
python3 -I - "$EMPTY_STATUS" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    report = json.load(stream)
assert set(report) == {"contract_version", "runs", "unavailable_runs"}, report
assert report["contract_version"] == 2, report
assert report["runs"] == [], report
assert report["unavailable_runs"] == [], report
PY
START="$TMP_ROOT/start.json"
"$BIN" run 'acceptance lifecycle' --root "$REPO" --budget 12 --json > "$START"
assert_action "$START" orient
RUN_ID=$(json_get "$START" run_id)
RUN_DIR="$REPO/.git/slipway/runs/$RUN_ID"
[ -f "$RUN_DIR/journal.jsonl" ] || fail 'authoritative journal.jsonl was not created'
[ ! -e "$RUN_DIR/events.jsonl" ] || fail 'legacy events.jsonl compatibility residue was created'
ORIENT_ID=$(json_get "$START" action.action_id)

ORIENT_OUTCOME="$TMP_ROOT/orient-outcome.json"
make_outcome "$ORIENT_OUTCOME" "$ORIENT_ID" orient completed 'Repository facts observed.' '{"suggested_actions":[{"kind":"clarify","brief":"Ask for the release channel."}]}'
CLARIFY="$TMP_ROOT/clarify.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$ORIENT_ID" --outcome-file "$ORIENT_OUTCOME" > "$CLARIFY"
assert_action "$CLARIFY" clarify
CLARIFY_ID=$(json_get "$CLARIFY" action.action_id)

RETRY="$TMP_ROOT/retry.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$ORIENT_ID" --outcome-file "$ORIENT_OUTCOME" > "$RETRY"
cmp -s "$CLARIFY" "$RETRY" || fail 'identical Outcome retry did not return the derived current Action'
CONFLICT_OUTCOME="$TMP_ROOT/conflict-outcome.json"
make_outcome "$CONFLICT_OUTCOME" "$ORIENT_ID" orient completed 'Conflicting retry.' '{}'
expect_error 3 outcome_conflict skip-action "$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$ORIENT_ID" --outcome-file "$CONFLICT_OUTCOME"

CLARIFY_OUTCOME="$TMP_ROOT/clarify-outcome.json"
make_outcome "$CLARIFY_OUTCOME" "$CLARIFY_ID" clarify needs_input 'Release channel requires a user decision.' '{"pause_reason":"decision_required"}'
PAUSED="$TMP_ROOT/paused.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$CLARIFY_ID" --outcome-file "$CLARIFY_OUTCOME" > "$PAUSED"
assert_state "$PAUSED" paused decision_required answer-decision
IMPLEMENT="$TMP_ROOT/implement.json"
"$BIN" _machine answer --root "$REPO" --run "$RUN_ID" --action "$CLARIFY_ID" --text stable > "$IMPLEMENT"
assert_action "$IMPLEMENT" orient
OLD_IMPLEMENT_ID=$(json_get "$IMPLEMENT" action.action_id)

STOPPED="$TMP_ROOT/stopped.json"
STOPPED_AGAIN="$TMP_ROOT/stopped-again.json"
"$BIN" stop "$RUN_ID" --root "$REPO" --json > "$STOPPED"
"$BIN" stop "$RUN_ID" --root "$REPO" --json > "$STOPPED_AGAIN"
assert_state "$STOPPED" stopped "" resume-ad-hoc
cmp -s "$STOPPED" "$STOPPED_AGAIN" || fail 'repeated stop was not idempotent'
STOPPED_OUTCOME="$TMP_ROOT/stopped-outcome.json"
make_outcome "$STOPPED_OUTCOME" "$OLD_IMPLEMENT_ID" orient completed 'Should not be accepted.' '{"implementation_result":"not_needed"}'
expect_error 3 run_not_active resume-ad-hoc "$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$OLD_IMPLEMENT_ID" --outcome-file "$STOPPED_OUTCOME"

RESUMED="$TMP_ROOT/resumed.json"
"$BIN" _machine resume "$RUN_ID" --root "$REPO" > "$RESUMED"
assert_action "$RESUMED" orient
RESUMED_ORIENT_ID=$(json_get "$RESUMED" action.action_id)
expect_error 3 stale_action skip-action "$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$OLD_IMPLEMENT_ID" --outcome-file "$STOPPED_OUTCOME"
STATUS="$TMP_ROOT/status.json"
"$BIN" status "$RUN_ID" --root "$REPO" --json > "$STATUS"
python3 -I - "$STATUS" "$OLD_IMPLEMENT_ID" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    run = json.load(stream)
records = [record for record in run["actions"] if record["action"]["action_id"] == sys.argv[2]]
assert len(records) == 1, records
assert records[0].get("voided") is True, records[0]
assert run["state"] == "active", run
PY

RESUMED_ORIENT_OUTCOME="$TMP_ROOT/resumed-orient-outcome.json"
make_outcome "$RESUMED_ORIENT_OUTCOME" "$RESUMED_ORIENT_ID" orient completed 'Repository re-oriented after resume.' '{"suggested_actions":[{"kind":"implement","brief":"Implement the accepted work."}]}'
IMPLEMENT2="$TMP_ROOT/implement2.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$RESUMED_ORIENT_ID" --outcome-file "$RESUMED_ORIENT_OUTCOME" > "$IMPLEMENT2"
assert_action "$IMPLEMENT2" implement
IMPLEMENT2_ID=$(json_get "$IMPLEMENT2" action.action_id)
printf 'implementation change\n' >> "$REPO/README.md"
FAILED_ACTIVITY_OUTPUT="$TMP_ROOT/failed-activity-output.txt"
FAILED_ACTIVITY_COMMAND="sh -c 'printf deterministic-failure >&2; exit 17'"
set +e
sh -c 'printf deterministic-failure >&2; exit 17' > "$FAILED_ACTIVITY_OUTPUT" 2>&1
FAILED_ACTIVITY_EXIT=$?
set -e
[ "$FAILED_ACTIVITY_EXIT" -eq 17 ] || fail "deterministic failing activity returned $FAILED_ACTIVITY_EXIT"
FAILED_ACTIVITY_SUMMARY=$(cat "$FAILED_ACTIVITY_OUTPUT")
[ "$FAILED_ACTIVITY_SUMMARY" = 'deterministic-failure' ] || fail 'deterministic failing activity output changed'
FAILED_ACTIVITY_EXTRA=$(python3 -I - "$FAILED_ACTIVITY_COMMAND" "$FAILED_ACTIVITY_EXIT" "$FAILED_ACTIVITY_SUMMARY" <<'PY'
import json
import sys

command, exit_code, summary = sys.argv[1:]
print(json.dumps({
    "implementation_result": "applied",
    "files_changed": [],
    "activities": [{
        "kind": "test",
        "command": command,
        "exit_code": int(exit_code),
        "summary": summary,
    }],
}, separators=(",", ":")))
PY
)
IMPLEMENT2_OUTCOME="$TMP_ROOT/implement2-outcome.json"
make_outcome "$IMPLEMENT2_OUTCOME" "$IMPLEMENT2_ID" implement completed 'Implementation report preserved a real non-zero test exit.' "$FAILED_ACTIVITY_EXTRA"
REVIEW="$TMP_ROOT/review.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$IMPLEMENT2_ID" --outcome-file "$IMPLEMENT2_OUTCOME" > "$REVIEW"
assert_action "$REVIEW" review
REVIEW_ID=$(json_get "$REVIEW" action.action_id)

REVIEW_OUTCOME="$TMP_ROOT/review-outcome.json"
make_outcome "$REVIEW_OUTCOME" "$REVIEW_ID" review completed 'Review reported one advisory finding.' '{"review_result":"findings_reported","findings":[{"location":"README.md:1","summary":"Advisory finding","detail":"Report only; do not repair automatically."}]}'
SUMMARIZE="$TMP_ROOT/summarize.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$REVIEW_ID" --outcome-file "$REVIEW_OUTCOME" > "$SUMMARIZE"
assert_action "$SUMMARIZE" summarize
SUMMARIZE_ID=$(json_get "$SUMMARIZE" action.action_id)
SUMMARY_OUTCOME="$TMP_ROOT/summary-outcome.json"
make_outcome "$SUMMARY_OUTCOME" "$SUMMARIZE_ID" summarize completed 'Final report prepared.' '{}'
ENDED="$TMP_ROOT/ended.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$SUMMARIZE_ID" --outcome-file "$SUMMARY_OUTCOME" > "$ENDED"
assert_state "$ENDED" ended
python3 -I - "$ENDED" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
assert "observed_since_start: the current Git observation differs from the run-start snapshot." in data["summary"], data
assert "attribution_uncertainty: concurrent user edits, another Run, or tools may have contributed" in data["summary"], data
assert "- test: sh -c 'printf deterministic-failure >&2; exit 17' (exit 17): deterministic-failure" in data["summary"], data
PY
FINAL_STATUS="$TMP_ROOT/final-status.json"
"$BIN" status "$RUN_ID" --root "$REPO" --json > "$FINAL_STATUS"
python3 -I - "$FINAL_STATUS" "$IMPLEMENT2_ID" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    run = json.load(stream)
records = [record for record in run["actions"] if record["action"]["action_id"] == sys.argv[2]]
assert len(records) == 1, records
activity = records[0]["outcome"]["implementation"]["activities"][0]
assert activity == {
    "kind": "test",
    "command": "sh -c 'printf deterministic-failure >&2; exit 17'",
    "exit_code": 17,
    "summary": "deterministic-failure",
}, activity
assert run["activities"] == [activity], run["activities"]
assert "exit 17" in run["summary"], run["summary"]
PY
ENDED_REPLAY="$TMP_ROOT/ended-replay.json"
"$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$SUMMARIZE_ID" --outcome-file "$SUMMARY_OUTCOME" > "$ENDED_REPLAY"
assert_state "$ENDED_REPLAY" ended
SUMMARY_CONFLICT="$TMP_ROOT/summary-conflict.json"
make_outcome "$SUMMARY_CONFLICT" "$SUMMARIZE_ID" summarize completed 'Different final report.' '{}'
expect_error 3 outcome_conflict none "$BIN" _machine submit --root "$REPO" --run "$RUN_ID" --action "$SUMMARIZE_ID" --outcome-file "$SUMMARY_CONFLICT"
expect_error 3 run_already_ended none "$BIN" stop "$RUN_ID" --root "$REPO" --json
expect_error 3 run_already_ended none "$BIN" _machine resume "$RUN_ID" --root "$REPO"

# Skipping Review goes directly to Summarize and is recorded in the report.
SKIP_REPO="$TMP_ROOT/skip"
new_repository "$SKIP_REPO"
SKIP_START="$TMP_ROOT/skip-start.json"
"$BIN" run 'skip review' --root "$SKIP_REPO" --budget 5 --json > "$SKIP_START"
SKIP_RUN=$(json_get "$SKIP_START" run_id)
SKIP_ORIENT=$(json_get "$SKIP_START" action.action_id)
SKIP_ORIENT_OUT="$TMP_ROOT/skip-orient-out.json"
make_outcome "$SKIP_ORIENT_OUT" "$SKIP_ORIENT" orient completed 'Oriented.' '{"suggested_actions":[{"kind":"implement","brief":"Implement the accepted work."}]}'
SKIP_IMPLEMENT_JSON="$TMP_ROOT/skip-implement.json"
"$BIN" _machine submit --root "$SKIP_REPO" --run "$SKIP_RUN" --action "$SKIP_ORIENT" --outcome-file "$SKIP_ORIENT_OUT" > "$SKIP_IMPLEMENT_JSON"
SKIP_IMPLEMENT=$(json_get "$SKIP_IMPLEMENT_JSON" action.action_id)
printf 'skip scenario change\n' >> "$SKIP_REPO/README.md"
SKIP_IMPLEMENT_OUT="$TMP_ROOT/skip-implement-out.json"
make_outcome "$SKIP_IMPLEMENT_OUT" "$SKIP_IMPLEMENT" implement completed 'Changed README.' '{"implementation_result":"applied","files_changed":["README.md"]}'
SKIP_REVIEW_JSON="$TMP_ROOT/skip-review.json"
"$BIN" _machine submit --root "$SKIP_REPO" --run "$SKIP_RUN" --action "$SKIP_IMPLEMENT" --outcome-file "$SKIP_IMPLEMENT_OUT" > "$SKIP_REVIEW_JSON"
SKIP_REVIEW=$(json_get "$SKIP_REVIEW_JSON" action.action_id)
SKIP_SUMMARY="$TMP_ROOT/skip-summary.json"
"$BIN" _machine skip --root "$SKIP_REPO" --run "$SKIP_RUN" --action "$SKIP_REVIEW" > "$SKIP_SUMMARY"
assert_action "$SKIP_SUMMARY" summarize
SKIP_SUMMARY_ID=$(json_get "$SKIP_SUMMARY" action.action_id)
SKIP_SUMMARY_OUT="$TMP_ROOT/skip-summary-out.json"
make_outcome "$SKIP_SUMMARY_OUT" "$SKIP_SUMMARY_ID" summarize completed 'Skip report prepared.' '{}'
SKIP_ENDED="$TMP_ROOT/skip-ended.json"
"$BIN" _machine submit --root "$SKIP_REPO" --run "$SKIP_RUN" --action "$SKIP_SUMMARY_ID" --outcome-file "$SKIP_SUMMARY_OUT" > "$SKIP_ENDED"
assert_state "$SKIP_ENDED" ended
python3 -I - "$SKIP_ENDED" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
assert "Review was skipped by the user." in data["summary"], data
PY

# Contract skew must fail closed with the exact refresh command.
VERSION_REPO="$TMP_ROOT/version"
new_repository "$VERSION_REPO"
VERSION_START="$TMP_ROOT/version-start.json"
"$BIN" run 'version skew' --root "$VERSION_REPO" --budget 3 --json > "$VERSION_START"
assert_action "$VERSION_START" orient
VERSION_RUN=$(json_get "$VERSION_START" run_id)
VERSION_ACTION=$(json_get "$VERSION_START" action.action_id)
VERSION_OUTCOME="$TMP_ROOT/version-outcome.json"
make_outcome "$VERSION_OUTCOME" "$VERSION_ACTION" orient completed 'Wrong contract.' '{}'
python3 -I - "$VERSION_OUTCOME" <<'PY'
import json
import sys
path = sys.argv[1]
with open(path, encoding="utf-8") as stream:
    data = json.load(stream)
data["contract_version"] = 999
with open(path, "w", encoding="utf-8") as stream:
    json.dump(data, stream, separators=(",", ":"), sort_keys=True)
    stream.write("\n")
PY
expect_error 3 contract_version_mismatch refresh-adapters "$BIN" _machine submit --root "$VERSION_REPO" --run "$VERSION_RUN" --action "$VERSION_ACTION" --outcome-file "$VERSION_OUTCOME"

# Host-reported files do not create a Review when the CLI observes no diff.
REPORTED_REPO="$TMP_ROOT/reported-only"
new_repository "$REPORTED_REPO"
REPORTED_START="$TMP_ROOT/reported-start.json"
"$BIN" run 'reported-only change' --root "$REPORTED_REPO" --budget 4 --json > "$REPORTED_START"
REPORTED_RUN=$(json_get "$REPORTED_START" run_id)
REPORTED_ORIENT=$(json_get "$REPORTED_START" action.action_id)
REPORTED_ORIENT_OUT="$TMP_ROOT/reported-orient-out.json"
make_outcome "$REPORTED_ORIENT_OUT" "$REPORTED_ORIENT" orient completed 'Oriented.' '{"suggested_actions":[{"kind":"implement","brief":"Implement the accepted work."}]}'
REPORTED_IMPLEMENT_JSON="$TMP_ROOT/reported-implement.json"
"$BIN" _machine submit --root "$REPORTED_REPO" --run "$REPORTED_RUN" --action "$REPORTED_ORIENT" --outcome-file "$REPORTED_ORIENT_OUT" > "$REPORTED_IMPLEMENT_JSON"
assert_action "$REPORTED_IMPLEMENT_JSON" implement
REPORTED_IMPLEMENT=$(json_get "$REPORTED_IMPLEMENT_JSON" action.action_id)
REPORTED_IMPLEMENT_OUT="$TMP_ROOT/reported-implement-out.json"
make_outcome "$REPORTED_IMPLEMENT_OUT" "$REPORTED_IMPLEMENT" implement completed 'Host reported a file without changing Git.' '{"implementation_result":"applied","files_changed":["ghost.txt"]}'
REPORTED_SUMMARY="$TMP_ROOT/reported-summary.json"
"$BIN" _machine submit --root "$REPORTED_REPO" --run "$REPORTED_RUN" --action "$REPORTED_IMPLEMENT" --outcome-file "$REPORTED_IMPLEMENT_OUT" > "$REPORTED_SUMMARY"
assert_action "$REPORTED_SUMMARY" summarize

# --no-review suppresses Review even when the CLI observes an actual diff.
NO_REVIEW_REPO="$TMP_ROOT/no-review"
new_repository "$NO_REVIEW_REPO"
NO_REVIEW_START="$TMP_ROOT/no-review-start.json"
"$BIN" run 'no review requested' --root "$NO_REVIEW_REPO" --budget 4 --no-review --json > "$NO_REVIEW_START"
NO_REVIEW_RUN=$(json_get "$NO_REVIEW_START" run_id)
NO_REVIEW_ORIENT=$(json_get "$NO_REVIEW_START" action.action_id)
NO_REVIEW_ORIENT_OUT="$TMP_ROOT/no-review-orient-out.json"
make_outcome "$NO_REVIEW_ORIENT_OUT" "$NO_REVIEW_ORIENT" orient completed 'Oriented.' '{"suggested_actions":[{"kind":"implement","brief":"Implement the accepted work."}]}'
NO_REVIEW_IMPLEMENT_JSON="$TMP_ROOT/no-review-implement.json"
"$BIN" _machine submit --root "$NO_REVIEW_REPO" --run "$NO_REVIEW_RUN" --action "$NO_REVIEW_ORIENT" --outcome-file "$NO_REVIEW_ORIENT_OUT" > "$NO_REVIEW_IMPLEMENT_JSON"
assert_action "$NO_REVIEW_IMPLEMENT_JSON" implement
NO_REVIEW_IMPLEMENT=$(json_get "$NO_REVIEW_IMPLEMENT_JSON" action.action_id)
printf 'no-review change\n' >> "$NO_REVIEW_REPO/README.md"
NO_REVIEW_IMPLEMENT_OUT="$TMP_ROOT/no-review-implement-out.json"
make_outcome "$NO_REVIEW_IMPLEMENT_OUT" "$NO_REVIEW_IMPLEMENT" implement completed 'Changed without review.' '{"implementation_result":"applied","files_changed":["README.md"]}'
NO_REVIEW_SUMMARY="$TMP_ROOT/no-review-summary.json"
"$BIN" _machine submit --root "$NO_REVIEW_REPO" --run "$NO_REVIEW_RUN" --action "$NO_REVIEW_IMPLEMENT" --outcome-file "$NO_REVIEW_IMPLEMENT_OUT" > "$NO_REVIEW_SUMMARY"
assert_action "$NO_REVIEW_SUMMARY" summarize
NO_REVIEW_STATUS="$TMP_ROOT/no-review-status.json"
"$BIN" status "$NO_REVIEW_RUN" --root "$NO_REVIEW_REPO" --json > "$NO_REVIEW_STATUS"
python3 -I - "$NO_REVIEW_STATUS" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    data = json.load(stream)
assert data["review_enabled"] is False, data
PY

# Budget exhaustion pauses deterministically; explicit zero is rejected and a
# default resume replenishes enough budget to issue Orient plus two Actions.
BUDGET_REPO="$TMP_ROOT/budget"
new_repository "$BUDGET_REPO"
BUDGET_START="$TMP_ROOT/budget-start.json"
"$BIN" run 'budget exhaustion' --root "$BUDGET_REPO" --budget 1 --json > "$BUDGET_START"
assert_action "$BUDGET_START" orient
[ "$(json_get "$BUDGET_START" action.remaining_budget)" -eq 0 ] || fail 'budget 1 did not spend its only Action on Orient'
BUDGET_RUN=$(json_get "$BUDGET_START" run_id)
BUDGET_ORIENT=$(json_get "$BUDGET_START" action.action_id)
BUDGET_OUTCOME="$TMP_ROOT/budget-outcome.json"
make_outcome "$BUDGET_OUTCOME" "$BUDGET_ORIENT" orient completed 'Oriented at budget limit.' '{}'
BUDGET_PAUSED="$TMP_ROOT/budget-paused.json"
"$BIN" _machine submit --root "$BUDGET_REPO" --run "$BUDGET_RUN" --action "$BUDGET_ORIENT" --outcome-file "$BUDGET_OUTCOME" > "$BUDGET_PAUSED"
assert_state "$BUDGET_PAUSED" paused budget_exhausted resume-ad-hoc
expect_error 2 invalid_budget inspect-run "$BIN" _machine resume "$BUDGET_RUN" --root "$BUDGET_REPO" --budget 0
BUDGET_RESUMED="$TMP_ROOT/budget-resumed.json"
"$BIN" _machine resume "$BUDGET_RUN" --root "$BUDGET_REPO" > "$BUDGET_RESUMED"
assert_action "$BUDGET_RESUMED" orient
[ "$(json_get "$BUDGET_RESUMED" action.remaining_budget)" -eq 2 ] || fail 'default budget resume did not replenish to three Actions'

# Natural-language yes declines destructive authority; only exact structured
# confirmation issues a fresh one-shot authorized Implement.
DESTRUCTIVE_REPO="$TMP_ROOT/destructive"
new_repository "$DESTRUCTIVE_REPO"
DESTRUCTIVE_START="$TMP_ROOT/destructive-start.json"
"$BIN" run 'destructive confirmation' --root "$DESTRUCTIVE_REPO" --budget 10 --no-review --json > "$DESTRUCTIVE_START"
assert_action "$DESTRUCTIVE_START" orient
DESTRUCTIVE_RUN=$(json_get "$DESTRUCTIVE_START" run_id)
DESTRUCTIVE_ORIENT=$(json_get "$DESTRUCTIVE_START" action.action_id)
DESTRUCTIVE_ORIENT_OUT="$TMP_ROOT/destructive-orient-out.json"
make_outcome "$DESTRUCTIVE_ORIENT_OUT" "$DESTRUCTIVE_ORIENT" orient completed 'Destructive scope discovered.' '{"suggested_actions":[{"kind":"implement","brief":"Request exact destructive confirmation."}]}'
DESTRUCTIVE_IMPLEMENT_JSON="$TMP_ROOT/destructive-implement.json"
"$BIN" _machine submit --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_ORIENT" --outcome-file "$DESTRUCTIVE_ORIENT_OUT" > "$DESTRUCTIVE_IMPLEMENT_JSON"
assert_action "$DESTRUCTIVE_IMPLEMENT_JSON" implement
DESTRUCTIVE_IMPLEMENT=$(json_get "$DESTRUCTIVE_IMPLEMENT_JSON" action.action_id)
DESTRUCTIVE_DIGEST=$(python3 -I - <<'PY'
import hashlib
import json
scope = {
    "impact": "delete the exact acceptance target permanently",
    "request_id": "44444444-4444-4444-8444-444444444444",
    "scope_version": 1,
    "targets": [{"kind": "path", "value": "/absolute/acceptance target"}],
}
canonical = json.dumps(scope, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()
print("sha256:" + hashlib.sha256(canonical).hexdigest())
PY
)
DESTRUCTIVE_EXTRA=$(python3 -I - "$DESTRUCTIVE_DIGEST" <<'PY'
import json
import sys
print(json.dumps({
    "pause_reason": "destructive_confirmation_required",
    "question": "Confirm the exact acceptance target?",
    "destructive_request": {
        "request_id": "44444444-4444-4444-8444-444444444444",
        "targets": [{"kind": "path", "value": "/absolute/acceptance target"}],
        "impact": "delete the exact acceptance target permanently",
        "scope_sha256": sys.argv[1],
    },
}, separators=(",", ":")))
PY
)
DESTRUCTIVE_PAUSE_OUT="$TMP_ROOT/destructive-pause-out.json"
make_outcome "$DESTRUCTIVE_PAUSE_OUT" "$DESTRUCTIVE_IMPLEMENT" implement needs_input 'Exact destructive confirmation required.' "$DESTRUCTIVE_EXTRA"
DESTRUCTIVE_PAUSED="$TMP_ROOT/destructive-paused.json"
"$BIN" _machine submit --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_IMPLEMENT" --outcome-file "$DESTRUCTIVE_PAUSE_OUT" > "$DESTRUCTIVE_PAUSED"
assert_state "$DESTRUCTIVE_PAUSED" paused destructive_confirmation_required confirm-destructive
DESTRUCTIVE_DECLINED="$TMP_ROOT/destructive-declined.json"
"$BIN" _machine answer --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_IMPLEMENT" --text yes > "$DESTRUCTIVE_DECLINED"
assert_action "$DESTRUCTIVE_DECLINED" orient
DESTRUCTIVE_REORIENT=$(json_get "$DESTRUCTIVE_DECLINED" action.action_id)
DESTRUCTIVE_REORIENT_OUT="$TMP_ROOT/destructive-reorient-out.json"
make_outcome "$DESTRUCTIVE_REORIENT_OUT" "$DESTRUCTIVE_REORIENT" orient completed 'User feedback recorded without authority.' '{"suggested_actions":[{"kind":"implement","brief":"Request a fresh exact destructive scope."}]}'
DESTRUCTIVE_IMPLEMENT2_JSON="$TMP_ROOT/destructive-implement2.json"
"$BIN" _machine submit --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_REORIENT" --outcome-file "$DESTRUCTIVE_REORIENT_OUT" > "$DESTRUCTIVE_IMPLEMENT2_JSON"
assert_action "$DESTRUCTIVE_IMPLEMENT2_JSON" implement
DESTRUCTIVE_IMPLEMENT2=$(json_get "$DESTRUCTIVE_IMPLEMENT2_JSON" action.action_id)
DESTRUCTIVE_DIGEST2=$(python3 -I - <<'PY'
import hashlib
import json
scope = {
    "impact": "delete the exact acceptance target permanently",
    "request_id": "55555555-5555-4555-8555-555555555555",
    "scope_version": 1,
    "targets": [{"kind": "path", "value": "/absolute/acceptance target"}],
}
canonical = json.dumps(scope, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()
print("sha256:" + hashlib.sha256(canonical).hexdigest())
PY
)
DESTRUCTIVE_EXTRA2=$(python3 -I - "$DESTRUCTIVE_DIGEST2" <<'PY'
import json
import sys
print(json.dumps({
    "pause_reason": "destructive_confirmation_required",
    "question": "Confirm the fresh exact acceptance target?",
    "destructive_request": {
        "request_id": "55555555-5555-4555-8555-555555555555",
        "targets": [{"kind": "path", "value": "/absolute/acceptance target"}],
        "impact": "delete the exact acceptance target permanently",
        "scope_sha256": sys.argv[1],
    },
}, separators=(",", ":")))
PY
)
DESTRUCTIVE_PAUSE2_OUT="$TMP_ROOT/destructive-pause2-out.json"
make_outcome "$DESTRUCTIVE_PAUSE2_OUT" "$DESTRUCTIVE_IMPLEMENT2" implement needs_input 'Fresh destructive confirmation required.' "$DESTRUCTIVE_EXTRA2"
DESTRUCTIVE_PAUSED2="$TMP_ROOT/destructive-paused2.json"
"$BIN" _machine submit --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_IMPLEMENT2" --outcome-file "$DESTRUCTIVE_PAUSE2_OUT" > "$DESTRUCTIVE_PAUSED2"
assert_state "$DESTRUCTIVE_PAUSED2" paused destructive_confirmation_required confirm-destructive
expect_error 3 destructive_scope_mismatch confirm-destructive "$BIN" _machine answer --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_IMPLEMENT2" --confirm-destructive --scope-sha256 "sha256:0000000000000000000000000000000000000000000000000000000000000000"
DESTRUCTIVE_AUTHORIZED="$TMP_ROOT/destructive-authorized.json"
"$BIN" _machine answer --root "$DESTRUCTIVE_REPO" --run "$DESTRUCTIVE_RUN" --action "$DESTRUCTIVE_IMPLEMENT2" --confirm-destructive --scope-sha256 "$DESTRUCTIVE_DIGEST2" --text 'confirmed in acceptance host' > "$DESTRUCTIVE_AUTHORIZED"
assert_authorized_action "$DESTRUCTIVE_AUTHORIZED" "$DESTRUCTIVE_IMPLEMENT2" "$DESTRUCTIVE_DIGEST2"

# Issue-bound source is pinned once; material refresh pauses without applying
# budget, and candidate choice remains idempotent after the file is deleted.
SOURCE_REPO="$TMP_ROOT/source"
new_repository "$SOURCE_REPO"
SOURCE_FILE="$TMP_ROOT/source-envelope.json"
python3 -I - "$SOURCE_FILE" <<'PY'
import json
import sys

import hashlib
import struct

issue_url = "https://github.com/signalridge/slipway/issues/434"

def framed(*fields):
    digest = hashlib.sha256()
    for field in fields:
        encoded = field.encode("utf-8")
        digest.update(struct.pack(">Q", len(encoded)))
        digest.update(encoded)
    return "sha256:" + digest.hexdigest()

definitions = [
    ("outcome", "outcome", "Outcome", "# Outcome\n\nExercise the issue-bound protocol.\n"),
    ("requirements", "requirements", "Requirements", "# Requirements\n\nKeep the original accepted requirement.\n"),
    ("acceptance-examples", "acceptance_examples", "Acceptance examples", "# Acceptance examples\n\nA deleted source file is not needed after import.\n"),
    ("constraints", "constraints", "Constraints", "# Constraints\n\nNever persist the source-file path.\n"),
    ("non-goals", "non_goals", "Non-goals", "# Non-goals\n\nNo provider implementation.\n"),
]
comments = []
sections = []
for index, (key, role, title, payload) in enumerate(definitions, start=1):
    database_id = 2000 + index
    body = f"<!-- slipway-section:v1 key={key} -->\n{payload}"
    node_id = f"IC_acceptance_{key}"
    comments.append({
        "node_id": node_id,
        "database_id": database_id,
        "url": f"{issue_url}#issuecomment-{database_id}",
        "updated_at": "2026-07-12T09:00:00Z",
        "author_id": "U_acceptance",
        "is_minimized": False,
        "body": body,
    })
    sections.append({
        "key": key,
        "role": role,
        "title": title,
        "comment_node_id": node_id,
        "comment_database_id": database_id,
        "body_sha256": framed("slipway-comment-body/v1", body),
    })
manifest = {"manifest_version": 2, "profile": "change/v2", "sections": sections}
body = "<!-- slipway-level: change/v2 -->\n\n```slipway-manifest\n" + json.dumps(manifest, separators=(",", ":"), sort_keys=True) + "\n```\n"
data = {
    "source_version": 2,
    "provider": "github",
    "host": "github.com",
    "repository_id": "R_acceptanceSourceRepository",
    "issue_id": "I_acceptanceSourceIssue",
    "issue_number": 434,
    "canonical_url": issue_url,
    "updated_at": "2026-07-12T08:00:00Z",
    "fetched_at": "2026-07-12T09:01:00Z",
    "title": "[Change] Acceptance source lifecycle",
    "body": body,
    "labels": ["level:change", "kind:refactor"],
    "comments": comments,
}
with open(sys.argv[1], "w", encoding="utf-8") as stream:
    json.dump(data, stream, separators=(",", ":"), sort_keys=True)
    stream.write("\n")
PY
SOURCE_LINK="$TMP_ROOT/source-envelope-link.json"
ln -s "$SOURCE_FILE" "$SOURCE_LINK"
expect_error 2 invalid_source start-with-source \
  "$BIN" run --root "$SOURCE_REPO" --source-file "$SOURCE_LINK" --budget 8 --json -- 'reject source symlink'
SOURCE_START="$TMP_ROOT/source-start.json"
"$BIN" run 'issue-bound acceptance' --root "$SOURCE_REPO" --source-file "$SOURCE_FILE" --budget 8 --json > "$SOURCE_START"
assert_issue_action "$SOURCE_START" orient
SOURCE_RUN=$(json_get "$SOURCE_START" run_id)
SOURCE_PARENT_REQUIREMENTS=$(json_get "$SOURCE_START" action.source.requirements_revision)
SOURCE_MATERIAL="$TMP_ROOT/source-material.json"
"$BIN" _machine material --root "$SOURCE_REPO" --run "$SOURCE_RUN" --action "$(json_get "$SOURCE_START" action.action_id)" --section requirements > "$SOURCE_MATERIAL"
python3 -I - "$SOURCE_MATERIAL" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    material = json.load(stream)
assert material["contract_version"] == 2, material
assert material["message_type"] == "action_material", material
assert "original accepted requirement" in material["section"]["markdown"], material
PY
expect_error 3 source_mode_required use-pinned-source "$BIN" _machine resume "$SOURCE_RUN" --root "$SOURCE_REPO"
SOURCE_PINNED="$TMP_ROOT/source-pinned.json"
"$BIN" _machine resume "$SOURCE_RUN" --root "$SOURCE_REPO" --use-pinned-source > "$SOURCE_PINNED"
assert_issue_action "$SOURCE_PINNED" orient
python3 -I - "$SOURCE_FILE" "$SOURCE_PARENT_REQUIREMENTS" <<'PY'
import hashlib
import json
import struct
import sys

path, parent_revision = sys.argv[1:]
def framed(*fields):
    digest = hashlib.sha256()
    for field in fields:
        encoded = field.encode("utf-8")
        digest.update(struct.pack(">Q", len(encoded)))
        digest.update(encoded)
    return "sha256:" + digest.hexdigest()
with open(path, encoding="utf-8") as stream:
    data = json.load(stream)
manifest_text = data["body"].split("```slipway-manifest\n", 1)[1].rsplit("\n```", 1)[0]
manifest = json.loads(manifest_text)
for comment in data["comments"]:
    if comment["node_id"] == "IC_acceptance_requirements":
        comment["body"] = comment["body"].replace(
            "Keep the original accepted requirement.",
            "Keep the materially amended accepted requirement.",
        )
        comment["node_id"] = "IC_acceptance_requirements_replacement"
        comment["database_id"] = 3002
        comment["url"] = data["canonical_url"] + "#issuecomment-3002"
        comment["updated_at"] = "2026-07-12T10:00:00Z"
        for section in manifest["sections"]:
            if section["key"] == "requirements":
                section["comment_node_id"] = comment["node_id"]
                section["comment_database_id"] = comment["database_id"]
                section["body_sha256"] = framed("slipway-comment-body/v1", comment["body"])
manifest["parent_requirements_revision"] = parent_revision
data["body"] = "<!-- slipway-level: change/v2 -->\n\n```slipway-manifest\n" + json.dumps(manifest, separators=(",", ":"), sort_keys=True) + "\n```\n"
data["updated_at"] = "2026-07-12T10:00:00Z"
with open(path, "w", encoding="utf-8") as stream:
    json.dump(data, stream, separators=(",", ":"), sort_keys=True)
    stream.write("\n")
PY
SOURCE_CANDIDATE="$TMP_ROOT/source-candidate.json"
"$BIN" _machine resume "$SOURCE_RUN" --root "$SOURCE_REPO" --source-file "$SOURCE_FILE" --budget 20 > "$SOURCE_CANDIDATE"
CANDIDATE_ID=$(json_get "$SOURCE_CANDIDATE" source_candidate.candidate_id)
assert_state "$SOURCE_CANDIDATE" paused decision_required keep-pinned
[ "$(json_get "$SOURCE_CANDIDATE" budget_applied)" = false ] || fail 'candidate creation applied the replacement budget'
[ "$(json_get "$SOURCE_CANDIDATE" resume_operation)" = source_candidate_created ] || fail 'candidate response omitted its operation'
SOURCE_STATUS_BEFORE="$TMP_ROOT/source-status-before.json"
"$BIN" status "$SOURCE_RUN" --root "$SOURCE_REPO" --json > "$SOURCE_STATUS_BEFORE"
[ "$(json_get "$SOURCE_STATUS_BEFORE" remaining_budget)" -eq 6 ] || fail 'candidate creation changed the preserved budget'
rm -f "$SOURCE_FILE"
SOURCE_ADOPTED="$TMP_ROOT/source-adopted.json"
"$BIN" _machine resume "$SOURCE_RUN" --root "$SOURCE_REPO" --source-choice adopt --candidate "$CANDIDATE_ID" --budget 5 > "$SOURCE_ADOPTED"
assert_issue_action "$SOURCE_ADOPTED" orient
SOURCE_ADOPTED_MATERIAL="$TMP_ROOT/source-adopted-material.json"
"$BIN" _machine material --root "$SOURCE_REPO" --run "$SOURCE_RUN" --action "$(json_get "$SOURCE_ADOPTED" action.action_id)" --section requirements > "$SOURCE_ADOPTED_MATERIAL"
python3 -I - "$SOURCE_ADOPTED_MATERIAL" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as stream:
    material = json.load(stream)
assert "materially amended accepted requirement" in material["section"]["markdown"], material
PY
[ "$(json_get "$SOURCE_ADOPTED" action.remaining_budget)" -eq 4 ] || fail 'candidate choice did not apply replacement budget before Orient'
SOURCE_RETRY="$TMP_ROOT/source-retry.json"
"$BIN" _machine resume "$SOURCE_RUN" --root "$SOURCE_REPO" --source-choice adopt --candidate "$CANDIDATE_ID" --budget 999 > "$SOURCE_RETRY"
cmp -s "$SOURCE_ADOPTED" "$SOURCE_RETRY" || fail 'identical candidate choice retry issued another Action'
SOURCE_STATUS="$TMP_ROOT/source-status.json"
"$BIN" status "$SOURCE_RUN" --root "$SOURCE_REPO" --json > "$SOURCE_STATUS"
python3 -I - "$SOURCE_STATUS" "$SOURCE_FILE" "$CANDIDATE_ID" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    run = json.load(stream)
serialized = json.dumps(run, sort_keys=True)
assert run.get("source_candidate") is None, run
assert run["last_source_choice"]["candidate_id"] == sys.argv[3], run
assert run["last_source_choice"]["choice"] == "adopt", run
assert run["last_resume_result"]["budget_applied"] is True, run
assert [section["key"] for section in run["pinned_source"]["sections"]] == [
    "outcome", "requirements", "acceptance-examples", "constraints", "non-goals",
], run
assert "materially amended accepted requirement" not in serialized, run
assert sys.argv[2] not in serialized, run
assert "source-envelope.json" not in serialized, run
assert "<!-- slipway-level: change/v2 -->" not in serialized, run
PY

printf 'machine protocol acceptance: ok\n'
