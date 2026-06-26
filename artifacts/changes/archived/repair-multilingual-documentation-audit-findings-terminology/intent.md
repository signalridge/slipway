# Intent

## Summary
Repair multilingual documentation audit findings, terminology consistency, command references, image links, and surface manifest drift.
## Complexity Assessment
complex
Rationale: the change touches multilingual documentation, generated command
surface metadata, README parity, image/link references, and verification for
the docs/toolgen surfaces. It is docs-profile work, but it spans several
languages and generated manifests, so it needs planned verification rather than
a trivial edit.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Repair the confirmed documentation audit findings across English, Simplified
  Chinese, and Japanese markdown surfaces.
- Fix stale factual references such as the documentation stack name and
  command examples.
- Restore command/reference parity for public CLI surfaces, including the
  missing `slipway hook` surface.
- Fix broken localized image links and README parity issues identified by the
  audit.
- Normalize high-impact terminology in localized docs where inconsistent terms
  create reader confusion.
- Regenerate or validate `docs/SURFACE-MANIFEST.json` from the current
  toolgen registry.

## Out of Scope
- Product behavior changes outside documentation/toolgen public-surface
  metadata.
- Broad copyediting of unrelated prose not tied to the audit findings.
- Committing or cleaning unrelated local scratch files such as `.gemini/` or
  `coverage.out`.

## Constraints
- Preserve existing documentation structure and locale coverage.
- Use current-worktree Slipway lifecycle output as authority.
- Do not hand-edit engine-owned verification verdicts.
- Keep generated manifest output aligned with generator checks.

## Acceptance Signals
- `go test ./internal/toolgen/...` passes.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check` passes.
- Link/image references in changed localized docs resolve to files in the
  worktree.
- Targeted searches show the audited stale terms and command examples were
  either corrected or intentionally left with a defensible reason.
- `go run . validate --json --change repair-multilingual-documentation-audit-findings-terminology`
  reports the governed change can advance or names only lifecycle review gates
  that have been explicitly addressed.

## Open Questions
None

## Deferred Ideas
- Full editorial rewrite of every localized documentation page.
- Automated locale terminology linting beyond the manifest/link checks needed
  for this repair.

## Approved Summary
Confirmed by the user's request on 2026-06-26 to fully repair and optimize the
reported documentation audit findings through a governed change, with blockers
allowed. This change will repair the audit-backed multilingual docs issues,
localized terminology drift, README/link parity defects, stale command examples,
and surface-manifest drift while excluding unrelated product behavior changes
and local scratch cleanup.
