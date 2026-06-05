# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/engine/progression/evidence_digests.go` — engine-owned digest
    stamping + freshness. Two emitters of the deadlock token `input_digest_missing`:
    the read path `skillDigestFreshnessBlockersWithSummary` (`:163-173`) and the
    stamp path `stampPassingSkillDigests` (`:262-266`). Both short-circuit on
    `digests != nil` instead of applying the `digestInputsChangedAfterVerdict`
    safety check used by the `== nil` branch (`:174-181`).
  - `internal/engine/progression/advance_governed.go` — `beginStalePlanningRecovery`
    (`:459-556`) deletes 5 skill verification records (plan-audit + spec/code
    reviews + goal-verification + final-closeout) and wave-plan/execution-summary,
    via `removeFileIfExists` (`:558`); it does **not** prune `evidence-digests.yaml`,
    leaving zombie digests. `stalePlanningRecoveryNeededForPlanAuditDigest`
    (`:568-592`) triggers reopen only when plan-audit has a
    `required_skill_stale:plan-audit:` digest blocker.
  - `internal/engine/progression/evidence.go` — `evaluateRequiredSkills`
    (`:124-130`) calls `skillDigestFreshnessBlockers` and routes digest drift to
    `required_skill_stale:<skill>:<input>` blockers (second return), which become
    `readiness.Blockers`.
  - `cmd/next_skill_view.go` — `isRequiredSkillBlocker` (`:443`) omits
    `required_skill_stale`; `buildRequiredSkillEvidence` (`:463`) renders skill
    `Status` with only `missing`/`passing` on the precomputed path (`:500-514`)
    and `stale` only for run-version mismatch on the non-precomputed path
    (`:533-547`). `view.Blockers` already carries the `required_skill_stale`
    blockers (`cmd/next.go:375`) before this runs (`cmd/next.go:417`).
  - `cmd/evidence.go` — `makeEvidenceCmd` parent with only the `task` subcommand.
  - `cmd/repair.go` — `repairDriftNextAction` (`:587-599`) maps a drift reason to
    a generic next action; repair has no governance-skill digest-staleness
    detector today.
- Dependency chain: `cmd/next.go` → `EvaluateGovernanceReadiness` →
  `evaluateRequiredSkills` → `skillDigestFreshnessBlockers` →
  `certifiedSkillInputDigest`/`addPlanningArtifactInputs`. `cmd/evidence restamp`
  will → `progression` Tier-0 wrapper → `stampEvidenceDigestForSkill`.
- Blast radius: digest staleness semantics (read + stamp), the destructive
  reopen, the `next` skill-evidence view, and a new additive CLI subcommand. No
  change to durable artifact shapes (`change.yaml`, `verification/*.yaml`,
  `evidence-digests.yaml`).
- Constraints/invariants: never silently forge a pass (Tier-0 only when the
  verdict provably still holds); digests still detect judgment-affecting drift; a
  digest entry never outlives its verification record; recovery is idempotent;
  the view must not re-implement gate decisions (single source of truth).

### Patterns
- Existing conventions: `cobra` subcommands built by `make*Cmd()`, registered via
  `cmd.AddCommand`; typed CLI errors via `newInvalidUsageError`/
  `newStateIntegrityError` with `(code, message, remediation, details)`; table-
  driven tests with `withWorkspace`/`initTestWorkspace`/temp git repos.
- Reusable abstractions to leverage (no re-implementation):
  - `digestInputsChangedAfterVerdict` (`evidence_digests.go:859`) — the canonical
    "did certified inputs change after the verdict" check. Both deadlock paths and
    the new restamp wrapper must reuse it.
  - `StampEvidenceDigestForSkill` (`:89`) / `stampEvidenceDigestForSkill` (`:99`).
  - `state.LoadOptionalEvidenceDigestsForChange` / `state.SaveEvidenceDigests` for
    digest prune.
  - `blockerSkillName` + `model.ReasonCode{Code,Detail}` (split on first `:`,
    `reason_code.go:235`) to derive the stale skill set from `view.Blockers`.
