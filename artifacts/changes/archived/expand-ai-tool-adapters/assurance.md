# Assurance

## Scope Summary

This change adds the P1 AI-tool adapter set selected for
`expand-ai-tool-adapters`: `pi`, `qwen`, `kiro`, `copilot`, `windsurf`, and
`kilo`. The P1 adapters join the existing `claude`, `codex`, `cursor`,
`gemini`, and `opencode` adapters, and are available through explicit
`slipway init --tools <id>` selection and through deterministic `--tools all`
expansion.

The delivered implementation keeps adapters thin and project-local. Generated
host files route users back to the `slipway` CLI and do not implement a
parallel lifecycle engine. The implementation adds narrow generator axes for
Copilot `.prompt.md` command files, Pi registration settings, Qwen/Kiro
command-skill surfaces, and Windsurf/Kilo workflow-style surfaces. Refresh
behavior remains sentinel/ownership controlled and preserves user-owned files in
shared host directories, including Copilot's shared `.github` root.

The governed scope includes toolgen implementation, adapter contract and
generation tests, adapter documentation, installation documentation, the public
surface manifest, implementation verification notes, and exempt codebase-map
context updates.

## Verification Verdict

Verdict: pass for the governed P1 adapter objective.

All selected S3 review skills have passing Slipway evidence at
`run_version: 1`:

- `spec-compliance-review`: pass, recorded at
  `artifacts/changes/expand-ai-tool-adapters/verification/spec-compliance-review.yaml`.
- `code-quality-review`: pass, recorded at
  `artifacts/changes/expand-ai-tool-adapters/verification/code-quality-review.yaml`.
- `independent-review`: pass, recorded at
  `artifacts/changes/expand-ai-tool-adapters/verification/independent-review.yaml`.
- `goal-verification`: pass, recorded at
  `artifacts/changes/expand-ai-tool-adapters/verification/goal-verification.yaml`.

The final shared full-suite proof is
`artifacts/changes/expand-ai-tool-adapters/verification/goal-verification-full-suite.txt`.
It records `go test -count=1 -timeout=20m ./...` exiting 0, including
`cmd 313.718s`, `internal/toolgen 207.466s`, and
`internal/toolgen/cmd/gen-surface-manifest 1.320s`. The suite digest in
`artifacts/changes/expand-ai-tool-adapters/verification/suite-result.yaml` is
`7ad511145d33124a74970c16162dd3a5c3ba1d3adc40c8e758d10a1eb771bacd` and
matches `run_summary_version: 1`.

`slipway validate --json` after recording the selected S3 review evidence
reported `skills_ready` pass for the four selected reviewers, fresh evidence,
valid requirements/tasks/decision contracts, and `scope_contract.status: pass`.
The remaining blockers at that point were limited to the expected final-closeout
and assurance controls.

## Evidence Index

- Requirements: `artifacts/changes/expand-ai-tool-adapters/requirements.md`
  defines REQ-001 through REQ-005.
- Tasks: `artifacts/changes/expand-ai-tool-adapters/tasks.md` records completed
  tasks `t-01` through `t-04`, including the S3 repair scope update for
  `docs/installation.md`.
- Decision: `artifacts/changes/expand-ai-tool-adapters/decision.md` selects the
  full P1 adapter set and the thin-adapter generator approach.
- Implementation evidence:
  `artifacts/changes/expand-ai-tool-adapters/verification/implementation.md`.
- S3 repair evidence:
  `artifacts/changes/expand-ai-tool-adapters/verification/review-fix-notes.md`.
- Execution summary:
  `artifacts/changes/expand-ai-tool-adapters/verification/execution-summary.yaml`.
- Suite result:
  `artifacts/changes/expand-ai-tool-adapters/verification/suite-result.yaml`.
- Full-suite transcript:
  `artifacts/changes/expand-ai-tool-adapters/verification/goal-verification-full-suite.txt`.
- Spec review:
  `artifacts/changes/expand-ai-tool-adapters/verification/spec-compliance-review.yaml`.
- Code-quality review:
  `artifacts/changes/expand-ai-tool-adapters/verification/code-quality-review.yaml`.
- Independent review:
  `artifacts/changes/expand-ai-tool-adapters/verification/independent-review.yaml`.
- Goal verification:
  `artifacts/changes/expand-ai-tool-adapters/verification/goal-verification.yaml`.

## Requirement Coverage

