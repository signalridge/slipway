# Assurance

## Project Context
- Tech Stack: Go, Markdown
- Conventions: repo-native `go test`, colocated `*_test.go`, governed change
  artifacts under `artifacts/changes/{slug}/`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Scope Summary
Delivered scope keeps durable governed archive records eligible for Git while
keeping raw runtime proof local-only. The change covers archive serialization,
archive discovery/listing across visible worktrees, idempotent ignore
management at local-state entry points, learning diagnostics for intentionally
absent archived event logs, and operator documentation. It intentionally does
not add backward-compatibility schema shims for older archive variants.

## Verification Verdict
Pass. Targeted package tests, full repository tests, build verification,
`go vet`, strict documentation build, diff whitespace checks, coverage
diagnostics, and Git ignore checks all passed.

## Evidence Index
- `verification/intake-clarification.yaml`
- `verification/research-orchestration.yaml`
- `verification/plan-audit.yaml`
- `verification/wave-orchestration.yaml`
- `verification/coverage-analysis.yaml`
- `verification/goal-verification.yaml`
- `go test -count=1 ./internal/state ./internal/bootstrap ./cmd ./internal/toolgen`
- `go test -count=1 ./...`
- `go build ./...`
- `go vet ./...`
- `mkdocs build --strict`
- `git diff --check`
- `git check-ignore -v -- artifacts/codebase/ARCHITECTURE.md`
- `git check-ignore -v -- artifacts/changes/{slug}/events/lifecycle.jsonl`
- `git check-ignore -v -- artifacts/changes/{slug}/verification/plan-audit.yaml`
- `git check-ignore -v -- artifacts/changes/{slug}/evidence/tasks/rv1/t-01.json`
- `git check-ignore -v -- artifacts/changes/archived/{slug}/events/lifecycle.jsonl`
- `git check-ignore -v -- artifacts/changes/archived/{slug}/verification/final-closeout.yaml`
- `git check-ignore -v -- artifacts/changes/archived/{slug}/evidence/tasks/rv1/t-01.json`
- `git check-ignore --non-matching -- artifacts/changes/{slug}/change.yaml`
  and `artifacts/changes/archived/{slug}/change.yaml` returned non-ignored
  status as expected.
- Coverage diagnostic: current changed-package coverage was 5864/7806
  statements (75.12%) versus exported `HEAD` baseline 5818/7750 statements
  (75.07%).

## Requirement Coverage
- REQ-001: `t-02`, `t-05`, `t-06`
- REQ-002: `t-01`, `t-05`, `t-06`
- REQ-003: `t-01`, `t-05`, `t-06`
- REQ-004: `t-03`, `t-05`, `t-06`
- REQ-005: `t-04`, `t-06`

## Residual Risks and Exceptions
No unresolved risks remain for newly written archives. New archived records
omit `worktree_path` and use relative artifact paths, worktree archives remain
listable through normal archived-change discovery, and raw proof directories
are ignored without hiding top-level governed records. Older archive variants
with obsolete fields may remain explicit diagnostics until a separate migration
or cleanup is requested.

## Rollback Readiness
Rollback is a normal Git revert of the code and documentation changes. Active
`change.yaml` schema remains unchanged.

## Archive Decision
The governed change is archived as done. Top-level archived records are
Git-manageable; raw `evidence/`, `events/`, and `verification/` directories
remain local-only.
