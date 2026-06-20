# Decision

## Alternatives Considered

### Option 1: Add only an auto checkpoint event marker
This would repair the highest-confidence audit gap by adding an
`auto_checkpoint_acknowledged` side effect to auto-resolved `human_verify`
checkpoint events. It is the smallest code change, but leaves skill softening
with default-permissive polarity and leaves redline/C1 regression gaps.

### Option 2: Focused hardening of existing auto-mode seams
This adds an auto-specific checkpoint event marker, separates learn signals for
manual and auto checkpoint resolution, converts skill auto-softening to an
explicit pure-pacing allowlist, and adds targeted regression tests for redline
surfaces and blocker precedence. It changes only existing auto-mode seams and
tests, while preserving evidence gates and security-review hard stops.

### Option 3: Broaden auto-mode observability API
This includes Option 2 and additionally adds top-level JSON fields for effective
auto mode and extra session-start audit events. It may be useful later, but it
creates a wider public surface than the confirmed findings require.

## Selected Approach
Select Option 2.

The confirmed issues are localized to current auto-mode seams: checkpoint event
metadata, skill boundary polarity, documentation/test pins, and one precedence
test. Option 2 fixes all confirmed issues without expanding public JSON APIs or
touching unrelated lifecycle gates. `security-review` remains a hard stop both
inside and outside guardrail domains.

## Interfaces and Data Flow
- `cmd/run.go` continues to inject `autoAcknowledgedResponse` only for eligible
  fresh non-guardrail `human_verify` checkpoints, and also threads a
  non-user-controlled auto-acknowledgment signal for that injected response.
- `cmd/next_context_build.go` continues to carry the response in
  `resumeCheckpoint.UserResponsePayload` and exposes the explicit auto signal
  to the checkpoint consumption path.
- `cmd/next.go` consumes active checkpoints and will mark
  `checkpoint.resolved` events as auto acknowledged only when the explicit auto
  signal is set for the consumed checkpoint. Manual `--resume-response` text,
  including the literal sentinel string, remains manual.
- `cmd/learn.go` will expose separate lifecycle signal counts for manual
  checkpoint resolutions and auto-acknowledged checkpoint resolutions.
- `internal/engine/progression/confirmation_boundaries.go` will expose an
  explicit pure-pacing allowlist used by `cmd/next.go` to decide whether a skill
  handoff may soften under auto. The allowlist preserves current auto-softened
  non-sensitive host handoffs for `intake-clarification`,
  `research-orchestration`, `plan-audit`, `wave-orchestration`,
  `spec-compliance-review`, `code-quality-review`, `independent-review`,
  `goal-verification`, and `final-closeout`; `security-review`,
  `worktree-preflight`, and unknown skills remain hard stops.
- `README.md`, `internal/toolgen/toolgen.go`, and
  `internal/tmpl/templates/_partials/command-run-body.tmpl` keep their public
  text semantics; tests pin their redline phrases. `README.md` is a verification
  target for these assertions, not an expected edit unless the existing text must
  be minimally clarified for testability.

No file format migration is needed because lifecycle events already support
side effects and diagnostics.

## Rollout and Rollback
Rollout is the normal Slipway source release path after tests pass.

Rollback path:
- Revert the code and test changes in this change.
- Verify with `go test ./cmd ./internal/toolgen ./internal/tmpl` and
  `go build ./...`.

Because the event marker uses existing optional event fields, rollback does not
require data migration. Older events without the marker remain readable.

## Risk
- Changing skill auto-softening to an allowlist can make future or unclassified
  handoffs require manual confirmation under auto. This is intentional
  fail-closed behavior and only affects pacing, not evidence gates. To prevent a
  current-behavior regression, tests must also prove every listed current
  pure-pacing skill still softens under auto.
- Auto checkpoint event attribution must not rely on the internal sentinel value
  alone because `--resume-response` is user-controlled. Tests must prove manual
  response text equal to the sentinel is not marked as auto.
- Redline tests should assert stable phrases, not full paragraphs, to avoid
  brittle copy-edit failures while still preventing safety text removal.
