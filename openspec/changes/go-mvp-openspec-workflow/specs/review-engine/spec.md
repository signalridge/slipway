## ADDED Requirements

### Requirement: Layered artifact review (R0-R3)
The system SHALL implement four artifact review layers executed in order: R0 (structural validation), R1 (intra-artifact quality), R2 (cross-artifact consistency), R3 (risk and safety review). All layers SHALL always exist in the taxonomy; level/guardrail rules control execution depth and blocking thresholds.

#### Scenario: R0 structural validation
- **WHEN** R0 runs on spec.md
- **THEN** it SHALL check template/schema correctness, required sections present, section IDs valid, artifact metadata consistency

#### Scenario: R1 intra-artifact quality
- **WHEN** R1 runs on design.md
- **THEN** it SHALL check clarity, completeness, testability, anti-overbuild, anti-underbuild

#### Scenario: R2 cross-artifact consistency
- **WHEN** R2 runs
- **THEN** it SHALL check REQ -> DEC -> TASK -> TEST -> EVID linkability, upstream/downstream coherence, stale dependencies resolved

#### Scenario: R3 risk and safety review
- **WHEN** R3 runs on a guardrail-domain change
- **THEN** it SHALL check security/privacy handling, data integrity, rollback feasibility, external contract compatibility

### Requirement: Implementation review layers (IR1-IR3)
The system SHALL implement three implementation review layers for code/task deltas: IR1 (spec compliance), IR2 (code quality and test quality), IR3 (safety/security). Execution order for applicable layers SHALL be fixed: IR1 before IR2 before IR3.

#### Scenario: IR1 spec compliance check
- **WHEN** IR1 runs on implementation deltas
- **THEN** it SHALL verify code changes align with spec requirements

#### Scenario: IR3 triggered by guardrail domain
- **WHEN** guardrail domain is present
- **THEN** IR3 SHALL be required for governed lanes (`L2/L3`)

### Requirement: Level- and Guardrail-Controlled Review Depth
Review execution SHALL be controlled by level and guardrail presence:
- `L1`: no mandatory governance review layers by default
- `L2/L3` without guardrail domain: minimum required set is `IR1 + R0`
- `L2/L3` with guardrail domain: minimum required set is `IR1 + IR3 + R0 + R3`
- `R1`, `R2`, and `IR2` are optional in MVP and run only when explicitly requested by policy/operator
- optional layers MUST NOT become implicit blockers in baseline MVP flow

L1 lightweight boundary:
- L1 `S7_REVIEW` / `S8_VERIFY` lightweight checks are governed by action-workflow/cli contracts, not by governance `R*`/`IR*` review layers

Routing compatibility note:
- in MVP auto mode, guardrail-domain requests route to `L3`; guardrail review depth rules above apply to any governed context where guardrail is present

#### Scenario: L2 baseline review
- **WHEN** level is L2 and no guardrail domain
- **THEN** required review layers SHALL be `IR1 + R0` only in MVP baseline

#### Scenario: Guardrail review escalation
- **WHEN** guardrail domain is present in governed lane
- **THEN** required review layers SHALL be `IR1 + IR3 + R0 + R3` before review pass

#### Scenario: IR2 triggered by executable deltas
- **WHEN** reviewed scope contains code/test deltas and operator/policy requests deeper implementation checks
- **THEN** IR2 MAY run as optional depth extension

### Requirement: Changed-only default review
The system SHALL default to reviewing only changed and stale artifacts/units. Full review SHALL be available via explicit `spln review --all` flag.

Changed-only scope resolution contract:
- governed artifacts are in-scope when artifact state is `draft` or `stale`, or artifact version advanced since last passing review evidence for current request
- implementation units are in-scope when latest frozen run summary for current `run_summary_version` reports changed files or non-pass debt
- MVP does not support `--artifact` targeting; selection is automatic (`--changed-only` default) or full (`--all`)

#### Scenario: Changed-only review
- **WHEN** `spln review` runs without flags
- **THEN** only artifacts marked as changed or stale SHALL be reviewed

#### Scenario: Full review override
- **WHEN** `spln review --all` is run
- **THEN** all artifacts SHALL be reviewed regardless of change status

### Requirement: Review fail protocol
When a review layer finds a blocker, the system SHALL enter a fix -> re-review loop. When two consecutive review failures are caused by intent drift, the system SHALL mark `pivot_required` and require explicit operator pivot intent (`spln pivot`) before `G_pivot` evaluation.

#### Scenario: Blocker triggers re-review
- **WHEN** IR1 finds a spec compliance blocker
- **THEN** the system SHALL require a fix and re-run IR1

#### Scenario: Intent drift triggers pivot
- **WHEN** two consecutive review failures are attributed to intent drift
- **THEN** review output SHALL mark `pivot_required` with remediation to run `spln pivot`; `G_pivot` evaluation SHALL occur only after explicit pivot invocation

