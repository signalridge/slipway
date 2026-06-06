# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: engine packages under internal/engine (read-only over model); cmd are thin orchestrators; model is a leaf package; one verdict-evidence YAML per skill under verification/.

## Summary
Recovery UX P2 (#98): generalize automatic re-walk to the earliest affected authority. When a governed artifact or relevant code input changes after a stage's evidence was accepted, `slipway run` reopens the earliest affected authority, clears that authority and its downstream evidence, preserves orthogonal (runtime task) evidence, and routes the existing host skill — no `slipway recover`, no Tier-2 attested restamp, no digest/timestamp hand-edits. Supersedes #85 (whose recover-command / restamp direction is removed). Eliminates the recovery dead-ends #81, #89, #90, #96, #97.

## Complexity Assessment
critical
<!-- Rationale -->
Touches the governance kernel: evidence freshness, the stale-recovery state machine, and the wave-plan/execution-summary hashing that gates execution. A wrong cut could let stale or unsafe evidence pass a gate, or strand a change with no recovery path. The reopen also performs destructive (but regenerable) clearing of governance evidence. Hence critical, with fail-closed boundaries (guardrail-domain changes re-run/review, never restamp) and a monotone "clear-and-re-walk" invariant.

## Guardrail Domains
`external_api_contracts` — intent-based classification. The implementation
changes externally consumed Slipway CLI/JSON behavior and generated command /
skill surfaces: `next` / `validate` / `status` / governance-blocked `CLIError`
recovery JSON, `repair` remediation, `evidence restamp` command availability,
README/docs, `CLAUDE.md` / `AGENTS.md` agent instruction guidance, and generated
host-tool guidance. It still only clears
engine-owned, regenerable governance evidence (verification records,
wave-plan.yaml, execution-summary.yaml, evidence-digests.yaml), but because the
operator/host contract changes, the plan must include external-contract review,
rollback readiness, and RED/contract tests before changing public recovery
surfaces. Guardrail-domain recovery itself remains fail-closed to rerun/review:
no restamp, attestation, force-close, or manual engine-owned evidence edit may
satisfy stale evidence.

## In Scope
Core — generalized automatic re-walk:
- NEW `internal/engine/progression/stale_evidence_recovery.go`: detect the earliest stale authority by iterating the governance registry and comparing each skill's certified digest inputs; reopen to that `(state, substep)`; clear it and every downstream authority's evidence; preserve runtime task evidence. Replaces the S3/S4-only `beginStalePlanningRecovery`.
- `internal/engine/progression/advance_governed.go`: wire the unified reopen into `AdvanceGoverned` for **all** states (remove the S3/S4 gate); delete `beginStalePlanningRecovery` and `stalePlanningRecoveryNeededForPlanAuditDigest`.
- `internal/engine/progression/advance_intake.go`: first-class S0 re-walk for stale `intake-clarification` (#90) via clear-and-rerun.
- `cmd/next_skill_view.go` + read-only surfaces: `next`/`validate`/`status`/`CLIError` report the same root authority and next public action that mutating `run` will perform (`StaleEvidenceRecoveryAvailable`). Also the F1 S1/audit `no_skill_required` → actionable `slipway run` fix.
- Public surface alignment: remove normal-path `evidence restamp` guidance and Tier 0/1/2 user vocabulary from CLI help, recovery summaries, repair guidance, README/docs, and generated tool surfaces. Commit-before-done diff-class staleness converges by re-running review, not restamping.
- External API contract hardening: preserve the documented recovery JSON shape
  (`primary_command`, `primary_action`, `recovery_class`, `steps[]`) except for
  intentional, tested vocabulary/command changes; update docs and generated
  surfaces in the same slice as public behavior changes; require S3
  external-contract review tokens (`layer:R3=pass` and `layer:IR3=pass`) before
  closeout.
- Agent instruction boundary: `CLAUDE.md` and `AGENTS.md` are required
  principle-only agent entry surfaces, not Slipway command manuals. Their
  presence removes ambiguity around repository-level agent instructions. They
  must avoid concrete lifecycle command recipes and JSON examples, require
  black-box use of the latest current-worktree Slipway CLI, and treat any
  guess-required handoff as a product/usability defect to fix immediately
  through the self-optimization loop.

KEEP items rebuilt cleanly (not in `main`; ported/refined from the prior spike):
- Content-hash freshness: `internal/engine/progression/evidence_digests.go` — remove mtime and digest `run_version` freshness semantics; freshness is pure content-hash plus verdict/digest timestamp ordering.
- #89 structural/scope hash split: `internal/engine/wave/parse.go` (`TaskPlanStructuralHash`/`TaskPlanScopeHash`/`TaskPlanSemanticHash`), `internal/model/wave_execution.go`, `internal/state/wave_execution.go`, `internal/engine/progression/wave_sync.go` (S2 in-place scope re-materialize only for strict target_files-only edits, and runtime task-evidence drift keyed on structural hash rather than semantic hash), `internal/engine/progression/readiness.go` (S2 scope guidance), `internal/state/health.go`.

Dead-end fixes:
- #96: `cmd/pivot_execution.go` — reroute/rescope preserve runtime task evidence (no `os.RemoveAll(ChangeDir)`), consistent with the reopen primitive.
- #97 + S2 repair gap: execution-summary and wave-plan are regenerated when their source inputs changed (clear-on-reopen + `repair` re-materialize), never reused only because readable and never routed into evidence-loss rescope when a valid re-walk/rebuild is available.

Removals (escape-hatch direction must not exist):
- Ensure there is no `slipway recover` command / dependency-ordered recovery graph / Tier-2 attested restamp / Tier 0/1/2 user-facing vocabulary.
- Remove `slipway evidence restamp` as a normal recovery path once content-hash freshness + auto re-walk cover the cases.
- No hand-maintained authority ordering tables (state/plan-substep/review-layer rank maps) — ordering is the canonical lifecycle (model state order + `planSubStepOrder`) and registry `State`/`PlanSubStep`.

Skills alignment:
- Each stale case routes back to the owning stage skill; no `recover` route, no special recover skill. All governed host skills, Slipway workflow/command skills, generated command references, and repo docs must align with the latest current CLI logic, public JSON, and lifecycle semantics after this change.

## Out of Scope
- A `slipway recover` command, dependency-ordered recovery graph over blockers, or Tier-2 operator-attested restamp (explicitly removed, not deferred).
- Force-close policy redesign (#75) beyond ensuring force-close stays an audited terminal escape, not a recovery substitute.
- Worktree rebind / dual-active cleanup (#86 / P3) unless they directly block automatic re-walk.
- New mechanical validators that only report stale state without making the next action executable.

## Constraints
- Import layering: `progression` may import `skill`/`state`/`model`; `model` stays a leaf; no `model -> engine` cycle.
- Freshness must be content-hash only — no `time.Now`/`ModTime`/digest `run_version` in the freshness comparison (freshness-guard stays green).
- Only skill-digest freshness loses `run_version`: remove `SkillDigest.RunVersion`
  and the skill-digest `stored.RunVersion != record.RunVersion` freshness
  checks, but preserve runtime run-version authorities such as
  `VerificationRecord.RunVersion`, `ExecutionSummary.RunSummaryVersion`, task
  evidence run binding, S3/S4 run-summary-bound review checks, and closeout reuse
  checks.
- Authority ordering derives from the canonical lifecycle and the registry; no separate hand-maintained rank tables.
- Reopen is monotone: it reopens to an at-or-before authority and clears forward, so it converges (re-run produces fresh evidence; no loop).
- Reopen clears the affected authority + downstream, and always preserves orthogonal runtime task evidence.
- Guardrail-domain changes fail closed to rerun/review; never recoverable via restamp or force-close bypass.
- Public CLI/JSON behavior is an external contract: intentional removals or
  vocabulary changes must have contract tests, docs/generated-surface updates,
  and rollback notes in the same implementation wave.
- Generated-surface refresh proof must use the latest current-worktree binary:
  run `go run . init --refresh --tools all`, then prove zero project-visible
  drift with `git diff --exit-code` or an equivalent recorded check. Do not use
  an installed `slipway` binary for this proof.
- Read-only and mutating surfaces must agree on the root authority and the next public action.
- Earliest-affected recovery is only safe if certified-input builders cover each skill's true upstream dependencies. Add executable coverage assertions for governed artifacts and downstream skill input propagation so a missing certified input fails tests rather than silently reopening too late.
- S1 ordering must account for the actual model substeps, including `validate` where relevant; canonical ordering may skip transition-only substeps only with an explicit test-backed rationale.
- Self-bootstrap checkpoint: after the content-hash freshness and compile-safe
  restamp-removal boundary lands, immediately run this worktree's current
  `go run . validate/next/health` against this change. If the new engine marks
  its own existing evidence stale or non-actionable, fix that recovery path
  in-repo before continuing.
- Self-governance: this change is driven through Slipway's own flow using `go run .` from this worktree (latest code) as a black-box caller. Any point that requires guessing, reading source, or hand-editing engine-owned evidence to proceed is a defect to fix in-repo, not to work around.
- Agent docs must not duplicate concrete Slipway usage mechanics. `CLAUDE.md`
  and `AGENTS.md` must state the black-box/current-worktree self-loop contract
  without command walkthroughs or JSON classification examples, and must treat
  guess-required nodes as immediate product defects.

## Acceptance Signals
- `go build ./...`, `go vet ./...`, `go test ./...` green; toolgen self-loop (regenerated skills/commands) has zero drift; `go run . validate --json` green from the current worktree.
- For every governed state, changing a governed artifact or relevant code input after an accepted verdict → `go run . run --json` reopens the earliest affected authority and hands off the correct existing skill, with no `slipway recover`, evidence restamp, manual digest edit, timestamp edit, or source inspection required.
- Each prior dead-end is replayable without guessing: #90 (S0 intake stale), #89 (S2 structural reopen + scope-only in-place rebuild), #81 (S1 plan-audit stale), #96 (pivot preserves compatible task evidence, including TaskPID cleanup), #97 and the S2 repair gap (execution-summary/wave-plan regenerate from changed source).
- #89 scope-only replay preserves compatible runtime task evidence only when the
  task IDs, objective, wave/dependencies, task_kind, covers, evidence,
  acceptance, and checkpoint_type are unchanged and only target_files changed:
  `tasksPlanChangedSinceTaskEvidenceBlockers` and task-evidence
  `freshness_inputs.tasks_plan_hash` do not stale task evidence for that narrow
  target_files-only edit, while any other task contract change reopens S1/audit.
- No `slipway recover` surface anywhere (`slipway --help`, generated skills/commands); no Tier-2; no Tier 0/1/2 recovery vocabulary.
- Read-only `next`/`validate`/`status`/error surfaces report the same root authority and next public action as mutating `run`.
- Input-builder coverage tests prove each governed artifact is certified by at least one authority and every downstream skill includes the upstream artifacts it relies on, or carries an explicit test-backed exemption.
- After t-01, this change's own `go run . validate --json`, `go run . next --json --diagnostics`, and `go run . health --governance --json` remain actionable under the new content-hash freshness semantics without hand-editing engine-owned evidence.
- External API contract proof: recovery JSON compatibility/intentional changes,
  command availability, docs, and generated surfaces are verified by automated
  tests; later review evidence must include `layer:R3=pass` and `layer:IR3=pass`.
- `CLAUDE.md` and `AGENTS.md` contain no concrete Slipway command tutorial,
  JSON classification example, or duplicated lifecycle mechanics; they require
  black-box latest-current-worktree Slipway use and immediate product repair
  for guess-required nodes.
- All governed host skills, Slipway workflow/command skills, command
  references, and repo docs match the latest CLI behavior and recovery JSON
  vocabulary after regeneration.

## Resolved Technical Questions
<!-- S1 research closed these; no blocking open questions remain. -->
- Freshness call sites are identified: mtime freshness is limited to the `digest*ChangedAfterVerdict` family, and digest `run_version` freshness is limited to skill-digest comparisons. Task/runtime run-version binding remains separate and is not removed.
- The prior content-hash and #89 split approach ports to current `main`; the branch starts from `30f6fa5` with no implementation diff.
- Canonical ordering derives from `action.WorkflowPath`, registry `State`/`PlanSubStep`, and `planSubStepOrder`; S1 `validate` requires explicit handling/test rationale because it exists in the model but is not part of the normal forward plan-substep sequence.
- User-facing recovery must be a normal `go run . run` re-walk. If this change's own governance state needs source reading, digest edits, timestamp edits, or a second recovery command to proceed, that is an implementation defect in this change.

## Deferred Ideas
- Per-task scope-hash granularity (whole-plan scope hash is sufficient for #98).
- Auto-stamping verdict timestamps to reduce hand-authored-timestamp burden (separate follow-up).

## Approved Summary
Deliver Recovery UX P2 (#98) as an `external_api_contracts` governed change:
generalize stale-evidence recovery so that, in every governed state, a changed
governed artifact or code input makes `slipway run` reopen the earliest
 affected authority, clear that authority and its downstream evidence, preserve
runtime task evidence, and route the existing host skill. One PR, full delivery:
the generalized reopen core + rebuilt content-hash freshness (no
mtime/run_version) + the #89 structural/scope hash split (S2 in-place rebuild
for benign target_files edits and structural task-evidence freshness) + the #96 pivot evidence-preservation fix + #97
regenerate-on-change + the F1 S1/audit fix. The #85 escape-hatch direction is
removed entirely (no recover command, no Tier-2 attested restamp, no Tier 0/1/2
vocabulary, no normal-path evidence restamp). Authority ordering derives from
the canonical lifecycle/registry, not hand-maintained rank tables. Public
CLI/JSON and generated-surface changes require contract tests, docs updates,
rollback notes, and S3 external-contract review tokens before closeout.
CLAUDE/AGENTS agent instruction files must stop being concrete Slipway command
manuals and instead enforce black-box current-worktree self-dogfooding plus the
self-optimization loop; all governed and Slipway skills must be regenerated or
updated to match current CLI logic and lifecycle behavior.

Scope boundaries — Out of scope: a recover command / dependency-ordered recovery graph / Tier-2 attested restamp (removed, not deferred); force-close redesign (#75); worktree-rebind / dual-active cleanup (#86). S1 research resolved the technical questions; remaining risk is covered by executable tasks and verification.

Primary acceptance: every prior dead-end (#81, #89, #90, #96, #97) is replayable with current-worktree `go run . run` alone — no recover / restamp / manual digest edits / source reading / guessing — and `go build/vet/test ./...` plus `go run . validate` are green after `go run . init --refresh --tools all` with zero generated-surface drift.

Confirmed by user: 2026-06-06T05:19:12Z.
