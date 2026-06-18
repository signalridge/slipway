# Tasks

## Task List

- [x] `t-00` Repair the S1/audit stale-intake lifecycle dead end discovered
  during governed advancement.
  - depends_on: []
  - target_files: [internal/engine/progression/evidence_repair.go, internal/engine/progression/evidence_repair_test.go]
  - task_kind: code
  - covers: [REQ-006]
  - acceptance: stale S0 intake evidence remains actionable before fresh
    plan-audit owns current planning inputs, but no longer blocks S1/audit
    advancement when a passing digest-fresh plan-audit certifies those inputs.
  - evidence: progression regression tests cover both the preserved fail-closed
    S1/research behavior and the S1/audit fresh-plan-audit supersession path.

- [x] `t-01` Add the Slipway handoff authoring contract to generated workflow
  and context-pressure guidance.
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/workflow/SKILL.md.tmpl, internal/tmpl/templates/_partials/command-run-body.tmpl, cmd/context_pressure_hook.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: workflow guidance owns the complete
    `.git/slipway/runtime/handoff.md` authoring contract; run and
    context-pressure guidance only point to that contract or add short trigger
    wording.
  - evidence: rendered guidance names when to write a handoff, current position,
    session work completed, next-session focus, path references instead of
    duplication, redaction, and fresh `slipway next --json` fields for suggested
    next skills; guidance states that `handoff.md` is advisory and cannot
    replace `slipway status --json`, `slipway next --json`, lifecycle gates,
    freshness, or evidence.

- [x] `t-02` Add narrow skill-template quality and decision supersession
  guidance to existing template references.
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/_shared/references/checklist-quality.md, internal/tmpl/templates/artifacts/decision.md]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - acceptance: skill-template quality guidance is scoped to editing generated
    Slipway skill templates; it does not turn the requirements checklist into a
    broad prompt-writing manual.
  - evidence: checklist guidance covers familiar leading words, reliable context
    pointers, checkable completion criteria, and no-op pruning while preserving
    `next_skill.name`, `verification_dir`, reason codes, command names, and
    evidence paths; decision guidance describes marking replaced decisions as
    superseded by a concrete replacement decision or section, without adding a
    new artifact type or importing the `teach` workspace model.

- [x] `t-03` Pin the template and runtime guidance contracts with regression
  tests.
  - depends_on: [t-01, t-02]
  - target_files: [internal/tmpl/templates_test.go, cmd/context_pressure_hook_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: template tests fail if handoff guidance is missing or if stale
    wording makes handoff a lifecycle authority, evidence source, freshness
    input, gate, or governed host skill.
  - evidence: tests assert the positive handoff contract, redaction guidance,
    fresh `slipway next --json` next-skill derivation, scoped skill-quality
    checklist terms, and decision supersession wording; if SessionStart hook
    output is touched, tests preserve the path-only/non-embedding contract so
    handoff body content is not interpreted or printed by the hook.

- [x] `t-04` Pin cross-adapter generated workflow propagation with toolgen
  regression tests.
  - depends_on: [t-01, t-02]
  - target_files: [internal/toolgen/toolgen_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: all supported host adapters receive the workflow handoff
    contract through generated source, not by editing generated `.codex`,
    `.claude`, or other adapter copies directly.
  - evidence: toolgen tests render or generate the supported adapter surfaces and
    assert the handoff/non-authority contract, required Slipway contract tokens,
    and absence of stale bypass wording across adapters.

- [x] `t-05` Run targeted and full verification, then record implementation
  evidence.
  - depends_on: [t-00, t-03, t-04]
  - target_files: [artifacts/changes/add-handoff-authoring-guidance/verification/wave-orchestration.yaml]
  - task_kind: verification
  - covers: [REQ-005, REQ-006]
  - acceptance: verification uses targeted checks before full-suite proof and
    records implementation evidence through Slipway-owned evidence commands.
  - evidence: run `go test ./internal/tmpl/...`, `go test ./internal/toolgen/...`,
    `go test ./cmd/...` when hook wording changes,
    `go test ./internal/engine/progression/...`, and final `go test ./...`;
    optional throwaway `slipway init --tools all` smoke proves generated adapter
    surfaces include the new guidance and omit bypass wording before closeout if
    targeted tests do not already prove propagation.
