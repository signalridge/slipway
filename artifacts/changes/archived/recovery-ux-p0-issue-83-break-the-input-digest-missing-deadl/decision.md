# Decision

## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI layer (cobra) over internal/engine/* kernel; typed CLI errors; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered
Three approaches were evaluated in `research.md`:
- **A. RFC-faithful** — Tier-0 logic lives in the engine (reuses the canonical
  `digestInputsChangedAfterVerdict`); `evidence restamp` wraps a new exported
  engine Tier-0 function; the `stale` view status is derived from `view.Blockers`;
  digest prune via one helper that removes a verification record and its digest
  entry together; repair routing kept minimal.
- **B. cmd-inline Tier-0** — re-implements the Tier-0 safety check in
  `cmd/evidence.go`. Rejected: duplicates digest semantics outside the engine and
  can drift from the canonical check (violates single-source-of-truth).
- **C. A + full repair detector** — adds a complete governance-skill
  digest-staleness scanner to `repair`. Deferred: overlaps P1's recovery surface
  (#84) and pulls phase scope forward.

## Selected Approach
**A — RFC-faithful.** Confirmed by the user at research handoff (2026-06-05).

Engine (`internal/engine/progression`):
1. Unify the `digestInputsChangedAfterVerdict` safety check across **both**
   deadlock paths — `skillDigestFreshnessBlockersWithSummary` (read) and
   `stampPassingSkillDigests` (stamp) — so a recorded orphan with inputs unchanged
   after the verdict is backfilled, and a genuinely drifted one reports the
   specific `required_skill_stale:<skill>:<input>` instead of the generic
   `input_digest_missing` token.
2. Remove `assurance.md` from `addPlanningArtifactInputs` (plan-audit digest);
   it remains a `final-closeout` digest input. No compatibility shim is added for
   stored digests from the previous input set; if such a stored digest is present,
   the normal stale/re-certification path applies.
3. Add a single helper that deletes a verification record **and** its
   `evidence-digests.yaml` entry together, used by `beginStalePlanningRecovery`
   for the five skills it clears (plan-audit + spec/code reviews +
   goal-verification + final-closeout); wave-orchestration's record and digest are
   preserved.
4. Add an exported `RestampEvidenceDigestTier0(root, change, skill, dryRun)` that
   stamps only when the verdict is passing, the certified input set is
   computable, and inputs are unchanged after the verdict; otherwise it refuses
   and names the skill to re-run. Reuses `digestInputsChangedAfterVerdict` +
   `stampEvidenceDigestForSkill`, with an explicit input-availability preflight
   so `--dry-run` cannot over-report eligibility.

CLI (`cmd`):
5. Add `required_skill_stale` to `isRequiredSkillBlocker` so a stale skill routes
   as the actionable next skill.
6. Report the `stale` evidence status on both `buildRequiredSkillEvidence` paths,
   derived from the `required_skill_stale` entries already in `view.Blockers`.
7. Add `slipway evidence restamp --skill X [--dry-run]` over
   `RestampEvidenceDigestTier0`.
8. Route `repair` planning/digest drift at `slipway evidence restamp --dry-run`
   (digest-aware next action; state that repair does not mutate engine-owned
   digests), keeping the breadth for P1.

## Interfaces and Data Flow
- New engine API: `progression.RestampEvidenceDigestTier0(root string, change
  model.Change, skillName string, dryRun bool) (EvidenceRestampOutcome, error)`;
  `EvidenceRestampOutcome{Skill, Eligible, Stamped, DryRun, Reason, ChangedInputs,
  RerunSkill}`. Pure over `digestInputsChangedAfterVerdict` +
  `stampEvidenceDigestForSkill`; no new hashing.
- New prune helper (unexported) within `progression`:
  removes `verification/<skill>.yaml` and the matching `evidence-digests.yaml`
  entry atomically; invoked by `beginStalePlanningRecovery`.
- Data flow unchanged at the boundaries: `cmd/next.go` →
  `EvaluateGovernanceReadiness` → `evaluateRequiredSkills` →
  `skillDigestFreshnessBlockers`. The view reads `view.Blockers` (already
  populated) to derive `stale`. `cmd/evidence restamp` → engine wrapper →
  `evidence-digests.yaml`.
- Durable artifact shapes (`change.yaml`, `verification/*.yaml`,
  `evidence-digests.yaml`) are unchanged; the `stale` status value and
  `evidence restamp` subcommand are additive.

## Rollout and Rollback
- Rollout: single PR closing #83; test-first for the decouple and the deadlock
  heal (red → green), then the remaining tasks. `go build ./...` + `go test ./...`
  green before finalize. Governed via Slipway (dogfood).
- Rollback: revert the PR. No data migration; no persisted-state shape change, so
  reverting restores prior behavior cleanly. In-flight changes that restamped a
  plan-audit digest without `assurance.md` simply re-include it on the next run
  after a revert.

## Risk
- **Tier-0 over-trust** (high impact if wrong): strictly gated on
  `digestInputsChangedAfterVerdict` empty + verdict passing; never fabricates a
  pass. Covered by deadlock heal/drift tests.
- **Decouple audit gap** (low): verified none — assurance is enforced by
  `AssuranceContractBlockers` at S3/S4, independent of the plan-audit digest.
- **No compatibility shim for prior plan-audit input sets** (accepted): stored
  digests that still contain the old `assurance.md` key are not specially
  interpreted. Operators re-run or restamp the affected governance skill through
  the standard stale-evidence path.
- **Dry-run over-eligibility** (medium): `evidence restamp --dry-run` must not
  claim Tier-0 eligibility when digest inputs are unavailable. The engine now
  checks input availability before the mtime/verdict comparison.
- **Prune over-reach** (medium): prune only the records recovery deletes;
  wave-orchestration preserved. Covered by the recovery prune test.
- **Phase creep** (low): repair routing kept minimal; the full recovery surface
  stays in P1/#84.
