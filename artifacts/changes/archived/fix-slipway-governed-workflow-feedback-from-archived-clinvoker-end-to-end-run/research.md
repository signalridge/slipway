# Research

## Research Findings

Question: which archived workflow-feedback items still require Slipway code, template, or documentation changes in the current checkout?

### Architecture
- Affected modules: `cmd/next.go`, `cmd/next_skill_view.go`, `cmd/codebase_map.go`, `cmd/done.go`, `internal/engine/governance/runtime_actions.go`, `internal/engine/governance/actions.go`, `internal/engine/progression/skill_resolution.go`, `internal/engine/progression/advance_governed.go`, `internal/engine/artifact/codebase_map.go`, `internal/engine/wave/parse.go`, `internal/state/paths.go`, `internal/state/lifecycle.go`, `internal/state/stats.go`, `internal/tmpl/templates/skills/*`, and `docs/*`.
- Dependency chains: CLI commands call progression/governance/artifact/state packages; `internal/state` owns canonical paths and cannot depend on engine packages; templates feed generated `.codex/skills` surfaces.
- Blast radius: external agent JSON contracts, governed artifact parsing/validation, codebase-map advisory status, worktree-bound bundle/archive paths, and generated skill guidance.
- Constraints: preserve existing state authority (`change.yaml`) and lifecycle trace behavior; keep JSON additions backward compatible; avoid unreviewed archive relocation that makes existing archived bundles undiscoverable.

### Patterns
- Existing conventions: command JSON view structs live in `cmd`; reason codes stay snake_case; tests usually seed temp workspaces and assert view fields/reason codes.
- Reusable abstractions: `state.ResolveChangePaths`, `state.DisplayPath`, `artifact.ResearchStructureBlockers`, `progression.ResolveNextSkill`, `governance.ResolveRuntimeRequiredActions`, and `wave.ParseTaskPlan`.
- Convention deviations: codebase-map placeholder detection is currently absent; S1 bundle is intentionally hostless; archive paths always resolve to the project root.

### Risks
- Technical risks: changing worktree/archive path behavior can break active/archived discovery; changing plan substep host resolution can alter `run` state transitions; changing task parser metadata affects wave semantic hashes.
- Guardrail domains: `external_api_contracts` because agent-facing JSON and generated skill instructions are part of the integration contract.
- Reversibility: documentation/template/parser/action-text fixes are low risk; worktree-local archive relocation and early worktree binding are higher-risk and should be split or guarded by compatibility tests.

### Test Strategy
- Existing coverage: worktree preflight deadlock already has regression coverage in `cmd/worktree_preflight_test.go`; command surfaces have extensive `cmd/*_test.go`; artifact and wave parsers have package tests.
- Infrastructure needs: add focused tests for S0 required-action wording, scaffold-only codebase-map state, task metadata keys, research template headings, plan-audit future lifecycle warning guidance, and archive artifact path rewriting.
- Verification approach: run targeted `go test ./cmd ./internal/engine/... ./internal/state/... -count=1`, then full `go test -timeout=20m ./... -count=1` and `go build ./...`.

### Unknowns Resolved
- Already fixed: S2 worktree-preflight evidence deadlock has a current regression test (`TestNextL3WorktreePreflightEvidenceUnblocksS2Execute`).
- Still unfixed: S0 research action wording still says `complete research.md`; `codebase-map` still creates scaffold-only files and reports them as created; research host instructions still surface `### Unknowns Resolved` rather than top-level schema headings; task parser still rejects `evidence` and `acceptance`; S1 bundle still has no host handoff.
- Documentation/policy candidates: read-only lock contention can be documented as non-parallelizable unless shared read locks are implemented; full-suite commands should use explicit timeout; root `slipway` catalog duplication needs generated-manifest/toolgen policy, not manual deletion.
- Higher-risk follow-up: binding worktrees before S1 bundle creation and moving archives into worktree-local archive roots affect path discovery; this change should at least fix archive-local artifact paths if keeping project-root archives.

### Remaining Questions
- None blocking implementation. The plan can split lower-risk contract/schema/template fixes first and leave early worktree binding as a clearly scoped later task only if path-discovery compatibility cannot be proven safely in this change.

## Alternatives Considered

1. Minimal documentation-only response.
   - Tradeoff: fast, but leaves reproducible runtime friction in `next`, `codebase-map`, research validation, and task parsing.
   - Rejected because the user asked to fix all issues and test the workflow, not only record policy notes.

2. Broad architecture rewrite: early worktree binding before S1, shared read locks, worktree-local archives, generated catalog replacement.
   - Tradeoff: more complete, but high risk across discovery, archive lookup, repair, and generated surfaces.
   - Rejected as the first batch because the current checkout needs surgical, test-backed fixes before path architecture changes.

3. Layered compatibility fix.
   - Tradeoff: addresses low-risk runtime/template/parser defects now, documents exclusive-lock and catalog policy clearly, rewrites archive artifact metadata for root archives, and leaves only compatibility-sensitive path relocation for a dedicated task if tests show it cannot be safely landed.
   - Selected because it resolves the agent-facing blockers while preserving existing archive discovery and worktree state authority.

## Unknowns

- No implementation-blocking unknowns remain for the first batch.
- Early worktree binding before S1 artifact creation remains a compatibility-sensitive design decision; tests must prove archive and active-change discovery before changing that path.

## Assumptions

- Adding JSON fields to `codebase-map` output is backward compatible for existing consumers.
- Accepting `evidence` and `acceptance` task metadata is safer than instructing agents not to write those fields, because plan-audit already asks for evidence shape and acceptance detail.
- Keeping project-root archives is acceptable if archived artifact paths are rewritten to archive-local paths and `done` reports/validates the relocation.
- Documentation is an acceptable resolution for `status`/`next` parallel lock contention if the command lock remains intentionally exclusive.

## Canonical References

- `artifacts/changes/archived/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/workflow-feedback.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `internal/engine/governance/actions.go`
- `internal/engine/artifact/codebase_map.go`
- `internal/tmpl/templates/skills/research-orchestration/SKILL.md`
- `internal/engine/wave/parse.go`
- `internal/engine/progression/skill_resolution.go`
- `internal/state/lifecycle.go`
- `cmd/worktree_preflight_test.go`
