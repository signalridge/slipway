# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: repo-native (slipway governed change)

## Summary
Reject placeholder/scaffold assurance.md content before done so standard/strict changes cannot archive with boilerplate-only closeout sections (issue #47)
## Complexity Assessment
simple
<!-- Rationale: bounded change, low coordination, no sensitive guardrail domain.
Per user direction it is a durable root-cause fix using a two-layer (hybrid)
architecture, not a one-line phrase-list patch. Design is already settled. -->

## In Scope
- **Layer 2 — deterministic floor** in `internal/engine/artifact` (manager.go):
  a template-derived scaffold detector for `assurance.md` sections. The embedded
  `assurance.md` template is the single source of truth for "unedited scaffold"
  per section, so detection cannot drift from the template wording. Extend
  `AssuranceStructureBlockers` to emit `assurance_section_placeholder:<heading>`
  when a structurally non-empty required section still holds only scaffold prose
  (verbatim, or the seeded sentence left in place). One change covers both
  `slipway done` (`ValidateAssuranceStructure`) and `slipway validate`/advance
  (`AssuranceContractBlockers`) — they share this validator.
- **Layer 1 — AI attestation enforcement** in
  `internal/engine/progression`: under a standard/strict effective preset,
  S4 readiness requires `final-closeout`, and ship authority
  requires its passing verification record to carry the
  `closeout:assurance_complete=pass` reference. Emit
  `closeout_assurance_attestation_missing` when the record is missing or the
  passing record omits it (fail-closed against rubber-stamping).
- **final-closeout skill** (`internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl`
  + regenerated `.claude`/`.codex` copies): add per-section authored-vs-scaffold
  judgment guidance and promote `closeout:assurance_complete=pass` from advisory
  to a required reference on standard/strict.
- Tests: placeholder-only, partial-placeholder, fully-authored, and
  template-drift safety (Layer 2); attestation-missing enforcement (Layer 1);
  update `internal/tmpl/templates_test.go` to the new required-reference
  contract; update CLI done/next fixtures for standard required closeout and
  light optional closeout.

## Out of Scope
- Rewriting or migrating existing archived `assurance.md` records (AC: existing
  archives are not rewritten unless explicitly repaired/migrated).
- Enabling placeholder *rejection* for the other governed artifacts' done-gating
  (detector stays assurance-specific in this change).
- Changing light-preset closeout semantics: `assurance.md` stays optional there.

## Constraints
- `go build ./...` and `go test ./...` must pass.
- Single source of truth: Layer 2 detection derives from the embedded template,
  not a hand-maintained phrase list.
- No change to light-preset closeout semantics.

## Acceptance Signals
- `slipway done` blocks on standard/strict preset when an assurance.md section
  still contains scaffold placeholders; a fully authored assurance.md passes.
- `slipway validate` reports the same placeholder blocker (shared validator).
- Under standard/strict, a missing `final-closeout` record or a record missing
  `closeout:assurance_complete=pass` blocks the ship gate.
- The blocker names the offending section so the author knows what needs content.
- Tests cover placeholder-only / partial-placeholder / authored / template-drift
  (Layer 2) plus attestation-missing (Layer 1).
- Light preset still finalizes without `assurance.md`.

## Open Questions
<!-- None: the two-layer (hybrid) design is settled and codebase paths are traced. -->

## Approved Summary
<!-- User-confirmed 2026-06-01 -->
Two-layer (hybrid) fix for issue #47.

**Layer 2 — deterministic floor.** A template-derived scaffold detector for
`assurance.md` sections lands in `internal/engine/artifact`; the embedded
template is the single source of truth so detection cannot drift.
`AssuranceStructureBlockers` is extended to emit
`assurance_section_placeholder:<heading>` for any required section still holding
scaffold prose, naming the section. One change covers both `slipway done` and
`slipway validate`/advance because they share this validator.

**Layer 1 — AI attestation enforcement.** The `final-closeout` host skill (AI)
judges each assurance section authored-vs-scaffold and records its verdict as
references in `verification/final-closeout.yaml`, including the required
`closeout:assurance_complete=pass`. The CLI does not re-read prose; under a
standard/strict effective preset it deterministically requires a passing
final-closeout record with that reference, emitting
`closeout_assurance_attestation_missing` when the record or reference is absent.
CLI enforces the single aggregate reference; per-section
`closeout:assurance_section:<name>=authored` lines are skill-side evidence detail
(kernel stays decoupled from section names). The skill template is strengthened
and `.claude`/`.codex` copies regenerated.

**Boundaries.** No migration of archived records; no placeholder rejection for
other artifacts; light-preset closeout semantics unchanged (assurance.md stays
optional there). Primary acceptance: `done`/`validate` block scaffold sections
(naming them); a standard/strict change missing `final-closeout` or its
attestation blocks ship; tests cover placeholder-only / partial / authored /
template-drift + attestation-missing; light preset still finalizes without
assurance.md.
