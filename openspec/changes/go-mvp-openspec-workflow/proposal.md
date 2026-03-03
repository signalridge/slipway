## Why

The current draft still mixes two different control models:
- route/admission decisions (`S0/S1`) and governed execution states (`S2..S8`) are not cleanly separated
- advisory-only conversations are modeled as in-workflow levels, creating low-value state noise

That creates contradictions around level state ownership, artifact requirements, and command behavior.

The user-confirmed MVP direction is:
- assess and route first
- create governed change artifacts only when required
- keep `L2/L3` on the same governed artifact contract
- keep direct lane `L1` lightweight and non-governed by default
- keep pure Q&A outside speclane request lifecycle

## What Changes

- Define a two-phase control model:
  - **Admission phase** (all levels): `S0_INTAKE -> S1_ANALYZE`
  - **Execution phase**:
    - `L1`: direct lane `S6 -> S7 -> S8 -> DONE` without creating `aircraft/changes/<slug>/` by default
    - `L2`: governed lane `S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
    - `L3`: governed lane `S2 -> S3 -> S4 -> S5 -> S6 -> S7 -> S8 -> DONE`
- Enforce speclane scope boundary:
  - pure Q&A/advisory requests SHALL be rejected at `spln new` with remediation to use normal chat
  - only executable requests enter speclane and receive `request_id`
  - executable-intent classification SHALL be AI-first and language-agnostic, using structured semantic assessment + deterministic threshold consumption; keyword/path signals are auxiliary only and never authoritative
- Keep one canonical state taxonomy (`S0..S8`, `DONE`), but split **state storage ownership**:
  - admission/direct lane state in `.spln/runtime/admissions/<request_id>.yaml`
  - governed lane runtime state in `.spln/runtime/changes/<request_id>.yaml`
  - governed spec artifact bundle in `aircraft/changes/<slug>/`
  - runtime layout semantics remain documentation-owned; `spln init` SHALL NOT scaffold `.spln/README.md` or other non-executable narrative files under `.spln/`
  - after governed handoff, admission becomes sealed snapshot (read-only lookup record)
  - on governed archive:
    - governed spec artifacts move to `aircraft/changes/archived/<slug>/`
    - governed runtime state moves to `.spln/archive/changes/<request_id>.yaml`
    - linked sealed admission snapshot moves to `.spln/archive/admissions/<request_id>.yaml`
- Clarify `S0/S1` responsibility boundary:
  - `S0_INTAKE`: capture intent and level mode input only
  - `S1_ANALYZE`: compute/validate route result and safety readiness
- Keep `S1_ANALYZE` mandatory for fixed-level runs, with explicit blocking on hard safety conflicts (no silent level rewrite)
- Remove CLI `--scores`; scoring remains internal analysis output (persist raw scores only, derive values on read)
- Keep routing as two-dimension evaluation with:
  - executable final grade `L1|L2|L3`
  - explicit `non_spln` classification for non-executable intake
- Keep explicit 5-dimension scoring (`N/A/I/R/V`) aligned with baseline model
- Keep gate family as:
  - `G_scope` (L3 only)
  - `G_plan` (L2/L3)
  - `G_pivot` (conditional)
  - `G_ship` (governed lane only)
- Define guardrail high-risk check catalog for `G_ship` with deterministic missing/failing reason codes
- Define guardrail high-risk check ID registry and naming convention:
  - fixed format `<domain_slug>.<check_slug>`
  - failure reason format remains `high_risk_check_missing:<check_id>` / `high_risk_check_failed:<check_id>`
- Define governed required artifact bundle for L2/L3:
  - `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `assurance.md`
  - `L3` adds `explore.md`
- Keep governed manifest artifact `change.yaml` under `aircraft/changes/<slug>/`
- Keep governed runtime state file under `.spln/runtime/changes/<request_id>.yaml`
- Keep governed artifact version/state authority in runtime `ChangeState.Artifacts` (manifest remains minimal metadata)
- Keep `assurance.md` as required governed artifact, but scope it to closeout verdict and evidence indexing
  - `assurance.md` minimum sections in MVP:
    - `Scope Summary`
    - `Verification Verdict`
    - `Evidence Index`
    - `Residual Risks and Exceptions`
    - `Archive Decision`
  - structure checks require canonical sections/headings; content-depth quality is validated by review layers
- Remove standalone `risk.md` from MVP; risk analysis lives in `design.md` risk section
- Keep L1 governance skills optional by default (L2/L3 remain level-enforced)
- Clarify L1 `S7/S8` as lightweight auto-check stages (not governed review/verify gates)
  - L1 lightweight checks are command-owned by `spln do` (same invocation executes `S6` work plus ordered `S7`/`S8` lightweight evaluation); no dedicated L1 review/verify subcommand
