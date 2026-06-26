# Requirements

### Requirement: Reject invalid shared task sessions early
REQ-001: `slipway evidence task --result-file` MUST reject duplicate non-empty
`session_id` values within the active task-evidence run when those values would
later produce `session_isolation_warning` for wave orchestration.

#### Scenario: Duplicate session IDs are rejected before persistence
GIVEN two result-file imports for different planned tasks in the same active run
AND both result files declare the same non-empty `session_id`
WHEN `slipway evidence task --result-file <a> --result-file <b> --json` runs
THEN the command fails with an actionable diagnostic
AND no task evidence from the invalid batch is persisted.

### Requirement: Explain malformed engine-owned verification records
REQ-002: Slipway MUST keep malformed `verification/<skill>.yaml` records
fail-closed while explaining that these YAML files are engine-owned verification
records and free-form notes belong in a separate notes file passed through
`slipway evidence skill --notes-file`. The same engine-owned boundary MUST be
stated preventively in the generated `wave-orchestration` skill guidance, not
only in the post-failure error path.

#### Scenario: Free-form notes in verification YAML receive recovery guidance
GIVEN `verification/wave-orchestration.yaml` contains free-form text rather than
a `VerificationRecord`
WHEN a command loads governed verification evidence
THEN the error explains the engine-owned file boundary
AND the remediation names `verification/wave-orchestration-notes.md` plus
`slipway evidence skill --skill wave-orchestration --notes-file`.

#### Scenario: Generated wave-orchestration guidance states the engine-owned boundary
GIVEN the generated `wave-orchestration` skill guidance
WHEN an author reads it before recording evidence
THEN it reserves `verification/<skill>.yaml` as an engine-owned VerificationRecord
written only by `slipway evidence skill`
AND it directs free-form notes to `verification/<skill>-notes.md` via `--notes-file`.

### Requirement: Show ready lifecycle states as ready
REQ-003: `slipway next --json --diagnostics` MUST NOT present
`blocked_by_governance` or "resolve governance blockers" wording when the
current lifecycle state can advance and no skill is required.

#### Scenario: Ready S2 output points to advancement
GIVEN a governed S2 change has passing fresh `wave-orchestration` evidence
AND `slipway validate --json` reports `can_advance=true`
WHEN `slipway next --json --diagnostics` runs
THEN the confirmation or recovery wording identifies the state as ready to
advance
AND it does not instruct the user to resolve governance blockers.

### Requirement: Disambiguate archived same-slug active residue
REQ-004: Orphaned active bundle residue recovery MUST distinguish stale active
state from an archived/delivered change when the same slug also has an archived
record.

#### Scenario: Archived same-slug residue avoids destructive worktree wording
GIVEN `artifacts/changes/<slug>/` exists without `change.yaml`
AND `artifacts/changes/archived/<slug>/change.yaml` exists
WHEN status or validation surfaces orphaned active bundle residue recovery
THEN the recovery text names the target as incomplete active-state residue
AND it says the archived record and source commits are not the target
AND it does not recommend `--worktree` as part of the primary remediation.

### Requirement: Keep freshly stamped wave-orchestration evidence fresh
REQ-005: A successful `slipway evidence skill --skill wave-orchestration` MUST NOT
leave its own freshly stamped evidence immediately stale. The wave-plan freshness
digest MUST exclude lifecycle bookkeeping (run-summary version, per-wave parallel
flag, generated-at timestamp) so that re-materializing a structurally identical
wave plan at a different run version does not flip the just-recorded evidence to
`required_skill_stale`, while genuine task-plan structural, scope, or semantic
changes MUST still stale it.

#### Scenario: Re-materialized wave plan keeps fresh evidence fresh
GIVEN a recorded passing `wave-orchestration` verification for the active run
WHEN the wave plan re-materializes at a different run-summary version with the
same task structure
THEN `slipway validate --json` does not report
`required_skill_stale:wave-orchestration:wave-plan.yaml`
AND a real change to a task's target files in the plan still stales the evidence.
