# Intent

## Summary
Fix #136: make S2 `scope_contract_drift` non-destructive. An out-of-scope changed
file in the worktree (e.g. an untracked scratch file or the dogfooded `slipway`
build binary) currently makes `slipway run` silently reopen S2 in place and
delete `wave-orchestration.yaml` + `execution-summary.yaml`, masking the real
cause behind a `wave-orchestration missing` blocker. Change the scope-contract
advance gate so drift-only failures block visibly with actionable remediation
while preserving recorded wave evidence; missing task changed-file evidence still
reopens to S2 to re-record.

## Complexity Assessment
complex
<!-- Rationale: changes a governed lifecycle gate (S2->S3 advance) whose behavior
     is a public CLI/JSON contract; touches the engine progression layer plus the
     recovery/diagnostic surfaces; must not weaken fail-closed scope enforcement. -->

## Guardrail Domains
<!-- none detected — change makes governance strictly safer (non-destructive) and
     keeps drift fail-closed; no auth/credentials/financial/schema/irreversible
     surface is introduced. -->

## In Scope
- `internal/engine/progression/advance_governed.go`: scope-contract advance gate —
  drift-only failures return `blockedAdvanceSummary` (non-destructive) instead of
  `reopenToStaleStage`; missing task changed-file evidence still reopens to S2.
- `internal/engine/progression/stale_evidence_recovery.go`: add
  `scopeContractDriftOnly` classifier distinguishing `scope_contract_drift` from
  `scope_contract_changed_files_missing` / `scope_contract_missing`.
- `internal/model/recovery.go`: enrich `scope_contract_drift` remediation +
  `CommandTemplate` (remove/ignore the file, or `slipway pivot --rescope`; note
  evidence is preserved).
- `internal/engine/progression/readiness.go`: update
  `scopeContractRecoveryGuidanceDiagnostic` text so the public guidance matches
  the new non-destructive behavior.
- Tests: `internal/engine/progression/scope_contract_gate_test.go` (classifier
  unit test) and `cmd/scope_contract_test.go` (e2e: drift blocks visibly and
  preserves `wave-orchestration.yaml` + `execution-summary.yaml`).

## Out of Scope
- Adding build outputs (`/slipway`, `/dist/`, coverage) to `.gitignore`
  (#136 proposal #1) — explicitly excluded; the scope contract already honors
  `.gitignore` via `git ls-files --others --exclude-standard`, and forcing users
  to ignore every scratch file is the anti-pattern we are removing.
- Weakening the Scope Contract for genuine out-of-plan **source** changes — drift
  still fails closed (blocks), just non-destructively.
- Static SAST CI (#137) and `assurance.md` deferral (#141) — separate issues.

## Constraints
- Must not delete or restamp engine-owned freshness state for an incidental file
  (CLAUDE.md Evidence Discipline).
- The public surface (`run`/`next` JSON + remediation) must carry the next action
  (CLAUDE.md Self-Optimization Loop).
- Existing legitimate reopen path (`changed_files_missing` -> S2) must stay green.

## Acceptance Signals
1. `slipway run` with an untracked out-of-scope file blocks with
   `scope_contract_drift` (not masked as `wave-orchestration missing`), and
   `wave-orchestration.yaml` + `execution-summary.yaml` remain on disk.
2. After the out-of-scope file is removed, advancement resumes on the preserved
   evidence (no re-run of wave-orchestration required).
3. Missing task changed-file evidence still reopens to S2
   (`TestScopeContractReopenTargetReopensToS2WhenChangedFilesMissing` stays green).
4. `scope_contract_drift` remediation and the recovery-guidance diagnostic
   describe the remove/ignore/rescope path and state that evidence is preserved.
5. `go build ./...`, `go vet ./...`, `go test ./...` pass; `gofmt` clean.

## Open Questions
None.

## Deferred Ideas
- A `slipway run` side-effect/diagnostic entry that proactively names an
  out-of-scope untracked file even before the S2->S3 advance attempt (current fix
  surfaces it via the blocker on the advance attempt, which is sufficient).

## Approved Summary
Make S2 `scope_contract_drift` non-destructive and visible. An out-of-scope
changed file in the worktree (untracked scratch file or the dogfooded `slipway`
build binary) currently makes `slipway run` silently reopen S2 in place and
delete `wave-orchestration.yaml` + `execution-summary.yaml`, masking the cause as
`wave-orchestration missing`. The fix blocks drift-only failures visibly with
actionable remediation (remove/ignore the file, or `slipway pivot --rescope`)
while preserving the recorded wave evidence, so removing the file lets
advancement resume on that evidence. Genuine missing task changed-file evidence
(`scope_contract_changed_files_missing`) still reopens to S2 to re-record. The
public surface (`run`/`next` blocker + remediation + recovery-guidance
diagnostic) is updated to match.

Out of scope: editing `.gitignore` (the contract already honors `.gitignore` via
`--exclude-standard`); weakening fail-closed enforcement for genuine out-of-plan
source changes; #137 (SAST CI) and #141 (assurance deferral).

Primary acceptance signal: `slipway run` with an untracked out-of-scope file
reports `scope_contract_drift` while `wave-orchestration.yaml` and
`execution-summary.yaml` remain on disk.

Confirmed by user: 2026-06-08T13:31:29Z.
