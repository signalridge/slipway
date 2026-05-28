# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go, Markdown
- Languages: Go, Markdown
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
make Slipway archived change records Git-safe while keeping raw evidence events verification local
## Complexity Assessment
complex
The change crosses filesystem layout, archive serialization, ignore policy, and
validation/repair guardrails. It must preserve active runtime authority while
making archived governed records suitable for Git management.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Treat top-level governed record files as Git-manageable project records:
  `change.yaml`, `intent.md`, `research.md`, `requirements.md`,
  `decision.md`, `tasks.md`, and `assurance.md`.
- Keep raw runtime proof local by default: `evidence/`, `events/`,
  `verification/`, `artifacts/codebase/`, and `.worktrees/`.
- Make archived `change.yaml` portable by removing machine-local absolute
  paths and avoiding redundant active-runtime artifact metadata where possible.
- Add or update idempotent `.gitignore` management at Slipway entry points that
  create local governed state.
- Add validation or repair visibility for tracked/raw local artifact mistakes.
- Cover archive behavior, ignore behavior, and Git-safe record invariants with
  focused tests.

## Out of Scope
- Moving historical user archives in unrelated branches.
- Uploading or centralizing raw evidence, events, or verification records.
- Replacing Slipway's active `change.yaml` authority model.
- Introducing a hosted service or SaaS projection layer.

## Constraints
- Active changes must remain executable and recoverable; only archived,
  Git-managed records should be normalized for portability.
- Git ignore rules must be precise enough not to hide user-authored agent
  configuration or unrelated `artifacts/` content.
- Worktree-bound archive discovery must keep working after newly archived
  records stop storing absolute `worktree_path` values.
- Do not add backward-compatibility schema shims for older archived
  `change.yaml` variants.

## Acceptance Signals
- `slipway init`, `slipway new`, and `slipway codebase-map` ensure precise
  local-state ignore rules without overwriting user-authored `.gitignore`
  content.
- New archived `change.yaml` records do not contain `/Users/`, `/tmp/`, or
  absolute artifact paths.
- Archived top-level Markdown files and sanitized `change.yaml` remain eligible
  for Git management while `evidence/`, `events/`, `verification/`,
  `artifacts/codebase/`, and `.worktrees/` are ignored by default.
- `go test ./internal/state ./internal/bootstrap ./cmd` passes for targeted
  behavior.
- `go test ./...` passes before closeout.

## Open Questions

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
Approved 2026-05-27: implement a complete Slipway fix so archived governed
records are Git-safe project records, while raw `evidence/`, `events/`, and
`verification/` subdirectories remain local-only by default.
