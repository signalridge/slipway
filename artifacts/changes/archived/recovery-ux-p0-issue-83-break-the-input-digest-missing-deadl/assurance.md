# Assurance
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Recovery UX P0 (issue #83) delivered as one governed change across seven tasks:
the `input_digest_missing` deadlock is healed by a Tier-0 digest backfill;
`assurance.md` is decoupled from the plan-audit digest; stale-planning recovery
prunes the digest entries of the records it deletes; `required_skill_stale` is an
actionable blocker and the skill-evidence view reports a `stale` status for
digest drift on both paths; a minimal `slipway evidence restamp --skill X
[--dry-run]` exposes a Tier-0 restamp and refuses unavailable digest inputs even
in dry-run mode; and `repair` routes governance-skill digest drift at
`slipway evidence restamp`. Changes are confined to
`internal/engine/progression/{evidence_digests,advance_governed}.go` and
`cmd/{next_skill_view,evidence,repair}.go` plus their tests. Tier-2 attested
restamp, the view-level recovery object, the `slipway recover` planner, and the
blocker-parser vocabulary remain out of scope (deferred to #84/#85/#86).

## Verification Verdict
`go build ./...` and `go test ./...` both pass (exit 0). Test-first (RED→GREEN)
was applied to the assurance decouple and the deadlock heal: the tests that
locked the old/destructive contract were converted to the new contract and
failed against the unchanged engine before the fix, then passed after. New
characterization tests cover the late-`assurance.md`-edit non-staleness, the
recovery digest-prune, the `stale` evidence status, the repair routing, and the
`evidence restamp` eligible/refused/unavailable paths.

## Evidence Index
- `verification/intake-clarification.yaml` (pass)
- `verification/research-orchestration.yaml` (pass)
- `verification/plan-audit.yaml` (pass)
- `verification/wave-orchestration.yaml` (pass, run_version=1)
- `evidence/tasks/t-01.json … t-05.json` (run_version=1, verdict=pass)
- `go build ./...` exit 0; `go test ./...` exit 0
- Source: `internal/engine/progression/evidence_digests.go`,
  `internal/engine/progression/advance_governed.go`,
  `cmd/next_skill_view.go`, `cmd/repair.go`, `cmd/evidence.go`

## Requirement Coverage
- REQ-001 (Tier-0 backfill heals deadlock) — t-01;
  `evidence_digests.go` (both paths) + `evidence_digests_test.go`
  (`TestFeatureActiveMissingDigestEntryBackfillsWhenInputsUnchanged`,
  `TestStampPassingSkillDigestsBackfillsFeatureActiveMissingResearchDigestWhenUnchanged`).
- REQ-002 (assurance.md decoupled) — t-01;
  `addPlanningArtifactInputs` + `TestPlanAuditInputDigestExcludesAssuranceAndIncludesSemanticTasks`,
  `TestLateAssuranceEditDoesNotStalePlanAuditAtS1`.
- REQ-003 (recovery prunes digests) — t-02;
  `beginStalePlanningRecovery` + recovery integration test digest-prune assertions.
- REQ-004 (required_skill_stale actionable) — t-03;
  `isRequiredSkillBlocker` + `TestRequiredSkillStaleIsActionableAndSetExtractsSkillNames`.
- REQ-005 (stale evidence status both paths) — t-03;
  `buildRequiredSkillEvidence` + `TestBuildRequiredSkillEvidenceMarksDigestDriftedSkillStale`.
- REQ-006 (evidence restamp Tier-0) — t-01 engine + t-05 cmd;
  `RestampEvidenceDigestTier0` + `TestEvidenceRestampTier0EligibleDryRunThenApply`,
  `TestEvidenceRestampTier0RefusesInputsChangedAfterVerdict`,
  `TestEvidenceRestampTier0DryRunRefusesUnavailableInputs`,
  `TestEvidenceRestampRequiresSkill`.
- REQ-007 (repair routes digest drift) — t-04;
  `buildGovernanceDigestDriftFindings`/`repairDriftNextAction` +
  `TestRepairRoutesStaleGovernanceDigestToEvidenceRestamp`.

## Residual Risks and Exceptions
- Tier-0 over-trust is bounded: backfill/restamp occur only when the verdict is
  passing and the certified inputs did not change after the verdict (the existing
  conservative `digestInputsChangedAfterVerdict` predicate); judgment-affecting
  drift still reports stale with the specific changed inputs.
- No compatibility shim is provided for prior plan-audit digest input sets. If an
  existing stored digest still contains the old `assurance.md` key, it follows
  the normal stale/re-certification path.
- `evidence restamp` uses mtime-based input-change detection, so an input whose
  mtime moved with identical content is conservatively refused (re-run the skill).
- `evidence restamp --dry-run` now preflights digest-input availability, so it
  refuses with `input_digest_unavailable` instead of promising eligibility when
  the eventual stamp could not compute inputs.
- No guardrail-domain surface is touched; the fail-closed
  `domain_review`/`rollback_required` policy is unchanged. No Tier-2 surface.

## Rollback Readiness
Reverting the change restores prior behavior cleanly: all edits are additive or
localized, with no durable artifact-shape change (`change.yaml`,
`verification/*.yaml`, `evidence-digests.yaml` are unchanged in shape) and no data
migration. After a revert, an in-flight plan-audit digest stamped without
`assurance.md` simply re-includes it on the next run. Rollback unit: revert the
single PR that closes #83.

## Archive Decision
Approve for finalization at done-ready. Active `slipway validate --json` at
S4_VERIFY reports `G_plan`, `G_scope`, and `G_ship` approved with no blockers and
`evidence_freshness: fresh`; `slipway run --json --diagnostics` then advanced
with `action=done_ready`. Fresh ship-gate proof was captured before any finalize,
not reconstructed from an archived bundle. `slipway done` is intentionally
deferred to the operator: this bundle reaches done-ready and is NOT yet archived.
Archive only via an explicit `slipway done` after operator review.
