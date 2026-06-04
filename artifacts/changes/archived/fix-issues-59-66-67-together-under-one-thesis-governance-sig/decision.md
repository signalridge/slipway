# Decision

## Alternatives Considered
The architecture fork is *where the engine records "what content a passing verdict
certified"* so freshness can be content-addressed instead of time/mtime-addressed.

- **Approach A — `InputDigest` on `VerificationRecord`, engine writes the host YAML.**
  Literal reading of issue #66 Phase 1. Rejected: the engine has no writer for
  `verification/<skill>.yaml` today (host-authored, strict `KnownFields(true)` read,
  delete-only); adding one creates a new write authority into a host-owned, gitignored
  path that races the host's own write, and the digest still lands in gitignored local
  state — invasive with the weakest integrity.
- **Approach B — engine-owned digest map on git-tracked `change.yaml`.** Rejected by
  user direction: accepted digests are *derived runtime state*, not part of the durable
  change definition, and should not pollute the single git-tracked authority.
- **Approach C — hybrid agent-advisory `InputDigest` + engine authority.** Rejected:
  two sources of truth, no integrity gain.
- **Approach B′ — engine-owned digest store in a gitignored evidence file. SELECTED.**

## Selected Approach
Resolve #66, #70, and #67 under one principle — *never project a governance signal to a
scalar (timestamp / mtime / opaque blocker) at the consume boundary; name the specific
artifact and the supported remediation* — via internally-separable commits in one critical
PR. #66 and #70 are the same mtime/timestamp-freshness root cause and ship together. #67
rides with the core PR because artifact naming depends on #66 diagnostics. #59 is split
into a separate quick PR because it is low-risk and independent; this bundle only carries
the #59 ledger and does not close #59.

### #66 — content-addressed evidence freshness (Approach B′)
DEC-001 (Approach B′) satisfies REQ-001, REQ-002, REQ-003, REQ-004, and REQ-007.

Bind skill-evidence freshness to a **content digest** the engine computes and locks at
verdict-acceptance, stored in an **engine-owned, gitignored** file
`verification/evidence-digests.yaml` (a sibling of `execution-summary.yaml` /
`wave-plan.yaml`). Properties: timestamp-free; tamper-resistant (engine writes it, the
host agent never does); no schema change to the host `VerificationRecord` or to
`change.yaml`.

- **Stamp at acceptance.** A shared `stampAcceptedEvidenceDigests` runs at every
  *mutating* acceptance site — the required-skill block in `AdvanceGoverned`
  (`advance_governed.go:143-161`, S1→S2) and the S3/S4 auto-pass authority paths
  (`autopass.go`). For each accepted skill it computes that skill's certified input-set
  digest and writes/updates its entry. Directly accepted verdicts are always stamped in
  that mutating pass. Historical accepted records are backfilled only for legacy
  file-absent changes after the one-time safety gate passes; a feature-active digest file
  whose map lacks an already-accepted skill entry stays not-fresh until that skill is
  rerun. Read-only projections (`status`/`validate`/`next`/`health`) never write.
- **tasks.md uses the semantic hash.** `wave.TaskPlanSemanticHash` (checkbox-/format-
  invariant) — a raw-byte hash would re-trip on checkbox writeback and reproduce the bug.
  Prose docs use `model.ComputeInputHash` over normalized body (EOL-insensitive). File
  sets use a sorted `{path: ComputeFileContentHash}` fed to `ComputeInputHash`.
  `plan-audit` includes `assurance.md` because standard/strict plan-audit certifies that
  artifact too.
- **Consume by comparison.** `EvidenceFreshness(stored, current)` recomputes current
  digests and returns `(fresh, changed[])`; `changed` names exactly which artifact
  invalidated the verdict. Steady-state skill evidence freshness is replaced by this
  comparison. The old `EvaluateEvidenceFreshness` timestamp fields remain as a documented
  compatibility channel for non-structural callers, but execution-summary freshness now
  supplies a zero latest-relevant baseline so artifact clocks cannot drive the governed
  steady-state path.
