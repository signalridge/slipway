# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
deeply research and optimize backward compatibility layers, redundant layers, unnecessary upstream references, and unnecessary version metadata
## Complexity Assessment
complex
Rationale: the work spans CLI progression behavior, persisted evidence formats,
generated adapter cleanup, artifact metadata, documentation, and archived
governance bundles. The investigation must separate removable compatibility
from intentional governance boundaries before implementation.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Deeply research current backward-compatibility layers, redundant projection or
  adapter layers, unnecessary upstream/local reference material, and unnecessary
  version metadata in the Slipway repository.
- Produce a research-backed optimization plan that groups findings by risk and
  recommends a concrete implementation sequence.
- Present a second confirmation point after research and before code edits.
- After that confirmation, implement the approved cleanup in this governed
  change as a concentrated pass where practical, instead of splitting every
  cleanup into a separate change.
- Refresh affected docs, tests, generated/governance artifacts, and verification
  evidence required by the approved implementation scope.

## Out of Scope
- Rewriting the governance kernel or replacing the current state machine model.
- Introducing new user-facing "compact", "lite", or equivalent mode layers as
  the answer to context or compatibility concerns.
- Changing unrelated product behavior, package manager setup, release channels,
  or external upstream repositories.
- Removing migration/compatibility behavior that is still required for tracked
  active or archived changes without an explicit migration or deprecation path.

## Constraints
- Work in the dedicated governed worktree for this change.
- Preserve Slipway's authority boundaries: `change.yaml` remains current-state
  authority, lifecycle events remain trace, and generated host surfaces remain
  projections from source registries.
- Keep generated cleanup marker-gated and allowlist-driven; do not delete
  user-managed adapter files.
- Keep the default command contract disciplined by construction rather than
  adding another user-facing mode.
- Do not begin code edits until the research bundle and recommended approach
  have been shown for second confirmation.

## Acceptance Signals
- `research.md` covers architecture, patterns, risks, test strategy, and
  alternatives with concrete file references.
- The research output gives a second confirmation point that names the proposed
  one-pass implementation scope and any exclusions.
- If implementation is confirmed, the approved cleanup is completed in this
  governed change with focused tests for touched behavior plus final
  `go test ./...`, `go build ./...`, and `go run . validate --json` evidence
  unless a narrower verification command is explicitly justified.
- Docs and archived/governance artifacts touched by the cleanup are refreshed so
  they do not describe stale paths, obsolete upstream dependencies, or removed
  compatibility behavior.

## Open Questions
(none)

## Deferred Ideas
- Broader architecture simplification beyond the identified compatibility,
  redundancy, upstream-reference, and version-metadata surfaces.
- Multi-release public deprecation policy for downstream users if research shows
  a compatibility layer should not be removed immediately.

## Approved Summary
Confirmed 2026-05-27T06:35:06Z: perform deep research first, present a second
confirmation point with a concrete optimization plan, then after confirmation
modify as much as practical in one concentrated governed implementation pass.
