# Intent

## Summary
Fix two P1 governed-lifecycle defects plus a related governance-UX gap in
Slipway's own engine, in one change:

- **#95 — premature advance.** At S2_EXECUTE the wave-orchestration sync loads
  the full wave-plan task set but never asserts every planned task has passing
  evidence, so a host that records evidence for only the early tasks advances to
  S3_REVIEW with later tasks unexecuted.
- **#88 — high-risk dead-end + false repair surface.** A guardrail-domain change
  is blocked at G_ship by `high_risk_check_missing: <domain>.safety_baseline`
  with no operator-visible, fail-closed way to satisfy it, and
  `slipway repair --focus sast` is a no-op that falsely advertises running SAST.
- **worktree provisioning — main occupied, no parallelism.** Governed changes
  only get a dedicated `.worktrees/<slug>` (`feat/<slug>`) worktree when
  `needs_discovery` is true; non-discovery changes are skipped
  (`worktree_skipped_reason: discovery_not_required`) and run their entire
  S0→done lifecycle in the main checkout, so the user cannot work in parallel and
  the documented "auto-provision `feat/<slug>`" contract is not honored.

## Complexity Assessment
complex
<!-- Rationale -->
Three governance-engine concerns spanning many surfaces (progression engine,
gate catalog, reason codes, recovery remediation, capability registry, worktree
provisioning + change creation, generated skill templates, docs, toolgen,
tests). #88 touches high-risk safety-gate semantics that must stay fail-closed
with no new bypass; the worktree change alters change-creation/lifecycle wiring
with non-trivial test blast radius. This change runs in the main checkout as the
bootstrap (the worktree fix cannot retroactively relocate itself); subsequent
governed changes inherit dedicated worktrees.

## Guardrail Domains
none. This change modifies Slipway's own governance engine and CLI/JSON surface;
it does not operate in a sensitive product domain (no auth / credentials / PII /
financial / schema-migration / irreversible-ops / external-API **product**
change). The `safety_baseline` gate code is edited, but the change keeps it
fail-closed and adds no bypass. Public CLI, JSON, generated skills, and docs are
reviewed as external contracts.

## In Scope
**#95 — execution completeness (fail-closed; per-task blocker):**
- `internal/engine/progression/wave_sync.go`: in `evaluateGovernedWaveExecution`,
  after building `runs`, assert every planned `WavePlan.TaskIDs()` task has a
  passing run; emit one `incomplete_execution_task:<taskID>` blocker per missing
  task in both the preview and mutate paths (guarded by no plan-drift).
