# cli-commands Specification

## Purpose
TBD - created by archiving change go-mvp-openspec-workflow. Update Purpose after archive.
## Requirements
### Requirement: Command Surface
The system SHALL implement 11 commands.

Core commands:
- `init`, `new`, `do`, `status`, `context`, `done`, `cancel`, `pivot`

Repair command:
- `repair`

Governance override commands:
- `analyze`, `review`

#### Scenario: Command availability
- **WHEN** `spln --help` is run
- **THEN** all 11 commands SHALL be listed

### Requirement: CLI Failure Contract
Command failures SHALL be deterministic and machine-integrable.

Source-of-truth binding:
- exit-code taxonomy and error envelope schema defined in this requirement are the canonical MVP contract for CLI/runtime behavior
- design/proposal documents MAY restate this contract for rationale, but SHALL NOT redefine conflicting taxonomy values or envelope fields

Exit code taxonomy:
- `0`: success
- `2`: invalid usage / argument contract violation
- `3`: precondition blocked
- `4`: state integrity/consistency failure
- `5`: governance blocked
- `6`: runtime execution failure

Category-to-exit-code mapping table:

| Failure category | Exit code | Typical examples |
|---|---|---|
| invalid_usage | `2` | unsupported flags, invalid argument combination |
| precondition_blocked | `3` | no active request, ambiguous active context, lock timeout, `non_spln_intent` intake rejection |
| state_integrity | `4` | malformed state key/value identity, required metadata inconsistency |
| governance_blocked | `5` | gate blocked, required review/evidence missing |
| runtime_failure | `6` | unexpected internal/tool execution failure |

Failure output contract:
- on non-zero exit, command SHALL emit structured JSON error envelope to stderr with:
  - `error_code`
  - `category`
  - `message`
  - `remediation`
  - `exit_code`
  - optional `request_id`
  - optional `details`
- human-readable text MAY be emitted, but JSON envelope SHALL be parse-stable

#### Scenario: Invalid usage failure contract
- **WHEN** command is invoked with invalid/unsupported flag contract
- **THEN** exit code SHALL be `2` and stderr SHALL include structured error envelope with actionable remediation

#### Scenario: Governance gate block failure contract
- **WHEN** command progression is blocked by gate/review/evidence contract
- **THEN** exit code SHALL be `5` and stderr SHALL include structured error envelope listing blockers

### Requirement: `spln init`
`spln init` SHALL create:
- `.spln/config.yaml`
- `.spln/runtime/admissions/`
- `.spln/runtime/changes/`
- `.spln/evidence/`
- `.spln/evidence/skills/`
- `.spln/evidence/tasks/`
- `.spln/evidence/runs/`
- `.spln/archive/`
- `.spln/archive/admissions/`
- `.spln/archive/changes/`
- `.spln/archive/config/`
- `aircraft/changes/`
- `aircraft/changes/archived/`

Runtime-minimality contract:
- `spln init` SHALL create only runtime-essential files/directories under `.spln/`
- `spln init` SHALL NOT create non-executable narrative files under `.spln/` (for example, `.spln/README.md`)

It SHALL support `--tools <list>`.
`--tools all` SHALL generate tool artifacts for all supported adapters.
`--tools none` SHALL skip tool artifact generation.
`spln init --refresh` SHALL regenerate tool artifacts deterministically.
`--tools`/`--refresh` semantics are sidecar-only and SHALL NOT change core runtime layout/init success criteria.
Core workflow commands (`new/do/status/context/done/cancel/pivot/repair/analyze/review`) SHALL remain usable when tool artifacts are not generated (for example, `--tools none`).

#### Scenario: Fresh init
- **WHEN** `spln init --tools claude` runs on an uninitialized repo
- **THEN** required directories and tool artifacts SHALL be created

#### Scenario: Init keeps runtime layout minimal
- **WHEN** `spln init` runs
- **THEN** `.spln/README.md` SHALL NOT be created

#### Scenario: Init without tool artifacts keeps core workflow available
- **WHEN** `spln init --tools none` runs
- **THEN** runtime state layout SHALL be created and core workflow commands SHALL remain usable without generated tool artifacts

