## Research Findings

### Architecture
- Affected modules: `cmd/*_test.go` owns CLI contract tests and shared test fixtures; `internal/tmpl/templates/skills/worktree-preflight/SKILL.md` owns governed worktree preflight instructions; `docs/workflow-test-menu.md` owns manual governed closeout test menus.
- Dependency chains: command tests call `initTestWorkspace` and `createGovernedRequest`; those helpers create repo/workspace fixtures before exercising `cmd` surfaces and `internal/engine/progression` readiness. Worktree-preflight skill text feeds generated host guidance through `internal/tmpl`.
- Blast radius: test-only fixture setup, removal of redundant command tests covered by stronger tests, and governance guidance for when expensive baseline verification should run.
- Constraints: keep `change.yaml` as current-state authority, preserve next/status/validate contract coverage, and keep full-suite verification available before completion.

### Patterns
- Existing conventions: focused regression tests before full-suite verification are already documented in `artifacts/codebase/TESTING.md`; command tests commonly use temp workspaces and helper-written verification YAML.
- Reusable abstractions: `ensureTestGitRepo`, `initTestWorkspace`, `createGovernedRequest`, and `writeSkillVerification` are the right places to reduce fixture cost without changing production code.
- Convention deviations: replacing repeated `git init` subprocesses with a minimal test `.git` skeleton is a test-helper-only deviation; it remains compatible with real git commands used by tests that need commits or worktrees.

### Risks
- Technical risks: medium - over-deleting tests could weaken JSON/governance contract coverage; mitigated by deleting only strict-subset tests and retaining stronger coverage. Low - minimal `.git` skeleton could miss behavior from real `git init`; mitigated by targeted tests that still run git commit/worktree paths.
- Guardrail domains: none. This changes test fixtures, tests, docs, and governance skill guidance, not auth, credentials, PII, finance, migrations, destructive operations, or external API contracts.
- Reversibility: high. Deleted redundant tests can be restored from git; helper and documentation changes are localized.

### Test Strategy
- Existing coverage: `cmd` package dominates runtime. Baseline `go test -json -count=1 ./...` took 133.93s real time; package elapsed for `github.com/signalridge/slipway/cmd` was 132.699s. Next slowest packages were `internal/state` at 11.666s and `internal/toolgen` at 8.176s.
- Infrastructure needs: no new test framework. Keep using Go test JSON logs, focused `go test ./cmd -run ...`, and final `go test ./...`.
- Verification approach: run targeted tests for the touched fixtures and templates, compare `cmd` package runtime after changes, then run final full-suite and build verification after implementation stabilizes.

## Alternatives Considered
- Approach A: Only raise Go test parallelism. Tradeoff: low code churn, but measured `go test -parallel=64 ./cmd -count=1` still took 127.595s package time versus 132.699s baseline, so it does not address cwd-serialized fixture work or redundant tests.
- Approach B: Bounded fixture and redundancy cleanup. Tradeoff: deletes only strict-subset tests, optimizes the shared git fixture, and narrows governance preflight guidance so expensive full-suite proof is not repeated before final verification. This gives measurable runtime improvement while preserving contract coverage.
- Approach C: Larger command-test harness refactor to remove global cwd dependency. Tradeoff: likely larger long-term win, but it touches command construction/root resolution broadly and requires careful targeted coverage.
- Selected: Approach B + Approach C, selected by user on 2026-05-25T16:17:00Z. Keep the bounded fixture/redundancy cleanup and additionally refactor the command test harness so command tests can exercise CLI code without global process cwd mutation where practical.

## Unknowns
- Resolved: Which package dominates full-suite runtime? -> `cmd`, with 132.699s package elapsed in the baseline JSON log.
- Resolved: Does simply increasing Go test parallelism solve it? -> No; `cmd -parallel=64` still took 127.595s package time.
- Resolved: Are there redundant tests safe to delete? -> Yes; removed tests were strict subsets of retained tests covering the same behavior plus stronger assertions.
- Remaining: Identify the smallest command-root injection seam that removes broad `withWorkspace` cwd serialization without changing public CLI behavior.

## Assumptions
- Final full-suite verification must still run once after the diff stabilizes. Evidence: `AGENTS.md` build/test policy and `justfile` test target.
- Worktree preflight should prove starting worktree validity, not completed-change correctness. Evidence: `internal/tmpl/templates/skills/worktree-preflight/SKILL.md` purpose and `final-closeout` responsibility for final proof.
- The minimal `.git` skeleton is acceptable only for tests that need git repository identity; tests that need commits or worktrees continue to run real git commands. Evidence: targeted worktree-preflight tests passed after the helper change.

## Canonical References
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/codebase/TESTING.md`
- `cmd/new_test.go`
- `cmd/stats_test.go`
- `cmd/progression_next_test.go`
- `cmd/worktree_preflight_test.go`
- `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`
- `docs/workflow-test-menu.md`
- `/tmp/slipway-baseline-test.json`
