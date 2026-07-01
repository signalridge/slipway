# Decision

## Alternatives Considered

### Option A: Keep Validation Only In Ship/Readiness
The existing `context_origin_handle_invalid` readiness blocker already rejects
passing selected-review YAML without a valid review context-origin handle.

Tradeoff: zero write-path churn, but it preserves the false-completion state
from issue #394 because invalid `verdict: pass` YAML can still be persisted.

### Option B: Add Record-Time Validation In `evidence skill`
Build the candidate `VerificationRecord` before persistence, validate selected
S3 review pass evidence against a model-level exact-one review context-origin
parser helper, then write only structurally valid records.

Tradeoff: small command and parser change, but it closes the bug at the point
where the false authority record would otherwise be created. This keeps the
later readiness gate as defense in depth.

### Option C: Centralize Candidate Validation In Progression Authority
Move the selected-review pass candidate through progression authority before
writing evidence.

Tradeoff: maximally centralized, but it couples command candidate validation to
broader readiness evaluation and is more invasive than the bug requires.

## Selected Approach

Select Option B.

The selected design adds a targeted record-time guard in `cmd/evidence.go` and
a parser helper in `internal/model/context_attestation.go`. The guard applies
only when all of these are true:

- the active change is in `S3_REVIEW`;
- the target skill is currently selected as an S3 review skill;
- the submitted verdict is `pass`;
- the submitted references do not contain exactly one valid
  `context_origin:stage=review=<handle>` token.

The guard runs before reference de-duplication and before
`state.SaveVerification`, so malformed records cannot be persisted and duplicate
identical context-origin tokens are not hidden by CLI normalization.

## Interfaces and Data Flow

- CLI input: `slipway evidence skill --skill <selected-review-skill>
  --verdict pass --reference "context_origin:stage=review=<handle>"
  --notes-file artifacts/changes/<slug>/verification/<skill>-notes.md`.
- Command flow: parse flags -> validate stage/actionability -> build candidate
  `VerificationRecord` from raw references -> validate exact-one review
  context-origin for selected S3 reviewer passes -> de-duplicate references ->
  run digest checks -> save YAML -> stamp digest/change evidence refs.
- Parser flow: `ExactlyOneReviewContextOriginHandleFromVerification` reuses the
  model context-origin token grammar and rejects missing, malformed, conflicting,
  or repeated review-stage handles for record-time validation.
- Documentation flow: generated evidence command surfaces, review skill
  templates, capability remediations, surface manifest, and command reference
  docs show the safe selected-review pass shape.

## Rollout and Rollback

Rollout is additive in the current codebase. No state migration is required.
Existing invalid YAML remains rejected later by readiness; new invalid pass
submissions are blocked earlier.

Rollback is a code revert of the command guard, parser helper, and surface docs.
Verification command: rerun the focused `cmd`, `internal/model`,
`internal/tmpl`, `internal/toolgen`, and `internal/engine/capability` tests, then
the broader package suite before shipping.

## Risk

- Existing agents that omit the review context-origin handle will fail earlier.
  This is intentional and provides the remediation command shape.
- The exact-one helper is stricter than the readiness parser for duplicate
  identical review handles. The stricter behavior is scoped to selected-review
  pass evidence at record time.
- Public docs in English, Chinese, and Japanese must stay aligned with the
  manifest token. The change updates all current command reference surfaces.
