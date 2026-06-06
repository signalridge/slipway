# Decision
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Selected Approach
Approach A (full delivery), selected via research-orchestration with user confirmation. A single generalized stale-evidence recovery primitive replaces the S3/S4-only `beginStalePlanningRecovery`:

1. **Detect** the earliest stale authority by iterating the governance registry and, for each skill at or before the current lifecycle position, comparing its certified digest inputs by content hash. Order authorities by the canonical lifecycle — `action.WorkflowPath` plus `planSubStepOrder` plus registry `State`/`PlanSubStep` — with no separate hand-maintained rank table.
2. **Reopen** to that authority's `(state, substep)` from any state, clearing that authority and every downstream authority's evidence, while preserving runtime task evidence; route the existing host skill. Reopen is monotone (target is at/before current; clears forward) so it converges.
3. **Freshness** becomes pure content-hash: `model.EvidenceFreshness` is the sole authority; the mtime `*ChangedAfterVerdict` family and digest `run_version` checks/field are removed. Stamp-on-accept guarantees a first-run digest so a freshly-authored stage is fresh after its verdict (kills the #90 mtime fragility).

Locked sub-decision (user-selected): an S1 reopen **clears** wave-orchestration evidence (uniform "clear target and all downstream" rule; runtime task evidence is preserved so wave-orchestration re-derives). This flips the old preserve assertion.

Carried KEEP items: the #89 structural/scope hash split (strict target_files-only edit rebuilds the wave-plan in place at S2; any task contract edit reopens S1/audit; runtime task-evidence drift keys on structural hash so only compatible target_files-only edits preserve task evidence). Removed: the Tier-0 `slipway evidence restamp` surface and `repair`'s routing to it (repair now routes to `slipway run`). Not present on main and therefore not added: any `slipway recover` command, dependency-ordered recovery graph, Tier-2 attested restamp, or Tier 0/1/2 vocabulary.

## Key Decisions
- Full delivery in one PR (generalized reopen + content-hash freshness + #89 + #96 + #97 + escape-hatch removal). [user-selected]
- Guardrail domain is `external_api_contracts` because this changes public
  Slipway CLI/JSON and generated host-tool contracts, not just internal
  evidence plumbing. [plan-audit]
- S1 reopen clears wave-orchestration evidence (correctness over old preserve). [user-selected]
- Ordering source = canonical `action.WorkflowPath` + `planSubStepOrder`, not a new rank table. [research]
- Restamp removed entirely; guardrail-domain changes are therefore rerun/review-only by construction (no bypass to fail open). [research]
- Reopen granularity is the whole state for S2/S3/S4 (S1 keeps substep granularity), so S3/S4 same-position skills need no tie-break.
- Freshness/restamp removal has a compile-safe boundary: the engine restamp
  function and CLI command registration/call are removed together in t-01, while
  later repair/recovery vocabulary cleanup is t-06. [plan-audit]
- Self-bootstrap is dependency-locked: t-11 is the only wave-2 task after t-01;
  t-04/t-05/t-06 depend on t-11 so the current worktree black-box
  validate/next/health check cannot be skipped by same-wave parallelism.
  [plan-audit]
- Agent instruction files (`CLAUDE.md` and `AGENTS.md`) are required
  principle-only black-box lifecycle contracts, not Slipway command manuals:
  remove concrete command recipes and JSON examples, require the latest
  current-worktree CLI self-loop, and treat guess-required nodes as
  product/usability defects to fix immediately through the self-optimization
  loop.
  [user-selected]
- All governed host skills, Slipway workflow/command skills, generated command
  references, and repo docs must align to the current command registry, public
  JSON, recovery vocabulary, and lifecycle semantics after implementation.
  [user-selected]

## Rejected Alternatives
- Minimal extension (widen `beginStalePlanningRecovery`'s gate, keep mtime/run_version + Tier-0 restamp): leaves #90 fragility and the escape-hatch; fails the user's full-delivery choice.
- Diff-driven affected-authority index powering a recover surface: reintroduces the #85 machinery #98 removes.

## Interfaces and Data Flow
- NEW `internal/engine/progression/stale_evidence_recovery.go` exporting `StaleEvidenceRecoveryAvailable(root, change, blockers) (label, state, ok)` for read-only surfaces; internal `staleReopenTarget`, `staleReopenFromExecSummary`, `reopenToStaleStage`.
- `AdvanceGoverned` calls the unified reopen at the two existing trigger points (digest pre-bundle; exec-summary post-sync); `beginStalePlanningRecovery`, `stalePlanningRecoveryNeededForPlanAuditDigest`, and `stale_planning_recovery.go` are deleted.
- `model.EvidenceFreshness` is the only freshness comparator; `SkillDigest.RunVersion` is removed.
- #89 plan hashes split by use: structural hash drives plan-audit freshness,
  execution-summary source freshness, and runtime task-evidence drift, and it
  includes all task execution/evidence contract fields except target_files
  (task ID, objective, wave/dependencies, task_kind, covers, evidence,
  acceptance, checkpoint_type); scope hash detects target_files-only rebuilds;
  semantic hash remains only where an explicitly intentional
  non-structural/legacy comparison needs it.
- Data flow on reopen: clear verification records + digests for authorities at/after target; delete wave-plan.yaml/execution-summary.yaml when target ≤ S2; preserve `evidence/tasks/`; transition + reset substep.
- Public contract flow: `next` / `validate` / `status` / CLIError continue to
  expose the documented recovery object shape; intentional command/vocabulary
  changes route stale inputs to `slipway run` and are tested against docs and
  generated surfaces.
- Agent/skill flow: `CLAUDE.md` and `AGENTS.md` provide only the black-box
  current-worktree lifecycle principles and self-optimization contract, while
  generated governed skills and Slipway workflow/command skills provide the
  current command/handoff surface.

## Rollout and Rollback
- Rollout: behind no flag; behavior is the recovery path itself. Verified via
  the dependency-locked early self-bootstrap gate after t-01, `go build/vet/test ./...`,
  `freshness_guard` (zero ModTime), current-worktree generated-surface refresh
  (`go run . init --refresh --tools all`) plus zero project-visible drift,
  toolgen self-loop, `go run . validate --json`, external API contract tests for
  recovery JSON/help/docs/generated surfaces, CLAUDE/AGENTS black-box-doc audit,
  all-skill alignment checks, and dogfood replay of
  #90/#89/#81/#96/#97 with current-worktree `go run . run` alone.
- Rollback: revert the branch. No data migration; cleared evidence is
  regenerable by re-running stages; no durable user data touched. Rollback must
  restore public recovery guidance and generated surfaces together with code
  because this is an external API contract change.

## Risk
- Under-block if a passing skill is never stamped → mitigate by unconditional stamp-on-accept and routing the no-digest branch through `EvidenceFreshness`.
- Read-only vs mutating divergence → both use the same `reopenStagePosition`/detection helpers.
- Clearing wave-orchestration on S1 reopen discards a valid verdict → accepted (re-derived from preserved task evidence); test expectation updated.
- #96 fix must preserve `evidence/tasks/` while still clearing derived state → targeted removal, not `RemoveAll(ChangeDir)`.
- Earliest-affected recovery can fail open if a skill's certified-input builder omits a true upstream dependency → add executable input-coverage assertions and downstream propagation checks before accepting the generalized reopen primitive.
- Compile-safety risk: removing engine restamp before the CLI caller would break
  the wave-1 binary → remove the engine function and CLI restamp command
  registration/call in t-01, then do remaining repair/recovery wording in t-06.
- Self-bootstrap risk: t-01 changes the freshness semantics that govern this change's own existing plan evidence → run this change's `go run . validate/next/health` immediately after the freshness/restamp boundary and fix any non-actionable self-stale state as part of this change.
- Same-wave bootstrap bypass risk: the orchestrator may run tasks in the same
  wave concurrently → make t-11 the only wave-2 task and require t-04/t-05/t-06
  to depend on it.
- #89 task-evidence drift risk: leaving runtime task evidence keyed on semantic
  hash would stale all target_files-only edits, but an over-broad structural hash
  would preserve incompatible task evidence → require the structural hash to
  include all task execution/evidence contract fields except target_files, and
  require `tasksPlanChangedSinceTaskEvidenceBlockers` plus per-task freshness
  inputs to compare that structural hash.
- Public UX drift risk: removing restamp changes commit-before-done recovery expectations → update docs/generated skills so diff-class review staleness converges by rerunning review, not by restamping.
- External contract risk: host tools may branch on recovery JSON or command
  availability → preserve documented recovery fields where possible; document
  intentional removals; require `layer:R3=pass` and `layer:IR3=pass` review
  evidence before closeout.
- Agent/skill drift risk: root instruction files or generated skills can
  outlive the CLI behavior and teach stale flows → audit `CLAUDE.md` and
  `AGENTS.md` for no command recipes, regenerate all governed/Slipway skills
  with the current-worktree init refresh command, and block closeout on
  generated-surface drift.
