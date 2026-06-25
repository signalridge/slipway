# Assurance

## Scope Summary

This change repairs Slipway public command-surface drift for the current
worktree. The delivered scope:

- Registers `slipway config` in `commandRegistry` as a public CLI-only setup
  command with `HasPromptSurface: false`.
- Makes root help and `config` command help source the `config` description
  from the registry.
- Removes the unsupported `review --artifact` public flag.
- Keeps generated adapter prompt command surfaces unchanged for `config`.
- Aligns command references, localized detailed command pages, JSON token
  tables, README handoff guidance, adapter design prose, SVG adapter copy, and
  `docs/SURFACE-MANIFEST.json`.
- Adds regression coverage for root help registry descriptions, Cobra flag
  coverage, detailed JSON token tables, and registered adapter names in design
  docs and SVG.
- Cleans low-risk internal naming residue by renaming the hidden root helper
  constructor and replacing `learn_test.go` with `retired_commands_test.go`.

## Verification Verdict

Verdict: pass.

All implementation waves completed with task evidence at run summary version 1.
The selected S3 peer review set converged with passing evidence:

- `spec-compliance-review`: pass.
- `code-quality-review`: pass.
- `independent-review`: pass.
- `security-review`: pass.

The current scope contract is passing, and the change remains within the
approved `tasks.md` target files. Final ship verification still owns the
terminal full-suite proof, high-risk `external_api_contracts.safety_baseline`
attestation, and done-readiness decision.

## Evidence Index

- Task evidence:
  - `t-01`: registry, CLI surface, and command-surface tests.
  - `t-02`: command, adapter, handoff, and manifest documentation.
  - `t-03`: generated-surface consistency verification.
- Runtime task ledger:
  - `.git/slipway/runtime/changes/repair-public-command-surfaces/evidence/tasks/t-01.json`
  - `.git/slipway/runtime/changes/repair-public-command-surfaces/evidence/tasks/t-02.json`
  - `.git/slipway/runtime/changes/repair-public-command-surfaces/evidence/tasks/t-03.json`
- Wave orchestration:
  - `verification/wave-orchestration.yaml`
  - `verification/wave-orchestration-notes.md`
- S3 reviews:
  - `verification/spec-compliance-review.yaml`
  - `verification/spec-compliance-review-notes.md`
  - `verification/code-quality-review.yaml`
  - `verification/code-quality-review-notes.md`
  - `verification/independent-review.yaml`
  - `verification/independent-review-notes.md`
  - `verification/security-review.yaml`
  - `verification/security-review-notes.md`
- Fresh verification already captured before this assurance:
  - `go test ./cmd ./internal/toolgen -count=1`
  - `go test ./...`
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check`
  - `go run . --help`
  - `go run . review --help`
  - `go run . config --json`
  - Stale-token and stale-adapter-copy `rg` scans.

## Requirement Coverage

- REQ-001: covered by `config` registry inclusion, root help registry
  descriptions, manifest command rows, and command docs.
- REQ-002: covered by `HasPromptSurface: false`, prompt command ID filtering,
  and tests asserting `config` does not generate adapter command surfaces.
- REQ-003: covered by canonical JSON tokens in `docs/reference/commands.md` and
  EN/JA/ZH detailed command pages, plus manifest token tests.
- REQ-004: covered by design docs and SVG naming the ten registered adapters,
  plus registry-derived adapter-list tests.
- REQ-005: covered by removal of `review --artifact`, help regression tests, and
  generated command-reference cleanup.
- REQ-006: covered by README and command reference `handoff write --section`
  documentation.
- REQ-007: covered by the hidden root helper rename and retired command test
  rename, with no user-facing command behavior change.

## Residual Risks and Exceptions

No unresolved blockers are accepted. Residual risk is limited to documentation
translation nuance in localized pages; the stable executable tokens are literal
and covered by tests. `config` remains CLI-only by design, so adapter users must
invoke it through the CLI rather than a generated prompt wrapper.

## Rollback Readiness

Rollback is straightforward before merge: revert this change's edits to `cmd/`,
`internal/toolgen`, `internal/tmpl`, `docs/`, `README.md`, and the governed
bundle. Runtime task evidence is stored under the repo-local Slipway runtime
ledger for this change and does not affect unrelated changes. No schema,
dependency, external service, or irreversible data migration is part of this
change.

## Archive Decision

Archive readiness decision: ready once terminal ship-verification records
passing evidence for the current run.

Active `validate --json` proof was captured during S3 review before `done`; the
active validate gate proves the current active governed state, not an archived
bundle. After ship-verification passes, the change may proceed to done-ready and
then `slipway done` can archive it if the operator requests finalization.
