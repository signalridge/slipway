# Requirements

## Requirements

### Requirement: Out-of-scope drift blocks non-destructively
REQ-001: When the S2 Scope Contract fails only with out-of-scope drift (a changed
file outside the plan, e.g. an untracked scratch file or build artifact), the
governed advance gate MUST block visibly and surface `scope_contract_drift`, and
it MUST NOT delete the recorded wave evidence (`wave-orchestration.yaml`,
`execution-summary.yaml`) or reopen S2 in place.

#### Scenario: Untracked out-of-scope file on run
GIVEN a governed change at S2_EXECUTE with a satisfied, recorded execution summary
and `wave-orchestration.yaml`
WHEN an untracked file outside the plan's `target_files` exists in the worktree
and `slipway run` is invoked
THEN the run is blocked with a `scope_contract_drift` blocker naming the file
AND `wave-orchestration.yaml` and `execution-summary.yaml` still exist on disk
AND the change remains at S2_EXECUTE without a `cleared_stale_generated_evidence`
side effect.

### Requirement: Removing the file resumes on preserved evidence
REQ-002: After the out-of-scope file is removed (or added to the plan via rescope),
re-running `slipway run` MUST advance using the preserved wave evidence, without
requiring wave-orchestration to be re-run.

#### Scenario: Drift cleared by removing the file
GIVEN a change blocked on `scope_contract_drift` for an untracked out-of-scope file
WHEN the file is removed and `slipway run` is invoked again
THEN the `scope_contract_drift` blocker is gone
AND advancement proceeds on the still-present recorded wave evidence.

### Requirement: Missing task changed-file evidence still reopens
REQ-003: When the Scope Contract fails because a planned task is missing its
changed-file evidence (`scope_contract_changed_files_missing` /
`scope_contract_missing`), the gate MUST still reopen to S2_EXECUTE so the
evidence can be re-recorded in its owning stage.

#### Scenario: Task missing changed files
GIVEN a governed change whose execution summary records a code task with no
changed files
WHEN the scope-contract advance gate evaluates the change
THEN the gate reopens to S2_EXECUTE carrying the
`scope_contract_changed_files_missing` blocker.

### Requirement: Public guidance matches non-destructive behavior
REQ-004: The public surfaces for `scope_contract_drift` — the blocker remediation
and the readiness recovery-guidance diagnostic — MUST describe the actionable
remove/ignore/rescope path and MUST state that the recorded wave evidence is
preserved.

#### Scenario: Operator reads drift guidance
GIVEN a `scope_contract_drift` blocker is surfaced by `validate`/`next`/`run`
WHEN the operator reads its remediation and the recovery-guidance diagnostic
THEN the guidance names removing/ignoring the file or `slipway pivot --rescope`
AND states that the recorded wave evidence is preserved (not cleared).
