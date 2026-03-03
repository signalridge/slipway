## ADDED Requirements

### Requirement: Artifact Lifecycle States
Governed artifacts SHALL use:
`draft -> in_review -> approved -> frozen`, with `stale` as non-frozen cross-cut state.

#### Scenario: Stale propagation
- **WHEN** upstream artifact changes
- **THEN** dependent artifacts SHALL be marked `stale`

### Requirement: Lane Artifact Boundary
Artifact requirements SHALL vary by lane level.

- L1/direct lane: no governed artifact bundle required by default
- L2/L3/governed lane: required bundle under `aircraft/changes/<slug>/`

#### Scenario: L1 no governed artifacts by default
- **WHEN** level is L1
- **THEN** governed artifact files SHALL not be mandatory

### Requirement: Governed Required Bundle
Governed lanes SHALL enforce minimum required artifact bundle.

Required for L2:
- `change.yaml`, `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `assurance.md`

L3 adds:
- `explore.md`

#### Scenario: L3 requires explore
- **WHEN** level is L3
- **THEN** `explore.md` SHALL be required before scope gate pass

### Requirement: Explore Minimum Structure
`explore.md` SHALL include ordered headings:
- `## Objectives`
- `## Unknowns`
- `## Assumptions`
- `## Scope Boundaries`
- `## Validation Plan`

Each section SHALL contain non-empty content.

#### Scenario: Missing explore section blocks scope
- **WHEN** any required heading is missing or empty
- **THEN** scope readiness SHALL fail

### Requirement: Assurance Minimum Structure
`assurance.md` SHALL include:
- `## Scope Summary`
- `## Verification Verdict`
- `## Evidence Index`
- `## Residual Risks and Exceptions`
- `## Archive Decision`

#### Scenario: Missing assurance section blocks closeout
- **WHEN** required heading is missing
- **THEN** closeout readiness SHALL fail

### Requirement: Assurance Ownership and Update Timing
Governed assurance content SHALL be updated by review/verify stages (not by ad-hoc manual timing).

Ownership contract:
- at `S7_REVIEW`, review stage SHALL update:
  - `## Scope Summary`
  - `## Evidence Index`
  - `## Residual Risks and Exceptions`
- at `S8_VERIFY`, verify stage SHALL update:
  - `## Verification Verdict`
  - `## Archive Decision`

Closeout readiness SHALL require these sections to reflect latest frozen summary version.

#### Scenario: Verify updates archive decision
- **WHEN** governed flow reaches `S8_VERIFY`
- **THEN** assurance SHALL include updated verification verdict and archive decision before closeout pass

### Requirement: Artifact DAG
Governed artifacts SHALL follow deterministic dependency DAG.

Governed artifact dependency graph:
- `proposal -> spec -> design -> tasks -> assurance`
- `explore -> design`

#### Scenario: Proposal update propagates staleness
- **WHEN** `proposal.md` changes
- **THEN** downstream artifacts SHALL be marked stale

### Requirement: Scaffold Policy
Artifact scaffolding SHALL follow lane-aware entry rules.

- L2/L3: scaffold governed bundle on governed entry
- L1: no governed scaffold by default
- L1 -> governed escalation: scaffold at escalation point

#### Scenario: L1 escalates to L2
- **WHEN** pivot escalates L1 to governed lane
- **THEN** governed bundle SHALL be scaffolded at escalation time

### Requirement: Archive Freeze Rules
Archive flow SHALL freeze governed artifacts before completion.

On governed archive (`done` or `cancel`):
- all governed artifacts become `frozen`
- move `aircraft/changes/<slug>/` -> `aircraft/changes/archived/<slug>/`
- move runtime change file to `.speclane/archive/changes/<request_id>.yaml`
- move linked sealed admission snapshot to `.speclane/archive/admissions/<request_id>.yaml`
- move run record to `.speclane/archive/runs/<request_id>.yaml`

On direct-lane archive:
- move runtime admission to `.speclane/archive/admissions/<request_id>.yaml`
- move run record to `.speclane/archive/runs/<request_id>.yaml`

#### Scenario: Governed archive freeze
- **WHEN** governed archive completes
- **THEN** archived governed artifacts SHALL all be `frozen`
