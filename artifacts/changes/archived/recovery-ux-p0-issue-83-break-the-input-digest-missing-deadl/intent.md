# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: standard Go; table-driven tests; cmd/* CLI layer over internal/engine/* kernel

## Summary
Recovery UX P0 (issue #83): break the input_digest_missing deadlock via Tier-0 digest backfill, prune digest entries on stale-planning recovery, decouple assurance.md from the plan-audit digest, surface required_skill_stale as an actionable blocker with a stale evidence status, add a minimal Tier-0 evidence restamp --dry-run, and route repair at the recovery path

## Complexity Assessment
complex
<!-- Rationale: -->
Multi-step, coordination-heavy across 6+ source files (engine progression +
cmd layer) plus test contract rewrites. Touches governance trust semantics
(digest staleness), so each change must preserve invariants. Not `critical`
because P0 is deliberately scoped to provably-safe changes (no new trust
surface; Tier-2 attested bypass is excluded).

## Guardrail Domains
<!-- none detected -->
None (`""`). Confirmed at intake: the durable on-disk artifacts (`change.yaml`,
`verification/*.yaml`, `evidence-digests.yaml`) do not change shape, so this is
not an external-contract break. The new `stale` evidence-status value and the
`evidence restamp` subcommand are clean additions. No durable artifact schema
changes and no compatibility shim for prior digest input sets.

## In Scope
Implements the seven P0 tasks from issue #83 (Recovery UX phase P0):

1. **Tier-0 backfill** — `internal/engine/progression/evidence_digests.go`,
   `stampPassingSkillDigests`: replace the unconditional skip in the
   `existingDigests != nil` branch (~:262-266) with the same
   `changedAfterVerdict` safety check used in the `== nil` branch; auto-backfill
   a digest when the verdict is still passing and inputs did not change after the
   verdict; report stale only when they genuinely did.
2. **Decouple assurance.md ↔ plan-audit digest** — same file, remove
   `assurance.md` from `addPlanningArtifactInputs` (~:458); it remains a
   legitimate final-closeout digest input (~:668). Existing stored digests from
   the previous input set are handled by the normal stale/re-certification path,
   not by compatibility filtering. Test-first.
3. **Prune digests on stale-planning recovery** —
   `internal/engine/progression/advance_governed.go`,
   `beginStalePlanningRecovery` (~:459-556): when it deletes
   `verification/<skill>.yaml`, prune the matching `evidence-digests.yaml`
   entries via a single helper that removes a verification record and its digest
   entry together (invariant: a digest entry never outlives its record).
4. **`required_skill_stale` actionable** — `cmd/next_skill_view.go`,
   `isRequiredSkillBlocker` (~:443): include `required_skill_stale` so the stale
   skill is surfaced/routed as the actionable next skill.
5. **`stale` evidence status** — `cmd/next_skill_view.go`: report `stale` for
   digest drift on both the precomputed path (~:504-510, currently only
   `missing`/`passing`) and the non-precomputed path (~:534-537, currently
   `stale` only for run-version mismatch).
6. **Minimal `slipway evidence restamp --skill X [--dry-run]`** — new subcommand
   in `cmd/evidence.go` wrapping `progression.StampEvidenceDigestForSkill`
   (~:89), Tier-0 only: stamp when verdict passing + inputs unchanged after
   verdict; otherwise refuse and state why and which skill to re-run, including
   dry-run refusal when the certified input set is unavailable.
7. **`repair` routes to recovery** — `cmd/repair.go` (~:587-599): on
   planning/digest drift, point at `slipway evidence restamp --dry-run` and
   state the non-repairable root cause + why repair does not auto-handle it.

Test work: convert `internal/engine/progression/evidence_digests_test.go`
(~:540-619), `cmd/repair_test.go` (~:1051), and `cmd/lifecycle_commands_test.go`
(~:965) from locking the old/destructive semantics to asserting the new
contract; add a new characterization test proving a late `assurance.md` edit
does not stale the plan-audit digest or produce plan-audit blockers at S1.

## Out of Scope
- Tier-2 operator-attested restamp (`recover --restamp --attest`) → P2 (#85).
- View-level `recovery` object and compact-handoff recovery plan → P1 (#84).
- `internal/engine/recovery` planner, `slipway recover`, `--from-artifact`
  dependency index → P2 (#85).
- Blocker parser + remediation vocabulary table → P1 (#84).
- P3 narrow lifecycle gaps (worktree rebind, dual-active naming, abort→repair,
  S2 scope guidance, full docs refresh) → #86.

## Constraints
- Preserve governance invariants: never silently forge a pass (Tier-0 only when
  the verdict provably still holds); digests still detect judgment-affecting
  drift; a digest entry never outlives its verification record; recovery is
  idempotent.
- Test-first (red → green) for the assurance.md decoupling and the deadlock fix.
- Go; `go build ./...` and `go test ./...` must stay green.
- Reuse existing engine machinery (`StampEvidenceDigestForSkill`, the existing
  `changedAfterVerdict`/refresh-after-stored predicate); do not re-implement
  digest hashing.

## Acceptance Signals
- A late `assurance.md` edit at S4 no longer triggers a plan-audit reopen; only
  final-closeout goes stale (recoverable by re-running that one skill). [test]
- An `input_digest_missing` orphan with a passing verdict and unchanged inputs
  is auto-backfilled by `slipway run` — no internal Go helper required. [test]
- `stale planning recovery` leaves no zombie digest entries (digest pruned
  alongside the deleted verification record). [test]
- `next`/`validate` route a `required_skill_stale` blocker to the actual skill
  and show `stale` (not `missing`/`passing`). [test]
- `slipway evidence restamp --skill X --dry-run` explains a Tier-0 refusal with
  the next skill to re-run, including unavailable digest inputs. [test]
- `go build ./...` and `go test ./...` pass.

## Open Questions
- [x] RFC line references and digest behavior verified at main during research (see research.md Unknowns).
- [x] AssuranceContractBlockers returns nil before S3_REVIEW, so decoupling assurance.md from the plan-audit digest opens no audit gap.
- [x] Digest-semantics callers and tests enumerated: two input_digest_missing emit sites, plus the assurance-coupling and deadlock tests.
- [x] Existing fail-closed guardrail policy is untouched; no Tier-2 surface introduced.

## Deferred Ideas
<!-- Identified but postponed ideas -->
- P1/P2/P3 follow-ups tracked in #84/#85/#86.

## Approved Summary
<!-- User-confirmed final summary + confirmation timestamp -->
Deliver all seven P0 tasks from issue #83 as one governed change (test-first):
(1) Tier-0 digest backfill to break the `input_digest_missing` deadlock when the
verdict is still passing and inputs are unchanged after the verdict; (2) decouple
`assurance.md` from the plan-audit digest so a late closeout edit no longer
retroactively reopens S1; (3) prune `evidence-digests.yaml` entries whenever
stale-planning recovery deletes a verification record (digest never outlives its
record); (4) make `required_skill_stale` an actionable blocker; (5) report a
`stale` evidence status for digest drift on both view paths; (6) add a minimal
Tier-0 `slipway evidence restamp --skill X [--dry-run]` that stamps only when
provably safe and otherwise refuses with the next skill to re-run; (7) route
`repair` at the recovery surface for planning/digest drift.

Scope boundaries: **in** — the seven tasks above plus their test-contract
rewrites, in `internal/engine/progression/*` and `cmd/*`. **Out** — Tier-2
attested restamp, the view-level `recovery` object, the `slipway recover`
planner/`--from-artifact`, and the blocker-parser/remediation vocabulary (all
deferred to #84/#85/#86).

Guardrail domain: none (durable artifacts unchanged; clean additive surface; no
compatibility shim for prior digest input sets).

Primary acceptance signal: a late `assurance.md` edit at S4 no longer triggers a
plan-audit reopen (only final-closeout goes stale), and an `input_digest_missing`
orphan with a passing verdict + unchanged inputs is auto-backfilled by
`slipway run` with no internal Go helper — both proven by tests, with
`go build ./...` and `go test ./...` green.

Confirmed by user: 2026-06-05T09:19:47Z
