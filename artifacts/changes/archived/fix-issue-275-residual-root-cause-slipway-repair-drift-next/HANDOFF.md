# HANDOFF — fix-issue-275-residual-root-cause-slipway-repair-drift-next

> Change-scoped resume notes. Use the worktree dev binary; operate from THIS worktree cwd.
> Build first: `go build -o /tmp/slipway-275 .` (from this worktree). Always rebuild on resume.

## Objective
Fix issue #275 residual root cause: `slipway repair` drift `next_action` falls through to a
misleading "run `slipway run`" for tasks.md parse-failure drift. Route it to "fix tasks.md".

## GROUND TRUTH (captured via live repro this session — do NOT re-investigate)
The repro (HANDOFF original) was run; exact `slipway repair --json` output for an S2 change
whose tasks.md has an unknown metadata key `scope_amendment`:
- NonRepairableFindings: `issue275-...: task "task-02" uses unknown metadata key "scope_amendment"`
- DRIFT[1] (THE BUG): reason=`task "task-02" uses unknown metadata key "scope_amendment"`,
  target=`<slug>`, next_action=`run `slipway run` to repair the current lifecycle evidence and
  continue alignment` ← the DEFAULT at `cmd/repair.go:614`. reason contains "unknown metadata key",
  NOT "wave plan", so it misses the existing `case "wave plan"` (line 605) and hits default.
- DRIFT[0] (NOT a bug, leave alone): reason=`stale_planning_evidence`,
  next_action=`rerun wave-orchestration so Slipway refreshes the task-derived wave projection...`
  from `internal/state/execution_summary.go:747` (general stale-planning guidance). Once tasks.md
  is fixed per DRIFT[1], "rerun wave-orchestration" becomes correct → DRIFT[0]+DRIFT[1] form a
  coherent recovery sequence. Do NOT touch the freshness layer (over-scope + sensitive).

## THE FIX (S2 — small)
In `cmd/repair.go` `repairDriftNextAction` (line 600-616), add a case BEFORE the default:
- match reason containing "unknown metadata key" (and "wave_plan_load_failed" / wave-plan
  derivation failure) → return guidance like: "edit tasks.md to fix or remove the unsupported
  metadata key, then re-run `slipway repair` / `slipway validate`".
- Keep the existing `case "wave plan"` (line 605) — but note it ends with "...then run
  `slipway run`", which is fine for rebuildable wave-plan drift; the NEW case is for the
  unparseable/unknown-key case where repair canNOT rebuild. Order the new case to win for the
  unknown-key reason (it does not contain "wave plan", so a new `case strings.Contains("unknown
  metadata key")` is sufficient; also cover wave_plan_load_failed token).
Sibling surfaces already route to fixing tasks.md (verify, align only if divergent):
- `cmd/common.go:766` and `:862` wave_plan_load_failed → "Update tasks.md ... before continuing."
- validate tasks_checklist_invalid_format → "Fix the tasks.md checklist format before continuing."
FAIL-CLOSED: do NOT auto-rewrite governed tasks.md.

## Regression test (t-02) — scaffold proven this session
New file `cmd/issue275_repair_guidance_test.go`. Reuse helpers (all in cmd pkg):
`createGovernedRequest(t, root, "L2", ...)` → `issue227SeedTwoWaveExecution(t, root, slug)`
(defined in cmd/issue227_228_execution_summary_test.go:87) → record task evidence for `task-01`
(cmd/checkpoint.go) + `task-02` (cmd/evidence.go) via `makeEvidenceCmd()` task subcmd
(`--run-summary-version 1 --task-kind code --verdict pass`) → record wave-orchestration skill
evidence (`skill --skill progression.SkillWaveOrchestration --verdict pass`) which materializes
execution-summary.yaml → overwrite tasks.md (writeBundleArtifactFile) adding
`  - scope_amendment: x` under task-02 → run `makeRepairCmd()` `--json`, unmarshal into
`repairSummary` → assert the unknown-metadata-key drift finding's next_action mentions tasks.md
and has NO "slipway run". Also run validate/run/next `--json` in same state and assert all four
point at fixing tasks.md. (A throwaway version of this ran green this session and produced the
GROUND TRUTH above; it was deleted to keep S1 artifact-only.)