- Clarify L1 synthetic task ID compact format:
  - use `l1-<request_short>-NN`, where `request_short` is deterministic short form (first 8 chars of `request_id`)
- Clarify completion trigger:
  - `S7/S8` checks may auto-run, but `S8 -> DONE` is finalized only via explicit `spln done`
- Persist L1 direct-lane execution traces (`task_runs`, `action_history`) in admission state, with `evidence_refs` limited to non-task evidence indexing
- Clarify governed handoff trace ownership:
  - `task_runs` and `action_history` are lane-local and SHALL NOT be copied from admission state into governed change state during L1->L2/L3 handoff
- Require `G_scope` worktree evidence fields:
  - `worktree_path`, `worktree_branch`
- Require `G_scope` worktree authenticity checks:
  - path exists/access is valid
  - path is current-repo Git worktree
  - checked-out branch matches persisted metadata
- Require `G_scope` L3 explore readiness checks:
  - `explore.md` exists
  - required sections present with non-empty content (`Objectives`, `Unknowns`, `Assumptions`, `Scope Boundaries`, `Validation Plan`)
- Require level durability fields until archive in governed lane:
  - `level`, `level_source`, `level_history`, `last_level_update_at`
  - `level_history` MUST always exist (empty array allowed)
- Persist `route_snapshot` (including `routing_rationale[]`, guardrail domain, raw scores, and optional `blocking_conflicts[]`) as durable lane state
- Derive required contracts (`required_artifacts/gates/skills`) from level + guardrail rules at read time instead of persisting them in `route_snapshot`
- Define deterministic `new_project` / `major_refactor` signal derivation at `S1_ANALYZE` so auto-route L3 triggers are implementation-stable
- Align guardrail auto-route floor to governed discovery lane:
  - in auto mode, guardrail-domain requests route to `L3`
  - `L2` remains non-guardrail high-control lane (`control_score >= 8`)
- Add deterministic CLI failure contract for automation:
  - stable non-zero exit code taxonomy
  - structured JSON error envelope on failure
- Split diagnostics from repair mutations:
  - `spln status` is read-only diagnostics surface
  - `spln repair` is explicit mutating repair command
  - multi-active ambiguity repair is bounded/safe only:
    - auto-repair allowed when duplicate active records represent the same `request_id` handoff fault
    - different-request multi-active conflicts stay non-repairable and require explicit operator action
- Define MVP active-request resolution:
  - runtime permits at most one active non-terminal request across admissions + governed changes
  - `spln new` SHALL reject when an active request already exists
  - `spln do/done/cancel/pivot/analyze/review` resolve exactly one active request; none or multiple active requests are blocking errors
  - `spln context` and `spln status` remain available as diagnostics-first read surfaces when active set is `0` or `>1`
  - mutating repairs are explicit via `spln repair`
- Strengthen local lock contract for `.spln/state.lock`:
  - bounded lock wait timeout via config
  - stale-lock diagnostics and cleanup only through explicit `spln repair`
  - mutating commands SHALL NOT force-unlock on timeout
- Add storage hygiene controls for long-running workspaces:
  - `execution.evidence_retention_days` (default `30`) for bounded evidence GC
  - `execution.evidence_gc_low_disk_free_mb` (default `512`) for low-disk opportunistic auto-GC trigger on evidence-writing flows
  - `execution.max_level_history_entries` (default `100`) to cap `level_history` growth
- Require single-key handoff metadata:
  - admission and governed change share the same immutable `request_id`
- Define ID generation contract (fixed in MVP):
  - `request_id=uuidv7` (canonical immutable cross-lane key)
  - governed `slug={title_kebab}` with numeric collision suffixing
- Clarify governed manifest level semantics:
  - `aircraft/changes/<slug>/change.yaml` `created_at_level` is a governed-creation snapshot
  - live level authority after creation is runtime top-level change state metadata (`level`, `level_source`, `level_history`, `last_level_update_at`)
  - `route_snapshot` remains routing audit evidence only (not live level authority)
  - pivot level updates mutate runtime change state (and history/snapshot), not the manifest snapshot field
- Persist `S1_ANALYZE` semantic classifier output in admission state (`intake_assessment`) for audit/replay explainability
- Add explicit cancellation lifecycle:
  - `admission_status` supports `cancelled` for direct-lane termination
  - governed `change_status` supports `cancelled`
  - add `spln cancel`
  - `spln cancel` SHALL perform terminal cancellation and request-scoped archive in one command
  - when cancellation occurs during active wave execution, runtime SHALL interrupt in-flight subprocesses (`SIGINT` then `SIGKILL` after `execution.cancel_grace_period_seconds`, default `10`)
  - interrupted terminal archive migration after `done`/`cancel` SHALL be repair-forward via `spln repair` (idempotent completion, no lifecycle reopen)
