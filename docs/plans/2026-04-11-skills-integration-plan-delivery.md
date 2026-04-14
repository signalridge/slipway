# Skills Integration Plan - Delivery

## Acceptance Criteria

### Kernel and catalog separation

1. `ResolveNextSkill` remains the only progression authority.
2. The current governed hosts remain the runtime kernel:
   `intake-clarification`, `research-orchestration`, `plan-audit`,
   `worktree-preflight`, `wave-orchestration`, `tdd-governance`,
   `spec-compliance-review`, `code-quality-review`, `goal-verification`, and
   `final-closeout`; 9 are registry-backed governance definitions and
   `worktree-preflight` remains a kernel-owned standalone surface.
3. The new 25-skill catalog is modeled as a separate catalog layer, not as a
   second runtime state machine.
4. Capability packs are demoted to tags and documentation views; they are no
   longer the primary architecture.
5. Slipway does not require manual invocation of absorbed skills during normal
   runtime flows.
6. `review`, `validate`, `repair`, `status`, and `health` remain the command
   surfaces; none of them become a second workflow engine.

### Catalog structure

7. `skills_ref/` remains the authoritative source corpus; rollout batching with
   a narrower working set does not change disposition or provenance coverage
   accounting.
8. The plan defines 25 independent Slipway skills across 9 domains.
9. The plan defines 6 source skills that are absorbed as posture only rather
   than promoted as standalone targets.
10. The plan defines a non-catalog disposition matrix that explicitly classifies
    `view-only`, `route-only`, `absorbed`, and `deferred` sources/surfaces
    rather than leaving ambiguous leftovers.
11. Every target skill has documented `domain`, `function`, `tier`,
    `primary_attachment`, `summary`, bounded `trigger_signals`,
    `evidence_contract`, `bindings`, and `provenance_ref`.
12. The model explicitly allows one source skill to feed multiple target skills
    and one target skill to absorb multiple source skills, with structured
    source decisions recorded in `provenance.yaml`.

### Binding registry and auto resolver

13. Runtime binding authority is owned by a Go-side binding registry.
14. Generated `SKILL.md` frontmatter is descriptive and export-facing, not the
    runtime source of truth for routing.
15. An auto capability resolver is defined with explicit inputs, outputs,
    guardrails, and a bounded trigger DSL.
16. The auto resolver may attach support skills or choose routed command paths,
    but it must not change the next governed host chosen by `ResolveNextSkill`.
17. Explicit operator flags override automatic route selection.

### Implementation surfaces

18. The source layout for catalog skills is defined as required `SKILL.md` and
    `provenance.yaml`, optional typed templates (`PROSE.tmpl`,
    `CHECKLIST.tmpl`, `VERDICT.tmpl`), and constrained optional support
    directories (`references/`, `scripts/`).
19. The plan defines an assembler or toolgen extension that compiles multi-file
    catalog sources into adapter-visible outputs with fixed ordering:
    frontmatter -> `SKILL.md` body -> conditional typed-template injection.
20. The distillation documentation surface is defined as:
    `docs/distillation/schema.md`,
    `docs/distillation/catalog.md`, `docs/distillation/by-source.md`,
    `docs/distillation/domains/*.md`, and
    `docs/distillation/routed-surfaces.md`.

### Host and command bindings

21. Foundational host bindings are defined for the governed kernel.
22. `review` is defined as the routed surface for quality, security, and
    change-shape skills, with `second-opinion` modeled as a route rather than a
    standalone catalog skill.
23. `validate` is defined as the routed surface for spec, coverage, property,
    mutation, and performance skills.
24. `repair` is defined as the routed surface for repair / CI specialist
    routes; `status` and `health` are defined as the routed surfaces for
    incident, observability, and queue views, with incident landing only on
    `status` / `health`.
25. No current `skill` command family is required by this plan; any factory
    tooling remains a deferred future command family.

### Export and product boundary

26. Export targets tool adapters, not home-directory installation.
27. No mission/work-package/dashboard runtime is introduced.
28. Slipway remains multi-tool capable and is not reduced to a tool-specific
    skill installer.