### Requirement: `config.yaml` Schema (MVP)
`spln init` SHALL write `.spln/config.yaml` with explicit MVP keys:
- `tools[]`
- `defaults.level_mode` (`auto` default)
- `execution.parallelization` (`true` default)
- `execution.max_retries_per_task` (`2` default)
- `execution.lock_wait_timeout_seconds` (`10` default)
- `execution.lock_stale_after_seconds` (`120` default)
- `execution.cancel_grace_period_seconds` (`10` default)
- `execution.evidence_retention_days` (`30` default)
- `execution.evidence_gc_low_disk_free_mb` (`512` default)
- `execution.max_level_history_entries` (`100` default)

Unknown top-level keys SHALL be ignored but preserved on rewrite.
Nested unknown keys inside known top-level sections are out of MVP compatibility scope and MAY be dropped on rewrite.

#### Scenario: Init config defaults
- **WHEN** `spln init` runs without explicit options
- **THEN** config SHALL contain the MVP defaults above

### Requirement: Config Parse/Validation Failure Handling
Config load failure SHALL be deterministic and repairable.

Failure handling contract:
- malformed/unparseable `.spln/config.yaml` SHALL fail config-consuming commands as state-integrity failure
- command stderr SHALL include remediation to run `spln repair`
- `spln repair` SHALL back up broken config to `.spln/archive/config/config.yaml.broken.<timestamp>.yaml` and rewrite deterministic MVP defaults

#### Scenario: Corrupted config blocks command with repair remediation
- **WHEN** `.spln/config.yaml` is malformed and `spln new` is invoked
- **THEN** command SHALL fail as state-integrity error with remediation to run `spln repair`

### Requirement: `spln new` (route-first behavior)
`spln new "<description>"` SHALL:
1. capture intake payload (`S0_INTAKE`)
2. run `S1_ANALYZE`
3. classify executable vs non-executable intent using structured semantic intake assessment
4. if executable, resolve level via `--level` mode
5. if fixed-level safety checks report hard conflict, fail before creating `request_id` or runtime state
6. otherwise create admission record
7. persist route result to admission state as `route_snapshot`
8. persist semantic classifier output to admission state as `intake_assessment`
9. create governed change artifacts only when level is `L2` or `L3`
10. persist routed landing `current_state` before command returns:
   - `L1 -> S6_RUN_WAVES` (admission lane)
   - `L2 -> S4_SPEC_BUNDLE` (governed lane)
   - `L3 -> S2_DISCOVER` (governed lane)

Responsibility split:
- `S0_INTAKE`: capture level mode input only
- `S1_ANALYZE`: compute or validate final level result and semantic executable-intent classification

Supported flags:
- `--level auto|L1|L2|L3`

Not supported:
- `--scores`

Level selection behavior:
- explicit `--level L1|L2|L3` => fixed mode
- `--level auto` => auto mode
- omitted `--level` + valid `defaults.level_mode` in config => use config value
- omitted `--level` + TTY => prompt `auto|L1|L2|L3` with config value as default option
- omitted `--level` + non-TTY => apply config value directly
- if config value is missing/invalid => fallback to `auto` with deterministic remediation hint

#### Scenario: L1 route does not create governed change
- **WHEN** `spln new "fix login" --level L1` is run
- **THEN** admission state SHALL be created and `aircraft/changes/<slug>/` SHALL NOT be required

#### Scenario: Pure Q&A rejected from speclane new
- **WHEN** `spln new` input is identified as pure Q&A/advisory with no executable change intent
- **THEN** command SHALL classify route as `non_spln`, reject request creation with remediation to use normal chat flow, and SHALL NOT create `request_id` or runtime state files

Failure mapping for `non_spln` rejection:
- `category=precondition_blocked`
- `exit_code=3`
- `error_code=non_spln_intent`

#### Scenario: Fixed-level hard conflict rejected before persistence
- **WHEN** `spln new "..." --level L1` is run and `S1_ANALYZE` detects hard guardrail conflict
- **THEN** command SHALL fail before `request_id` generation/state creation and return remediation to rerun with `--level auto|L3`

