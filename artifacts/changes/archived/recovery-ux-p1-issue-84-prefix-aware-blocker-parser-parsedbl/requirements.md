# Requirements
## Project Context
- Tech Stack: Go
- Conventions: governance engine in `internal/engine`, model types in `internal/model`, CLI views in `cmd/`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Single prefix-aware blocker parser
REQ-001: The system MUST provide one `ParsedBlocker{Code, Subject, Detail, Raw}`
type and a single parser that decomposes a blocker (a `model.ReasonCode` or its
`code:subject:detail` spec) into Code (first segment), Subject (second segment,
e.g. a skill name or task ID), and Detail (remainder). This MUST be the only
decomposition point for prefix tokens; the existing ad-hoc split in
`cmd/next_skill_view.go` (`blockerSkillName`) MUST be refactored to call it.

#### Scenario: Three-segment stale token parses into code/subject/detail
GIVEN the blocker spec `required_skill_stale:plan-audit:assurance.md`
WHEN it is parsed
THEN Code is `required_skill_stale`, Subject is `plan-audit`, Detail is
`assurance.md`, and Raw is the original spec.

#### Scenario: Two-segment and bare tokens parse without panicking
GIVEN the specs `tasks_plan_changed_since_task_evidence:t-03` and `plan_audit_failed`
WHEN each is parsed
THEN the first yields Subject `t-03` with empty Detail, and the second yields
empty Subject and empty Detail, with Code set in both cases.

### Requirement: Code-keyed remediation vocabulary
REQ-002: The system MUST provide a remediation table parallel to
`canonicalReasonDefinitions`, keyed by Code, mapping each recovery-relevant
blocker to `{Remediation, CommandTemplate, RecoveryClass}`. Because every prefix
family's Code is its first segment, Code-keying MUST cover both exact codes and
prefix families. Lookups MUST return a non-empty remediation for every
recovery-relevant token (`required_skill_*`, `tasks_plan_changed_since_task_evidence`,
`stale_planning_*`, `stale_execution_evidence`, `plan_audit_*`, `scope_contract_*`,
`wave_*`, `verification_evidence_missing`, `preset_confirmation_required`,
`run_slipway_run_to_advance`), and CommandTemplates MUST be filled from the parsed
Subject/Detail.

#### Scenario: Stale skill token yields a rerun remediation and command
GIVEN a blocker `required_skill_stale:plan-audit:assurance.md`
WHEN its remediation is looked up
THEN the remediation names re-running the `plan-audit` skill, and the command
references a real public surface (`slipway run`, or `slipway evidence restamp
--skill plan-audit --dry-run`), with the skill drawn from the parsed Subject.

#### Scenario: Every table entry is non-empty
GIVEN the full remediation table
WHEN each entry is inspected
THEN every entry has a non-empty Remediation and a RecoveryClass.

### Requirement: Read-only recovery object on next/run/validate
REQ-003: `next --json`, `run --json`, and `validate --json` MUST carry a top-level
`recovery` object when an actionable blocker is present, containing one
`primary_command` / `primary_action` chosen by a static stage-priority rule and a
`steps` list. Actionable blockers MUST be grouped by their parsed `(Code,
Subject)`: each step carries its `Code`/`Subject`, the sorted, de-duplicated
`Details` the group spans, plus Remediation and Command — neither of which may
vary by Detail within a group, so one skill's many stale artifacts surface as a
single step rather than one step per artifact. The object MUST be absent
(omitempty) when no actionable recovery exists. `cmd/validate.go`, which has no
recovery channel today, MUST gain one. No blocker producer, gate, or state
transition may change.

#### Scenario: Blocked state surfaces a recovery object with a primary command
GIVEN an active change whose blockers include a `required_skill_stale` token
WHEN `validate --json` (and the compact `next`/`run` JSON) is produced
THEN `recovery.primary_command` is present and non-empty and `recovery.steps`
includes a step for the stale blocker with a non-empty remediation.

#### Scenario: One skill's stale artifacts collapse into a single step
GIVEN blockers `required_skill_stale:code-quality-review:CLAUDE.md` and
`required_skill_stale:code-quality-review:README.md`
WHEN the recovery object is produced
THEN it contains a single `code-quality-review` step whose `details` list both
artifacts, not one step per artifact.

#### Scenario: Clean state omits the recovery object
GIVEN an active change with no actionable blockers
WHEN the views are produced
THEN the `recovery` field is absent and the existing `blockers` array is
unchanged.

### Requirement: Compact handoff preserves the primary recovery command
REQ-004: The compact `next`/`run` handoff (`buildNextHandoffView`) MUST carry the
same `recovery` object — at minimum its `primary_command`/`primary_action` — so
the first surface a host hits is not left without a next action, unlike
`freshness_diagnostics` which the compact view intentionally strips.

#### Scenario: Primary recovery command survives the compact projection
GIVEN a blocked change whose full view carries a recovery primary command
WHEN the compact handoff view is built
THEN the compact JSON still contains `recovery.primary_command`.

### Requirement: Canonical messages for internal prefix tokens
REQ-005: Prefix tokens that currently fall through `humanizeReasonCode` to bare
titles MUST gain canonical messages in `canonicalReasonDefinitions`, at minimum
`tasks_plan_changed_since_task_evidence`, so no recovery-relevant token renders a
machine-derived title.

#### Scenario: tasks_plan_changed_since_task_evidence renders a written message
GIVEN a blocker `tasks_plan_changed_since_task_evidence:t-03`
WHEN its message is rendered
THEN the message is the written canonical sentence, not the humanized
"Tasks plan changed since task evidence".

### Requirement: CLIError surfaces the same recovery builder
REQ-006: `CLIError` MUST expose a `recovery` object built from its `Reasons` by
the same `model.BuildRecovery` the views use, so the error surface and the view
surfaces present an identical recovery vocabulary for the same blockers.

#### Scenario: Governance-blocked error carries a recovery object
GIVEN a governance-blocked `CLIError` constructed with reasons including an
actionable blocker
WHEN the error is encoded to JSON
THEN it carries a `recovery` object whose primary command matches what the views
would produce for the same reasons.

### Requirement: Read-only/additive contract and documentation
REQ-007: The change MUST be additive and read-only: all new JSON fields are
`omitempty`, the persisted `model.ReasonCode` shape (its yaml-tagged fields) is
unchanged, and no producer/gate/transition logic changes. The new host-facing
`recovery`/remediation JSON fields MUST be documented in README.md and CLAUDE.md.

#### Scenario: Persisted evidence shape is unchanged
GIVEN the change is applied
WHEN `model.ReasonCode` serialization to YAML is inspected
THEN it contains no new presentation fields, and existing verification/gate
records are byte-compatible.

#### Scenario: Docs describe the new fields
GIVEN README.md and CLAUDE.md after the change
WHEN the routed-command JSON documentation is read
THEN it describes the read-only `recovery` object and grouped remediation steps.