### Documentation and testing

29. Main plan and delivery docs remain synchronized in EN and zh-CN.
30. The implementation plan includes regression coverage for catalog loading,
    provenance coverage, binding resolution, routed command selection,
    assembler/export behavior, and CI gates for schema, binding, provenance
    coverage, and size-budget discipline.

### Tier, attachment, and rollout discipline

31. Every catalog skill declares a `tier` (`T1` core capability, `T2`
    specialist route, or `T3` diagnostic view). `tier` encodes semantic role
    rather than required binding count; narrowly bound T1 skills (for
    example `threat-modeling`) remain T1.
32. Every catalog skill declares a `primary_attachment` drawn from the frozen
    set `posture` / `procedure` / `checklist` / `tool-recipe` /
    `report-schema`; the resolver uses attachment mode to decide injection
    position.
33. The `technique-hint` binding reuses the existing `TechniqueHints` surface
    in `cmd/next_skill_view.go`; Go returns skill id plus hint kind while
    the host LLM organizes the hint wording.
34. B1 must deliver end-to-end: `internal/engine/capability/` registry +
    trigger DSL + resolver + `TechniqueHints` integration, together with the
    five-skill foundation set (`scope-clarification`, `plan-authoring`,
    `tdd-proof`, `fresh-verification-evidence`, `independent-review`). No
    later batch may start before B1 proves the loop. Writing 25 catalog
    skeletons before B1 is explicitly out of scope.
35. Routed command flags are shipped: `review` / `validate` / `repair`
    expose `--mode`, and `status` / `health` expose `--view`; explicit flag
    overrides take precedence over auto-route fallback.
36. CI gate automation (`schema-lint`, `size-lint`, `binding-compare`,
    `provenance-coverage-scan`) is enforced by tests in
    `internal/engine/capability/gates_test.go` and `by_source_test.go`.
    `size-lint` is tier-aware with warning bands and rationale gates.
    Warning-band overages are informative logs only; failure requires a
    hard-max overrun without the required rationale:
    T1 target <=2 KB (warn 2-6 KB; rationale above 6 KB),
    T2 target <=3 KB (warn 3-8 KB; rationale above 8 KB),
    T3 target <=1.5 KB (warn 1.5-3 KB; rationale above 3 KB).

## Implementation Checklist

Rollout is organized as linear batches B0-B8. One batch lands per PR; the
next batch cannot start until the prior batch is merged. Inter-batch context
transfer relies on merged `provenance.yaml` files plus the maintained
`docs/distillation/by-source.md`.

### B0 - Contract freeze

1. [x] Freeze `docs/distillation/schema.md`: tier definitions, frozen
       attachment mode set, trigger DSL operators, `provenance.yaml` shape,
       typed-template responsibilities, support-directory rules, fixed
       assembler ordering.
2. [x] Initialize skeletons for `catalog.md`, `by-source.md`, and
       `routed-surfaces.md`.
3. [x] Document the PR checklist that will enforce schema / size / binding /
       provenance coverage during B0-B7.
4. [x] Keep EN and zh-CN docs synchronized.

### B1 - End-to-end proof

5. [x] Implement `internal/engine/capability/{registry,trigger,resolver,
       provenance}.go` backed by the frozen B0 schema.
6. [x] Wire resolver output into the existing `TechniqueHints` surface in
       `cmd/next_skill_view.go`; do not introduce a parallel hint renderer.
7. [x] Fully distill the five foundation T1 skills:
       `scope-clarification`, `plan-authoring`, `tdd-proof`,
       `fresh-verification-evidence`, `independent-review`.
8. [x] Bind the five B1 skills to their governed hosts and to `review`
       (`independent-review` only) per §5.2 / §5.3.
9. [x] Tests: registry load, B1-scope resolver selection, technique-hint
       emission, host binding, and provenance coverage for B1 sources.
       B1 does not implement or test `hydrate_references[]` /
       `llm_tiebreak`.
10. [x] Do not author catalog skeletons beyond the five B1 skills in this batch.