Executable-intent check for `spln new` SHALL use routing-engine AI-first semantic contract:
- classifier output MUST include structured `intake_assessment` with at least:
  - `intent_type`, `is_executable`, `confidence`, `change_targets[]`, `intended_delta`, `acceptance_anchor`, `blocking_unknowns[]`
- deterministic consumer thresholds SHALL be sourced from routing-engine semantic contract

#### Scenario: L3 route creates governed change
- **WHEN** `spln new "re-architect auth" --level L3` is run
- **THEN** `aircraft/changes/<slug>/` SHALL be created with governed required artifacts and `.spln/runtime/changes/<request_id>.yaml` SHALL be created for runtime state

#### Scenario: New persists routed landing state
- **WHEN** `spln new` completes with executable intake
- **THEN** `current_state` SHALL be persisted to the first execution state of routed level (`S6` for L1, `S4` for L2, `S2` for L3)

#### Scenario: Invalid configured level mode falls back safely
- **WHEN** operator omits `--level` and `.spln/config.yaml` has invalid `defaults.level_mode`
- **THEN** command SHALL fall back to `auto` and include deterministic remediation to fix config

### Requirement: Request ID and Slug Generation
`spln new` SHALL generate IDs using fixed MVP scheme `request_v1`:
- `request_id=uuidv7` (UUIDv7 lowercase canonical string)
- governed `slug={title_kebab}` (L2/L3 only)
- collision handling uses numeric suffixes (`-2`, `-3`, ...)

Slug collision handling:
- if target slug directory already exists, append `-2`, `-3`, ... until unique

ID immutability:
- `request_id` and `slug` SHALL NOT change after initial creation

#### Scenario: New admission id format
- **WHEN** `spln new` creates an admission record
- **THEN** `request_id` SHALL use fixed MVP scheme (`request_v1` -> `uuidv7`)

#### Scenario: Slug collision suffixing
- **WHEN** generated slug path already exists
- **THEN** system SHALL create a suffixed slug (`-2`, `-3`, ...) without overwriting existing `aircraft/changes/<slug>/` directories

### Requirement: Active Request Context Resolution (MVP)
MVP command execution SHALL use implicit active-request resolution with strict ambiguity blocking.

Scope:
- request-scoped commands requiring unique active context: `do`, `done`, `cancel`, `pivot`, `analyze`, `review`
- diagnostics-capable commands: `context`, `status`
- active records are runtime lane files with lifecycle status `active`:
  - admission lane: `admission_status=active`
  - governed lane: `change_status=active`
- terminal/non-active statuses are excluded: `done`, `cancelled`, `sealed_handoff`
- `context` and `status` SHALL remain available as diagnostics-first commands even when active set size is `0` or `>1`

Resolution contract:
- exactly one active request => request-scoped commands proceed with that request; `context`/`status` MAY return request-scoped payloads
- zero active requests => request-scoped commands fail with remediation (`spln new`); `context`/`status` return diagnostics payloads
- more than one active request => request-scoped commands fail with ambiguity remediation; `context`/`status` return diagnostics payloads
- `spln status` with active set size `0` or `>1` SHALL return global integrity diagnostics; request-scoped progression fields are omitted in this mode
- `spln context` with active set size `0` or `>1` SHALL return compact diagnostics context (`lane_mode=diagnostics`, `evidence_freshness=unknown`) without requiring request-scoped progression fields

MVP SHALL NOT require an explicit `--request` flag.
Instead, runtime SHALL enforce at most one active request for deterministic command behavior.

`spln new` precondition:
- if any active request exists, request creation SHALL be blocked until that request reaches terminal lifecycle status

#### Scenario: New blocked by active request
- **WHEN** `spln new "add metrics"` is run and one active request already exists
- **THEN** command SHALL fail with remediation to finish/cancel/archive the active request first

