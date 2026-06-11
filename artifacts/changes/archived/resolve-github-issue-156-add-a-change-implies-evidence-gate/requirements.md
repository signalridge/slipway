# Requirements

## Requirements

### Requirement: Schema migration evidence

REQ-001: The system MUST block governed readiness when a passed execution task
changes a schema migration or schema file without owning task evidence marked
with `migration-applied`.

#### Scenario: Migration change lacks applied evidence

GIVEN an execution summary with a passed task that changed
`db/migrations/001_create_users.sql`
WHEN readiness evaluates the change without a `migration-applied` marker
THEN readiness reports `sensitive_evidence_missing:schema_migration:db/migrations/001_create_users.sql`.

### Requirement: Auth and authorization evidence

REQ-002: The system MUST block governed readiness when a passed execution task
changes an auth, authz, permission, or RBAC-sensitive file without owning task
evidence marked with `auth-review`.

#### Scenario: Authz change lacks review evidence

GIVEN an execution summary with a passed task that changed
`internal/authz/policy.go`
WHEN readiness evaluates the change without an `auth-review` marker
THEN readiness reports `sensitive_evidence_missing:auth_authz:internal/authz/policy.go`.

### Requirement: API contract evidence

REQ-003: The system MUST block governed readiness when a passed execution task
changes an API contract file without owning task evidence marked with
`contract-test`.

#### Scenario: OpenAPI change lacks contract test evidence

GIVEN an execution summary with a passed task that changed `api/openapi.yaml`
WHEN readiness evaluates the change without a `contract-test` marker
THEN readiness reports `sensitive_evidence_missing:api_contract:api/openapi.yaml`.

### Requirement: Canonical blocker detail

REQ-004: The system MUST emit canonical reason code
`sensitive_evidence_missing` with detail formatted as `<category>:<path>` for
each sensitive file whose owning evidence is missing.

#### Scenario: Missing evidence detail is stable

GIVEN a sensitive schema migration file is missing owning evidence
WHEN readiness builds blockers
THEN the blocker code is `sensitive_evidence_missing` and its detail includes
the category and normalized file path.

### Requirement: Separate verification ownership

REQ-005: The system MUST accept matching owning evidence from any passed task in
the same execution summary, including a dedicated verification or test task.

#### Scenario: Verification task owns contract evidence

GIVEN one task changes `api/openapi.yaml` and another passed test task records
`contract-test:go test ./api`
WHEN readiness evaluates the execution summary
THEN the API contract change passes the sensitive-evidence gate.

### Requirement: Existing readiness gates preserved

REQ-006: The system MUST append the sensitive-evidence gate to governed
readiness without replacing freshness, scope-contract, worktree, artifact,
review, or ship blockers.

#### Scenario: Existing blockers remain visible

GIVEN an execution summary is ready and an existing scope-contract blocker is
present
WHEN sensitive-evidence evaluation also runs
THEN readiness keeps the existing blocker and adds any sensitive-evidence
blocker independently.

### Requirement: Recovery without bypass

REQ-007: The system MUST provide recovery guidance that points operators to
`slipway evidence task` and required markers, and MUST NOT document an
environment-variable, force, or bypass path.

#### Scenario: Recovery names marker and command

GIVEN a `sensitive_evidence_missing` blocker for a schema migration
WHEN recovery guidance is built
THEN the remediation names `slipway evidence task`, `migration-applied`, and
the affected file path without mentioning a bypass.

### Requirement: Public skill evidence recording

REQ-008: The system MUST expose a `slipway evidence skill` command that records
governance skill verification through the CLI, so generated host instructions do
not require hand-editing `verification/*.yaml`. Passing skill evidence MUST be
recorded with its engine-owned freshness digest and MUST reject downstream skill
records until the current actionable predecessor has passing evidence. When a
non-passing skill record overwrites a prior passing record, the system MUST
prune that skill's prior engine-owned freshness digest.

#### Scenario: Plan audit records verification through CLI

GIVEN an active change at `S1_PLAN/audit` and a plan-audit notes file
WHEN `slipway evidence skill --skill plan-audit --verdict pass --reference plan-audit:pass --notes-file <path>` runs
THEN the command writes `verification/plan-audit.yaml`, updates the change
evidence reference, writes `verification/evidence-digests.yaml`, and does not
advance lifecycle state.

#### Scenario: Review evidence cannot skip predecessor

GIVEN an active change at `S3_REVIEW` with a ready execution summary and no
passing `spec-compliance-review` evidence
WHEN `slipway evidence skill --skill code-quality-review --verdict pass` runs
THEN the command fails before writing `verification/code-quality-review.yaml`.

#### Scenario: Closeout evidence cannot skip goal verification

GIVEN an active change at `S4_VERIFY` with a ready execution summary and no
passing `goal-verification` evidence
WHEN `slipway evidence skill --skill final-closeout --verdict pass` runs
THEN the command fails before writing `verification/final-closeout.yaml`.

#### Scenario: Failing skill evidence clears stale digest

GIVEN an active change at `S1_PLAN/audit` with passing `plan-audit` evidence and
an existing `verification/evidence-digests.yaml` entry for `plan-audit`
WHEN `slipway evidence skill --skill plan-audit --verdict fail --blocker plan_audit_failed` runs
THEN the command overwrites `verification/plan-audit.yaml` and removes the
`plan-audit` digest entry.
