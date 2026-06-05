# Assurance
## Project Context
- Tech Stack: Go
- Conventions: governance engine under `internal/engine`, model types under `internal/model`, CLI views under `cmd/`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Recovery UX P1 (#84), part of the broader issue #81 recovery UX track, delivers a
presentation-only recovery vocabulary for blocked/stale governed states. The
implemented scope is:

- a single prefix-aware blocker parser (`ParsedBlocker`, `ParseBlocker`, and
  `ParseBlockerSpec`) in `internal/model`;
- a Code-keyed remediation table parallel to `canonicalReasonDefinitions`;
- a read-only `recovery` object on `next --json`, `run --json`, `validate --json`,
  and governance-blocked `CLIError` envelopes;
- grouped `recovery.steps[]` entries keyed by `(code, subject)`, so one skill's
  many stale artifacts surface as one actionable step with sorted `details[]`;
- a static stage-priority `primary_command` / `primary_action`, explicitly not
  the P2 dependency planner;
- canonical messages for recovery-relevant internal prefix tokens; and
- README.md / CLAUDE.md documentation for the additive JSON surface.

The change is additive and read-only. Existing `blockers[]` arrays remain
`[]ReasonCode`; persisted `ReasonCode` serialization is unchanged; blocker
producers, gates, state transitions, and governed semantics are not changed.
The P2 recovery planner / `slipway recover` / Tier-2 restamp (#85) and P3
narrow-gap lifecycle fixes (#86) remain out of scope.

## Verification Verdict
Current S1/S2/S3 governance evidence is refreshed and passing. The active
`validate --json` proof captured with `/tmp/slipway-p1-closeout validate --json
--change recovery-ux-p1-issue-84-prefix-aware-blocker-parser-parsedbl` reports:

- `evidence_freshness: fresh`;
- `G_plan: approved`;
- `G_scope: approved`;
- `scope_contract.status: pass`;
- `requirements_contract.status: valid`;
- `G_ship: blocked` only because S4 `goal-verification` and `final-closeout`
  evidence still need to be written and accepted before `done`.

Fresh local command proof for the current source state:

- `go build ./...` passed;
- `go test -count=1 ./...` passed across all packages;
- `go vet ./...` passed;
- `git diff --check` passed;
- `gofmt -l` over changed Go files returned no files.

The two required review layers have been re-run after the post-review repair:
`spec-compliance-review.yaml` records `layer:R0=pass`, `scope_contract:pass`,
and `negative_path:pass`; `code-quality-review.yaml` records `layer:IR1=pass`.

## Evidence Index
- `verification/intake-clarification.yaml` — pass, scoped P1 standalone and
  explicitly excludes P2/P3 work.
- `verification/research-orchestration.yaml` — pass, re-certified after wording
  repair; selected approach C remains current.
- `verification/plan-audit.yaml` — pass, required artifacts present, 8D checklist
  passes, codebase map partiality recorded as advisory.
- `verification/wave-plan.yaml` — 7 tasks, 4 waves,
  `tasks_plan_hash=e4b1ed38ed3d079c34a45ecfe3b06c51f3cfc50b90ec941e066ef134be44d47e`.
- `verification/wave-orchestration.yaml` — pass, run_version 1, t-01..t-07 task
  evidence recorded via `slipway evidence task` with the current tasks plan hash.
- `verification/execution-summary.yaml` — pass, run_summary_version 1,
  completed_tasks t-01..t-07.
- `verification/spec-compliance-review.yaml` — pass, `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`.
- `verification/code-quality-review.yaml` — pass, `layer:IR1=pass`.
- S4 records still to be accepted before `done`: `verification/goal-verification.yaml`
  and `verification/final-closeout.yaml`.

Primary source/test references:

- `internal/model/recovery.go`
- `internal/model/reason_code.go`
- `cmd/next.go`
- `cmd/next_handoff.go`
- `cmd/validate.go`
- `cmd/next_skill_view.go`
- `cmd/errors.go`
- `cmd/repair.go`
- `internal/model/recovery_test.go`
- `cmd/recovery_view_test.go`
- `cmd/lifecycle_commands_test.go`
- `cmd/validate_artifact_gate_test.go`
- `README.md`
- `CLAUDE.md`

## Requirement Coverage
- REQ-001 single parser: implemented by `internal/model/recovery.go`, consumed by
  `cmd/next_skill_view.go` and `cmd/repair.go`, covered by
  `TestParseBlockerSegments` and `TestParseBlockerIsSingleDecompositionPoint`.
- REQ-002 remediation table: implemented by `blockerRemediations`, covered by
  remediation table completeness, recovery-relevant canonical token coverage,
  and produced-token recovery tests.
- REQ-003 recovery object on next/run/validate: implemented by `nextView`,
  `nextHandoffView`, `validateView`, and validate gate-detail recovery, covered
  by command tests including compact handoff and validate recovery tests.
- REQ-004 compact handoff primary command: implemented by
  `buildNextHandoffView`, covered by `TestNextHandoffViewCarriesRecovery`.
- REQ-005 canonical messages: implemented in `canonicalReasonDefinitions` for
  recovery-relevant internal tokens, covered by canonical-message and
  produced-token tests.
- REQ-006 CLIError parity: implemented in `cmd/errors.go`, covered by
  `TestGovernanceBlockedErrorCarriesRecovery`.
- REQ-007 read-only/additive/docs: implemented with `omitempty` recovery fields,
  unchanged `ReasonCode` persistence, and README/CLAUDE docs; covered by
  serialization-shape and clean-state omitempty tests.

All requirements have Exists, Substantive, and Wired proof through source,
tests, and S2 execution evidence. No requirement is deferred.

## Residual Risks and Exceptions
- `primary_command` uses a static recovery-class priority list, not a
  dependency-ordered planner. This is intentional P1 scope; the planner remains
  P2 (#85).
- The remediation table covers recovery-relevant tokens. Non-recovery diagnostic
  tokens may still render through their existing paths; this is outside P1.
- JSON consumers that reject unknown fields could notice the additive
  `recovery` field. The field is `omitempty`, documented, and read-only; this is
  accepted as a backward-compatible host-facing extension.
- Codebase map status is partial and some map prose describes older work. The
  current governed bundle and direct source/test reads are the authority for this
  change; the map limitation is advisory and did not block plan audit.

## Rollback Readiness
Rollback is clean and immediate: remove `internal/model/recovery.go`, remove the
additive `Recovery` fields and `model.BuildRecovery` calls from the CLI views
and `CLIError`, remove the `blockerSkillName` parser delegation, remove the new
canonical/remediation entries, and revert the tests/docs. No persisted state,
evidence schema, gate behavior, migration, or destructive operation is involved.

## Archive Decision
Archive-ready after the S4 verification records are accepted and final active
validation confirms `G_ship: approved`. The active pre-done validation proof has
already been captured against the active bundle and current worktree with the
fresh local binary `/tmp/slipway-p1-closeout`; at this stage it reports only the
expected missing `goal-verification` / `final-closeout` S4 evidence blockers.
`slipway done` must be run only after those records are written, the
final-closeout assurance attestation `closeout:assurance_complete=pass` is
accepted, and active `validate --json` reports no blockers.