#### Scenario: Ambiguous active context blocks execution
- **WHEN** `spln do` is run and runtime contains multiple active requests
- **THEN** command SHALL fail with explicit ambiguity error and remediation to run `spln status` (and `spln repair` for repairable faults) to restore deterministic runtime state

### Requirement: `spln do`
`spln do` SHALL execute one next action from current lane state.

- admission/direct lane source: admission state file
- governed lane source: change state file

It SHALL persist state updates atomically and append one action history event.

When governed lane is active, `spln do` SHALL treat linked admission record as sealed snapshot and SHALL NOT mutate admission progression fields.

For L3 scope confirmation:
- when `current_state=S3_SCOPE_CONFIRMATION`, `spln do` SHALL persist `worktree_path` and `worktree_branch` into governed runtime state before evaluating `G_scope`
- `spln do` SHALL validate persisted worktree metadata authenticity before `G_scope`:
  - `worktree_path` exists and is accessible
  - `worktree_path` is a Git worktree of current repository
  - checked-out branch at `worktree_path` equals persisted `worktree_branch`

Decision/resume contract for `S6_RUN_WAVES`:
- when execution is paused by checkpoint (`decision|human_verify|human_action`) or blocked by non-pass outcomes, `spln do` SHALL emit/consume a checkpoint continuation contract
- checkpoint payload SHALL include a `resume_signal` and, when applicable, explicit options for valid responses
- interactive TTY mode MAY prompt for user response text directly
- non-interactive mode SHALL require explicit `--resume-response "<text>"`
- response text SHALL be persisted in continuation evidence (`user_response_payload`) and validated against checkpoint expectations
- invalid or unsupported responses SHALL fail with deterministic remediation including expected response hints

L1 lightweight auto-check contract:
- when active lane is L1 and `current_state=S6_RUN_WAVES`, the same `spln do` invocation SHALL:
  1. execute one S6 action
  2. evaluate `S7_REVIEW` lightweight checks
  3. evaluate `S8_VERIFY` lightweight checks (only if `S7` passed)
- both checks pass => persist `current_state=S8_VERIFY` as done-ready (not `DONE`) and return remediation to run `spln done`
- any lightweight check fails => persist `current_state=S6_RUN_WAVES` with blockers
- MVP SHALL NOT require a separate L1-only review/verify command

#### Scenario: Level-aware progression
- **WHEN** `spln do` runs after `spln new`
- **THEN** progression SHALL follow level path rules from the action-workflow contract

#### Scenario: L1 auto-checks are command-owned
- **WHEN** active lane is L1 and `spln do` is run from `S6_RUN_WAVES`
- **THEN** `spln do` SHALL evaluate `S7` and `S8` lightweight checks in the same invocation before returning

#### Scenario: Non-interactive checkpoint requires explicit response
- **WHEN** `spln do` is invoked in non-interactive mode while `S6` requires checkpoint continuation
- **THEN** command SHALL fail unless `--resume-response` is provided

#### Scenario: Checkpoint resume captures payload
- **WHEN** `spln do --resume-response "approved"` resumes a paused checkpoint task
- **THEN** command SHALL resume execution and persist response payload into continuation evidence

### Requirement: `spln status`
`spln status` SHALL support both modes:
- admission-only mode (no governed change)
- governed-linked mode

`spln status` SHALL output machine-readable JSON by default (no `--json` flag required), including:
- admission metadata (`request_id`, `level`, `level_source`)
- lane mode:
  - `admission_only`: exactly one active request resolved from admission lane
  - `governed`: exactly one active request resolved from governed lane
  - `diagnostics`: no unique active request context (`0` or `>1` active requests)
- lifecycle status (`admission_status` in admission lane, `change_status` in governed lane)
- current state/action and next-ready actions
- blockers
- gate status (if governed)
- evidence pointers
- evidence freshness summary:
  - `fresh`: all required evidence references are current for active context
  - `stale`: one or more required evidence references are outdated
  - `unknown`: no active context or insufficient inputs to evaluate freshness

`next-ready actions` and `blockers` in status output SHALL be runtime-computed projections and SHALL NOT require persisted YAML fields as source-of-truth.

