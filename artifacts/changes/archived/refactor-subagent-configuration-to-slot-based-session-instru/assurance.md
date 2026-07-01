# Assurance

## Scope Summary

This change replaces the earlier provider-profile/prompt subagent design with a
slot-based delegated-session configuration surface in `.slipway.yaml`.

Delivered scope:

- Added `subagents.default`, `subagents.plan_audit`, `subagents.executor`,
  `subagents.review`, `subagents.fix`, and `subagents.verify`.
- Added slot fields `type`, `name`, `session_instructions`, and `timeout`.
- Kept `native` as the default provider family and restricted provider family
  values to `native`, `mcp`, and `skills`.
- Removed user-configurable provider internals from the public config model;
  provider routing detail is expressed through `name` and optional
  `session_instructions`.
- Projected resolved directives into plan-audit, wave executor, S3 review
  batch, fix, and ship-verification host JSON surfaces.
- Updated generated skill/template wording and English, Chinese, and Japanese
  documentation, including the `slipway init --refresh` guidance after subagent
  config changes.

Plan authoring remains in the main session. `review` is a single shared dispatch
slot for the selected review batch; review providers may fan out internally.

## Verification Verdict

Pre-ship verdict: pass, pending only the terminal `ship-verification` evidence
record. All selected S3 peer reviews have passed after one documentation drift
repair.

Verification performed:

- Config model, validation, and config catalog tests passed.
- Command JSON projection tests passed for plan-audit, review, verify,
  executor, and fix surfaces.
- Generated template/tooling checks passed.
- Full Go test suite passed.
- Website documentation build passed after the S3 documentation repair.
- `git diff --check` passed.
- `slipway validate` after peer review evidence showed `scope_contract: pass`,
  fresh evidence, and all selected peer review skills ready/pass; the only
  remaining blockers were the deferred assurance file before authoring and the
  not-yet-recorded terminal ship-verification evidence.

## Evidence Index

- `verification/implementation-verification-notes.md`
  - `go test ./internal/model -count=1`: pass.
  - Focused `cmd` subagent projection tests: pass.
  - `go test ./cmd -count=1`: pass.
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check`: pass.
  - `go test ./internal/model ./cmd ./internal/tmpl ./internal/toolgen -count=1`: pass.
  - `npm run build` in `website/`: pass.
  - `git diff --check`: pass.
  - `go test ./... -timeout=20m -count=1`: pass.
- `verification/spec-compliance-review.yaml`: pass with `layer:R0=pass`.
- `verification/code-quality-review.yaml`: pass with `layer:IR1=pass`.
- `verification/security-review.yaml`: pass.
- `verification/independent-review.yaml`: pass.
- `verification/spec-compliance-review-notes.md`: S3 R0 repair and re-review
  details.
- `verification/code-quality-review-notes.md`: implementation quality review
  details.
- `verification/security-review-notes.md`: security review details.
- `verification/independent-review-notes.md`: independent review details.

## Requirement Coverage

- REQ-001 Slot-Based Config Surface
  - Covered by `internal/model/config.go`,
    `internal/model/config_test.go`, and catalog tests.
  - Verified by model tests and spec-compliance review.
- REQ-002 Session Instruction Semantics
  - Covered by resolver inheritance and override tests.
  - Verified by command projection tests and docs/template review.
- REQ-003 Provider Routing Boundary
  - Covered by type validation, native defaulting, and `mcp` / `skills`
    effective-name validation.
  - Verified by negative config tests and security review.
- REQ-004 Host Projection Coverage
  - Covered by `cmd` projection paths for plan-audit, review batch, wave
    executor, fix, and ship verification.
  - Verified by focused `cmd` tests and independent review.
- REQ-005 Documentation and Generated Surface Alignment
  - Covered by English, Chinese, Japanese docs and generated skill/template
    wording.
  - Verified by website build and S3 spec-compliance re-review after the
    documentation drift repair.

## Residual Risks and Exceptions

- Provider target existence is intentionally not validated by Slipway. For
  `mcp` and `skills`, Slipway validates that an effective `name` exists and is
  syntactically non-empty; provider-specific existence, model choice, and
  provider-private routing remain the host/provider responsibility.
- `timeout` is a host-facing hint. Slipway validates only whitespace; provider
  interpretation is intentionally outside the config schema.
- No review-substep-specific configuration is provided. This is intentional:
  `review` is the shared S3 review dispatch slot, and provider/hub internals own
  any fan-out.

No unresolved S3 review findings remain.

## Rollback Readiness

Rollback before release is local and mechanical:

- Remove `Config.Subagents`, slot resolution, validation, and catalog entries.
- Remove subagent directive projection from next/review/fix/wave/verify JSON
  surfaces.
- Remove the new subagent reference docs/sidebar entry and generated template
  wording.
- Remove added tests.

No persisted runtime migration is required because this change introduces a
project config surface and host-facing JSON projection only.

## Archive Decision

Archive readiness decision: ready to proceed to terminal ship verification.

The active change has not been archived yet. Before `slipway done`, active
`slipway validate` freshness/readiness proof must be captured against this
worktree and the current governed bundle. Archived bundles must not be described
as revalidated through the active validate gate after `done`; the active proof
belongs before finalization.
