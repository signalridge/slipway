# Tasks
## Project Context
- Tech Stack: Go
- Conventions: governance engine in `internal/engine`, model types in `internal/model`, CLI views in `cmd/`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Model recovery foundation in a new `internal/model/recovery.go` plus
  `internal/model/reason_code.go`: add `ParsedBlocker{Code, Subject, Detail, Raw}`
  and the single `ParseBlocker` parser (Detail split into Subject (2nd segment) +
  Detail (remainder)); the Code-keyed `blockerRemediation{Remediation,
  CommandTemplate, RecoveryClass}` table + lookup covering every recovery-relevant
  token; `RecoverySummary`/`RecoveryStep` + `BuildRecovery([]ReasonCode)` that
  builds one step per actionable `(code, subject)` group (collecting the distinct
  `Details`) and selects the primary by a static `recoveryClassPriority` list (not
  a dependency graph), returning nil when none;
  and the missing canonical message(s) (at least `tasks_plan_changed_since_task_evidence`).
  - wave: 1
  - depends_on: []
  - target_files: ["internal/model/recovery.go", "internal/model/reason_code.go", "internal/engine/progression/advance_governed.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005]

- [x] `t-02` Wire the recovery object into the read-only views and centralize
  decomposition: add `Recovery *model.RecoverySummary json:"recovery,omitempty"`
  to `nextView` (cmd/next.go), `nextHandoffView` (cmd/next_handoff.go), and
  `validateView` (cmd/validate.go), each populated from the view's existing
  `[]ReasonCode` via `model.BuildRecovery`; make `buildNextHandoffView` copy the
  recovery so the compact handoff preserves the primary command; refactor
  `blockerSkillName` (cmd/next_skill_view.go) to delegate to `model.ParseBlocker`
  and update its other caller in `cmd/repair.go` to pass the ReasonCode.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/next.go", "cmd/next_handoff.go", "cmd/validate.go", "cmd/next_skill_view.go", "cmd/repair.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-03` Wire the same recovery into `CLIError` (cmd/errors.go): add
  `Recovery *model.RecoverySummary json:"recovery,omitempty"`, populated from
  `Reasons` via `model.BuildRecovery` inside `newCLIErrorWithReasons`.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/errors.go"]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-04` Document the new read-only `recovery` object and grouped
  `recovery.steps[]` remediation JSON fields under the routed-command sections
  of README.md and CLAUDE.md, noting they are additive/omitempty and leave
  existing `blockers[]` arrays unchanged.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["README.md", "CLAUDE.md"]
  - task_kind: code
  - covers: [REQ-007]

- [x] `t-05` Model unit tests in `internal/model/recovery_test.go`: table-driven
  `ParseBlocker` (3-/2-/1-segment), remediation-table coverage (every entry
  non-empty; every recovery-relevant token resolves), `BuildRecovery` (steps per
  blocker, static primary selection, nil on clean), the canonical-message
  assertion for `tasks_plan_changed_since_task_evidence`, and a `ReasonCode` YAML
  round-trip asserting no new persisted fields.
  - wave: 3
  - depends_on: [t-01]
  - target_files: ["internal/model/recovery_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005, REQ-007]

- [x] `t-06` CLI tests in `cmd/recovery_view_test.go`: seed a blocked/stale
  governed change and assert `recovery.primary_command` non-empty on `validate`,
  compact `next`, and compact `run`; assert the compact handoff preserves the
  primary command; assert a governance-blocked `CLIError` carries a matching
  `recovery`; assert a clean state omits `recovery` (omitempty).
  - wave: 3
  - depends_on: [t-02, t-03]
  - target_files: ["cmd/recovery_view_test.go", "cmd/validate_artifact_gate_test.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: verification
  - covers: [REQ-003, REQ-004, REQ-006, REQ-007]

- [x] `t-07` Run `go build ./...` and `go test ./...`; both MUST be green
  (evidence: full build + test transcript). Gates compilation of the new model
  code and that both new test suites pass.
  - wave: 4
  - depends_on: [t-04, t-05, t-06]
  - target_files: ["internal/model/recovery.go", "internal/model/recovery_test.go", "cmd/recovery_view_test.go"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