### B2 - Scale foundation

11. [x] Distil remaining foundation T1 skills: `context-assembly`,
        `parallel-executor-contract`, `root-cause-tracing`,
        `security-review`, `spec-trace`.
12. [x] Tests: multi-skill resolver stability, binding compare, and provenance
        coverage for B2 sources. `context-assembly` declares
        `hydrate_references[]` in frontmatter, while resolver emission remains
        reserved (no runtime output yet).

### B3 - Security cluster

13. [x] Distil T1 `threat-modeling`.
14. [x] Distil T2 `sast-orchestration`, `gha-security-review`,
        `supply-chain-audit`; preserve tool-call differences inline in
        `SKILL.md`, using `references/` only when a longer example is
        actually needed for the Semgrep / CodeQL / SARIF trio.
15. [x] Tests: T2 command-route binding behavior and tool-recipe attachment
        injection.

### B4 - Change shape and verification

16. [x] Distil T1 `multi-reviewer-calibration`, `differential-review`,
        `variant-analysis`, `coverage-analysis`, `property-testing`,
        `mutation-testing`, `performance-profiling`.
17. [x] Tests: host binding coverage, provenance coverage for B4 sources.

### B5 - Repair/CI and ops

18. [x] Distil T2 `ci-triage`, `review-comment-triage`, `git-recovery`.
19. [x] Distil T3 `incident-response`; bind to `status` / `health` /
        export only; do not route through `repair`.
20. [x] Draft minimum view schemas for `sentry`, `skill-scanner`, and the
        other `view-only` surfaces so T3 `incident-response` does not drift
        away from them stylistically (captured in
        `docs/distillation/routed-surfaces.md`).
21. [x] Tests: T3 view-only binding and explicit view selector behavior.

### B6 - Non-catalog disposition cleanup

22. [x] Finalize `routed-surfaces.md` with `view-only`, `route-only`, and
        `deferred` entries and their command landing zones.
23. [x] Annotate the six posture-only absorptions (`using-superpowers`,
        `executing-plans`, `mission-system`, `runtime-next`,
        `agent-orchestrator`, `error-handling-patterns`) with target catalog
        skill and attachment mode.
24. [x] Run manual provenance-coverage scan across the full source corpus;
        plan-listed sources are reviewed against `provenance.yaml`
        (`extracted`, `dropped`, `conflicts_with`). Automation in
        `internal/engine/capability/by_source_test.go` is currently scoped to
        `standalone` / `partial-only` rows plus reverse consistency
        (`provenance` source must appear in `by-source.md`).

### B7 - Routed command rollout

25. [x] Implement `review` auto routing plus `--mode` override flag.
26. [x] Implement `validate` auto routing plus `--mode` override flag.
27. [x] Implement `repair` auto routing plus `--mode` override flag.
28. [x] Implement `status` auto view routing plus `--view` override flag.
29. [x] Implement `health` diagnostics view routing plus `--view` override
        flag.
30. [x] Tests: route selection, view selection, operator override precedence,
        fallback behavior. If a real DSL tie appears, validate
        `llm_tiebreak` hand-off behavior there rather than in B1. Keep
        scanner-heavy execution out of the governed kernel.

### B8 - Export and gate automation

31. [x] Extend toolgen / assembler to compile multi-file catalog sources with
        fixed ordering (frontmatter -> `SKILL.md` body -> conditional typed
        templates) and provenance metadata.
32. [x] Emit the `using-slipway-catalog.md` export target for external agents
        (`capability.BuildCatalogManifest` +
        `toolgen.CatalogManifestPath`).
33. [x] Automate `schema-lint`, tier-aware `size-lint`, `binding-compare`,
        and scoped `provenance-coverage-scan` as CI gates (enforced by
        `internal/engine/capability/gates_test.go` +
        `by_source_test.go`, running in `go test ./...`).
34. [x] Confirm repo-local `skill` command family remains deferred in this
        rollout. If introduced later, scope it to authoring and audit tooling
        only; install-to-home remains out of scope.
