# Research

## Alternatives Considered

### Architecture
- Affected modules: `cmd/evidence.go`, `cmd/evidence_skill_test.go`, `internal/tmpl/templates/_partials/command-evidence-body.tmpl`, `internal/toolgen/toolgen.go`, and `docs/reference/commands.md`.
- Dependency chains: `makeEvidenceSkillCmd` parses CLI flags and validates skill/stage state before writing `model.VerificationRecord` through `state.SaveVerification` and stamping digests through `progression.StampEvidenceDigestForSkill` (`cmd/evidence.go:108`, `cmd/evidence.go:196`, `cmd/evidence.go:217`, `cmd/evidence.go:235`, `cmd/evidence.go:247`). The actionable gate for selected S3 review skills is `validateEvidenceSkillActionable` (`cmd/evidence.go:2241`).
- Blast radius: the change should stay inside the evidence skill command, tests, and generated command surface text. It should not alter `VerificationRecord` storage or lifecycle stage advancement.
- Constraints: current passing review evidence is deliberately rejected by default (`cmd/evidence.go:2257`) while narrow in-place replacement already exists for invalid review context origin and repair alignment cases (`cmd/evidence.go:2243`, `cmd/evidence.go:2250`).

### Patterns
- Existing conventions: `evidence skill` is a Cobra command with local flags and no positional args (`cmd/evidence.go:117`); flags are registered near command construction (`cmd/evidence.go:364`). Command handlers return typed CLI errors instead of printing or exiting.
- Reusable abstractions: existing duplicate/current evidence rejection can be extended by threading a boolean through `validateEvidenceSkillActionable` rather than adding a new persistence shape.
- Convention deviations: allowing refresh by default would violate the current fail-closed default, so any refresh path needs an explicit opt-in flag and clear help text.

### Risks
- Technical risks: medium. Incorrectly relaxing the duplicate check could let agents restamp evidence silently and mask stale review state.
- Guardrail domains: none detected; this is governance workflow logic, not auth, credentials, privacy, finance, schema migration, irreversible operations, or external API contracts.
- Reversibility: high. The recommended change is additive CLI surface plus tests; rollback removes the flag and restores the previous actionable check.

### Test Strategy
- Existing coverage: `TestEvidenceSkillAllowsSelectedReviewerRestampForInvalidContextOrigin` proves the narrow replacement door for invalid context origin (`cmd/evidence_skill_test.go:527`). `TestEvidenceSkillRejectsSelectedReviewerRestampWithValidContextOrigin` proves valid/current duplicate review evidence is rejected today (`cmd/evidence_skill_test.go:583`).
- Infrastructure needs: no new helpers are required; existing fixtures already create S3 review state, execution summary, selected review evidence, and evidence digests.
- Verification approach: add tests showing ordinary duplicate recording remains rejected, while the same duplicate with an explicit refresh flag overwrites the selected review skill record, restamps the digest, and returns JSON with recorded/stamped true.

### Options
- Option 1: Make current passing selected review evidence always overwriteable. Tradeoff: simplest implementation, but it weakens the default fail-closed guard and makes accidental restamps indistinguishable from intentional operator reruns.
- Option 2: Add an explicit `--refresh-current` flag for selected S3 review skills that already have current passing evidence. Tradeoff: one public flag plus documentation/tests, but preserves existing default rejection and gives operators the missing intentional path from #384.
- Option 3: Add a supplemental notes model without overwriting verification records. Tradeoff: preserves history better, but requires new storage/read surfaces and does not satisfy the existing CLI's expectation that the rerun pass can update the current review handle.
- Selected: Option 2. It is the smallest public-surface fix that addresses #384 without weakening ordinary duplicate evidence rejection.

## Unknowns
- Resolved: Whether this needs a schema change -> no; the current `VerificationRecord` write/stamp flow already supports replacing a record once `validateEvidenceSkillActionable` permits it.
- Resolved: Whether docs/agent surfaces need updates -> yes; the evidence command partial and command manifest currently list no refresh path (`internal/tmpl/templates/_partials/command-evidence-body.tmpl:14`, `internal/tmpl/templates/_partials/command-evidence-body.tmpl:53`, `internal/toolgen/toolgen.go:345`).
- Remaining: None.

## Assumptions
- `--refresh-current` should be limited to S3 selected review skills that already have passing evidence for the current review set. Evidence: the observed issue concerns rerun review peers in S3, and `validateEvidenceSkillActionable` has S3 selected-review-specific logic (`cmd/evidence.go:2241`).
- Ordinary duplicate recording without explicit refresh should continue to fail. Evidence: existing regression test asserts this fail-closed behavior for valid context origin (`cmd/evidence_skill_test.go:583`).
- The codebase map is stale for this change. Evidence: `artifacts/codebase/ARCHITECTURE.md` and related docs describe route/freshness/capability public surfaces, not `evidence skill` refresh behavior.

## Canonical References
- `cmd/evidence.go:108`
- `cmd/evidence.go:196`
- `cmd/evidence.go:217`
- `cmd/evidence.go:2241`
- `cmd/evidence.go:2257`
- `cmd/evidence_skill_test.go:527`
- `cmd/evidence_skill_test.go:583`
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl:14`
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl:53`
- `internal/toolgen/toolgen.go:345`