- Convention deviations: none required. The `stale` view status is derived from
  existing blockers so the evidence view cannot diverge from the blocker list.

### Risks
- Technical risks:
  - **Tier-0 over-trust (high if wrong)**: backfilling a digest when inputs truly
    changed would forge a pass. Mitigation: gate strictly on
    `digestInputsChangedAfterVerdict` returning empty (mtime <= verdict) AND verdict
    passing; this is the existing conservative semantics, only now applied to the
    `digests != nil` branch too.
  - **Decoupling under-coverage (low)**: removing `assurance.md` from the
    plan-audit digest could open an audit gap only if plan-audit were the assurance
    authority. It is not — `AssuranceContractBlockers` returns nil before
    `S3_REVIEW` (`validation.go:482-487`) and plan-audit runs at S1; assurance is
    enforced at S3/S4 and stays a `final-closeout` digest input
    (`evidence_digests.go:648-668`). Verified.
  - **Prune over-reach (medium)**: pruning a digest whose record survives would
    violate the invariant inversely. Mitigation: prune only for the exact skill
    records `beginStalePlanningRecovery` deletes; wave-orchestration's record (and
    digest) is preserved.
  - **Prior plan-audit input sets (accepted)**: existing changes may already
    have a stored plan-audit digest *with* `assurance.md`. This change does not
    add a compatibility shim for that prior input set; such records follow the
    normal stale/re-certification path.
  - **Dry-run over-eligibility (medium)**: `evidence restamp --dry-run` could
    report Tier-0 eligibility before `stampEvidenceDigestForSkill` had a chance
    to detect unavailable inputs. Mitigation: preflight digest input availability
    in the engine wrapper before the changed-after-verdict check.
- Guardrail domains: none. P0 introduces no Tier-2 attested bypass; the existing
  fail-closed `domain_review`/`rollback_required` policy is untouched.
- Reversibility: high. All changes are additive or localized; no destructive
  data migration.

### Test Strategy
- Existing coverage that locks the old/destructive contract (to convert):
  - `evidence_digests_test.go:17` `TestPlanAuditInputDigestIncludesAssuranceAndSemanticTasks`
    asserts the plan-audit digest **contains** `assurance.md` → invert to exclude.
  - `evidence_digests_test.go:46` `TestEvaluateRequiredSkillsUsesContentDigestNotMTime`
    uses `assurance.md` as the drift example and asserts
    `required_skill_stale:plan-audit:assurance.md` → switch the example artifact to
    a real plan-audit input (e.g. `requirements.md`).
  - `evidence_digests_test.go:540` and `:580` lock the `input_digest_missing`
    deadlock → convert to: orphan with inputs **unchanged after verdict** is
    backfilled (heals); orphan with inputs **changed after verdict** reports the
    specific `required_skill_stale:<skill>:<input>` (not the generic token).
  - `cmd/repair_test.go:1051` `TestRepairDoesNotRebuildWhenPlanningEvidenceIsStale`
    → keep "repair does not rebuild", extend next-action to point at recovery.
  - `cmd/lifecycle_commands_test.go:965`
    `TestRunStalePlanningEvidenceReopensPlanAuditAndPreservesRuntimeEvidence` →
    add evidence-digests setup + assert the 5 deleted skills' digests are pruned
    while wave-orchestration's digest survives.
