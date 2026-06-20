# Tasks

## Task List

<!--
Wave plan is engine-assigned from depends_on + target_files. Expected shape:
- Wave 1 (parallel, package-disjoint): t-01 (internal/model) ∥ t-02 (internal/engine/progression)
- Wave 2: t-03 (cmd) — depends on the config field and the AdvanceOptions field
- Wave 3: t-04 (internal/toolgen + README) — depends on the implemented surface
Each task co-locates its own tests with its production code (same Go package);
target_files are disjoint across any parallel wave.
-->

- [x] `t-01` Add the opt-in `Auto` field to `ConfigExecution` (`yaml:"auto,omitempty"`, default false, no new Validate constraint) with an `AutoEnabled()` accessor, and pin config round-trip behavior (default off + omitted; `auto: true` round-trips through ParseConfigYAML↔ToYAML).
  - depends_on: []
  - target_files: [internal/model/config.go, internal/model/config_test.go]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Add `Auto bool` to `AdvanceOptions` and implement the engine preset auto-confirm at the `PresetConfirmationBlockers` gate in `AdvanceGoverned`: when `options.Auto` and a preset confirmation is pending, confirm the suggested/effective preset upgrade-only (never downgrade), scaffold as the manual confirm path does, record a distinct auto-confirm lifecycle event, and continue advancement instead of returning the preset blocker. Pin upgrade-only behavior and that, under auto, a guardrail domain still forces its controls and missing ship/skill evidence still blocks.
  - depends_on: []
  - target_files: [internal/engine/progression/advance.go, internal/engine/progression/advance_governed.go, internal/engine/progression/advance_governed_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-006]

- [x] `t-03` Wire auto through the cmd surface. Add mutually-exclusive `--auto`/`--no-auto` to `slipway run` resolved as a TRI-STATE override (use `cmd.Flags().Changed("auto"/"no-auto")`, never two plain defaulting bools) so effective auto = flag-if-set else config (`loadConfigAtRoot`). Thread the effective auto into `buildNextViewForCommand` and its callers (run.go:217, stage.go:137, next.go:323/356) so `advanceIfReady` passes `AdvanceOptions.Auto` AND `deriveConfirmationRequirement` sees it; the `next` preview path (preview=true) MUST remain read-only — auto must NOT trigger any auto-confirm side effect during a query. Downgrade `review_batch` and non-sensitive `skill_handoff` confirmations to standing-authorization while keeping guardrail/sensitive boundaries `hard_stop`. Auto-acknowledge an active non-sensitive `human_verify` checkpoint by injecting the resume response in the run ENTRY path (`validateRunEntry`/`validateResumeEntryForCommand`, run.go:41/108) BEFORE it rejects an empty response — not only in `runGovernedLoop`; leave `decision`/`human_action` checkpoints, guardrail `human_verify`, and the intake Approved Summary manual; leave `light` auto-pass and all evidence gates untouched.
  Acceptance (cmd tests in cmd/auto_mode_test.go): (a) `--no-auto` beats config true, `--auto` beats config false, no flag falls back to config; (b) auto+non-guardrail → review_batch & skill_handoff are standing-authorization (prior_authorization_sufficient, not hard_stop); (c) auto+guardrail → both stay hard_stop; (d) CLI entry permits an auto `human_verify` ack but still REJECTS `decision`/`human_action` and guardrail `human_verify` (entry-validation level, not just loop); (e) `next` preview under auto records NO auto-confirm side effect; (f) auto does NOT auto-write the intake Approved Summary; (g) `light` auto-pass eligibility is unchanged under auto.
  - depends_on: [t-01, t-02]
  - target_files: [cmd/run.go, cmd/next.go, cmd/next_handoff.go, cmd/session_start_hook.go, cmd/stage.go, cmd/auto_mode_test.go, cmd/next_eval_fixture_test.go, cmd/progression_next_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-04` Document auto mode on the external contracts. The host-facing `run` prompt BODY is rendered from `internal/tmpl/templates/_partials/command-run-body.tmpl` (its `## Contract` section teaches `confirmation_requirement` branching) — teach there that under auto a standing-authorization boundary may be crossed on prior authorization through pure-pacing pauses, while sensitive/guardrail confirmations, the intake Approved Summary, decision/human_action checkpoints, and every evidence gate still hard-stop; mirror the standing-authorization note into `command-next-body.tmpl` if the `next` confirmation explanation needs it. Keep `internal/tmpl/templates_test.go` green (it asserts a ≤6500-byte cap on the generated run prompt — stay compact). Update the `slipway run` command surface (arguments/description) in `internal/toolgen/toolgen.go` to advertise `--auto`/`--no-auto` and keep its command-description/lock contract tests (`internal/toolgen/toolgen_test.go`) green. Document `execution.auto` plus the per-run override flags and their red lines in the README.
  - depends_on: [t-03]
  - target_files: [internal/tmpl/templates/_partials/command-run-body.tmpl, internal/tmpl/templates/_partials/command-next-body.tmpl, internal/tmpl/templates_test.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, README.md]
  - task_kind: code
  - covers: [REQ-008]
