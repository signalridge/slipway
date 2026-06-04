# Research

## Research Findings

### Architecture
- **Affected modules / blast radius:**
  - `internal/model/` — a new `evidence_digests.go` for the accepted-digest store
    type + freshness evaluator; `verification.go` (record model — *unchanged*);
    `evidence.go` (`ComputeInputHash`/`ComputeFileContentHash`). `change.yaml`/`Change`
    shape is **not** modified.
  - `internal/state/` — a `SaveEvidenceDigests`/`LoadEvidenceDigests` pair next to
    `SaveExecutionSummary`, persisting `verification/evidence-digests.yaml`.
  - `internal/engine/progression/` — `advance_governed.go:143-161` (the verdict
    **acceptance point**), `autopass.go:15-70` (S3/S4 auto-pass acceptance — a
    *second* acceptance route), `authority.go:300-419` (closeout-reuse freshness,
    four time/mtime gates), `evidence.go` (required-skill evaluation), `wave_sync.go`
    (wave evidence — stays).
  - `internal/state/` — `execution_summary.go:553-619,757-862` (stale-planning +
    `latestExecutionRelevantUpdateAt` mtime baselines), `execution_repair.go:160-205`
    (legacy mtime fallback), `verification.go` (reader — strict decode), `store.go:511`
    (`SaveChange` — the persistence commit).
  - `internal/engine/context/context.go:22-59` — `EvaluateEvidenceFreshness` (the
    structural-vs-time dual evaluator; retire the time half).
  - `cmd/evidence.go` + `internal/engine/gate/gate.go` + `authority.go` (#67 remediation).
- **Dependency chains:** `cmd/run.go → runGovernedLoop → advanceIfReady → tryAdvance
  → progression.Advance → AdvanceGoverned`. Acceptance of required-skill records:
  `AdvanceGoverned → EvaluateRequiredSkillsForChange → evaluateRequiredSkills →
  state.ListVerificationsForChange`. Commit: `state.SaveChange` at every transition
  (`advance_governed.go:286,312,333,356`; auto-pass `autopass.go:50,70`).
- **Constraints / invariants:**
  - The engine has **no writer** for `verification/<skill>.yaml`; records are
    host-authored, read with strict `KnownFields(true)` (`internal/state/verification.go:301`).
    Engine only *deletes* them (stale recovery). ⇒ accepted digests cannot be stamped
    into the host record; they must live in engine-owned persisted state.
  - Accepted digests are **derived runtime state**, not part of the durable change
    definition, so they do **not** belong in git-tracked `change.yaml`. They live in an
    engine-owned, gitignored evidence file alongside the engine's existing per-change
    summaries: `verification/evidence-digests.yaml`, written like
    `execution-summary.yaml`/`wave-plan.yaml` (the only two files the engine already
    writes into the gitignored `verification/` dir) and skipped by the record reader
    (`listVerificationsInDir`). Engine-owned + agent-never-writes preserves the
    anti-cheating property; git-tracking is unnecessary because freshness is an
    inherently local working-tree comparison and the whole evidence corpus is already
    local-only (only `change.yaml` is git-tracked).
  - `wave-orchestration` freshness already uses the embedded **logical** `CapturedAt`
    + structural field-maps (not filesystem mtime); issue #66 carves it out as
    already-correct. Boundary: "no mtime/wall-clock" means steady-state freshness does
    not consume filesystem `ModTime()` or wall-clock `now`; the two explicit carve-outs
    are wave-orchestration's logical `CapturedAt`/`run_version` binding and the
    legacy-file-absent migration safety gate that compares artifact mtime to the
    already-written verdict timestamp exactly once.

### Patterns
- **Existing structural-freshness convention to extend** (execution evidence):
  - `wave.TaskPlanSemanticHash` (`internal/engine/wave/parse.go:81`) — checkbox-/format-
    invariant semantic hash of the task plan; stored as `summary.TasksPlanHash`.
  - `isTasksPlanFreshnessRelevant` (`execution_summary.go:849`) — suppresses the tasks.md
    mtime path when the semantic hash is unchanged. This is the exact false-positive
    defense to generalize.
  - `EvidenceFreshnessInput{ExpectedStructuralInput, CurrentStructuralInput}` +
    `reflect.DeepEqual` (`context.go:42-47`) — the structural comparison channel; its
    sibling `EvidenceTimestamp/LatestRelevantUpdateAt` time channel (`:48-52`) is the
    convert/delete target.
- **Hashing primitives:**
  - `model.ComputeInputHash(map[string]any)` — sorts keys, folds `\r\n`→`\n`,
    deterministic. Use for prose docs (route body through it for EOL-insensitivity) and
    for an order-independent `{path: fileHash}` aggregate.
  - `model.ComputeFileContentHash(path)` — raw byte sha256 (whitespace-sensitive).
  - `wave.TaskPlanSemanticHash(content)` — semantic; **mandatory for tasks.md**.
- **Engine-owned summary-file write idiom:** `SaveExecutionSummary` /
  `SaveWavePlan` already atomically write engine-owned files into the gitignored
  `verification/` dir; mirror that for `verification/evidence-digests.yaml`
  (write at acceptance, read at projection, skip in the record reader).
- **#59 split-out candidate:** `GovernanceHealthCheck.TraceabilityGaps` exists and is
  populated (`health.go:275,282`); `TraceabilityGap` (`model/traceability.go:34`) lacks
  an artifact locator; `--observations`/`SignalObservation` (`control/derive.go`) emits
  only blast-radius/domain, no per-gap observation. This is independent of the freshness
  migration and should land as a quick PR outside this critical bundle.

### Risks
- **HIGH — dual acceptance routes:** S1→S2 accepts via `advance_governed.go:143-161`;
  S3/S4 accept via `autopass.go` (`EvaluateReviewAuthority`/`EvaluateShipAuthority`),
  which bypasses that block. The digest-stamping hook MUST cover *both* or review/verify
  skills never get a stored digest. Mitigation: one shared `stampAcceptedEvidenceDigests`
  helper called at every mutating acceptance site; read-only projections never stamp.
- **HIGH — false-positive reproduction:** if tasks.md is hashed by raw bytes instead of
  `TaskPlanSemanticHash`, checkbox writeback (`wave_sync.go` rewrites `[ ]`→`[x]`) flips
  the digest and reproduces the bug content-addressed. Mitigation: route tasks.md through
  the semantic hash; regression test asserting checkbox-only writeback keeps the digest.
- **MEDIUM — migration of in-flight changes:** ~20 active `feat/*` worktree changes have
  no stored digest. SELECTED policy is **guarded silent backfill**: legacy changes (no
  digest file) read fresh and self-heal once on the next `slipway run` only when no
  certified artifact is newer than the accepted verdict timestamp; otherwise they require
  re-verification. Feature-active changes missing a stamped entry read not-fresh. Rejected
  alternatives: "missing ⇒ always stale, re-verify once" (disruptive to in-flight changes)
  and "missing ⇒ always trust legacy" (can lock already-drifted content as baseline).
- **MEDIUM — diff-class untracked sensitivity:** non-ignored untracked reviewable files
  will stale review evidence. This is conservative because the review certifies the current
  working diff; ignored/runtime evidence is excluded, and goal-verification keys only on
  execution-summary changed/target files.
- **MEDIUM — completeness of the time-branch sweep:** a missed site = a surviving
  false-positive. Full inventory captured under Unknowns→Resolved; grep-level acceptance
  test guards regressions.
- **LOW — local-only digest store:** the digest file is gitignored runtime state, so it
  does not travel with the branch/PR. This is consistent with all other evidence
  (verdicts, execution-summary, wave-plan are already gitignored); a fresh clone simply
  re-establishes digests via the migration policy. No git-tracked pollution of
  `change.yaml`. Tamper-resistance comes from engine-ownership (agent never writes it),
  not from git-tracking.
- **Guardrail domain:** `schema_data_migration` (change.yaml schema + migration);
  `domain_review` + `rollback_required` fail-closed and stay enabled. Reversibility: the
  change is additive (new map field, new evaluator) + deletions of dead time branches;
  rollback = revert PR (no data destruction; old changes simply lack the field).

### Test Strategy
- **Existing coverage:** `wave/parse_test.go` (semantic hash incl. checkbox-invariance),
  `context/context_test.go` (structural vs time freshness), `progression/*_test.go`
  (advance/authority), `governance/traceability_test.go`, `governance/health_test.go`.
- **Infrastructure needs:** helpers to build a `Change` with stored `EvidenceDigests`, a
  bundle with editable artifacts, and to drive an accept→edit→re-evaluate cycle. Reuse
  existing test scaffolding in `progression`/`state` tests.
- **Verification approach per acceptance signal:**
  - checkbox-only tasks.md writeback ⇒ plan-audit digest unchanged ⇒ fresh (unit).
  - `git restore`/mtime bump, no content change ⇒ digest unchanged ⇒ fresh (unit).
  - real content edit to a certified artifact, including `assurance.md` for `plan-audit`,
    ⇒ digest differs ⇒ stale + blocker names the artifact (unit + an advance-path test).
  - legacy backfill safety gate ⇒ artifact newer than verdict refuses backfill; artifacts
    not newer than verdict self-heal once and emit `digest_backfilled_from_legacy_verdict`.
  - untracked input-set policy ⇒ non-ignored untracked reviewable file stales diff-class
    review; ignored/runtime evidence excluded; unrelated untracked does not stale
    goal-verification unless in changed/target.
  - grep test / assertion that no steady-state evidence-freshness path calls `ModTime()` or
    wall-clock `now` (excluding logical `CapturedAt`, the legacy migration safety gate, and
    out-of-scope GC/display sites enumerated below).
  - S4 source edit ⇒ `validate --json` blocker remediation + `evidence task` S4 error both
    route to goal-verification+final-closeout refresh.

## Alternatives Considered
- **Approach A — `InputDigest` on `VerificationRecord`, engine gains a YAML writer.**
  Literal reading of issue #66 Phase 1. Engine starts writing `verification/<skill>.yaml`
  to stamp the digest at acceptance. *Tradeoffs:* introduces a brand-new engine write
  authority into a host-owned, git-ignored path; races with the host's own write of the
  same file; must coordinate with strict `KnownFields(true)`; digest lives in git-ignored
  local state (not durable/auditable/tamper-resistant across machines). Most invasive,
  weakest integrity. **Rejected.**
- **Approach B′ — engine-owned accepted-digest store in a gitignored evidence file (RECOMMENDED, user-directed).**
  Persist a `skill → {artifact → semantic hash}` map in an engine-owned
  `verification/evidence-digests.yaml`, written with the same atomic pattern as
  `execution-summary.yaml`/`wave-plan.yaml` and skipped by `listVerificationsInDir`.
  A shared `stampAcceptedEvidenceDigests` runs at every mutating acceptance site
  (`advance_governed.go` required-skill block + `autopass.go` authority paths),
  computing each accepted skill's input-set digest and rewriting the file. Neither
  `VerificationRecord` (host-owned) nor `change.yaml` (durable authority) changes shape.
  A new `EvidenceFreshness(stored, current)` recomputes current digests in read-only
  projections and replaces every time/mtime freshness branch; mismatch names the changed
  artifact. *Tradeoffs:* engine-owned (agent never writes it ⇒ anti-cheating holds),
  timestamp-free, no host-record or change.yaml schema churn, no new write authority into
  host-authored paths; the store is local-only runtime state (consistent with all other
  evidence). Requires defining each skill's input-set (already enumerated).
  **Most faithful to #66's anti-cheating intent; keeps derived freshness state out of the
  durable change definition.**
- **Approach C — hybrid (agent-advisory `InputDigest` + engine-owned authority).** Keep an
  optional agent-written `InputDigest` for transparency but treat the engine-owned digest
  as authoritative. *Tradeoffs:* two sources of truth, redundant surface, no integrity
  gain over B. **Rejected.**
- **Selected: Approach B′** (user-directed: keep the digest out of `change.yaml`; use
  runtime/evidence state). Engine-owned `verification/evidence-digests.yaml`,
  timestamp-free in steady state, no host-record or change.yaml schema change, stamped at
  the existing acceptance sites. **Migration policy: guarded silent backfill:** a legacy
  skill with a passing verdict but no stored digest has its current input-set digest
  materialized once on `slipway run` only if no certified artifact is newer than the
  verdict timestamp; otherwise it must be re-verified. Bounded to already-accepted
  historical skills; this change and all new changes stamp natively at acceptance, never
  via backfill. The one-time mtime safety gate exists only to avoid weakening migration
  integrity for already-drifted in-flight changes.

## Unknowns
- **Resolved:** *Where does the engine accept a verdict and can it stamp the host YAML?* →
  Acceptance at `advance_governed.go:143-161` (S1→S2) and `autopass.go:15-70` (S3/S4); the
  engine **never writes** `verification/<skill>.yaml` (host-authored, strict-read,
  delete-only). ⇒ store accepted digests in an engine-owned gitignored
  `verification/evidence-digests.yaml` (Approach B′) — not in the host record and not in
  git-tracked `change.yaml`.
- **Resolved:** *Full inventory of in-scope time/mtime evidence-freshness branches* →
  `authority.go:301,312,339,380` (closeout reuse: content freshness replaced by digests;
  proof-ordering timestamp checks retained as domain-specific ordering gates) + `:317`
  `ExecutionSummaryFreshness` pull-in; `wave_sync.go:239` (CapturedAt vs record — keep as
  wave-orchestration logical-time), `:571` (mtime discriminator behind a semantic-hash
  gate); `execution_summary.go:563,574-575,584,600-602,767,802-814` (stale-planning +
  `latestExecutionRelevantUpdateAt` mtime baselines), `:822-847` (time half of
  `EvidenceFreshnessInput`); `execution_repair.go:188-202` (legacy mtime fallback);
  `context.go:48-52` (time path of `EvaluateEvidenceFreshness`). Out of scope (do not
  touch): `fsutil/atomic.go:106`, `state/worktree.go:521`, `state/stats.go:101`,
  `artifact/manager.go:1446`, `wave_execution.go:437,454`, `wave_sync.go:300`,
  `execution_repair.go:218`, `cmd/process_unix.go:47`, `cmd/evidence.go:364`, event/archive
  logging timestamps. (`wave_execution.go:437,454` waveStarted/Completed display stays out;
  the `generated_at`/`currentTaskPlanNodes` mtime path below is the #70 in-scope site.)
- **Resolved (#70):** *wave-plan capture signal* → `internal/state/wave_execution.go`
  `MaterializeWavePlan` sets `wave-plan.generated_at` from `currentTaskPlanNodes` (~:157),
  which returns `tasks.md` `ModTime()`; `internal/state/execution_summary.go` (~:627)
  consumes `generated_at` as the planning-stage capture time. ⇒ #70 root cause; same
  family as #66. Fix: stop using `tasks.md` mtime for `generated_at`; make `generated_at`
  display/audit materialization time only; and key the stale-planning chain on the semantic
  `tasks_plan_hash`, so a refreshed `plan-audit` with unchanged task content does not
  strand the chain after S4 recovery (preserving #53's real drift detection).
- **Resolved:** *Per-skill certified input-set* → bundle artifacts from
  `internal/engine/artifact/schemas.yaml` (expanded: intent/requirements/decision/tasks/
  assurance/research). plan-audit ⇒ planning set (intent, requirements, research when
  present, decision, assurance, tasks); goal-verification ⇒ changed/target-file set from
  `closeoutGoalVerificationReuseContentPaths` (`authority.go:391`); final-closeout ⇒ same
  set plus `assurance.md`; diff-class reviews ⇒ reviewable changed-file set
  (`readiness.go:653-690` git diff ∪ non-ignored untracked reviewable files ∪
  `summary.Tasks[].Changed/TargetFiles`, excluding ignored/runtime evidence) hashed as a
  sorted `{path: ComputeFileContentHash}` via `ComputeInputHash`.

## Assumptions
- The engine-owned digest belongs in gitignored runtime evidence state, not `change.yaml`
  (user-directed) — Evidence: the engine already writes engine-owned summaries into the
  gitignored `verification/` dir (`internal/state/execution_summary.go:235`
  `SaveExecutionSummary`, `internal/state/wave_execution.go:72,300` `SaveWavePlan`), which
  `listVerificationsInDir` (`internal/state/verification.go:287`) skips by name.
- Host records stay schema-stable — Evidence: `internal/state/verification.go:301`
  (`KnownFields(true)` strict decode would reject unknown fields in host files).
- `wave-orchestration` stays as-is — Evidence: `wave_sync.go:239` uses logical `CapturedAt`
  + `run_version` binding, not filesystem mtime (issue #66 carve-out).

## Canonical References
- `internal/engine/progression/advance_governed.go:143-161` — verdict acceptance point.
- `internal/engine/progression/autopass.go:15-70` — second (S3/S4) acceptance route.
- `internal/engine/progression/authority.go:300-419` — closeout content freshness gates
  replace time/mtime freshness; closeout proof ordering remains a domain ordering check.
- `internal/state/execution_summary.go:553-619,757-862` — stale-planning/mtime baselines + semantic-hash convention.
- `internal/engine/context/context.go:22-59` — `EvaluateEvidenceFreshness` dual evaluator.
- `internal/state/verification.go:261-309` — host-authored record reader (strict decode).
- `internal/state/execution_summary.go:235` (`SaveExecutionSummary`) + `internal/state/wave_execution.go:72,300` (`SaveWavePlan`) + `internal/state/verification.go:287` (reader skip-list) — engine-owned gitignored evidence-file write/read idiom for `verification/evidence-digests.yaml`.
- `internal/model/evidence.go:13-58` + `internal/engine/wave/parse.go:81-106` — hashing primitives.
- `internal/engine/artifact/schemas.yaml` + `internal/engine/artifact/manager.go:81-107` — bundle artifact enumeration.
- `internal/engine/governance/traceability.go` + `internal/model/traceability.go:34` + `internal/engine/control/derive.go` + `cmd/health.go` — #59 split-out quick PR surfaces, not part of this critical bundle.
- `cmd/evidence.go:77-84` + `internal/engine/gate/gate.go:131` + `internal/engine/progression/authority.go:475` — #67 remediation surfaces.
