## ADDED Requirements

### Requirement: Artifact Lifecycle State Model
Governed artifacts SHALL follow lifecycle states:
`draft -> in_review -> approved -> frozen`, with `stale` as a cross-cut state for non-frozen artifacts.

Stale artifacts SHALL NOT satisfy governed readiness checks.

#### Scenario: Stale after upstream update
- **WHEN** `spec.md` is updated after `design.md` approval
- **THEN** `design.md` SHALL be marked `stale`

### Requirement: Governance Boundary for Artifacts
Artifact governance SHALL distinguish lane types:

- Admission/direct lane (L1): runtime state artifact only
  - `.spln/runtime/admissions/<request_id>.yaml`
- Governed lane (L2/L3): change artifact set under `aircraft/changes/<slug>/`

`L1` SHALL NOT require governed artifact creation by default.

#### Scenario: L1 direct lane artifact boundary
- **WHEN** level is L1
- **THEN** governed artifact files SHALL not be mandatory

### Requirement: Governed Artifact Classes
Governed artifacts SHALL be classified as:

- Governance state:
  - `change.yaml`
  - `assurance.md`
- Plan/spec bundle:
  - `proposal.md`
  - `spec.md`
  - `design.md`
  - `tasks.md`
  - `explore.md` (L3)
- Evidence artifacts:
  - `.spln/evidence/**`

#### Scenario: Governed state lookup
- **WHEN** system evaluates governed action/gate status
- **THEN** runtime control data SHALL be read from `.spln/runtime/changes/<request_id>.yaml`, while governed manifest/artifact contract SHALL be read from `aircraft/changes/<slug>/change.yaml`

### Requirement: MVP risk documentation strategy
MVP SHALL NOT require a standalone `risk.md` artifact.
Risk analysis SHALL be documented in the risk section of `design.md`.
`assurance.md` SHALL reference risk decisions from `design.md` and record final closeout/verification outcomes.

#### Scenario: Guardrail change without risk.md file
- **WHEN** a guardrail-sensitive L2/L3 change is processed
- **THEN** risk findings SHALL be captured in `design.md` and SHALL NOT require creating `risk.md`

### Requirement: Required Artifact Sets by Level
Required artifacts SHALL be:

- L1: admission state only (no governed artifact requirement)
- L2: `change.yaml`, `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `assurance.md`
- L3: L2 set + `explore.md`

Ship-stage boundary:
- ship-stage approval set for `G_ship` is defined by gate-engine and excludes `explore.md`
- `explore.md` is required for L3 discovery/scope progression (`S2/S3 + G_scope`)

#### Scenario: L2 requires proposal.md
- **WHEN** level is L2
- **THEN** `proposal.md` SHALL be in the required governed artifact set

#### Scenario: L3 requires explore.md
- **WHEN** level is L3
- **THEN** `explore.md` SHALL be required before L3 scope readiness (`S2/S3 + G_scope`) passes

### Requirement: L3 Explore Minimum Structure (MVP)
For L3, `explore.md` SHALL satisfy a minimal structure contract (validated at `G_scope`):
- `Objectives`
- `Unknowns`
- `Assumptions`
- `Scope Boundaries`
- `Validation Plan`

Each section SHALL contain at least one non-empty bullet or paragraph.

Canonical markdown heading contract:
- section headings SHALL appear in the same order as above
- each required section SHALL use canonical heading form `## <Section Name>`
- additional subsections MAY exist, but SHALL NOT replace required canonical headings
- localized wording MAY appear in section body, but required canonical headings remain mandatory for deterministic parser checks

#### Scenario: Explore structure required for L3 scope readiness
- **WHEN** level is L3 and `explore.md` lacks one or more required sections
- **THEN** governed scope readiness SHALL fail until sections are completed

### Requirement: Assurance Minimum Structure (MVP)
For governed lanes (`L2/L3`), `assurance.md` SHALL satisfy a minimal closeout structure:
- `Scope Summary`
- `Verification Verdict`
- `Evidence Index`
- `Residual Risks and Exceptions`
- `Archive Decision`