`spln status` SHALL provide runtime health diagnostics (read-only):
- default status run includes integrity diagnostics summary
- repairable issues SHALL include explicit remediation to run `spln repair`
- non-repairable issues SHALL remain blocking and be reported with remediation guidance
- diagnostics mode SHALL work even when active set size is `0` or `>1`
- as read-only diagnostics, status/context SHALL not require acquiring mutation lock

#### Scenario: Admission-only status
- **WHEN** active request is L1 direct lane
- **THEN** `spln status` SHALL not require `.spln/runtime/changes/<request_id>.yaml`

#### Scenario: Status reports repairable metadata issue
- **WHEN** a state record misses repairable metadata and `spln status` is run
- **THEN** status SHALL report deterministic remediation to run `spln repair`

#### Scenario: Status diagnostics without unique active request
- **WHEN** active set size is `0` or `>1` and `spln status` is run
- **THEN** command SHALL return global integrity diagnostics without requiring request-scoped active-context resolution

#### Scenario: Diagnostics lane mode and unknown freshness
- **WHEN** active set size is `0` or `>1` and `spln status` is run
- **THEN** status output SHALL use `lane_mode=diagnostics` and `evidence_freshness=unknown`

### Requirement: `spln repair`
`spln repair` SHALL be the only mutating diagnostics-repair command in MVP.

Repair contract:
- command MAY run without unique active request context (`0` or `>1` active requests allowed)
- command SHALL apply deterministic safe local repairs only
- non-repairable issues SHALL be reported without state mutation

Supported MVP repair classes:
- stale-lock cleanup (when stale-lock conditions are satisfied)
- governed-create partial-failure forward repair (orphaned L2/L3 admission from interrupted `spln new`)
- terminal archive-migration forward repair (source already terminal, archive migration incomplete)
- same-request dual-active normalization (active admission + active governed record sharing one `request_id`)
- evidence retention GC for expired non-active evidence (`execution.evidence_retention_days`)
- corrupted config recovery (backup broken config + rewrite deterministic MVP defaults)

Multi-active repair boundary:
- safe auto-fix: same-request dual-active handoff fault
- non-repairable: different-request multi-active ambiguity (reported only; no mutation)

#### Scenario: Repair orphaned governed admission from interrupted new
- **WHEN** L2/L3 `spln new` was interrupted after admission creation and `spln repair` is run
- **THEN** repair SHALL recreate missing governed runtime/artifact state and complete admission sealing handoff

#### Scenario: Repair clears stale lock
- **WHEN** lock timeout occurs and stale-lock conditions are satisfied and `spln repair` is run
- **THEN** repair SHALL clear stale lock artifacts and report lock-repair result

#### Scenario: Repair normalizes same-request dual-active
- **WHEN** runtime has active admission + active governed records with the same `request_id`
- **THEN** repair SHALL keep governed execution active and seal the admission snapshot

#### Scenario: Repair reports non-repairable multi-active ambiguity
- **WHEN** runtime has active records for multiple distinct `request_id` values
- **THEN** repair SHALL report non-repairable ambiguity without mutating lifecycle ownership

#### Scenario: Repair prunes expired evidence
- **WHEN** `spln repair` runs and expired evidence is outside retention window and not linked to active requests
- **THEN** repair MAY prune those evidence files and include deterministic GC summary in output

#### Scenario: Repair recovers malformed config
- **WHEN** `.spln/config.yaml` is malformed and `spln repair` is run
- **THEN** repair SHALL back up broken config and rewrite deterministic MVP default config

#### Scenario: Repair completes interrupted terminal archive migration
- **WHEN** runtime source record is already terminal (`done` or `cancelled`) but request-scoped archive migration is incomplete
- **THEN** `spln repair` SHALL complete archive migration idempotently without altering terminal lifecycle outcome

### Requirement: Low-Disk Opportunistic Evidence GC
Evidence-writing command flows SHALL support low-disk opportunistic GC trigger.

