# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: repo-native Go; `gofmt`/`golangci-lint`; table-driven tests colocated as `*_test.go`

## Summary
Resolve the critical core of #66, #70, and #67 under one thesis: governance signals
must not be lossily projected to a scalar (timestamp/mtime/opaque blocker) at the
consume boundary; name the specific artifact and the supported remediation. #66 + #70:
rebind skill-evidence and stale-planning-chain freshness from wall-clock/mtime to
engine-owned content digests stored in gitignored `verification/evidence-digests.yaml`
(Approach B′); the host `VerificationRecord` and `change.yaml` schemas are unchanged;
the engine stamps digests only at mutating verdict acceptance; `plan-audit` includes
`assurance.md`; `tasks.md` uses `TaskPlanSemanticHash`; steady-state freshness does not
consume filesystem mtime or wall-clock timestamps. Legacy file-absent migration uses a
one-time verdict-timestamp safety gate before silent backfill. #70 makes
`wave-plan.generated_at` display/audit materialization time only, never a freshness
input. #67 routes S4 post-review remediation to goal-verification + final-closeout,
naming changed artifacts when digest diagnostics are available. #59 traceability
legibility is split into a separate quick PR and #59 stays open with a per-item ledger;
#71 is out of scope as review-verdict quality, not freshness.
## Complexity Assessment
critical
<!-- Rationale: #66/#70 rebind the evidence-freshness signal that gates every state
transition and the stale-planning recovery chain. A wrong digest comparison either
advances on stale/uncertified evidence (false completion) or permanently blocks
progression. Touches an engine-owned runtime digest store + migration of existing
evidence, the run-time acceptance path, and externally-consumed JSON surfaces. High
blast radius, severe if wrong. -->

## Guardrail Domains
schema_data_migration

## In Scope
Single unifying principle across the critical core: **stop projecting a structured
governance signal to a scalar at the consume boundary.** One thesis does not force one
PR: #59 is split out because it is independent and low-risk; this critical bundle keeps
the coupled freshness/remediation work.

