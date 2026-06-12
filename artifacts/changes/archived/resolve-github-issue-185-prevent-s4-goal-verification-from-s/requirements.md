# Requirements

## Requirements

### Requirement: Evidence refs do not self-stale S4 verification

REQ-001: The system MUST keep `goal-verification` and `final-closeout` evidence
fresh when the only current-change `change.yaml` mutation after stamping is an
engine-owned `evidence_refs` update for that skill.

#### Scenario: Goal verification evidence pointer is recorded

GIVEN a governed change is in `S4_VERIFY` and the execution summary target files
include `artifacts/changes/<slug>/change.yaml`
WHEN `goal-verification` evidence is stamped and the CLI records
`evidence_refs.goal-verification` in `change.yaml`
THEN required-skill freshness MUST NOT report
`required_skill_stale:goal-verification:artifacts/changes/<slug>/change.yaml`
for that evidence-ref-only mutation.

#### Scenario: Final closeout evidence pointer is recorded

GIVEN a governed change is in `S4_VERIFY` and the execution summary target files
include `artifacts/changes/<slug>/change.yaml`
WHEN `final-closeout` evidence is stamped and the CLI records
`evidence_refs.final-closeout` in `change.yaml`
THEN required-skill freshness MUST NOT report
`required_skill_stale:final-closeout:artifacts/changes/<slug>/change.yaml` for
that evidence-ref-only mutation.

### Requirement: Meaningful change authority drift still fails closed

REQ-002: The system MUST continue to stale required S4 skill evidence when any
meaningful non-`evidence_refs` field in the current `change.yaml` authority
changes after the skill is stamped.

#### Scenario: Change description mutates after S4 evidence

GIVEN a stored `goal-verification` or `final-closeout` digest includes the
current `artifacts/changes/<slug>/change.yaml`
WHEN a non-`evidence_refs` field such as `description` changes in that file
THEN required-skill freshness MUST report
`required_skill_stale:<skill>:artifacts/changes/<slug>/change.yaml`.

### Requirement: The fix stays scoped to the current change authority

REQ-003: The system SHALL apply evidence-ref normalization only to the current
change authority path `artifacts/changes/<slug>/change.yaml` while preserving
normal content hashing for all other target files and directories.

#### Scenario: Normal target file changes

GIVEN stored `goal-verification` evidence includes a normal target file such as
`tracked.go`
WHEN that file content changes
THEN required-skill freshness MUST continue reporting
`required_skill_stale:goal-verification:tracked.go`.