Low-disk trigger contract:
- trigger threshold is `execution.evidence_gc_low_disk_free_mb`
- when available disk is below threshold, command flow SHALL run one opportunistic retention-GC pass before evidence write retry
- opportunistic GC uses the same eligibility rule as retention GC (expired + non-active evidence only)
- if no eligible evidence exists, command SHALL continue and report low-disk warning context (write failure still follows runtime failure contract)

#### Scenario: Low disk triggers opportunistic GC
- **WHEN** `spln do` is about to emit evidence and free disk is below configured threshold
- **THEN** runtime SHALL run one opportunistic retention-GC pass before retrying evidence write

### Requirement: `spln context`
`spln context` SHALL output compact context (<50 lines typical) and support `--format text|yaml|json`.

It SHALL clearly indicate:
- lane mode
- level and level source
- current action
- blockers
- freshness summary

Context resolution behavior:
- exactly one active request: return request-scoped compact context for that request
- active set size `0` or `>1`: return diagnostics compact context with `lane_mode=diagnostics` and `evidence_freshness=unknown`

#### Scenario: Context in governed lane
- **WHEN** L3 is at `S6_RUN_WAVES`
- **THEN** context SHALL include governed artifact and wave summary references

#### Scenario: Context diagnostics without unique active request
- **WHEN** active set size is `0` or `>1` and `spln context` is run
- **THEN** command SHALL return compact diagnostics context without requiring request-scoped active-context resolution

### Requirement: `spln done`
`spln done` SHALL complete according to lane:

- L1 (admission/direct lane): require `S8_VERIFY` done-ready, finalize admission lifecycle to `done`, and archive by request
- L2/L3 (governed lane): require `G_ship` approval, archive by request, and persist final lifecycle `done` in archived governed state

Archive target semantics are request-scoped and default for `spln done`:
- if governed change exists for current `request_id`, archive governed runtime/artifacts and linked sealed admission snapshot
- otherwise archive direct-lane admission runtime record only

`spln done` SHALL apply request-scoped archive by default in MVP.

Governed done ordering contract:
1. verify `S8` completion and `G_ship` approval
2. validate archive preconditions while runtime governed state is still the active source (`change_status=active`)
3. mark governed runtime artifact lifecycle snapshot as `frozen`
4. migrate governed runtime/artifacts + linked admission snapshot to archive targets
5. persist final `change_status=done` in archived governed state record

For governed lane, `spln done` SHALL require `S8_VERIFY` completion, where conditional `final-closeout` refresh is handled inside `S8` before `G_ship`.

#### Scenario: L1 done defaults to archive
- **WHEN** direct lane reaches `S8_VERIFY` pass and `spln done` is run
- **THEN** admission record SHALL be finalized and migrated to `.spln/archive/admissions/<request_id>.yaml`

#### Scenario: Governed done defaults to archive
- **WHEN** L2/L3 passes ship checks and `spln done` is run
- **THEN** governed spec artifacts SHALL be archived under `aircraft/changes/archived/<slug>/`, governed runtime state SHALL be archived to `.spln/archive/changes/<request_id>.yaml`, linked `sealed_handoff` admission snapshot SHALL be migrated to `.spln/archive/admissions/<request_id>.yaml`, archived governed artifact lifecycle states SHALL be `frozen`, and archived governed lifecycle SHALL be `done`

#### Scenario: Direct-lane done archives by request key
- **WHEN** L1 reaches done-ready and `spln done` is run without governed change
- **THEN** direct-lane admission runtime record SHALL be archived to `.spln/archive/admissions/<request_id>.yaml` and removed from runtime admissions

### Requirement: `spln cancel`
`spln cancel` SHALL explicitly terminate and archive an active request.

Lane behavior:
- admission/direct lane: set `admission_status=cancelled`
- governed lane: set `change_status=cancelled`

Cancellation rules:
- command is allowed only for active non-terminal requests
- cancelled requests SHALL present no next-ready actions
- cancel command output SHALL include terminal lifecycle summary (`request_id`, lane mode, cancellation status)
- cancel command SHALL archive using request-scoped archive semantics:
  - if governed change exists for current `request_id`, archive governed runtime/artifacts and linked sealed admission snapshot (with governed artifact lifecycle snapshot set to `frozen`)
  - otherwise archive direct-lane admission runtime record only
