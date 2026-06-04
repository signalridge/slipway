# Assurance

## Scope Summary
This change is limited to the three active GitHub issues selected at intake:

- Issue #59: governance health must report current traceability/control diagnostics instead of preserving resolved stale controls or hiding gap identities; command docs must distinguish validate/run/health authority; a stale `wave-orchestration` record must not satisfy the wave gate after newer runtime task evidence (the plan-audit source-freshness half is deferred to #66 — see REQ-006); read-only gate projections must keep the planning gate approved after closeout assurance edits.
- Issue #61: hard-stop confirmation metadata must distinguish non-resumable skill handoffs and command boundaries from resumable checkpoints.
- Issue #62: the goal-verification placeholder scan must avoid GNU-only `grep -P` so the generated instruction works on macOS.

The implementation stays within health diagnostics, next-step confirmation diagnostics, the common required-skill evidence evaluator, command contract docs/templates, the goal-verification skill template, and focused regressions for those surfaces. It does not close the GitHub issues remotely, redesign the lifecycle state model, or change existing JSON field meanings.

## Verification Verdict
Pass. The execution, review, goal-verification, and final-closeout records for run version 1 show full test/build proof, source-scope coverage, external API contract review, evidence freshness, and closeout assurance. This turn intentionally stops at the pre-`done` state so final archive remains an explicit operator action.

Key proof already captured in run version 1:

- `go test ./...` passed.
- `go build ./...` passed.
- `git diff --check` passed.
- `go run . health --governance --json --change resolve-open-github-issues-59-61-and-62-align-governance-hea` reported healthy governance state.
- `go run . validate --json --change resolve-open-github-issues-59-61-and-62-align-governance-hea` reported fresh evidence and scope pass before final-closeout evidence was added.
- `go run . validate --json --change resolve-open-github-issues-59-61-and-62-align-governance-hea` is rerun after final-closeout evidence to confirm `G_ship` approval.

## Evidence Index
- Intake clarification: `verification/intake-clarification.yaml`
- Research orchestration: `verification/research-orchestration.yaml`
- Plan audit: `verification/plan-audit.yaml`
- Wave orchestration and task execution: `verification/wave-orchestration.yaml`, `verification/execution-summary.yaml`
- Spec compliance review: `verification/spec-compliance-review.yaml`
- Code quality review: `verification/code-quality-review.yaml`
- Goal verification: `verification/goal-verification.yaml`
- Final closeout: `verification/final-closeout.yaml`

## Requirement Coverage
- REQ-001: Satisfied by `internal/engine/governance/health.go`, `cmd/health.go`, and `cmd/health_test.go`; health now exposes traceability gap detail and recompute no longer carries resolved stale clarification controls forward.
- REQ-002: Satisfied by `cmd/next.go` and `cmd/progression_next_test.go`; confirmation metadata now reports `resume_response_supported` and a concrete `next_action` for skill handoffs, command boundaries, and checkpoints.
- REQ-003: Satisfied by `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` and `internal/tmpl/templates_test.go`; rendered goal-verification instructions use a portable Perl empty-object scan and reject `grep -P`.
- REQ-004: Satisfied by `scope_contract:pass` in validation and review evidence; changed files are limited to the planned issue surfaces and tests.
- REQ-005: Satisfied by additive JSON metadata and focused compatibility checks; existing JSON fields remain present and unchanged.
- REQ-006: Satisfied by `internal/engine/progression/wave_sync.go` and `internal/engine/progression/wave_sync_test.go`; a passing `wave-orchestration` record is rejected when runtime task evidence (`captured_at`) for the same run summary version is newer. The plan-audit source-freshness half is descoped to #66 — an mtime guard false-positives on Slipway's own `tasks.md` checkbox writeback — so `evidence.go`, `advance_governed.go`, and `readiness.go` retain their prior plan-audit semantics (missing/not-passing only) with no source-freshness check.
- REQ-007: Satisfied by `cmd/governance_gate_consistency_test.go`, `docs/commands.md`, generated command prompt partials, and `internal/tmpl/templates_test.go`; command authority boundaries, `advanced`/`blockers` semantics, and read-only gate projection consistency are documented/tested.

## Residual Risks and Exceptions
- Existing callers that ignore new confirmation metadata remain compatible, but downstream integrations must opt in to use `resume_response_supported` and `next_action`.
- An existing stale `wave-orchestration` record may now block where it previously passed; the intended remediation is to rerun wave orchestration against the current runtime task evidence.
- The portable scan now depends on Perl being available in the host environment; macOS includes Perl by default, and the template only uses it for local verification guidance.
- This change does not remotely close issues #59, #61, or #62. Closeout proves the code and governed bundle are ready for human review before those issue statuses are updated.

## Rollback Readiness
Rollback is a normal git revert of the modified command diagnostics, governance health recompute/display behavior, the wave-orchestration evidence freshness guard, command docs/templates, goal-verification template text, and associated tests. No persisted runtime state, database schema, migration, credential, network, or irreversible operation is introduced.

## Archive Decision
Ready for governed archive once the workflow reaches the pre-`done` state. The archive rationale is that all planned tasks for issues #59, #61, and #62 have passing run-version-1 evidence, both required review skills pass, goal-verification and final-closeout pass, scope validation is clean, and the remaining action is intentionally reserved for explicit `slipway done`.