Structure gate in MVP validates heading/section presence only.
Content-depth quality is evaluated by applicable review layers (`R1+`) when enabled.

#### Scenario: Assurance structure required for governed closeout
- **WHEN** `assurance.md` lacks one or more required sections
- **THEN** governed closeout readiness SHALL fail until required sections are completed

### Requirement: Governed Artifact DAG
The governed artifact dependency graph SHALL be:

- `proposal.md -> spec.md -> design.md -> tasks.md -> assurance.md`
- `explore.md -> design.md`

Stale propagation SHALL traverse downstream dependencies via BFS.

#### Scenario: Proposal update propagation
- **WHEN** `proposal.md` is updated
- **THEN** downstream governed artifacts (`spec/design/tasks/assurance`) SHALL be marked stale

### Requirement: Scaffold Policy
Artifact scaffolding SHALL be lane-aware:

- `spln new` in L2/L3 SHALL scaffold required governed artifacts from templates
- `spln new` in L1 SHALL not scaffold governed artifact bundle by default
- escalations to governed lane SHALL scaffold at escalation time

#### Scenario: L1 to L2 escalation scaffold
- **WHEN** pivot escalates L1 direct lane to L2
- **THEN** governed artifact scaffold SHALL be created during escalation

### Requirement: Artifact Versioning
Governed artifacts SHALL track versions in governed runtime change state `artifacts` map (`.spln/runtime/changes/<request_id>.yaml`).
`aircraft/changes/<slug>/change.yaml` SHALL remain minimal manifest metadata and SHALL NOT carry per-artifact version map in MVP.
Version increments on each approved content update.

#### Scenario: Design version increment
- **WHEN** `design.md` is updated and approved
- **THEN** `design.md` version in governed state SHALL increment by 1

### Requirement: Archive Freeze Rules
Archive freeze behavior SHALL follow the governed archive rules below.

On governed archive (`spln done` or `spln cancel`):
- all governed artifacts SHALL transition to `frozen`
- change artifact directory SHALL move to `aircraft/changes/archived/<slug>/`
- governed runtime change state SHALL move from `.spln/runtime/changes/<request_id>.yaml` to `.spln/archive/changes/<request_id>.yaml`
- linked `sealed_handoff` admission snapshot SHALL move from `.spln/runtime/admissions/<request_id>.yaml` to `.spln/archive/admissions/<request_id>.yaml`

Admission/direct-lane records SHALL keep runtime/audit boundary:
- runtime admissions for active/direct execution history
- archived admissions for:
  - governed handoff snapshots after governed archive
  - direct-lane records archived by `spln done`
  - direct-lane and governed cancellation records archived by `spln cancel`
- no automatic deletion

#### Scenario: Governed archive freeze
- **WHEN** governed archive completes for L2/L3 via `spln done` or `spln cancel`
- **THEN** governed artifacts SHALL be frozen and archived

#### Scenario: Governed archive migrates linked admission snapshot
- **WHEN** governed archive completes for governed change with linked `sealed_handoff` admission
- **THEN** linked admission snapshot SHALL be migrated to `.spln/archive/admissions/` and removed from runtime admissions

### Requirement: Admission Record Lifecycle
Admission artifacts SHALL follow explicit lifecycle statuses:
- `active`: admission/direct lane in progress
- `done`: L1 finished
- `cancelled`: L1 explicitly cancelled
- `sealed_handoff`: governed handoff completed (`L2/L3`)

For `sealed_handoff`, admission artifact SHALL remain immutable snapshot and SHALL only provide link/reference to governed change.
For `sealed_handoff`, admission artifact `current_state` SHALL remain `S1_ANALYZE` (last admission-phase executed state).

Admission artifacts are audit records and SHALL NOT be auto-deleted by runtime progression.

#### Scenario: Sealed handoff admission record
- **WHEN** L1 pivots to L2 and governed handoff completes
- **THEN** admission artifact status SHALL be `sealed_handoff` and further execution mutations SHALL occur only in change artifact