- **Carve-out.** `wave-orchestration` keeps its embedded logical `CapturedAt` +
  structural field-map freshness (already content/identity-based, not filesystem mtime).
  "No mtime/wall-clock" means filesystem `ModTime()` and wall-clock `now`, not the
  logical `CapturedAt`/`run_version` binding or closeout proof-ordering timestamp checks.
- **Migration — guarded silent backfill, bounded + observable.** Distinguish LEGACY
  from FEATURE-ACTIVE by the digest file's presence. A legacy change (no
  `evidence-digests.yaml`) with already-accepted passing skills reads **fresh** and is
  materialized once on the next `slipway run` only if the one-time safety gate passes:
  every certified artifact resolved from the same per-skill digest input-set must have a
  filesystem mtime at or before that skill verdict's timestamp. If any certified artifact
  is newer than the verdict, do not backfill; report stale/not-fresh and require
  re-verification. `wave-orchestration` resolves `runtime_task_evidence` to the runtime
  task evidence JSON files for the accepted run, so task JSON written after the verdict is
  included in that safety gate. A feature-active change (file present) whose map lacks an
  entry for an already-accepted skill treats that skill as **not fresh** (needs
  re-verification) — so backfill cannot silently bless an unstamped verdict. The same
  safety check is reused when a host writes a newer passing verdict after a stored digest
  drifted but before the next mutating restamp, but the refreshed-verdict window checks
  only inputs whose content digest actually changed; if the new verdict is demonstrably
  after those changed certified inputs, the run may restamp without a false stale blocker
  from an unchanged tasks.md mtime. This one-time verdict safety gate is the only
  filesystem-mtime carve-out; normal stored-digest comparison remains content-only.

### #59 — split-out traceability gap legibility
DEC-002 supports REQ-007 by defining the issue-closure and scope-control boundary; it is
not part of this critical implementation.

#59 traceability gap legibility is independent and should not wait behind #66/#70's
critical `schema_data_migration` guardrail. Split it into a quick PR that adds
`TraceabilityGap.Artifact`, per-gap health observations, and text/doctor rendering. Keep
#59 open because item 1 (snapshot cache), item 2 (single source of truth), and item 3
(`run` error-severity blocker framing after progress) are not solved here.

### #67 — S4 post-review recovery routing
DEC-003 satisfies REQ-006.

- `cmd/evidence.go`: the S4 `evidence_task_wrong_state` remediation routes to the
  supported path (task evidence is S2-only; in S4 refresh by re-running goal-verification
  + final-closeout).
- `authority.go` (`closeout_goal_verification_reuse_invalid`) and the reason-code
  rendering for `verification_evidence_missing` carry the same supported-refresh
  remediation and, via #66, name the changed artifact that invalidated the evidence.

### #70 — wave-plan / stale-planning chain content-based freshness
DEC-004 satisfies REQ-008. Same mtime root cause as #66.

- `internal/state/wave_execution.go` `MaterializeWavePlan` MUST stop deriving
  `wave-plan.generated_at` from `tasks.md` `ModTime()` (via `currentTaskPlanNodes`).
- Replacement value: `generated_at` becomes display/audit materialization time
  (actual/injected time when the wave plan is materialized). This does not violate the
  thesis because freshness must not consume `generated_at`.
- `internal/state/execution_summary.go` keys the stale-planning chain on the semantic
  `tasks_plan_hash` (already stored as `summary.TasksPlanHash`) rather than `generated_at`
  vs source-mtime timestamp ordering, so a refreshed `plan-audit` whose task content is
  unchanged cannot leave `wave-plan.yaml`/`execution-summary.yaml` permanently stale after
  S4 recovery. This is the same digest/semantic-hash conversion as #66 t-06, extended to
  the wave-plan capture signal.

## Interfaces and Data Flow
**New types (`internal/model/evidence_digests.go`):**
```go
type EvidenceDigests struct {
    Version int                     `yaml:"version"`
    Skills  map[string]SkillDigest  `yaml:"skills"`
}
type SkillDigest struct {
    RunVersion       int               `yaml:"run_version"`       // run_summary_version at stamp; 0 pre-exec
    VerdictTimestamp time.Time         `yaml:"verdict_timestamp"` // accepted VerificationRecord timestamp
    Inputs           map[string]string `yaml:"inputs"`            // artifact/path -> hash
}
// fresh=false names the artifacts whose current hash != stored hash.
func EvidenceFreshness(stored SkillDigest, current map[string]string) (fresh bool, changed []string)
```

