# Tasks
## Project Context
- Tech Stack: Go
- Conventions: engine under internal/engine (read-only over model); cmd thin orchestrators; generated skills/commands from internal/tmpl/templates + toolgen; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Engine relevance-advisory trigger: add `codebaseMapRelevanceAdvisory(status, nextSkillName)` in `cmd/next_skill_view.go` that fires for `CodebaseMapStatusPopulated`/`CodebaseMapStatusPartial` under all durable-map consumers (research-orchestration, plan-audit, AND wave-orchestration), surfaced independent of lifecycle state (not gated to S1_PLAN) so the issue #80 wave-orchestration handoff is covered, and routed through non-blocking `view.Warnings`; the wording states status reflects content presence not scope relevance and prompts a host relevance self-check + inline refresh.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/next_skill_view.go, cmd/next_skill_capability_hints_test.go]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: verdict
  - acceptance: A populated/partial map under a consuming skill surfaces a non-blocking `warnings` relevance advisory (not a blocker); non-consuming skills get none; the advisory and the existing non-durable advisory do not double-fire; public JSON field shape is unchanged; an advisory-matrix test pins the behavior and the prior populated->empty assertion is updated.

- [x] `t-02` Skill + reference-doc guidance: instruct the host to treat a `populated` map as present-not-verified-relevant, judge it against the current change scope, and re-author stale sections inline; rewrite the codebase-map reference staleness section to that host-AI semantic relevance judgment, removing the git-mtime/entry-point/lockfile fingerprint heuristics.
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/research-orchestration/SKILL.md, internal/tmpl/templates/skills/plan-audit/SKILL.md, internal/tmpl/templates/skills/codebase-mapping/SKILL.md, internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/context-assembly/references/codebase-map.md]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
  - evidence: verdict
  - acceptance: research-orchestration, plan-audit, codebase-mapping, AND wave-orchestration skills instruct the populated-map self-check + inline refresh; plan-audit no longer says `partial` gets no whole-map advisory; the reference doc defines staleness as host-AI semantic relevance with inline re-author, routes `slipway codebase-map` only to missing/scaffold/baseline scaffolding (not the stale-populated no-op), and contains no mtime/entry-point/lockfile staleness rule.

- [x] `t-03` Proof + generated-surface alignment: `go build ./...`, `go vet ./...`, `go test ./...`, `go test ./internal/toolgen/...`, `golangci-lint run`, and `go run . init --refresh --tools all` followed by a diff review confirming only the intended skill/reference additions; confirm the advisory is additive (public JSON field shape unchanged) and non-blocking.
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [internal/toolgen]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - evidence: checklist
  - acceptance: All proof commands pass under the current worktree binary; the generated skills/reference reflect only the intended additions; the relevance advisory surfaces non-blocking for populated/partial maps under consuming skills; the public JSON field shape is unchanged.
