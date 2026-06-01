# Requirements
## Project Context
- Tech Stack: Go
- Conventions: governance kernel; AI is always the host, kernel enforces evidence. Key paths — `internal/engine/artifact/manager.go` (`AssuranceStructureBlockers`, `requiredSectionsForArtifact`, `markdownSectionLines`, `TemplateContent`); `internal/engine/progression/authority.go` (`buildShipAuthorityFromReadiness`, `FinalCloseoutEvidenceRequired`); S4 readiness/advance wiring in `internal/engine/progression/readiness.go` and `internal/engine/progression/advance_governed.go`; `internal/engine/progression/validation.go:521` (`ComputeVerificationReadiness`); `cmd/next_skill_view.go` (`skill_evidence` diagnostics rendering); `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl`; `internal/tmpl/templates_test.go`. Assurance template scaffold lives in `internal/tmpl/templates/artifacts/assurance.md`.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Deterministic floor rejects scaffold assurance sections (Layer 2).
REQ-001: `AssuranceStructureBlockers` MUST, after the existing structure check passes, reject any required `assurance.md` section whose body is still only template scaffold (verbatim, or the seeded scaffold sentence left in place with extra prose appended). A structurally non-empty but semantically-scaffold section MUST NOT satisfy the assurance gate.

#### Scenario: Placeholder-only assurance is blocked
GIVEN an `assurance.md` with all seven required headings present and each body equal to the embedded template scaffold prose
WHEN `AssuranceStructureBlockers` runs
THEN it returns one `assurance_section_placeholder:<heading>` blocker per scaffold section and the result is non-empty.

#### Scenario: Fully authored assurance passes
GIVEN an `assurance.md` whose required sections all contain concrete, non-scaffold content
WHEN `AssuranceStructureBlockers` runs
THEN it returns no blockers.

### Requirement: The placeholder blocker names the offending section.
REQ-002: Each placeholder blocker MUST identify the specific section still holding scaffold content so the author knows what needs real content.

#### Scenario: Partial-placeholder names only the scaffold sections
GIVEN an `assurance.md` where some sections are authored and others remain scaffold
WHEN `AssuranceStructureBlockers` runs
THEN it returns `assurance_section_placeholder:<heading>` only for the still-scaffold sections, naming each.

### Requirement: Detection derives from the embedded template (single source of truth).
REQ-003: Layer 2 scaffold detection MUST derive its per-section canonical scaffold from the embedded `assurance.md` template, not a hand-maintained phrase list, so detection cannot drift from the template wording.

#### Scenario: Detector follows the template
GIVEN the per-section scaffold strings used for detection
WHEN they are compared to the bodies extracted from the embedded `assurance.md` template
THEN every required section's detection string matches the template-derived body (template drift would update detection automatically).

### Requirement: done and validate enforce the floor consistently (one validator).
REQ-004: The new floor MUST be applied through the shared `AssuranceStructureBlockers` so both `slipway done` (`ValidateAssuranceStructure`) and `slipway validate`/advance (`AssuranceContractBlockers`) reject scaffold assurance without duplicated logic.

#### Scenario: Both surfaces reject the same placeholder content
GIVEN a standard/strict change whose `assurance.md` still contains scaffold sections
WHEN `slipway done` and `slipway validate` evaluate it
THEN both report the placeholder blocker via the shared validator.

### Requirement: AI attestation is a required, enforced reference on standard/strict (Layer 1).
REQ-005: Under a standard/strict effective preset, the ship authority MUST require a passing `final-closeout` verification record that carries the `closeout:assurance_complete=pass` reference. When the record is missing or the passing record omits the reference, the ship authority MUST emit a `closeout_assurance_attestation_missing` blocker (fail-closed). The CLI MUST NOT re-read assurance prose to make this decision — it enforces presence of the AI's attestation only. JSON diagnostics MUST use the same final-closeout-required predicate when rendering `skill_evidence`, so API clients see the same required skills that route `next_skill` and block ship.

#### Scenario: Missing attestation blocks ship under standard/strict
GIVEN a standard/strict change with a passing `final-closeout` record whose `references` omit `closeout:assurance_complete=pass`
WHEN the ship authority is computed
THEN `closeout_assurance_attestation_missing` is present and the ship gate is not ready.

#### Scenario: Missing closeout record blocks ship under plain standard
GIVEN a plain standard change where `CloseoutRefreshRequired` is false and `goal-verification` is passing
WHEN the ship authority is computed without a `final-closeout` record
THEN `closeout_assurance_attestation_missing` is present and the ship gate is not ready.

#### Scenario: Diagnostics list missing closeout under plain standard
GIVEN a plain standard S4 change where `goal-verification` is passing and `final-closeout` is missing
WHEN `slipway next --json --diagnostics` renders `skill_evidence`
THEN `skill_evidence` includes `goal-verification` as passing and `final-closeout` as missing, matching the `next_skill: final-closeout` route and `required_skill_missing:final-closeout` blocker.

#### Scenario: Attestation present clears the Layer 1 check
GIVEN the same record with `closeout:assurance_complete=pass` in `references`
WHEN the ship authority is computed
THEN no `closeout_assurance_attestation_missing` blocker is raised by this check.

### Requirement: Boundaries — light preset and archived records unchanged.
REQ-006: Light-preset closeout semantics MUST remain unchanged (`assurance.md` optional; no attestation requirement). Existing archived `assurance.md` records MUST NOT be rewritten or migrated by this change.

#### Scenario: Light preset still finalizes without assurance
GIVEN a light-preset change with no `assurance.md`
WHEN closeout/done evaluation runs
THEN no assurance placeholder or attestation blocker is raised.