- New tests:
  - Characterization: a late `assurance.md` edit at S1 does not stale the
    plan-audit digest or produce plan-audit blockers (acceptance #1 mechanism).
  - `evidence restamp --skill X --dry-run` Tier-0 refusal explains why + which
    skill to re-run, including unavailable digest inputs; eligible orphan reports
    it would stamp; non-dry-run stamps.
- Infrastructure: reuse `writeDigestPlanningBundle`, `writeVerificationForTest`,
  `os.Chtimes` (mtime control vs verdict), `prepareStalePlanningRecoveryFixture`.
- Verification approach: `go build ./...` and `go test ./...` green; targeted
  package runs for `internal/engine/progression` and `cmd` during iteration.

## Alternatives Considered
- **A. RFC-faithful (recommended)**: unify the `changedAfterVerdict` check in
  both digest paths; clean-remove `assurance.md` from the plan-audit digest;
  prune via one helper that deletes a verification record and its digest entry
  together; derive the view `stale` status from `view.Blockers`; add
  `evidence restamp` over a **new exported engine Tier-0 wrapper** that reuses
  `digestInputsChangedAfterVerdict` + `stampEvidenceDigestForSkill`; route repair
  minimally (digest-aware next-action + a cheap non-repairable finding).
  Tradeoffs: keeps Tier-0 semantics in one canonical place (engine), faithful to
  the reviewed governance invariants; adds a small engine API surface.
- **B. cmd-inline Tier-0**: put the Tier-0 safety check inside `cmd/evidence.go`
  and call the existing `StampEvidenceDigestForSkill` directly. Tradeoffs: smaller
  engine surface but **duplicates digest semantics in cmd/**, risking drift from
  the canonical check — violates the single-source-of-truth invariant. Rejected.
- **C. Full repair detector in P0**: add a complete governance-skill
  digest-staleness scanner to `repair` for every active change. Tradeoffs: nicer
  repair UX but overlaps P1's recovery vocabulary/surface and risks phase scope
  creep. Defer the breadth to P1; P0 keeps repair routing minimal.
- Selected: **A** — see `decision.md`.

## Unknowns
- Resolved: RFC line refs / behavior at current `main` → all verified
  (digest paths, `addPlanningArtifactInputs` membership, recovery deletion set,
  view paths, `StampEvidenceDigestForSkill`, `repairDriftNextAction`).
- Resolved: does removing `assurance.md` from plan-audit open an audit gap? → No;
  `AssuranceContractBlockers` enforces only at S3/S4, independent of the
  plan-audit digest.
- Resolved: where does the view get staleness from? → `view.Blockers` (populated
  from `readiness.Blockers` at `cmd/next.go:375`) before
  `buildRequiredSkillEvidence` runs.
- Resolved: `input_digest_missing` token blast radius → only two non-test emit
  sites (`evidence_digests.go:170,264`), both being changed; one test file
  references it.
- Remaining (for plan-audit): exact phrasing of the `evidence restamp` refusal
  reasons and the repair next-action string; pick during planning. No open
  technical blocker.

## Assumptions
- `digestInputsChangedAfterVerdict`'s mtime-vs-verdict comparison is the accepted
  Tier-0 conservatism (mtime change with identical content refuses → re-run).
  Evidence: existing `== nil` branch (`evidence_digests.go:174-181`) and tests
  `evidence_digests_test.go:46,79`.
- `validate` already surfaces `required_skill_stale` via its blocker list (per
  issue #81 repro); the explicit `stale` evidence-status change targets the
  `next` skill-evidence view. Evidence: issue #81 `validate --json` output.
- No compatibility filter is required for prior plan-audit digest input sets;
  stale prior records are re-certified through the standard evidence path.

## Canonical References
- `internal/engine/progression/evidence_digests.go:163` (read path),
  `:262` (stamp path), `:452` (`addPlanningArtifactInputs`), `:859`
  (`digestInputsChangedAfterVerdict`), `:89` (`StampEvidenceDigestForSkill`)
- `internal/engine/progression/advance_governed.go:459`
  (`beginStalePlanningRecovery`), `:568`
  (`stalePlanningRecoveryNeededForPlanAuditDigest`)
- `internal/engine/progression/validation.go:476` (`AssuranceContractBlockers`)
- `internal/engine/progression/evidence.go:124` (digest-blocker routing)
- `internal/model/evidence_digests.go:100` (`EvidenceFreshness`),
  `internal/model/reason_code.go:235` (Code/Detail split)
- `cmd/next_skill_view.go:443` (`isRequiredSkillBlocker`), `:463`
  (`buildRequiredSkillEvidence`); `cmd/next.go:375,417`
- `cmd/evidence.go:28` (`makeEvidenceCmd`); `cmd/repair.go:587`
  (`repairDriftNextAction`)
