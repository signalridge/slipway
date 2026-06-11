# Structure

Re-authored for change `resolve-issue-163-decisions-gate` (GitHub issue #163).

- `internal/engine/artifact/`
  - `decision_contract.go`: decision artifact contract status, structured
    evaluation, substance blockers, and the planned parsed-decision status
    helper.
  - `decision_contract_test.go`: artifact-level tests for section substance and
    the planned parser/status behavior.
  - `manager.go`: existing selected-decision extraction helpers and markdown
    section utilities reused by artifact contracts.
  - `schemas.yaml`: authoritative required-section list for `decision.md`;
    status parsing must remain compatible because status is not currently a
    required schema section.
- `internal/engine/progression/`
  - `validation.go`: planning readiness gate that already invokes
    `DecisionContractBlockers` and should receive dead-status blockers.
  - `validation_test.go`: lifecycle-timing tests for decision blockers and the
    planned superseded/deprecated/unknown status regressions.
- `cmd/`
  - `next_skill.go`: next-skill constraint assembly currently reads decision
    text and routes it to pending or locked based on `G_plan`.
  - `next_skill_constraints_test.go`: existing pending-vs-locked coverage and
    planned dead-status handoff regressions.
- `internal/model/`
  - `reason_code.go`: canonical reason-code definitions for new decision status
    blockers.
  - `reason_code_contract_test.go`: frozen taxonomy snapshot that must be
    updated when new reason codes are added.
  - `recovery.go` and `recovery_test.go`: operator remediation for the new
    fail-closed diagnostics.
- `artifacts/changes/resolve-issue-163-decisions-gate/`
  - Governed artifact bundle for this change. `assurance.md` remains deferred
    until review/verify stages.
