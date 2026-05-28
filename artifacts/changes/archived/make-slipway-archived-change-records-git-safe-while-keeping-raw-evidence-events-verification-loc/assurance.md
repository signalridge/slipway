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
worktree-owned archive placement for worktree-bound changes, idempotent ignore
management at local-state entry points, learning diagnostics for intentionally
absent archived event logs, and operator documentation. It intentionally does
not add backward-compatibility schema shims for older archive variants.
Independent-review remediation also closes `done --json` archive path reporting
from the actual worktree invocation path, in-place archive repair sanitization,
archived slug/stat discovery, and default-worktree ignore management.

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
- `go test -count=1 ./internal/state ./cmd ./internal/bootstrap`
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
- REQ-001: `t-02`, `t-05`, `t-06`, `t-07`
- REQ-002: `t-01`, `t-05`, `t-06`, `t-07`
- REQ-003: `t-01`, `t-05`, `t-06`, `t-07`
- REQ-004: `t-03`, `t-05`, `t-06`
- REQ-005: `t-04`, `t-06`

## Residual Risks and Exceptions
No unresolved risks remain for newly written archives. New archived records
omit `worktree_path`, use relative artifact paths, and stay in the owning
workspace archive so the feature worktree owns the full governed record until it
is committed or merged. Repair sanitizes worktree-local archives in place, and
raw proof directories are ignored without hiding top-level governed records.

## Rollback Readiness
Rollback is a normal Git revert of the code and documentation changes. Active
`change.yaml` schema remains unchanged.

## Archive Decision
The governed change is archived as done. Top-level archived records are
Git-manageable; raw `evidence/`, `events/`, and `verification/` directories
remain local-only.
