# Requirements

## Requirements

### Requirement: Bound-worktree notes-file authority
REQ-001: The system MUST resolve `slipway evidence skill --notes-file` paths
against the active change's authoritative workspace so worktree-bound governed
changes can use `artifacts/...` paths from their bound worktree.

#### Scenario: notes file under bound worktree
GIVEN an active governed change is bound to a dedicated worktree
WHEN `slipway evidence skill --change <slug> --notes-file artifacts/changes/<slug>/verification/<notes>.md` is run
THEN the notes file is read from the bound worktree's `artifacts/...` path
AND the recorded verification notes match the bound-worktree file contents.

#### Scenario: invalid notes path remains rejected
GIVEN a user passes an absolute path or parent traversal path as `--notes-file`
WHEN evidence skill validation runs
THEN the command rejects the path before reading from disk.

### Requirement: Post-review evidence replacement guidance
REQ-002: The system MUST explain the correct S3/S4 replacement evidence surfaces
when task evidence or `wave-orchestration` evidence is requested after S2 has
already advanced.

#### Scenario: S3 task evidence wrong state
GIVEN a governed change is in `S3_REVIEW`
WHEN `slipway evidence task` is invoked
THEN the error keeps `evidence_task_wrong_state`
AND the remediation names S3 review evidence surfaces rather than only saying
task evidence is S2-only.

#### Scenario: S3 wave evidence wrong state
GIVEN a governed change is in `S3_REVIEW`
WHEN `slipway evidence skill --skill wave-orchestration` is invoked
THEN the error keeps `evidence_skill_wrong_state`
AND the remediation names review-driven repair evidence through
`spec-compliance-review`, `code-quality-review`, `goal-verification`, and
`final-closeout`.

### Requirement: Checkpoint resume and skill handoff clarity
REQ-003: The system MUST distinguish skill-handoff hard stops from active
checkpoint resume boundaries in both `confirmation_requirement` and
`--resume-response` error remediation.

#### Scenario: skill handoff is not checkpoint resume
GIVEN `slipway next --json` returns a governance skill handoff
WHEN `confirmation_requirement` is rendered
THEN `next_action_kind` remains `skill_handoff`
AND `resume_response_supported` remains false
AND `next_action` states that the operator must run/complete the named
governance skill and record evidence, not pass `--resume-response`.

#### Scenario: resume response without checkpoint is actionable
GIVEN no active checkpoint exists
WHEN `slipway run --resume-response <text>` is invoked
THEN the error keeps `no_active_checkpoint`
AND the remediation states that `--resume-response` is only valid for active
checkpoints and not for missing governance skill evidence.
