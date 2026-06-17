# HANDOFF — issue #240 governed change (continue in a fresh context)

> Working handoff for a Slipway-governed change, NOT a governed artifact.
> Delete/ignore at closeout. Authoritative scope is `intent.md` (approved).

## Where we are NOW (updated this session)
- **Change slug:** `feat-governance-host-native-subagent-enforced-cross-stage-in`
- **Worktree:** `.worktrees/feat-governance-host-native-subagent-enforced-cross-stage-in`
- **Lifecycle state:** `S2_EXECUTE` **COMPLETE** — wave-orchestration evidence recorded (run_version 2,
  all 9 dispatch/executor refs); `slipway validate` = `G_plan approved`, `G_scope approved`,
  `can_advance: True`, **no blockers**. Full `go test ./...` exit 0; gofmt clean.
  **NEXT = present S2 summary → user OK → `slipway run` → S3_REVIEW** (skip steps 1-2 below, they are DONE).
- **Preset/complexity:** strict / complex / needs_discovery=true / blast_radius=high / guardrail_domain="".
- **Implementation is COMPLETE on disk** from an earlier S2 run: 1433-line diff across 27 files
  (`git diff --stat main`). Grammar (`context_attestation.go`), plan gate (`advance_governed.go`),
  review+ship seams (`authority.go`), reason-code/recovery 4-file contract, 5 templates, 2 docs,
  + new `advance_governed_test.go`. `review_origin` cleanly retired (only retirement guards left in tests).
- **DONE this session:** S0 intake + S1 research re-certified after a freshness cascade (intent.md
  open-question edit); **S1 plan-audit re-audited PASS and evidence recorded** → `G_plan: approved`,
  `G_scope: approved`. Then advanced `slipway run` into `S2_EXECUTE` (user approved the plan→execute hard-gate).

