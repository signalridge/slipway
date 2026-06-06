# Assurance
## Project Context
- Tech Stack: Go
- Conventions: Slipway Agent Principles (CLAUDE.md). Source-then-regenerate.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Align every surface that documents Slipway command/flag behavior with the real
Cobra logic (bidirectional: generated docs list real flags; `--help` describes
real behavior), across `cmd/*.go` help, toolgen `commandRegistry` Arguments,
command body templates, generated skill surfaces, references, docs/, README; add
a reverse flag-contract guard; and redesign the entry skill for discoverability.
Approach A (handwritten surfaces + reverse guard). No runtime behavior change.

## Verification Verdict
PASS. Reverse flag-contract guard (`TestCobraFlagsCoveredByRegistryArguments`)
green; `--help`-vs-generated drift scan reports 0 missing flags across all 19
commands (regenerated with the worktree binary); `go test ./cmd/...
./internal/toolgen/... ./internal/tmpl/...` green (cmd 100s, toolgen 23s, tmpl
cached). No runtime behavior change; docs/skills/help aligned TO logic.

## Evidence Index
- Plan: intent.md, research.md, requirements.md, decision.md, tasks.md
- Execution evidence (S2): per-task records under
  `verification/` and the runtime evidence store.
- Verification (S3+): goal-verification / review verdicts (pending).

## Requirement Coverage
- REQ-001 → t-03 (+ t-09 verify)
- REQ-002 → t-01 (+ t-08 guard, t-09 verify)
- REQ-003 → t-02 (+ t-09 verify)
- REQ-004 → t-06 (+ t-09 verify)
- REQ-005 → t-06 (+ t-09 verify)
- REQ-006 → t-07 (+ t-09 verify)
- REQ-007 → t-08 (+ t-09 verify)
- REQ-008 → t-04, t-05 (+ t-09 verify)

## Residual Risks and Exceptions
- Entry-skill description quality is subjective; accepted with S3 human review.
- `--help`-vs-logic audit may surface a genuine behavior bug; accepted exception:
  recorded as an out-of-scope note, not fixed in this change.
- Body surfaces remain curated (not exhaustively complete by construction); only
  the registry Arguments surface is guarded for full coverage.

## Rollback Readiness
Pure `git revert` of the PR; no migration, no persisted state. Generated
`.claude/` tree is reproducible via `slipway init --refresh` from any commit, so
rollback cannot leave a half-migrated surface. Verification after rollback:
`go test ./cmd/... ./internal/toolgen/... ./internal/tmpl/...`.

## Archive Decision
Ready to archive. All eight requirements are covered with passing evidence, the
reverse guard and drift scan confirm full flag coverage, and the four-dimension
review (spec-compliance + code-quality) passed with no gaps. Active
`slipway validate --json` freshness/readiness proof is captured in this run
before `done`; the bundle is described as revalidated through that active gate,
not as a restamped archive. No guardrail domain is touched, so no safety-baseline
attestation is required.