- `spln done` and `spln do` SHALL be blocked after cancellation unless request is explicitly re-opened by future non-MVP tooling
- if cancellation targets an in-flight wave execution, runtime SHALL interrupt active task subprocesses (`SIGINT` then `SIGKILL` after `execution.cancel_grace_period_seconds`) before archive migration

Deterministic cancel ordering:
1. set lane lifecycle status to terminal cancelled (`admission_status=cancelled` or `change_status=cancelled`)
2. perform request-scoped archive migration
3. ensure archived record preserves cancelled status

#### Scenario: Direct-lane cancellation
- **WHEN** active L1 request runs `spln cancel`
- **THEN** terminal cancellation SHALL complete, direct-lane admission runtime record SHALL be archived to `.spln/archive/admissions/<request_id>.yaml`, and cancel command output SHALL report terminal cancelled lifecycle summary

#### Scenario: Governed cancellation
- **WHEN** active L2/L3 request runs `spln cancel`
- **THEN** terminal cancellation SHALL complete, governed/runtime/artifact archive migration SHALL use the same request-scoped targets as `spln done`, archived governed artifact lifecycle states SHALL be `frozen`, and cancel command output SHALL report terminal cancelled lifecycle summary

#### Scenario: Cancel interrupts in-flight tasks before archive
- **WHEN** `spln cancel` is invoked while active wave tasks are still running
- **THEN** runtime SHALL perform graceful-then-forced interruption before archive migration proceeds

#### Scenario: Cancel does not require reason flag
- **WHEN** `spln cancel` is run with default arguments
- **THEN** command SHALL continue to evaluate active-context and lifecycle/archive preconditions without argument-validation failure

#### Scenario: Governed done depends on S8 closeout completion
- **WHEN** L2/L3 `spln done` is requested and governed `S8` closeout refresh is still required
- **THEN** command flow SHALL require/trigger `S8` completion first and SHALL block final done until `G_ship` is approved

### Requirement: `spln pivot`
`spln pivot` SHALL trigger analyze-first pivot processing and apply reroute/rescope only after pivot gate approval.

`spln pivot` invocation contract:
- requires an active request context
- allowed only when current state is `S6_RUN_WAVES`, `S7_REVIEW`, or `S8_VERIFY`
- all other states SHALL be rejected with actionable remediation
- supported flags:
  - `--kind reroute|rescope` (default `reroute`)
  - `rescope` path MUST use explicit `--kind rescope`
  - `--kind rescope` is valid only when current state is governed `S6_RUN_WAVES`; requests from `S7_REVIEW`/`S8_VERIFY` SHALL be rejected with remediation to re-enter `S6` first

`spln pivot` SHALL execute in this order:
1. transition to `S1_ANALYZE`
2. refresh analyze evidence for current request
3. evaluate `G_pivot` using refreshed analyze evidence + explicit pivot kind
4. apply reroute/rescope transition only when `G_pivot` is approved

If pivot result escalates `L1 -> L2/L3`, system SHALL:
- append `level_history`
- create governed change artifacts at escalation point
- continue in governed lane

#### Scenario: L1 escalation pivot
- **WHEN** direct lane pivot raises level to L2
- **THEN** change directory SHALL be created and execution SHALL hand off to governed state

#### Scenario: Pivot from review is analyze-first
- **WHEN** pivot is requested at `S7_REVIEW`
- **THEN** command flow SHALL transition to `S1_ANALYZE`, refresh analyze evidence, evaluate `G_pivot`, and only then apply level/lane updates

#### Scenario: Pivot blocked outside allowed states
- **WHEN** `spln pivot` is requested while current state is `S4_SPEC_BUNDLE`
- **THEN** command SHALL fail with remediation to continue normal progression or invoke pivot from `S6/S7/S8`

#### Scenario: Pivot default kind without reason flag
- **WHEN** `spln pivot` is run without `--kind` and without reason argument
- **THEN** command SHALL default to `--kind reroute` and continue to evaluate active-context/state-boundary preconditions without argument-validation failure

