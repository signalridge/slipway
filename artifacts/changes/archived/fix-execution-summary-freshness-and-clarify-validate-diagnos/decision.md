# Decision

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Approach A: Minimal freshness DAG correction plus docs/tests
Remove summary/task captured timestamps from the normal per-task freshness
baseline, preserve unreadable-artifact fail-closed behavior, add validate
zero-write tests, and clarify active-only validate wording.

Tradeoffs: fixes the confirmed core bug with the smallest behavioral change, but
defers #30 tracked-runtime defense and #34 orphan-bundle diagnostic polish.

### Approach B: Broaden archive and repair defenses now
Add `done`/`repair` checks for tracked archived runtime evidence and structured
orphan-bundle diagnostics in the same delivery.

Tradeoffs: useful hardening, but it expands the blast radius beyond the only
must-fix issue and changes behavior for reports whose original root cause is not
confirmed on current HEAD.

### Approach C: Reinterpret `validate --change` as archived audit
Allow `validate --change <slug>` to read archived bundles after `done`.

Tradeoffs: conflicts with the current active-only readiness contract and mixes
active runtime state validation with frozen archive inspection.

## Selected Approach
Approach A. It matches the user's final triage: fix #28, clarify #29 wording,
add #32 zero-write regression coverage, and close #30/#32/#34 when current
evidence shows they are not must-fix implementation bugs.

## Interfaces and Data Flow
- No public command interface changes.
- `latestExecutionRelevantUpdateAt` continues to feed
  `ExecutionSummaryFreshness` and diagnostics, but its normal baseline now comes
  only from upstream planning artifacts (`intent.md`, `requirements.md`,
  `research.md`, `decision.md`, and semantically relevant `tasks.md` changes).
- `executionSummaryFreshnessEvaluation` keeps summary-level fallback freshness
  for summaries without detailed task inputs.
- Unreadable upstream artifact errors use a fail-closed helper anchored to the
  latest known summary/task evidence timestamp, so error paths remain stale.
- `validate --json` behavior is unchanged; tests assert no writes on diagnostic
  and failure paths.

## Rollout and Rollback
- Rollout: merge the code, docs/templates, tests, and governed bundle together.
  Close relevant GitHub issues after local verification passes.
- Rollback: revert the commit. The old behavior may reintroduce #28
  self-staleness, but no schema migration or irreversible data operation is
  involved.
- Verification commands: targeted package tests, then `go test -count=1 ./...`
  and `go build ./...`.

## Risk
- Main risk: weakening stale detection. Mitigation is explicit regression
  coverage for unreadable artifact fail-closed behavior and task structural
  timestamp drift.
- Secondary risk: over-closing #30/#32/#34. Mitigation is evidence-backed issue
  comments that ask for version/reproduction details if the reports are reopened.
- Contract risk: docs must not imply archived validation support. Mitigation is
  explicit active-readiness wording in `docs/commands.md`,
  `docs/operator-guide.md`, final-closeout template, and assurance template.
