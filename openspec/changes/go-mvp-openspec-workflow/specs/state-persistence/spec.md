## ADDED Requirements

### Requirement: Filesystem-Only Persistence
MVP SHALL use filesystem persistence only. No runtime DB is required.

Persistent paths:
- `.speclane/config.yaml`
- `.speclane/runtime/admissions/<request_id>.yaml`
- `.speclane/runtime/changes/<request_id>.yaml`
- `.speclane/runs/<request_id>.yaml`
- `.speclane/archive/admissions/<request_id>.yaml`
- `.speclane/archive/changes/<request_id>.yaml`
- `.speclane/archive/runs/<request_id>.yaml`
- `.speclane/archive/config/config.yaml.broken.<timestamp>.yaml`
- `aircraft/changes/<slug>/change.yaml`

MVP SHALL NOT require `.speclane/evidence/skills/`, `.speclane/evidence/tasks/`, `.speclane/evidence/runs/`.

#### Scenario: Fresh init persistence footprint
- **WHEN** `speclane init` runs in a fresh repo
- **THEN** runtime SHALL be usable with only the filesystem paths listed above

### Requirement: Typed YAML Serialization
Runtime state reads/writes SHALL use typed Go structs with `yaml.v3`.

#### Scenario: Round-trip integrity
- **WHEN** admission/change/run record is saved and reloaded
- **THEN** state SHALL round-trip without semantic loss

### Requirement: Config Unknown-Key Preservation
Top-level unknown keys in `.speclane/config.yaml` SHALL be preserved on rewrite.

#### Scenario: Preserve custom top-level config keys
- **WHEN** known config fields are updated
- **THEN** unknown top-level keys SHALL remain in rewritten config

### Requirement: Config Repair Flow
Malformed config SHALL be recoverable by `speclane repair`.

Repair contract:
- back up malformed source to `.speclane/archive/config/config.yaml.broken.<timestamp>.yaml`
- write deterministic MVP defaults

#### Scenario: Config corruption recovery
- **WHEN** config parsing fails
- **THEN** `speclane repair` SHALL restore runnable defaults with backup

### Requirement: Atomic Write Contract
All mutation writes SHALL be atomic:
1. write temp file in target directory
2. `fsync` temp file
3. rename over target
4. `fsync` parent directory

#### Scenario: Interrupted write safety
- **WHEN** process crashes during write
- **THEN** persisted file SHALL remain old-or-new, never partial

### Requirement: Concurrency Lock Contract
Mutations SHALL use exclusive `.speclane/state.lock`.

Config knobs:
- `execution.lock_wait_timeout_seconds` (default `10`)
- `execution.cancel_grace_period_seconds` (default `10`)

Stale lock cleanup:
- regular mutating commands SHALL NOT force-unlock
- `speclane repair` MAY clear stale lock when lock age exceeds deterministic runtime threshold

#### Scenario: Lock timeout
- **WHEN** lock cannot be acquired within timeout
- **THEN** mutation command SHALL fail without state change

### Requirement: Single Active Request (MVP)
At most one active request SHALL exist across admission + governed runtime files.

- `speclane new` SHALL reject when active set is not empty
- request-scoped commands (`do/done/cancel/pivot/analyze/review`) require exactly one active request
- `status/context` SHALL still run in diagnostics mode when active set is `0` or `>1`

#### Scenario: Active-context ambiguity
- **WHEN** runtime contains multiple active requests
- **THEN** request-scoped commands SHALL fail with deterministic remediation

### Requirement: Admission State Ownership
Admission state (`.speclane/runtime/admissions/<request_id>.yaml`) SHALL own:
- `S0/S1` for all levels
- full L1 execution path (`S6/S7/S8`) when route is L1
- route snapshot + intake assessment + level metadata

#### Scenario: L1 stays in admission lane
- **WHEN** routed level is L1
- **THEN** workflow progression SHALL persist only in admission runtime state

### Requirement: Governed Change State Ownership
Governed state (`.speclane/runtime/changes/<request_id>.yaml`) SHALL own L2/L3 execution from governed entry onward.

Required fields include:
- `request_id`, `slug`, `level`, `current_state`, `artifacts`, `gates`
- optional `worktree_path`, `worktree_branch` for L3 scope checks

#### Scenario: Governed handoff
- **WHEN** route is L2 or L3
- **THEN** governed runtime file SHALL become mutable execution source

### Requirement: Run Record Ownership
Run record (`.speclane/runs/<request_id>.yaml`) SHALL be the single request-scoped execution/checkpoint/check-result ledger.

Run record SHALL include at least:
- `request_id`
- `checks[]` (command checks)
- `human_confirmations[]`
- `wave_summaries[]` (append-only frozen summaries)
- `latest_summary_version` (pointer to latest frozen summary version, `0` when none)
- `history[]` (optional append-only event stream)

Run record SHALL also persist override events when operator approves continuation after failed checks.

Authority boundary:
- run record SHALL NOT be the authoritative source for lifecycle `current_state`, `level`, or lane status
- authoritative lifecycle state remains in admission/change runtime files

Check record contract:
- each `checks[]` item SHALL include `check_id`
- each `human_confirmations[]` item SHALL include `check_id`
- when operator override is applied to a failed command check, the corresponding `checks[]` item SHALL persist:
  - `override=true`
  - `override_note` (optional)
  - `override_at` timestamp

History contract:
- each `history[]` event SHALL include `event` and `at`
- `detail` is optional
- `history[]` is scoped to run-ledger events (check execution, summary freeze, checkpoint/override trace), not full workflow state-transition ownership

#### Scenario: Check + confirmation trace
- **WHEN** a gate is evaluated
- **THEN** related command-check results and confirmation answers SHALL be persisted in run record

### Requirement: Frozen Wave Summary in Run Record
Each wave execution attempt SHALL produce a frozen summary snapshot in run record.

Summary contract:
- monotonically incrementing `summary_version` per request, starting at `1`
- include completed/non-pass tasks and unresolved blockers
- new snapshot SHALL append into `wave_summaries[]` (no overwrite)
- prior summary entries remain immutable; `latest_summary_version` pointer may advance

#### Scenario: Retry produces new summary version
- **WHEN** retry creates a new wave result snapshot
- **THEN** `summary_version` SHALL increment and prior snapshots SHALL remain intact

### Requirement: Archive Persistence
Terminal lifecycle transitions (`done`/`cancel`) SHALL archive by request:
- direct lane: move admission runtime to `.speclane/archive/admissions/<request_id>.yaml`
- governed lane: move change runtime to `.speclane/archive/changes/<request_id>.yaml` and linked sealed admission to archive admissions
- both lanes: move run record to `.speclane/archive/runs/<request_id>.yaml`

Governed artifact bundle SHALL move:
- `aircraft/changes/<slug>/` -> `aircraft/changes/archived/<slug>/`

#### Scenario: Governed done archive
- **WHEN** governed request reaches terminal done
- **THEN** governed runtime + linked admission + artifact bundle SHALL be archived by request

### Requirement: Non-Executable Intake Has No Runtime Writes
For `non_speclane` intake outcomes, runtime SHALL not create admission/change/run files.

#### Scenario: Advisory request no-op persistence
- **WHEN** analyze classifies intake as advisory/non-executable
- **THEN** no request-scoped runtime file SHALL be created