**Persisted file** `artifacts/changes/<slug>/verification/evidence-digests.yaml` (gitignored):
```yaml
version: 1
skills:
  plan-audit:
    run_version: 0
    verdict_timestamp: "2026-06-04T09:40:24Z"
    inputs:
      intent.md: "..."          # ComputeInputHash(normalized body)
      requirements.md: "..."
      decision.md: "..."
      research.md: "..."        # only when needs_discovery
      assurance.md: "..."
      tasks.md: "..."           # TaskPlanSemanticHash
  goal-verification:
    run_version: 3
    verdict_timestamp: "2026-06-04T10:15:00Z"
    inputs: { "<changed/target file>": "<ComputeFileContentHash>" , ... }
```

`verdict_timestamp` is not a freshness scalar. It records which host
`VerificationRecord` the engine accepted when it stamped the digest. If inputs drift and
the host writes a newer passing verification record, the next mutating `slipway run` may
accept that refreshed verdict and replace the digest; the same old verdict still goes
stale on real input drift.

**State accessors (`internal/state/`):** `SaveEvidenceDigests` / `LoadEvidenceDigests`
mirroring `SaveExecutionSummary`; `listVerificationsInDir` skips
`evidence-digests.yaml` by name.

**Per-skill input-set (the digest keys):**
| Skill | Input set | Hash |
|---|---|---|
| `plan-audit` | intent, requirements, research(if discovery), decision, assurance (prose); tasks.md | prose→`ComputeInputHash`; tasks→`TaskPlanSemanticHash` |
| `wave-orchestration` | wave-plan structure with display-only `generated_at` removed; parsed runtime task evidence for the accepted run version | `ComputeInputHash` over semantic wave/task evidence fields |
| `goal-verification` | execution-summary changed∪target file set (`closeoutGoalVerificationReuseContentPaths`) | semantic path-set digest (`changed_target_files`) + per-file `ComputeFileContentHash`; the full `execution-summary.yaml`, `captured_at`, `run_summary_version`, and `tasks_plan_hash` are not content digest inputs |
| `final-closeout` | same as goal-verification + `assurance.md` | as above + prose |
| `spec-compliance-review`, `code-quality-review`, `security-review`, `independent-review` | reviewable diff set (`readiness.go:653-690` git diff ∪ non-ignored untracked reviewable files ∪ summary changed/target; exclude ignored/runtime evidence and Slipway governed bundles under `artifacts/changes/**`) | per-file `ComputeFileContentHash` |

**Untracked policy:** untracked reviewable files count for diff-class reviews because
those reviews certify the current working diff. Slipway governed bundles under
`artifacts/changes/**` are excluded so parallel active/archive artifacts do not stale an
unrelated change; plan/final digests own governed artifact freshness. Unrelated untracked
files do not stale `goal-verification` unless they are recorded as changed/target
execution evidence. A commit between review and finalization changes the diff boundary
and may make read-only projections report diff-class reviews stale until mutating
advancement restamps or the review is run again against the new diff boundary; an
empty-diff restamp must not be read as proof that the pre-commit diff was re-reviewed.

**Self-output policy:** a skill digest must not include an artifact the same skill
regenerates. `wave-orchestration` therefore does not digest `execution-summary.yaml`;
that summary is downstream output from the wave record plus runtime task evidence.

**Data flow:** `slipway run` → `AdvanceGoverned` evaluates required skills → on
`skillBlockers==0` (and in `autopass` authority paths) → `stampAcceptedEvidenceDigests`
computes + persists each accepted skill's digest → `SaveExecutionSummary`-style atomic
write. Later read-only projection → load stored digests → recompute current →
`EvidenceFreshness` → fresh or stale-naming-artifact, replacing the closeout-reuse
mtime/timestamp gates. `EvaluateEvidenceFreshness` keeps its legacy timestamp fields for
compatibility, but governed execution-summary callers pass a zero latest-relevant baseline.
Legacy file-absent backfill, and the equivalent refreshed-verdict restamp window, run only
on `slipway run` and only after the verdict-timestamp safety gate.

