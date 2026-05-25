# Research

## Research Findings

### Architecture
- Affected modules:
  - `cmd/new.go` owns the user-facing creation contract and can persist default
    worktree metadata before intent scaffolding.
  - `internal/state/worktree.go` owns worktree validation, registration, and path
    authority checks, so default worktree creation belongs there rather than in
    prompt guidance.
  - `internal/engine/artifact/codebase_map.go` owns `slipway codebase-map`
    creation and assessment, making it the correct runtime seam for replacing
    scaffold-only docs with deterministic baseline facts.
  - `cmd/done.go` and `internal/model/change.go` own final archive reporting and
    persisted change metadata, making them the correct seam for remediation
    source archive relationships.
  - `internal/state/execution_summary.go` owns execution evidence freshness, so
    targeted planning-vs-execution stale evidence classification belongs there.
  - `internal/toolgen/toolgen.go` owns generated catalog artifact shape, so root
    `slipway` catalog thinning belongs in toolgen rather than hand-editing
    generated files.
- Dependency chains:
  - `slipway new` -> `state.EnsureDefaultWorktreeForChange` -> `state.SaveChange`
    -> intent artifact scaffolding.
  - `slipway codebase-map` -> codebase-map docs -> stats/health/plan-audit
    references.
  - `slipway done` -> archive detection -> `change.yaml` persisted metadata ->
    JSON/text archive output.
  - execution summary freshness -> validate/review/done blockers.
  - toolgen catalog rendering -> `.codex`/`.claude` root catalog files.
- Blast radius: governed workflow creation, planning context, archive output,
  evidence freshness reason codes, generated catalog surfaces, and tests for the
  affected CLI/runtime contracts.
- Constraints: `change.yaml` remains current-state authority; lifecycle JSONL is
  trace evidence; default JSON handoff surfaces stay compact; generated surfaces
  must be deterministic.

### Patterns
- Existing conventions: CLI commands return structured JSON views, durable state
  mutations are delegated to `internal/state`, workflow decisions are in
  `internal/engine`, and generated skill/template behavior is protected by
  `internal/toolgen` and `internal/tmpl` tests.
- Reusable abstractions:
  - worktree validation/listing helpers in `internal/state/worktree.go`;
  - artifact assessment helpers in `internal/engine/artifact`;
  - `model.ReasonCode` definitions for stable blocker messages;
  - toolgen registry and catalog artifact renderers for generated surfaces.
- Convention deviations: none required; all fixes stay in existing ownership
  boundaries.

### Risks
- Technical risks:
  - Medium: creating a default worktree during `new` changes timing; mitigated by
    skipping fail-closed only when Git has no usable HEAD and by regression tests.
  - Medium: codebase-map baseline facts can overstate precision; mitigated by
    labeling them deterministic baseline context and preserving manual refinements.
  - Medium: archive relationship detection could over-link arbitrary archived
    paths; mitigated by scanning governed bundle markdown/yaml references and
    deduplicating normalized archive references.
  - Low: reason-code message changes can affect tests or scripts; mitigated by
    focused status/review/validate/done coverage.
  - Low: catalog thinning can remove useful duplicate prose; mitigated by
    explicit Instruction Authority pointers and retained support-file hydration.
- Guardrail domains: `external_api_contracts`, because CLI JSON output and
  generated agent contracts change.
- Reversibility: all runtime changes are local code/template changes and can be
  rolled back by reverting the patch; generated surfaces can be regenerated.

### Test Strategy
- Existing coverage: `cmd` covers CLI JSON and lifecycle behavior; `internal`
  packages cover state, artifact, progression, reason-code, and toolgen
  contracts.
- Infrastructure needs: no new external infrastructure; use existing temp Git
  repo/worktree fixtures, codebase-map command fixtures, and toolgen temp output.
- Verification approach:
  - focused regression tests for default worktree binding, codebase-map
    population, remediation archive sources, stale-evidence classification, and
    catalog thinning;
  - package-level tests for `cmd`, `internal/state`, `internal/engine/artifact`,
    `internal/tmpl`, and `internal/toolgen`;
  - full `go test -timeout=20m ./... -count=1` and `go build ./...`;
  - governed lifecycle evidence through S4/done-ready/done.

## Alternatives Considered
- Runtime-only minimum: update `workflow-feedback.md` dispositions without code
  changes. Rejected because the archived feedback described real runtime and
  generated-surface defects.
- Prompt-guidance-only remediation: teach agents to manually create worktrees,
  run codebase-mapping, and record archive relationships. Rejected because this
  keeps the same failure modes available in future automated runs.
- Selected: fix runtime ownership seams and generated contracts directly while
  keeping each change narrow: early worktree binding in state/new, deterministic
  codebase-map baseline population, remediation source archive metadata,
  targeted stale-evidence reason codes, and thin root catalog artifacts. This
  closes the actionable feedback without broad workflow redesign.

## Unknowns
- Resolved: codebase-map scaffold-only ownership -> `internal/engine/artifact`
  now populates missing/scaffold-only docs with deterministic baseline facts.
- Resolved: remediation archive relationship location -> `change.yaml`
  `remediation_sources` plus `done` JSON/text archive reporting.
- Resolved: root-vs-worktree artifact ambiguity -> discovery changes with usable
  Git HEAD bind `.worktrees/<slug>` before intent scaffolding.
- Resolved: stale-evidence over-routing -> planning artifacts produce
  `stale_planning_evidence`, task/execution drift remains
  `stale_execution_evidence`, and assurance-only edits no longer invalidate
  execution evidence.
- Resolved: root catalog duplication -> catalog artifacts now contain metadata
  and Instruction Authority pointers instead of copied full instructions.
- Remaining: None for the currently actionable archived-feedback items.

## Assumptions
- The active continuation request is explicit confirmation to select the
  runtime-fix alternative and continue through governed gates. Evidence:
  `intent.md` Approved Summary and user request to complete the current change.
- Future discovery changes should bind repo-local `.worktrees/<slug>` when Git
  has HEAD; no-head or non-Git environments should remain usable. Evidence:
  existing tests support temp/no-head workspaces, so default binding must skip
  rather than fail there.
- Catalog-only skills can stay usable as routed catalog entries if they retain
  registry metadata, support-file paths, and source-template/host-skill authority
  pointers. Evidence: toolgen catalog tests and generated manifest contract.

## Canonical References
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/changes/archived/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/workflow-feedback.md`
- `.worktrees/fix-slipway-governed-workflow-feedback/artifacts/changes/archived/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/workflow-feedback.md`
- `cmd/new.go`
- `internal/state/worktree.go`
- `internal/engine/artifact/codebase_map.go`
- `cmd/done.go`
- `internal/model/change.go`
- `internal/state/execution_summary.go`
- `internal/toolgen/toolgen.go`