### Requirement: Reviewer independence for L2/L3
For L2 and L3 changes, review approvals from artifact-review and final-closeout skills SHALL be produced by a reviewer identity different from the primary implementer. Different identity means different subagent session (session_id) with fresh context.

Reviewer-independence comparison contract:
- implementer baseline SHALL be selected by `(request_id, run_summary_version)` from latest `wave-orchestration` evidence for the same request/version
- reviewer evidence (`artifact-review`, `final-closeout`) SHALL carry the same `run_summary_version` as the reviewed frozen run summary
- reviewer evidence SHALL be blocked when reviewer `session_id` equals implementer baseline `session_id` for the same `(request_id, run_summary_version)`
- if implementer baseline is missing, governed review readiness SHALL be blocked with remediation to emit `wave-orchestration` evidence first

#### Scenario: L2 reviewer independence
- **WHEN** Level is L2 and artifact-review runs
- **THEN** the reviewer session_id SHALL differ from the implementer session_id

#### Scenario: Missing implementer baseline blocks governed review readiness
- **WHEN** governed review evidence exists but no `wave-orchestration` implementer baseline session is available for the current run summary
- **THEN** review readiness SHALL be blocked with remediation to produce missing implementer baseline evidence

#### Scenario: Review evidence version mismatch blocks readiness
- **WHEN** `artifact-review` evidence `run_summary_version` does not match the currently frozen run summary version
- **THEN** governed review readiness SHALL be blocked with remediation to rerun review on current run summary

### Requirement: Review and wave conflict resolution
Review subagent execution SHALL consume immutable wave run summaries, not live in-progress wave state.

Conflict rules:
- `S6_RUN_WAVES` and `S7_REVIEW` SHALL NOT mutate the same task status concurrently
- review reads the latest completed run summary snapshot from `.spln/evidence/runs/<request_id>/rv<latest_frozen_run_summary_version>.json`
- if review requests fixes, control returns to `S6` for a new run summary version

#### Scenario: Review starts while a wave is still running
- **WHEN** current wave run is incomplete
- **THEN** review SHALL be blocked until run summary is finalized

#### Scenario: Review findings require re-run
- **WHEN** review flags blockers on completed wave output
- **THEN** workflow SHALL transition to `S6_RUN_WAVES` and generate a new run summary version before next review

### Requirement: Artifact review matrix
For governed lane artifacts, the system SHALL apply review layers as defined:
- baseline (no guardrail): changed governed artifacts run `R0` only
- guardrail-sensitive governed scope: changed governed artifacts run `R0 + R3`
- `change.yaml` always includes `R0` only when in reviewed scope (MVP manifest is minimal snapshot metadata)

Consolidated MVP review matrix reference:

| Scope condition | Reviewed unit | Required layers | Notes |
|---|---|---|---|
| governed baseline (no guardrail) | changed governed artifacts (`proposal/spec/design/tasks/assurance`) | `R0` | `R1/R2/R3` optional only when explicitly escalated |
| governed baseline (no guardrail) | implementation deltas | `IR1` | `IR2` optional |
| guardrail-sensitive governed scope | changed governed artifacts (`proposal/spec/design/tasks/assurance`) | `R0 + R3` | `R1/R2` optional; guardrail safety review cannot be skipped |
| guardrail-sensitive governed scope | implementation deltas | `IR1 + IR3` | `IR2` optional |
| any governed scope | `change.yaml` (when reviewed) | `R0` only | manifest stays lightweight in MVP; only structure/identifier integrity is required |
| L3 discovery/scope stage | `explore.md` | controlled by `S2/S3` + `G_scope` | out of ship-stage review matrix scope |

Guardrail precedence rule:
- when guardrail rules and default matrix rows conflict, guardrail-required `R3`/`IR3` behavior SHALL take precedence

`explore.md` SHALL be governed by L3 discovery/scope controls (`S2/S3`, `G_scope`) and is out of ship-stage artifact review matrix scope for MVP.

#### Scenario: Design R3 conditional requirement
- **WHEN** a governed change is guardrail-sensitive
- **THEN** `design.md` SHALL include R3 review before ship readiness passes

#### Scenario: `change.yaml` always stays lightweight in MVP
- **WHEN** `change.yaml` is in reviewed scope for a governed request
- **THEN** review execution SHALL require R0 only for `change.yaml` in MVP (presence, schema shape, and identifier integrity)

### Requirement: Superpowers-Aligned Review Discipline
Review flow SHALL align with superpowers principles:
- review early against each new frozen run summary version
- no pass/complete claim without fresh evidence in reviewed scope

#### Scenario: New run summary requires new review
- **WHEN** `S6` emits a new frozen run summary version
- **THEN** prior review pass SHALL NOT be reused, and `S7` SHALL run on the new version