| Requirement | Coverage |
| --- | --- |
| REQ-001 P1 adapter selection | Covered by the P1 registry entries for `copilot`, `kilo`, `kiro`, `pi`, `qwen`, and `windsurf`, deterministic registry sorting, `ResolveTools("all")`, and tests asserting the 11-adapter order. Evidence: `spec-compliance-review.yaml`, `goal-verification.yaml`, and `implementation.md`. |
| REQ-002 P1 generated surfaces | Covered by generated Pi prompts/skills/settings, Copilot prompts/skills, Qwen/Kiro command skills, Windsurf/Kilo workflows, and contract tests for generated paths, triggers, settings, and command invocation text. Evidence: `spec-compliance-review.yaml`, `code-quality-review.yaml`, `goal-verification.yaml`, and `implementation.md`. |
| REQ-003 adapter ownership and refresh safety | Covered by sentinel/ownership generation, modified-managed-file refusal, bare P1 host directory non-detection, unknown cleanup preservation, and Copilot shared `.github` preservation tests. Evidence: `review-fix-notes.md`, `spec-compliance-review.yaml`, `independent-review.yaml`, and `goal-verification.yaml`. |
| REQ-004 documentation and manifest visibility | Covered by `docs/ai-tools.md`, `docs/reference/ai-tools.md`, repaired `docs/installation.md`, regenerated `docs/SURFACE-MANIFEST.json`, manifest derivation tests, committed manifest checks, and docs-token tests. Evidence: `review-fix-notes.md`, `code-quality-review.yaml`, `spec-compliance-review.yaml`, and `goal-verification.yaml`. |
| REQ-005 verification | Covered by focused toolgen tests, adapter contract tests, docs/manifest checks, `git diff --check`, and the final uncached full-suite run `go test -count=1 -timeout=20m ./...` with digest `7ad511145d33124a74970c16162dd3a5c3ba1d3adc40c8e758d10a1eb771bacd`. Evidence: `suite-result.yaml`, `goal-verification-full-suite.txt`, `goal-verification.yaml`, and `implementation.md`. |

## Residual Risks and Exceptions

- Some broad overview docs outside the governed REQ-004 surfaces still use
  pre-P1 wording that mentions only the original adapter set. The S3
  code-quality review classified this as non-blocking because the scoped
  adapter reference, installation guidance, and surface manifest are current.
  A later docs cleanup can refresh those overview summaries.
- The adapter ecosystem may continue to evolve outside this change. This change
  intentionally validates the current P1 set and keeps the generator extensible
  through narrow adapter config fields, not host-specific lifecycle engines.
- Copilot uses the shared `.github` directory. The implemented mitigation is a
  dedicated `.github/copilot/slipway` sentinel/ownership root plus refresh tests
  proving user-owned `.github` files are not claimed or overwritten.
- Pi settings registration is not hook registration. The implemented mitigation
  is a distinct Pi settings mode that merges `enableSkillCommands`, `skills`,
  and `prompts` while preserving unrelated settings.

No accepted exception bypasses a requirement, review gate, ownership check, or
full-suite verification.

## Rollback Readiness

Rollback is file-level and does not require a data migration, credential
rotation, schema change, or external service rollback. To roll back, revert the
toolgen registry/model changes, adapter tests, adapter docs,
`docs/installation.md`, `docs/SURFACE-MANIFEST.json`, and this change bundle's
governed artifacts. If the manifest is changed during rollback, rerun
`go run ./internal/toolgen/cmd/gen-surface-manifest --write` and verify with
`go test ./internal/toolgen/...`.

Generated adapter files in user workspaces are opt-in, project-local surfaces.
They remain under Slipway sentinel/ownership control and can be removed or
refreshed by users without changing repository runtime state.

## Archive Decision

Archive readiness decision: ready for final-closeout review, not yet archived.

Active `slipway validate --json` proof was captured after selected S3 reviewer
evidence was recorded and before `done`; it reported fresh evidence, valid
requirements/tasks/decision contracts, `scope_contract.status: pass`, and the
four selected review skills ready. The only remaining active blockers are the
expected final-closeout controls that must be recorded before lifecycle
advancement.

Before `slipway done`, final-closeout must still record fresh
`closeout:reviewer_independence=pass` and `closeout:assurance_complete=pass`
attestations, and the active validation gate must be checked again. Archived
bundles are not used as active validation input.