## Rollout and Rollback
- **Recommended PR split:** #59 gap legibility lands as a separate quick PR. This critical
  PR carries #66 + #70 content-addressed freshness (primitive + stamping + per-skill
  input-sets + replace steady-state artifact-clock branches incl. `wave-plan.generated_at`,
  with checkbox/git-restore/real-edit/legacy-gate/#70-refreshed-chain proof tests) and #67
  remediation routing on the #66/#70 mechanism.
- **Migration:** guarded silent backfill — no data migration step. Legacy changes self-heal
  their digest file on the next `slipway run` only when no certified artifact is newer
  than the accepted verdict; otherwise they require re-verification. Feature-active changes
  missing a stamped entry read not-fresh rather than backfilled.
- **Rollback:** revert the PR. The change is additive (new file type, new evaluator,
  new remediation strings) plus deletion of dead artifact-clock code; no destructive data
  operation. Reverting leaves orphan
  `evidence-digests.yaml` files which are gitignored and harmless (ignored by the old
  binary). Guardrail-domain `rollback_required` is satisfied by revert-ability.

## Risk
- **HIGH — dual acceptance routes.** S1→S2 via `advance_governed.go:143-161`; S3/S4 via
  `autopass.go`. Mitigation: single `stampAcceptedEvidenceDigests` called at *both*; test
  that review/goal/closeout skills get stamped through the auto-pass path.
- **HIGH — tasks.md false-positive reproduction.** Must use `TaskPlanSemanticHash`, never
  raw bytes. Mitigation: regression test — checkbox-only writeback leaves the tasks digest
  unchanged.
- **MEDIUM — incomplete time-branch sweep.** A missed site = surviving false-positive.
  Mitigation: convert/delete the artifact-clock sites in the research inventory; a guard
  test scans production progression files plus the execution-summary/context/execution-repair
  freshness boundary and allows `ModTime()` only inside the verdict safety-gate helpers
  `digestInputsChangedAfterVerdict` / `digestInputChangedAfterVerdict`. The legacy
  `EvaluateEvidenceFreshness` timestamp fields are kept but documented, and the governed
  execution-summary path pins the latest-relevant baseline to zero.
- **MEDIUM — legacy backfill can mask drift if ungated.** Mitigation: one-time
  verdict-timestamp safety gate for legacy file-absent changes and refreshed-verdict
  restamps, including `runtime_task_evidence` for wave-orchestration. This intentionally
  uses artifact mtime only to prove the verdict is newer than its certified inputs;
  refreshed restamps check only digest-changed inputs, stored-digest comparison remains
  content-only, and the guard test scopes that carve-out explicitly.
- **MEDIUM — #70 chain regression.** Reworking the wave-plan capture signal must not break
  the legitimate stale-planning recovery that #53 added (a real `tasks.md` semantic change
  MUST still invalidate the chain). Mitigation: key on `tasks_plan_hash`, with regressions
  for both "unchanged tasks → fresh" and "changed tasks → stale".
- **MEDIUM — diff-class untracked sensitivity.** New untracked reviewable files will stale
  review evidence. This is conservative and intentional because the review certified the
  working diff; ignored/runtime evidence and `artifacts/changes/**` governed bundles are
  excluded, and goal-verification does not stale on unrelated untracked files. A commit
  between review and finalization can also change the certified diff set; host surfaces
  should either let the next mutating advancement restamp at the new boundary or rerun the
  review when the pre-commit diff proof is still required.
- **MEDIUM — self-referential digest inputs.** A skill digest that includes its own output
  can create an infinite stale/restamp loop. Mitigation: `wave-orchestration` digests
  semantic wave-plan structure and parsed runtime task evidence, not
  `execution-summary.yaml`; regression coverage asserts this exclusion.
- **LOW — #59 delay avoided by split.** Traceability legibility should not wait behind this
  critical migration PR; track it as a quick independent PR and keep #59 open.
- **LOW — local-only store.** Gitignored, consistent with all other evidence; fresh clone
  re-establishes via backfill. Tamper-resistance from engine-ownership, not git-tracking.
- **Guardrail domain `schema_data_migration`:** `domain_review` + `rollback_required`
  fail-closed and stay enabled.
