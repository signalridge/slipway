# Decision

## Project Context
- Tech Stack: Go CLI with Cobra commands and filesystem-backed governance artifacts.
- Conventions: Keep production behavior stable, prefer focused command tests before full-suite verification, and write governed evidence under the active change bundle.
- Test Command: `go test ./...`
- Build Command: `go build ./...`
- Languages: Go

## Alternatives Considered

### Approach A: Raise Go Test Parallelism Only
- Evidence: `go test -parallel=64 ./cmd -count=1` still took 127.595s package elapsed versus a 132.699s `cmd` baseline.
- Benefit: smallest code change.
- Tradeoff: does not address cwd-serialized tests, repeated fixture subprocesses, or redundant contract tests.
- Decision: rejected as insufficient.

### Approach B: Bounded Fixture And Redundancy Cleanup
- Evidence: baseline `go test -json -count=1 ./...` took 133.93s real time, with `cmd` package elapsed at 132.699s. Several removed tests were strict subsets of retained tests with stronger assertions.
- Benefit: measurable improvement with low blast radius by optimizing `ensureTestGitRepo`, deleting redundant tests, and narrowing governance guidance that caused repeated expensive verification.
- Tradeoff: limited ceiling while command tests still rely on global cwd serialization.
- Decision: selected as one half of the solution.

### Approach C: Command-Test Harness Root Injection
- Evidence: many command tests use temp workspaces through helpers that mutate process-wide cwd, preventing safe `t.Parallel()` on otherwise isolated tests.
- Benefit: unlocks safe parallelization for isolated command tests and reduces future test-suite coupling to cwd.
- Tradeoff: touches root resolution across command constructors, so it needs targeted command coverage and must preserve public CLI behavior.
- Decision: selected with a minimal private test seam, not a public flag or user-facing mode.

## Selected Approach
Use Approach B + Approach C, as confirmed by the user on 2026-05-25T16:17:00Z.

The implementation keeps public CLI behavior unchanged while:
- optimizing test-only repository fixture setup in `cmd/new_test.go`;
- deleting strict-subset tests in `cmd/stats_test.go`, `cmd/progression_next_test.go`, and `cmd/worktree_preflight_test.go`;
- adding a private command context root override in `cmd/common.go`;
- migrating safe high-cost command tests to root overrides and `t.Parallel()`;
- updating generated worktree-preflight guidance and manual workflow-test guidance to avoid repeated full-suite runs before final closeout.

## Interfaces and Data Flow
- Public CLI: unchanged. Users still run commands from a workspace exactly as before.
- Private command execution seam: command constructors may resolve the project root from command context during tests, falling back to `projectRootFromWD()` in normal CLI execution.
- Initialization seam: `init` may use a context-provided workspace root for tests before `.slipway.yaml` exists, falling back to `os.Getwd()` in normal execution.
- Health workflow data flow: health contract checks receive both the Slipway project root and invocation workspace root so tests can avoid cwd mutation while preserving bound-worktree behavior.
- Template/docs flow: `internal/tmpl/templates/skills/worktree-preflight/SKILL.md` remains the generated host-skill source, with `internal/tmpl/templates_test.go` guarding the revised wording.

## Rollout and Rollback
- Rollout: land as one test/governance optimization change after targeted command tests, `go test ./cmd`, final `go test ./...`, and `go build ./...` pass.
- Rollback: revert the changed test files, root-resolution helper changes, and workflow guidance files. Restoring deleted tests is mechanical from git history.
- Reverification after rollback: run targeted command tests around `new`, `next`, `stats`, `worktree_preflight`, `health`, and `init`, then `go test ./...`.

## Risk
- Risk: a root override could accidentally change production root resolution. Mitigation: helper is context-only, no public flag/config is added, and production fallback remains `projectRootFromWD()`.
- Risk: deleting tests could weaken JSON/governance contract coverage. Mitigation: only strict-subset tests are removed, and retained tests cover the same behavior with stronger assertions.
- Risk: `t.Parallel()` could expose shared global state in command tests. Mitigation: parallelization is limited to tests using temp roots and root overrides; cwd-sensitive tests remain serialized.
- Risk: workflow guidance could under-verify changes. Mitigation: bounded verification applies to intermediate stages only; final closeout still requires fresh full-suite/build proof, with optional deeper checks triggered by risk.