## Lifecycle status (as of this session)
- State: **S1_PLAN at plan-audit** (`next_skill=plan-audit`; blocker `required_skill_missing:plan-audit`).
  Bundle AUTHORED + structurally CLEAN (`validate --json` blockers: null).
- Done this session: intake-clarification confirmed (Approved Summary written + user-confirmed
  2026-06-20) + intake evidence recorded; requirements.md (REQ-001 routing, REQ-002 fail-closed,
  REQ-003 four-surface consistency) + tasks.md (t-01 code cmd/repair.go+cmd/common.go;
  t-02 test) authored. Plan-audit RUN via independent fresh-context subagent (audit_origin).
- **PLAN-AUDIT RESOLUTION**: subagent returned BLOCK on B1/B2, but BOTH were REFUTED by a live
  repro run twice (see verification/plan-audit-notes.md "EMPIRICAL RESOLUTION"). Net verdict =
  **PASS with folded guardrails**. The premise (unknown-metadata-key drift hits the default at
  repair.go:614, not the "wave plan" case at :606) is empirically confirmed.
- **NEXT (resume here)**:
  1. Record plan-audit evidence with a PASS verdict + distinct origin handles:
     `/tmp/slipway-275 evidence skill --skill plan-audit --verdict pass --reference "plan-audit:pass"
     --reference "plan_origin:host-s1-author" --reference "audit_origin:subagent-aee9389e"
     --notes-file artifacts/changes/<slug>/verification/plan-audit-notes.md`
     (handles MUST be distinct — host authored, subagent audited). This is a HARD-GATE: present
     audit result to user, get explicit confirm, THEN `slipway run` to advance to S2.
  2. S2 wave-orchestration: implement t-01 in cmd/repair.go (add the dedicated case per "THE FIX"
     above + guardrails), add t-02 regression test (cmd/issue275_repair_guidance_test.go using the
     proven scaffold above), `go test ./...` + `gofmt -s -l` green, record task evidence
     (t-01 task_kind=code, t-02 task_kind=test, --run-summary-version 1) + wave-orchestration skill
     evidence → S3 reviews (selected_review_skills; distinct review_origin handles per reviewer)
     → closeout → `slipway done` → rebase + DIRECT clean PR closing #275.

## Gotchas
- README/toolgen contract test (TestReadmeAndCommandDescriptionsReflectCurrentEntrySurface) — a
  repair help/desc change can ripple; this fix changes runtime string only, not desc, so safe.
- Lint gate = golangci-lint gofmt **simplify**; verify `gofmt -s -l`.
- Multi-active workspace: other changes bound to their own worktrees; operate only on THIS one.
- Do NOT hand-edit verification YAML / digests; use public `slipway evidence`.
- task_kind valid set = {code, test}.

---
## SESSION 2026-06-20b — S2 CODE LANDED + GREEN (resume from here)
- State advanced to **S2_IMPLEMENT** (next_skill=wave-orchestration). plan-audit:pass recorded
  (plan_origin:host-s1-author / audit_origin:subagent-aee9389e).
- **t-01 LANDED** in `cmd/repair.go` `repairDriftNextAction`: new case BEFORE `case "wave plan"`
  matching `unknown metadata key` / `wave_plan_load_failed` → returns "edit tasks.md to fix or
  remove the unsupported metadata key, then re-run `slipway repair` / `slipway validate`". Builds OK.