- **#66 — content-addressed evidence freshness (INT-001; structurally novel core):**
  - New `internal/model/evidence_digests.go`: `EvidenceDigests`/`SkillDigest` types +
    `EvidenceFreshness(stored, current) (fresh bool, changed []string)`. The host
    `VerificationRecord` is **unchanged** (no `InputDigest` field); `change.yaml` is
    **unchanged**.
  - New `internal/state/evidence_digests.go`: `Save/LoadEvidenceDigests` persisting an
    engine-owned, gitignored `verification/evidence-digests.yaml`; the record reader
    (`listVerificationsInDir`) skips it by name.
  - **Engine stamps the digest when it accepts a passing verdict during `slipway run`**
    (agent writes only verdict/blockers/notes; engine locks "what content this verdict
    certified"). A shared `stampAcceptedEvidenceDigests` runs at both mutating acceptance
    sites: the required-skill block in `advance_governed.go` (S1→S2) and the S3/S4
    auto-pass authority paths in `autopass.go`.
  - `plan-audit` certifies `intent.md`, `requirements.md`, `decision.md`,
    `research.md` when present, `tasks.md`, and `assurance.md`; `tasks.md` uses
    `wave.TaskPlanSemanticHash` (checkbox-invariant); prose docs use
    `model.ComputeInputHash`; file sets use sorted `{path: ComputeFileContentHash}` via
    `ComputeInputHash`.
  - Migrate judgment skills: bundle input-set for `plan-audit`; changed/target set for
    `goal-verification`; same set + `assurance.md` for `final-closeout`; reviewable diff
    set for `spec-compliance-review`, `code-quality-review`, `security-review`, and
    `independent-review`.
  - Diff-class review input-set decision: non-ignored untracked **reviewable** files are
    included because a review certifies the current working diff; ignored/runtime evidence
    files and Slipway governed bundles under `artifacts/changes/**` are excluded.
    Goal/final verification do **not** stale on unrelated untracked files unless those
    files are in the execution summary changed/target set.
  - Convert/remove steady-state mtime+timestamp evidence-freshness branches:
    `internal/engine/progression/authority.go` closeout-reuse content freshness blockers;
    retain closeout proof-ordering timestamp checks as domain-specific ordering gates,
    not evidence-freshness signals;
    residual time branches in `internal/state/execution_summary.go`,
    `internal/state/execution_repair.go`, and `internal/engine/context/context.go`
    (`EvaluateEvidenceFreshness` time path). `wave-orchestration` keeps its embedded
    **logical** `CapturedAt`/`run_version` binding (not filesystem mtime).
  - Legacy file-absent migration is the only evidence-freshness mtime carve-out: before
    silent backfill, if any certified artifact has filesystem mtime after the verdict
    timestamp, do **not** backfill; report stale and require re-verification. Steady-state
    freshness remains digest-only.
- **#70 — wave-plan / stale-planning chain freshness must be content-based (INT-004):**
  - `internal/state/wave_execution.go` `MaterializeWavePlan` must not derive
    `wave-plan.generated_at` from `tasks.md` `ModTime()` (`currentTaskPlanNodes`).
    `generated_at` becomes display/audit materialization time only (an injected/actual
    materialization timestamp is acceptable), and no freshness path may consume it.
  - The stale-planning chain in `internal/state/execution_summary.go` keys freshness on
    `tasks_plan_hash` (semantic) rather than `generated_at` timestamp ordering, so a
    refreshed `plan-audit` with unchanged task content cannot leave
    `wave-plan.yaml`/`execution-summary.yaml` permanently stale after S4 recovery.
- **#67 — S4 post-review recovery routing (INT-003):**
  - `cmd/evidence.go`: when `evidence task` hits S4, the `evidence_task_wrong_state`
    remediation routes to the supported path (task evidence is S2-only; in S4 refresh
    by re-running goal-verification + final-closeout).
  - `internal/engine/progression/authority.go` (`closeout_goal_verification_reuse_invalid`
    detail) and `internal/engine/gate/gate.go` (`verification_evidence_missing` detail):
    carry the same supported-refresh remediation and, via #66 when available, name the
    changed artifact. The basic routing string is independent; artifact naming depends on
    digest diagnostics.
- **Tests** proving each acceptance signal below, including checkbox/`git restore`
  false-positive regressions, legacy-backfill safety-gate regressions, #70 refreshed-chain
  regressions, untracked input-set policy, and a guard proof that steady-state freshness
  does not compare filesystem mtime/wall-clock.

## Out of Scope
- **#59 traceability gap legibility implementation** — split into a separate quick PR
  because it is independent, low-risk, and should not wait behind this critical
  schema/migration guardrail. This bundle keeps a per-item #59 ledger and references #59
  but does not close it.
- **Governance snapshot cache (`health`) redesign** — a related-but-separate concern
  (per #66); not touched here. (User-confirmed boundary.)
- **#59 item 3** (`run` returns error-severity blockers after advancing) — a separate
  result-framing issue, not fixed here.
- **#71** (review gates passed despite missing MUST-level contract coverage) — review
  verdict quality/strictness, not freshness or artifact-remediation routing; track
  separately.
- Changing what any skill verifies *semantically*; only the freshness *signal* changes.
- Making `intake-clarification` / `research-orchestration` verdicts pure live-recompute
  (the engine cannot reproduce "the agent actually clarified/researched"); their
  evidence stays, only its freshness signal rebinds from time to content.
- Backward-compat shim for the old time-based behavior beyond the silent-backfill
  migration below; no compat scaffolding remains in steady state.

## Constraints
- Go; `go build ./...` and `go test ./...` green; `gofmt`/`golangci-lint` clean.
- Clean break in steady state: no backward-compat scaffolding for the old time-based
  behavior after migration. Migration is **guarded silent backfill** — a legacy change
  (no `evidence-digests.yaml`) with already-accepted passing skills reads fresh only if
  the one-time verdict-timestamp safety gate passes; the next `slipway run` materializes
  digests and emits `digest_backfilled_from_legacy_verdict`. A feature-active change whose
  digest file exists but lacks a stamped skill entry treats that skill as NOT fresh.
- Guardrail-domain protections (`domain_review`, `rollback_required`) are fail-closed
  and stay enabled.
- **Non-negotiable:** the `tasks.md` digest MUST be the semantic (checkbox-invariant)
  hash; a raw-byte hash would reproduce the exact false-positive this change removes.
- Lands as one critical PR with cleanly separable commits (#66/#70 core → #67 routing).
  #59 is a recommended split-out quick PR.

## Acceptance Signals
- GIVEN passing `plan-audit` evidence AND a checkbox-only `tasks.md` writeback,
  WHEN `G_plan` is projected, THEN `plan-audit` stays fresh (no stale blocker).
- GIVEN passing `plan-audit` evidence AND `git restore` bumps a planning `.md` mtime
  with no content change, THEN evidence stays fresh.
- GIVEN passing `plan-audit` evidence AND a real content edit to any certified artifact
  including `assurance.md`, THEN evidence is stale AND the blocker/output names the
  changed artifact.
- GIVEN a legacy file-absent change with a passing verdict whose certified artifact mtime
  is after the verdict timestamp, THEN silent backfill is refused and the skill requires
  re-verification; if all certified artifacts are not newer than the verdict, the next
  `slipway run` backfills once and records `digest_backfilled_from_legacy_verdict`.
- GIVEN S4 stale-planning recovery reopened S1, accepted a fresh `plan-audit`, and the
  task semantic hash is unchanged, WHEN the chain is re-materialized THEN
  `wave-plan.yaml`/`execution-summary.yaml` are NOT left permanently stale (#70).
- `wave-plan.generated_at` is a display/audit materialization timestamp only; freshness
  does not compare against it and keys the chain on `tasks_plan_hash`.
- A new non-ignored untracked reviewable file after review makes diff-class review
  evidence stale; ignored/runtime files are excluded; an unrelated untracked file does not
  stale goal-verification unless it is in the execution summary changed/target set.
- No steady-state evidence-freshness code path compares filesystem mtime or wall-clock
  `now` (the `wave-orchestration` logical `CapturedAt`, closeout proof-ordering gates,
  and legacy migration safety gate carve-outs excepted; guard proof + tests).
- After an S4 source/test edit, `slipway validate --json` blocker remediation names the
  supported refresh path; `slipway evidence task` in S4 returns a remediation routing
  to goal-verification + final-closeout refresh.
- `go build ./...` and `go test ./...` pass.

## Open Questions
- [x] Acceptance/stamping point — RESOLVED in research.md (acceptance at advance_governed required-skill block and autopass authority paths; engine never writes verification/*.yaml, so digests live in engine-owned verification/evidence-digests.yaml, Approach B').
- [x] Full inventory of timestamp/mtime evidence-freshness branches — RESOLVED in research.md (authority.go closeout reuse, execution_summary.go stale-planning baselines, wave_execution.go generated_at, context.go time path, execution_repair.go legacy fallback; out-of-scope time uses enumerated).
- [x] Diff-class review input-set — RESOLVED in research.md (changed-file set hashed as a sorted path-to-content-hash map via ComputeInputHash).

## Issue Ledger
- **#66:** closes. Core solution: engine-owned content digests, acceptance stamping,
  semantic task hash, steady-state digest-only freshness, guarded legacy migration.
- **#70:** closes. `wave-plan.generated_at` no longer derives from `tasks.md` mtime and
  is not consumed by freshness; stale-planning recovery keys on semantic `tasks_plan_hash`.
- **#67:** closes if remediation strings and digest-backed changed-artifact names land.
  The basic S4 routing is independent; changed-artifact naming depends on #66 diagnostics.
- **#59:** remains open. Item 1 (health snapshot cache) out of scope; item 2
  (validate/run/health single authority) out of scope; item 3 (`run` error-severity
  blocker framing after progress) out of scope; item 4 (stale file-based gate evidence)
  is addressed by #66/#70 with the pre-acceptance trust boundary documented; item 5
  (traceability gap identities) should be a separate quick PR and must not be blocked by
  this critical PR.
- **#71:** out of scope and should be tracked separately as review-verdict quality and
  MUST-level contract coverage, not evidence freshness.

## Deferred Ideas
- Reclassifying derivable skills (bundle-completeness, traceability, scope-contract,
  tasks-parse) to drop any redundant *persisted* attestation in favor of pure live
  recompute — valuable but a larger refactor; keep this change scoped to the freshness
  signal rebinding.
- Governance snapshot-cache single-authority cleanup that would let `validate` and
  `health` always agree — the broader half of #59's lineage; tracked separately.
- #59 quick PR: add `TraceabilityGap.Artifact`, per-gap health observations, and
  text/doctor rendering without carrying this critical schema/migration guardrail.

## Approved Summary
Resolve #66, #70, and #67 in one governed change under a single principle: a governance
signal must not be lossily projected to a scalar (timestamp/mtime/opaque blocker) at the
point an operator or the engine consumes it — name the specific artifact and supported
remediation instead.

- **#66 + #70 (core):** rebind skill-evidence and stale-planning-chain freshness from
  wall-clock/mtime to engine-owned content digests. The engine computes and stamps digests
  into gitignored `verification/evidence-digests.yaml` when it accepts passing verdicts
  during `slipway run` (Approach B′); `VerificationRecord` and `change.yaml` schemas stay
  unchanged; `plan-audit` includes `assurance.md`; `tasks.md` uses the checkbox-invariant
  semantic hash; judgment and diff-class review skills migrate; steady-state freshness
  stops comparing mtime/wall-clock; legacy file-absent backfill has a one-time
  verdict-timestamp safety gate.
- **#70 display boundary:** `wave-plan.generated_at` becomes display/audit
  materialization time, never a freshness input. Freshness is keyed by semantic
  `tasks_plan_hash`.
- **#67:** route S4 post-review remediation on `evidence task`,
  `verification_evidence_missing`, and closeout-reuse blockers to goal-verification +
  final-closeout; name changed artifacts when digest diagnostics are available.

Key scope boundary: #59 is split out as a quick PR and remains open with the per-item
ledger above; #71 is explicitly out of scope. One thesis explains the family, but blast
radius governs PR boundaries.

Confirmed by user: 2026-06-04T06:05:41Z (scope + snapshot-cache held out); B′ storage and
silent-backfill migration confirmed 2026-06-04T06:36:51Z; #70 folded in per review
2026-06-04; bundle revised 2026-06-04 to split #59, add legacy safety gate, define
`generated_at`, and lock untracked input-set policy.
