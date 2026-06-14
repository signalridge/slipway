# Requirements

## Requirements

### Requirement: Scope contract discloses exempted codebase-map files
REQ-001: The scope-contract report surfaced by `slipway validate --json`,
`slipway status --json`, and `slipway review --json` MUST explicitly list the
dirty `artifacts/codebase/**` files it exempts from `changed_files`, in a
dedicated structured field, so a reviewer no longer has to infer the hidden
filtering from a `git diff` disagreement. The exemption behavior itself MUST be
preserved: exempted files MUST stay out of `changed_files`/`out_of_scope_files`,
and `scope_contract.status` MUST stay `pass` when only context files are dirty.
The exemption and the new field MUST be documented in the user-facing docs.

#### Scenario: validate surfaces the exempted codebase-map file
GIVEN an active change whose worktree has a tracked, dirty
`artifacts/codebase/ARCHITECTURE.md` that is not in any task's target_files
AND an execution summary exists (run_summary_version >= 1)
WHEN the user runs `slipway validate --json`
THEN `scope_contract.exempt_context_files` includes
`artifacts/codebase/ARCHITECTURE.md`
AND `scope_contract.changed_files` does NOT include it
AND `scope_contract.status` is `pass`.

#### Scenario: exemption is documented
GIVEN the user reads the Slipway command/operator docs
WHEN they look up scope-contract reporting
THEN the docs state that `artifacts/codebase/**` is exempt from scope-contract
changed-file accounting and is disclosed via `scope_contract.exempt_context_files`.

### Requirement: Status does not surface a run version the evidence surface refuses
REQ-002: When no execution summary exists yet (no recorded wave run), the
user-facing "current run version" surfaces (`slipway status --json` and
`slipway next --json`) MUST NOT emit `run_summary_version=0`, because `0` is a
value that `slipway evidence task` rejects. The field MUST be omitted (or null)
rather than reported as `0`.

#### Scenario: early S2 status omits the zero run version
GIVEN an active change in S2_EXECUTE with no execution summary recorded yet
WHEN the user runs `slipway status --json`
THEN the output does NOT contain `run_summary_version` equal to `0`
(the field is omitted).

#### Scenario: status reports the real version once a run exists
GIVEN an active change with a recorded execution summary at run_summary_version 1
WHEN the user runs `slipway status --json`
THEN `progress.run_summary_version` is `1`.

### Requirement: Correct first task-evidence run version is discoverable and enforced
REQ-003: A public Slipway surface MUST make the correct first task-evidence run
version (`1`) discoverable so a user does not have to guess it after the status
field is omitted, and `slipway evidence task` MUST continue to reject
`--run-summary-version` values below `1`.

#### Scenario: evidence-task surface tells the user the first run version
GIVEN a user about to record the first task evidence for a change
WHEN they consult the `slipway evidence task` help / guidance surface
THEN it states that the first task-evidence run version is `1` (or to pass the
current wave-orchestration run_version).

#### Scenario: zero is still rejected
GIVEN a user records task evidence
WHEN they run `slipway evidence task --run-summary-version 0`
THEN the command fails with reason `evidence_task_run_summary_version_invalid`.