#### Scenario: Pivot rescope requires explicit kind
- **WHEN** operator intends governed rescope flow from `S6_RUN_WAVES`
- **THEN** command SHALL require `--kind rescope` before evaluating rescope-specific `G_pivot` conditions

#### Scenario: Pivot rescope rejected outside S6
- **WHEN** operator requests `spln pivot --kind rescope` while current state is `S7_REVIEW` or `S8_VERIFY`
- **THEN** command SHALL fail with deterministic remediation to return to `S6_RUN_WAVES` before rescope

### Requirement: Override commands
The override command set SHALL expose explicit operator controls for analysis and review.

- `spln analyze`: explicit analyze re-run at `S1`
- `spln review`: explicit review loop execution (`--changed-only`, `--all`)

MVP flag boundary for review:
- supported: `--changed-only`, `--all`
- not supported: `--artifact`
- if `--artifact` is provided, command SHALL fail with remediation to use default changed-only review or `--all`

`spln analyze` invocation contract:
- allowed from active non-`DONE` states
- if current state is not `S1_ANALYZE`, command SHALL transition to `S1_ANALYZE` first
- command SHALL persist refreshed analyze outputs (route metadata + `route_snapshot`, including refreshed raw scores) in current lane state
- `spln analyze` SHALL NOT apply lane/level reroute or governed rescope transitions; reroute/rescope requires explicit `spln pivot`
- analyze override SHALL preserve historical `task_runs` and `action_history` for audit; it SHALL mark existing run-summary readiness as superseded so downstream review/verify must be recomputed
- if fixed-level hard conflicts are detected during analyze override, command SHALL persist blockers in `route_snapshot.blocking_conflicts[]` while `current_state=S1_ANALYZE`, and SHALL NOT mutate lane/level routing outcome
- invocation on `DONE` SHALL be blocked with explicit remediation (start new request or pivot from active request)
- invocation without active request SHALL be blocked with explicit remediation (start executable request via `spln new`)
- invocation after `non_spln` intake rejection SHALL be blocked (no request context exists)

`spln review` invocation contract:
- requires an active request context
- allowed when current state is `S7_REVIEW`
- allowed from `S6_RUN_WAVES` only if a frozen run summary exists; command SHALL transition `S6 -> S7` before review
- allowed from `S8_VERIFY` only as explicit re-review override; command SHALL transition `S8 -> S7` before review
- all other states SHALL be rejected with remediation guidance
- review outputs SHALL follow normal review loop semantics (`pass` keeps forward readiness, `fail` returns to `S6`)

#### Scenario: Analyze override path
- **WHEN** `spln analyze` is run before normal `spln do` progression
- **THEN** it SHALL execute analyze checks and persist analyze outputs without requiring a separate promotion command

#### Scenario: Analyze from non-S1 state
- **WHEN** `spln analyze` is run while current state is `S6_RUN_WAVES`
- **THEN** command flow SHALL transition to `S1_ANALYZE`, execute analyze, and persist refreshed route metadata

#### Scenario: Analyze blocked without active request
- **WHEN** `spln analyze` is run with no active request context
- **THEN** command SHALL fail with remediation to start an executable request via `spln new`

#### Scenario: Review blocked without active request
- **WHEN** `spln review` is run with no active request context
- **THEN** command SHALL fail with remediation to start or resume an executable request

#### Scenario: Review blocked without frozen run summary
- **WHEN** `spln review` is run from `S6_RUN_WAVES` without a frozen run summary
- **THEN** command SHALL fail with guidance to complete/freeze wave summary first

#### Scenario: Review rejects unsupported artifact flag
- **WHEN** `spln review --artifact design.md` is invoked in MVP
- **THEN** command SHALL fail with remediation to use default changed-only review or `--all`

### Requirement: Help text grouping
Help output SHALL be grouped by:
- Daily path
- Situational
- Expert override

#### Scenario: Help organization
- **WHEN** `spln --help` is run
- **THEN** command groups SHALL reflect the progressive disclosure model
