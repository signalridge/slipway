# Assurance

## Scope Summary

This change aligns Slipway's user-facing docs, generated skill templates,
generated command-surface descriptions, recovery text, and selected CLI help
with the current code behavior for governed docs/help work.

Delivered scope:

- `status`, `review`, and `health` help now render
  `--hydrate-ref <skill-id>/<name>` instead of the stale
  `--hydrate-ref --hydrate` placeholder.
- Generated skill and command-surface templates describe the current selected
  S3 reviewer model. For the docs profile, `code-quality-review` is filtered
  out, while `security-review` remains selected when policy selects it.
- `security-review` selection is documented as policy-driven. In this change it
  is required because `blast_radius=high`, not because the docs profile or an
  auth/crypto path selected it.
- Docs distinguish git-local runtime evidence under
  `.git/slipway/runtime/changes/<slug>/evidence/...` from bundle-local
  verification artifacts under `artifacts/changes/<slug>/verification/`.
- Generated command grouping now treats `init` as setup, `codebase-map` as
  discovery, and `tool` as a CLI-only helper namespace.

## Verification Verdict

Verdict: pass for the implemented docs/help/template alignment.

Current selected S3 peer evidence is freshly recorded and passing:

- `spec-compliance-review`
- `independent-review`
- `goal-verification`
- `security-review`

Current `go run . validate --json` evidence before closeout shows:

- `scope_contract.status=pass`
- selected reviewers are exactly `spec-compliance-review`,
  `independent-review`, `goal-verification`, and `security-review`
- no `required_skill_stale` blockers remain after goal-verification was
  recorded
- remaining lifecycle blockers are the expected pre-closeout requirements:
  final-closeout evidence and closeout attestations

The fresh full-suite proof records `go test ./... -count=1` with exit code 0.
The suite-result digest points at that proof.

## Evidence Index

- `artifacts/changes/align-docs-skills-help/verification/full-suite-proof.md`
  records the full test suite command and pass output.
- `artifacts/changes/align-docs-skills-help/verification/suite-result.yaml`
  records `sha256:366f18c60ea0d5dc2494c93a669b248c9f1336a67ab82e08040da74ce4357ff7`
  for the full-suite proof.
- `artifacts/changes/align-docs-skills-help/verification/spec-compliance-review.yaml`
  records `layer:R0=pass`, `scope_contract:pass`,
  `negative_path:pass`, and `decision_fidelity:pass`.
- `artifacts/changes/align-docs-skills-help/verification/independent-review.yaml`
  records a fresh independent pass with a distinct review context handle.
- `artifacts/changes/align-docs-skills-help/verification/security-review.yaml`
  records a fresh security pass with a distinct review context handle.
- `artifacts/changes/align-docs-skills-help/verification/goal-verification.yaml`
  records fresh acceptance verification with the full-suite proof, suite-result,
  and `scope_contract:pass`.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check` reports
  `docs/SURFACE-MANIFEST.json is up to date`.
- `git diff --check` passed before evidence recording.
- `xmllint --noout docs/assets/diagrams/architecture.svg` passed.

## Requirement Coverage

- REQ-001 is covered by live help output for `status`, `review`, and `health`
  plus focused help tests in `cmd/template_flag_contract_test.go`.
- REQ-002 is covered by runtime selected-review filtering, profile-aware
  progression/authority tests, recovery text coverage, and generated skill
  template tests.
- REQ-003 is covered by docs and diagram updates, generated command-reference
  grouping checks, and command-surface tests that prevent CLI-only helpers from
  being presented as generated host surfaces.
- REQ-004 is covered by the fresh full suite, focused command/toolgen/model
  tests, the surface manifest check, SVG validation, and stale-phrase scans.

## Residual Risks and Exceptions

No implementation blockers remain.

Residual operational constraints:

- The root checkout still has unrelated pre-existing codebase-map dirt outside
  this governed worktree; it was not touched by this change.
- `security-review` remains required for this docs-profile change because the
  built-in policy selected it from `blast_radius=high`.
- `assurance.md` is authored at S3 review time and is not pre-seeded by the
  engine; final-closeout must validate this authored content and record the
  closeout attestations before `done`.

## Rollback Readiness

Rollback is straightforward: revert the changed docs, templates, command
registry/help text, tests, and governed artifacts in this change worktree. There
are no schema migrations, persistent data migrations, external API contract
changes, dependency upgrades, or irreversible operations.

If rollback is required after evidence recording, rerun `go run . validate
--json` and the focused help/template/toolgen tests against the reverted
worktree before attempting lifecycle advancement.

## Archive Decision

Archive readiness decision: proceed to final-closeout, then archive only if the
active worktree's `go run . validate --json` proof after final-closeout reports
the ship gate ready or done-ready.

This decision relies on active worktree validation before `done`. Archived
bundles are records of the completed change; they are not the surface used to
revalidate the active gate.
