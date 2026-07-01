# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/evidence.go`: `evidence skill` builds the candidate
    `VerificationRecord` before `state.SaveVerification`, which is the correct
    fail-before-persistence seam for issue #394 (`cmd/evidence.go:193`,
    `cmd/evidence.go:204`, `cmd/evidence.go:212`).
  - `internal/model/context_attestation.go`: owns the reusable
    `context_origin:stage=<stage>=<handle>` parser and existing review-handle
    extraction (`internal/model/context_attestation.go:154`).
  - `internal/engine/progression/authority.go`: remains the later S3
    readiness defense-in-depth gate for invalid selected-review context-origin
    evidence (`internal/engine/progression/authority.go:687`).
  - `internal/tmpl/templates/_partials/command-evidence-body.tmpl`,
    `internal/tmpl/templates/skills/*review*/SKILL.md.tmpl`,
    `internal/toolgen/surface_manifest.go`, and `docs/**/commands.md` are the
    public/agent-facing evidence examples that must not teach invalid pass
    evidence.
- Dependency chains:
  - CLI command -> model parser -> state verification write -> progression
    digest stamp.
  - Template/toolgen source -> generated command/skill docs -> agent command
    examples.
- Blast radius: low-to-medium. The runtime behavior changes only for passing
  selected S3 review skills at record time. Fail verdicts, non-review skills,
  unselected review skills, task evidence, and task/wave Markdown proof remain
  outside the hard blocker.
- Constraints:
  - The command must validate before `SaveVerification`, not after
    ship/readiness evaluation.
  - The check must run before CLI reference de-duplication, otherwise duplicate
    identical review handles become invisible.

### Patterns
- Existing conventions:
  - `cmd/evidence.go` already validates stage/actionability before writing and
    returns structured `newInvalidUsageError` values with actionable
    remediation.
  - `internal/model/context_attestation.go` keeps the attestation grammar free
    of command/template imports.
  - Existing tests use command-level fixtures in `cmd/evidence_skill_test.go`
    to assert whether YAML is written.
- Reusable abstractions:
  - Add a model-level exact-one parser helper for review context-origin handles
    instead of command-local string checks.
  - Reuse `selectedReviewSkillsForChange` to scope the guard to the active
    selected review set.
- Convention deviations: None required. The fix is additive and follows the
  existing CLI error/test patterns.

### Risks
- Technical risks:
  - Medium: rejecting selected-review pass evidence could block old agent
    workflows that relied on malformed records. This is desired fail-closed
    behavior for issue #394.
  - Low: duplicate identical context-origin references were previously
    idempotent in the readiness parser. The stricter helper is used only at
    record time for selected S3 review passes, preserving the broader parser's
    defense-in-depth behavior.
  - Low: docs and generated skill examples may drift. Template, toolgen, and
    capability tests now pin the context-origin and notes-file examples.
- Guardrail domains: none. This is governance/evidence integrity, not auth,
  secrets, PII, finance, schema migration, or irreversible operations.
- Reversibility: straightforward. The command guard, parser helper, and docs
  are additive and can be reverted without state migration.

### Test Strategy
- Existing coverage:
  - `cmd/evidence_skill_test.go` already exercises S3 selected-review evidence
    ordering, restamp, refresh-current, and unselected security-review behavior.
  - `internal/model/context_attestation_test.go` covers context-origin parser
    behavior.
  - `internal/tmpl/templates_test.go`, `internal/toolgen/surface_manifest_test.go`,
    and capability tests cover generated/public surfaces.
- Infrastructure needs: no new harness. Existing command fixtures can create an
  active S3 change, write execution summary evidence, run the command, and
  assert the YAML path remains absent.
- Verification approach:
  - Command-boundary tests for missing, malformed, duplicate conflicting, and
    duplicate identical review context-origin references.
  - Success test that keeps selected reviews unordered while requiring a valid
    context-origin handle.
  - Fail-verdict test proving the hard guard applies only to pass records.
  - Template/toolgen/capability tests proving examples and fallback text teach
    `context_origin:stage=review=<handle>`, `*-notes.md`, and fallback
    references.

### Options
- Option A: rely on the existing ship/readiness `context_origin_handle_invalid`
  blocker only.
  - Tradeoff: no code churn, but preserves the false-completion affordance:
    invalid pass YAML can still be written.
- Option B: add record-time validation in `cmd/evidence.go` backed by a
  model-level exact-one review context-origin parser helper, plus focused tests
  and public-surface updates.
  - Tradeoff: small command/parser change and doc churn, but it fails before
    persistence and directly satisfies the issue.
- Option C: move the entire selected-review evidence validation into the
  progression authority layer and call it from `evidence skill`.
  - Tradeoff: centralizes all review authority checks, but risks broader
    coupling and makes the record-time "candidate before persistence" path more
    complex than needed.
- Selected: Option B. It is the smallest fail-closed change at the write
  boundary, reuses the existing parser layer, keeps the later readiness gate as
  defense in depth, and updates the agent-facing examples that contributed to
  the bug.

## Unknowns
- Resolved: Locate the evidence write path, selected-review selection source,
  and reusable parser -> `cmd/evidence.go`, `cmd/next_skill_view.go`, and
  `internal/model/context_attestation.go`.
- Resolved: Identify generated/recovery text surfaces -> command evidence
  partial, S3 review skill templates, capability remediations, surface manifest,
  and command docs.
- Remaining: None.

## Assumptions
- A failing selected review may still be recorded without a context-origin
  handle because the fail record is not a passing selected-review completion
  claim. Evidence: the issue acceptance criteria explicitly target
  `--verdict pass`, and command tests cover fail verdicts separately.
- Duplicate identical review context-origin references should fail at record
  time even though the readiness parser treats identical repeats as idempotent,
  because issue #394 asks for exactly one valid reference and the command would
  otherwise de-duplicate them before persistence.

## Canonical References
- `cmd/evidence.go:193`
- `cmd/evidence.go:204`
- `cmd/evidence.go:212`
- `cmd/evidence.go:1537`
- `internal/model/context_attestation.go:154`
- `internal/model/context_attestation.go:168`
- `internal/engine/progression/authority.go:687`
- `cmd/evidence_skill_test.go:531`
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl:17`