- **t-02 LANDED** `cmd/issue275_repair_guidance_test.go` (2 tests) — BOTH GREEN, gofmt -s clean:
  - TestIssue275RepairDriftRoutesToFixingTasks: repair drift → tasks.md, no "slipway run" (REQ-001);
    second repair still drifts = fail-closed (REQ-002). PASS.
  - TestIssue275ParseFailureGuidanceConsistentAcrossSurfaces: repair/validate/run/next none emit
    the misleading default; each points at the actionable cause (REQ-003). PASS.
- **FINDING for user (decide scope)**: run/next fail-closed with error "failed to derive the current
  wave plan ... task uses unknown metadata key" — names the cause but does NOT print the
  "Update tasks.md..." remediation sentence (remediation is in the error struct, not stdout/--json,
  because the integrity error returns before JSON renders). NOT the #275 bug (no misleading
  "slipway run"). Possible follow-up: surface remediation in run/next error output. Likely OUT of
  #275 scope — confirm before expanding. cmd/common.go was NOT edited (no dead-end to fix there).
---
## SESSION 2026-06-20c — S2 evidence recorded, advanced to S3_REVIEW (resume from here)
- Rebuilt `/tmp/slipway-275`. Verified: `go test ./...` GREEN (all pkgs), `gofmt -s -l` clean,
  `go vet ./...` clean. #275 targeted tests both PASS.
- DECISION: `cmd/common.go` NOT edited — its `:766`/`:862` wave_plan_load_failed remediation
  already reads "Update tasks.md ... before continuing." (already aligned, no "slipway run"
  dead-end). t-01 keeps cmd/common.go as a declared-but-unchanged target. `scope_contract:pass`
  confirms this is safe (planned_targets superset of changed_files).
- Recorded S2 evidence via PUBLIC flow: `evidence task` t-01 (code, changed cmd/repair.go) +
  t-02 (test, changed cmd/issue275_repair_guidance_test.go), run-summary-version 1; then
  `evidence skill --skill wave-orchestration --verdict pass`. `slipway run` → **S3_REVIEW**.
- S3 facts: preset=light, run_summary_version=1, guardrail_domain=NONE (no SAST/safety_baseline).
  4 selected reviewers (parallel peers): spec-compliance-review (`layer:R0=pass`+scope_contract:pass
  +negative_path:pass), code-quality-review (`layer:IR1=pass`), independent-review (no token),
  goal-verification (no token, but PRODUCES `verification/suite-result.yaml` keystone + runs fresh
  suite). Then final-closeout STRICTLY LAST. distinct-handle gate = advisory on light (still doing
  distinct native-subagent handles). SuiteResult fields: version, run_summary_version,
  full_suite_digest, sast_digests(omit, no guardrail), captured_at.
- IN FLIGHT: background full-suite transcript → `verification/suite-transcript.txt` → sha256 →
  write `verification/suite-result.yaml` (run_summary_version 1). THEN spawn 4 native-subagent
  reviewers (distinct review_origin handles) → record each via `evidence skill` → `slipway run`
  → final-closeout → record → `slipway done` → rebase + DIRECT clean PR closing #275.

## (prior) NEXT
- (1) full `go test ./...` + `gofmt -s -l ./...` green. (2) Record S2 evidence via PUBLIC
  flow: `/tmp/slipway-275 evidence task --change <slug> --task-id t-01 --run-summary-version 1
  --task-kind code --verdict pass --evidence-ref test:t-01 --changed-file cmd/repair.go
  --target-file cmd/repair.go` ; same for t-02 (--task-kind test, --changed-file+--target-file
  cmd/issue275_repair_guidance_test.go) ; then `evidence skill --skill wave-orchestration
  --verdict pass --reference wave-orchestration:pass`. (3) `slipway run` → S3 reviews
  (selected_review_skills; distinct review_origin per reviewer via native subagents) → closeout
  → `slipway done` → rebase + DIRECT clean PR closing #275. Build /tmp/slipway-275 FIRST on resume.