- Add explicit pivot command contract:
  - add `spln pivot`
- Clarify fixed-level hard-conflict behavior:
  - fixed-level guardrail conflicts are fail-fast at `spln new` before `request_id`/state creation
  - active-request `spln analyze` hard-conflict keeps lane/level unchanged, persists blockers at `S1_ANALYZE`, and requires explicit `spln pivot` for reroute
- Define `S3_SCOPE_CONFIRMATION` worktree metadata write contract:
  - `spln do` at `S3` persists `worktree_path` and `worktree_branch` into governed runtime state before `G_scope`
- Enforce `task_runs` identity integrity:
  - map key `<task_id>__rv<run_summary_version>` MUST match payload `task_id` and `run_summary_version`
- Unify archive target semantics by request:
  - `spln done` SHALL default to request-scoped archive
  - for `spln done` and `spln cancel`, if governed change exists: archive governed change runtime + governed artifact bundle + linked sealed admission snapshot
  - for `spln done` and `spln cancel`, otherwise archive direct-lane admission record only
- Extend evidence contract with:
  - `run_summary_version`
  - `session_id`
  - `input_hash`
  - `mitigation_target`
  - `mitigation_target` MUST match registered skill-to-mitigation mapping
  - `run_summary_version` value domain:
    - pre-run-summary governance states (`S1/S3/S5`) MUST use `0`
    - run-summary-bound states (`S6/S7/S8`) MUST use `>=1`, and review/verify checks MUST match the current frozen run summary version
- Define deterministic evidence filename contract and collision handling
- Simplify deterministic evidence filename to improve operator readability:
  - `<session_id>--<skill_name>.json` (collision-safe suffixing)
- Remove redundant per-file `schema_version` fields from MVP config/state/manifest contracts
- Define explicit `.spln/config.yaml` MVP schema/defaults and preserve unknown top-level keys on rewrite
- Add config corruption handling:
  - malformed/unparseable `.spln/config.yaml` is state-integrity failure
  - `spln repair` SHALL back up broken config to `.spln/archive/config/config.yaml.broken.<timestamp>.yaml` and rewrite deterministic defaults
- Require `defaults.level_mode` to be consumed when `--level` is omitted (`auto` fallback on invalid value)
- Keep tool-adapter generation as optional sidecar (core workflow remains usable with `spln init --tools none`)
- Remove unused `aircraft/specs/` bootstrap directory from MVP init layout

## MVP Scope (Plan Artifact Level)

- MVP here means the OpenSpec planning artifact set itself (`proposal/design/spec/tasks`) is fully consistent and directly implementable.
- No runtime database.
- No hidden automation that bypasses routing/gates.
- No extra gate families beyond `G_scope/G_plan/G_pivot/G_ship`.
- No forced change scaffolding for `L1`.
- Tool-adapter generation remains optional sidecar and does not block core-MVP completion criteria.

## Source Strategy (Explicit)

This redesign is intentionally hybrid and source-attributed:

- **OpenSpec (reference base):** governed artifact contracts, status/validate/archive semantics, gate/readiness framing.
- **GSD (reference execution):** wave planning/execution, dependency layering, retry/skip/abort/pivot loop behavior.
- **Superpowers (reference interaction):** discovery and clarification flow, worktree confirmation discipline, review/verification interaction style.
- **SpecLane (local authority):** level routing rules, state ownership boundaries, command behavior, evidence schema.

When references conflict, SpecLane local contract is authoritative for this MVP.

## Capabilities

### New Capabilities

- `action-workflow`: admission + governed/direct split with canonical state taxonomy
- `state-persistence`: admission-state and governed-state dual persistence model
- `cli-commands`: `new` routes first; L1 no default governed change creation; explicit `cancel` command for terminal cancellation
- `artifact-lifecycle`: governed-only artifact requirements; direct lane minimal contract
- `gate-engine`: gates apply only where governance is required
- `skill-contracts`: state-bound skills with evidence extensions (`session_id`, `input_hash`, `mitigation_target`)

### Modified Capabilities

- `routing-engine`: remove CLI `--scores`, keep internal five-score model and final single-grade output
- `wave-execution`: include `task_kind=other` as explicit non-wave-governed escape hatch
- `design/tasks`: rewritten to remove contradictory lane assumptions

## Impact

- **Behavioral clarification:** `L1` no longer implies governed change artifacts by default; pure Q&A does not enter speclane.
- **Storage clarification:** level/state before governed creation is persisted in runtime admission files.
- **Governed consistency:** `L2/L3` share one governed artifact aircraft and mainline (`S4..S8`).
- **Implementation impact:** routing, command flow, persistence, and gate checks need aligned updates.