## Plan-audit evidence already recorded (do NOT redo)
`verification/plan-audit.yaml` carries: `plan-audit:pass`, `plan_origin:plan-author-240-base`,
`audit_origin:plan-audit-reclimb-q7n4` (distinct → satisfies the change's OWN new plan gate, REQ-004).
Fresh audit notes in `verification/plan-audit-notes.md` (auditor handle `plan-audit-reclimb-q7n4`).

## Wave plan — BOTH WAVES DONE, all 7 task evidence recorded (run_version 2)
- 2 waves, both `parallel: true`. **W1 = {t-01, t-02, t-07}**, **W2 = {t-03, t-04, t-05, t-06}**.
- **`run_version = 2` is LOCKED for this refresh.** All 7 tasks verified by REAL native-subagent fan-out;
  task evidence recorded at run_version 2 (handles `exec-w1-t01/t02/t07`, `exec-w2-t03/t04/t05/t06`).
- Per-wave verification results this session: t-01..t-07 all PASS; `go test ./internal/model/...`=GREEN (W1);
  W2 executors ran scoped tests GREEN on progression/tmpl/cmd; `gofmt -s -l .`=clean.
- A full `go test ./...` was launched (background id was bj5o0czi7); first pass showed NO FAIL/panic lines but
  the definitive log was incomplete at handoff — **RE-CONFIRM the full gate cleanly first** (see step 1).

## NEXT ACTIONS (fresh context — finish S2, then S3→S4→close)
1. **Rebuild CLI** `go build -o /tmp/slipway-240 .`. **Re-confirm the dogfood gate:** `go test ./...` (cmd
   pkg ~210s; run with a 10-min timeout, ONE invocation), `gofmt -s -l .` (empty), golangci-lint clean. If any
   FAIL, it's a real regression to fix before recording the wave record.
2. **Record wave-orchestration verification** at run_version 2 via
   `slipway evidence skill --skill wave-orchestration --verdict pass` with these references (engine stamps
   freshness — do NOT hand-edit YAML). NOTE: pass `--run-summary-version 2` if the flag exists; the task ledger
   is already at rv2 so the skill record must agree:
   `dispatch_mode:wave=1:parallel_subagents`, `executor_agent:wave=1:task=t-01:exec-w1-t01`,
   `executor_agent:wave=1:task=t-02:exec-w1-t02`, `executor_agent:wave=1:task=t-07:exec-w1-t07`,
   `dispatch_mode:wave=2:parallel_subagents`, `executor_agent:wave=2:task=t-03:exec-w2-t03`,
   `executor_agent:wave=2:task=t-04:exec-w2-t04`, `executor_agent:wave=2:task=t-05:exec-w2-t05`,
   `executor_agent:wave=2:task=t-06:exec-w2-t06`.
   (If `slipway validate`/`next` then shows a wave blocker — e.g. executor_agent_missing,
   incomplete_execution_task, or run_version mismatch — read it and reconcile; task ledger is rv2.)
3. `slipway validate` → confirm S2 gate clears. Present S2 execution summary; **HARD-GATE** user OK;
   `slipway run` → S3_REVIEW.
4. **S3 review** (parallel pair, OQ3→Option B, no new host): dispatch spec-compliance-review AND
   code-quality-review as native subagents on the shared worktree; each emits its `context_origin:stage=`
   token (handles below). Record each via `slipway evidence skill`. Then S4 goal-verification, then
   final-closeout (also carries #239 `closeout:reviewer_independence=pass` + chain-order tokens). Then `slipway done`.

## ⚠ DOGFOOD HANDLE RESERVATION (this change enforces its OWN cross-stage lattice — pairwise-distinct!)
Every stage handle must differ from every other AND from every executor task-handle. Use:
| Stage | Token | Handle |
|---|---|---|
| plan-audit author | `plan_origin:` | `plan-author-240-base` (recorded ✓) |
| plan-audit auditor / lattice participant | `audit_origin:` | `plan-audit-reclimb-q7n4` (recorded ✓) |
| executor set (S2 wave) | `executor_agent:wave=W:task=T:` | `exec-w1-t01 … exec-w2-t06` |
| spec-compliance-review (S3) | `context_origin:stage=spec=` | `spec-review-240-d1` |
| code-quality-review (S3) | `context_origin:stage=code=` | `code-review-240-d2` |
| goal-verification (S4) | `context_origin:stage=goal=` | `goal-verify-240-d3` |
| final-closeout | `context_origin:stage=closeout=` | `closeout-240-d4` |
These feed the review-authority (6 edges: executor/audit_origin/spec/code) and ship-authority (9 edges adding
goal/closeout) gates THIS change adds. Dogfood MUST pass them — any two equal handles fail closed on strict.
The plan-audit `audit_origin` handle IS the plan-audit lattice participant (authority.go:527-535), so spec/code/
goal/closeout/executor must all differ from `plan-audit-reclimb-q7n4`.

## S3 parallel-review structure (from intent OQ3 → Option B, user-selected)
`ResolveNextSkill` returns spec-compliance-review + code-quality-review as a parallel-dispatch PAIR at S3;
NO new orchestration host. Each review runs as a dedicated native subagent on the shared worktree per its
template and emits its `context_origin:stage=` token. The engine never enforced spec→code ordering.

## After S3: S4 goal-verification → final-closeout → done
Each emits its `context_origin:stage=` token (handles above). final-closeout also carries the #239
`closeout:reviewer_independence=pass` + chain-order tokens (kept as-is). Then `slipway done`.

## CLI / safety reminders
- Don't trust stale root `./slipway`; always rebuild `/tmp/slipway-240` from worktree source.
- Don't hand-edit verification YAML / timestamps / digests — record via `slipway evidence`.
- Don't edit `intent.md` again (re-triggers the freshness cascade: intent→intake→research re-climb).
- Two UNRELATED active changes exist (`add-explicit-black-box-safe-delete-...`, `fix-intake-open-question-digest`)
  — PRESERVE, do not touch.
- This change must pass `go test ./...`, `gofmt -s -l`, golangci-lint clean before ship (REQ-008).