- `internal/model/reason_code.go`: register canonical `incomplete_execution_task`.
- `internal/model/recovery.go`: add a `blockerRemediations` entry (refresh-wave
  class) — execute & record the task, or rescope `tasks.md`, then re-run.
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`: state the
  completeness contract (every planned task needs passing evidence before review;
  rescope to intentionally drop a task).

**#88 — safety_baseline satisfy-path + repair `--focus sast`:**
- Make the high-risk satisfy-path operator-visible and fail-closed: the owning
  governance skill records the `<domain>.safety_baseline` verdict from a real
  SAST run/triage; the public surface (blocker remediation, `slipway next`
  handoff, generated SKILL.md) carries the exact reference token, the producing
  skill, and the required action. No bypass / self-attestation path.
- `internal/model/recovery.go` (+ gate reason detail): make
  `high_risk_check_missing` / `high_risk_check_failed` remediation actionable
  (name the token, the producer skill, the command).
- `slipway repair --focus sast`: remove the false-promise focus from the `repair`
  command surface; keep the SAST focus where it legitimately hydrates
  sast-orchestration guidance (review / validate). repair must never advertise a
  scan it cannot run.
- Generated skill templates updated to document how/when the domain-specific
  `safety_baseline` reference is recorded.
- `docs/` + toolgen regeneration (zero drift).

**worktree provisioning — dedicated worktree for every governed change:**
- `internal/state/worktree.go` (`EnsureDefaultWorktreeForChange`): provision the
  dedicated `.worktrees/<slug>` worktree on `feat/<slug>` at `slipway new` for
  ALL governed changes, not only `needs_discovery`; keep skipping only when the
  environment cannot support it (not a git repo / no HEAD). The governed bundle
  is created inside the worktree from S0, so intake/planning/execution leave the
  main checkout free for parallel work.
- Reconcile the downstream lifecycle for a bound non-discovery change: the
  `advance_governed.go` worktree gates (`NeedsDiscovery`-only today) must
  short-circuit cleanly when already bound; `ValidateChangeWorktree`, the
  single-active-change creation guard (worktree-scoped per #50), and
  `done`/archive worktree cleanup must behave for non-discovery bound changes.
- Update affected tests/fixtures (prefer a single central default over per-test
  churn) so `slipway new` worktree provisioning is covered without breaking
  unrelated suites.

## Out of Scope
- Changing which guardrail domains require `safety_baseline` (keep the catalog).
- Running external SAST tooling from inside any Slipway command (the host runs
  scanners and records evidence; Slipway never executes scanners).
- Any bypass / force-close / private-attestation path for high-risk checks.
- `evidence-task-dx-issue.md` (unrelated local draft) and issue #96.

## Constraints
- Sensitive-domain `safety_baseline` must come from a real owning-skill verdict;
  no manual digest/timestamp edits, no self-attested gate tokens.
- Keep code, generated skills, docs, and agent instructions aligned (toolgen
  self-loop zero drift).
- Preserve unrelated local work.
- Smallest clean design; remove obsolete/false surfaces over compatibility shims.

## Acceptance Signals
- **#95:** a change at S2_EXECUTE with a planned task lacking evidence is BLOCKED
  from S3_REVIEW with `incomplete_execution_task:<taskID>` and an actionable
  remediation; completing or rescoping the task unblocks; `slipway validate`
  surfaces the same blocker; the all-tasks-recorded happy path still advances.
- **#88:** a guardrail-domain change has a documented, operator-visible,
  fail-closed path to satisfy `<domain>.safety_baseline` via the owning skill's
  recorded evidence (no bypass); the `high_risk_check_missing` remediation names
  the exact next action; `slipway repair --focus sast` no longer makes a false
  promise.
- **worktree:** `slipway new` for a non-discovery governed change provisions a
  dedicated `.worktrees/<slug>` worktree on `feat/<slug>` with the bundle inside
  it (the main checkout stays clean); the change advances S0→done bound to that
  worktree; `worktree_skipped_reason` is no longer `discovery_not_required`.
- `go build ./... && go vet ./... && go test ./...` green; toolgen zero drift.

## Open Questions
<!-- none requiring research; the #88 producer/recording wiring is a design
decision resolved during S1 planning from the current engine source. -->

## Deferred Ideas
- A short top-level alias for the highest-frequency `slipway evidence task`
  mutation (tracked separately in evidence-task-dx-issue.md).

## Approved Summary
Confirmed 2026-06-06T12:04:29Z (rescoped from the original two-issue change to add
the worktree-provisioning concern at the user's request; worktree design = "A").

One governed change fixes three related Slipway governance-engine concerns.
**#95:** the S2_EXECUTE wave sync gains an execution-completeness assertion — any
planned wave-plan task lacking passing evidence is blocked fail-closed from
S3_REVIEW with a per-task `incomplete_execution_task:<taskID>` blocker
(remediation: execute & record the task, or rescope `tasks.md`); the same blocker
surfaces under `slipway validate`; the all-recorded happy path still advances.
**#88:** goal-verification (already required at S4, its References already feed
the ship gate) documents recording `high_risk_check:<domain>.safety_baseline=pass|fail`
from a real SAST run; the `high_risk_check_missing/failed` remediation and the
`slipway next` handoff name the exact token, the producer, and the action; and
`slipway repair --focus sast` stops falsely advertising a scan it cannot run
(focus removed from repair, redirected to review/validate). **worktree:** a new
`governance.auto_provision_worktree` config (default true — chosen design A:
unconditional with opt-out) makes `slipway new` provision a dedicated
`.worktrees/<slug>` worktree on `feat/<slug>` for every governed change with the
bundle inside it, freeing the main checkout; the single-active-change guard stays
isolation-correct and archive still strips the worktree path; non-git / disabled
environments degrade gracefully.

Boundaries: **no** bypass / force-close / self-attestation for high-risk checks;
Slipway never runs external scanners itself; the guardrail catalog and the
unrelated `evidence-task-dx-issue.md` draft are untouched. This change runs in the
main checkout as the bootstrap; subsequent governed changes inherit dedicated
worktrees. Primary acceptance: incomplete execution cannot reach review, the
high-risk dead-end has a fail-closed next action, governed changes get isolated
worktrees, and `go build/vet/test` + toolgen self-loop stay green with zero drift.
