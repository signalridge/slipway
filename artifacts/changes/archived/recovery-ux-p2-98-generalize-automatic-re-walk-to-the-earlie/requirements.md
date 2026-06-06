# Requirements
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Generalized earliest-affected automatic re-walk
REQ-001: When a governed artifact or relevant code input changes after a stage's evidence was accepted, `slipway run` MUST reopen the earliest affected authority (by canonical lifecycle order), clear that authority and every downstream authority's evidence, preserve runtime task evidence, and route the existing host skill — in EVERY governed state, with no separate recovery command.

#### Scenario: Stale plan at S2 reopens S1/audit
GIVEN a change at S2_EXECUTE whose tasks.md structural plan changed after plan-audit passed
WHEN `slipway run` is invoked
THEN the change is reopened to S1_PLAN/audit, plan-audit + wave-plan + execution-summary + downstream review evidence are cleared, runtime task evidence is preserved, and next_skill is plan-audit.

#### Scenario: Code changed at S4 reopens the stale review at S3
GIVEN a change at S4_VERIFY where a reviewed code input changed after code-quality-review passed
WHEN `slipway run` is invoked
THEN the change is reopened to S3_REVIEW, the review/goal/closeout evidence is cleared, and next_skill is the first S3 review.

#### Scenario: Stale intake at S0 re-walks in place (#90)
GIVEN a change at S0_INTAKE where intent.md changed after intake-clarification passed
WHEN `slipway run` is invoked
THEN intake-clarification evidence is cleared and the host is routed to re-run intake-clarification, with no `evidence restamp` / manual digest edit required.

#### Scenario: S2 stale authority is handled without evidence-loss rescope
GIVEN a change at S2_EXECUTE with runtime task evidence and a stale authority detected at S2 or earlier
WHEN `slipway run` is invoked
THEN Slipway either rebuilds compatible S2 generated evidence in place or reopens to the earliest affected earlier authority, preserves compatible runtime task evidence, and does not route the operator to a rescope path that destroys task evidence.

### Requirement: Content-hash freshness is the sole authority
REQ-002: Evidence freshness MUST be decided purely by comparing stored input content hashes against current content (`model.EvidenceFreshness`); file mtime and digest `run_version` MUST NOT participate in any freshness decision.

#### Scenario: Re-authoring then re-emitting the verdict is fresh
GIVEN an accepted passing skill whose artifact is rewritten with identical content and the verdict re-emitted
WHEN freshness is evaluated
THEN the evidence is fresh (no spurious stale from a later mtime).

#### Scenario: A real content change is stale until the stage re-runs
GIVEN an accepted passing skill whose certified input content changed
WHEN freshness is evaluated
THEN the skill reads stale and the only resolution is re-running the owning stage (no restamp path).

### Requirement: Authority ordering derives from the canonical lifecycle
REQ-003: The earliest-affected computation MUST order authorities by the canonical lifecycle (`action.WorkflowPath` plus `planSubStepOrder`) and registry `State`/`PlanSubStep`, with no separately hand-maintained state, plan-substep, or review-layer rank table.

#### Scenario: Reopen target is the earliest stale stage, not a fixed S1/audit
GIVEN multiple stale authorities at different lifecycle positions
WHEN the reopen target is computed
THEN it is the earliest stale authority by canonical order, and read-only `next` reports the same target the mutating `run` reopens to.

### Requirement: Structural/scope plan-hash split (#89)
REQ-004: A benign `target_files`-only edit at S2 MUST re-materialize the wave-plan in place via `slipway run` (no reopen, no S0 rescope); a structural plan change MUST reopen S1/audit. "Scope-only" is narrow: task IDs, objective, wave/dependencies, task_kind, covers, evidence, acceptance, and checkpoint_type MUST remain unchanged, and only target_files may change. Plan-audit's tasks.md freshness and runtime task-evidence drift (`tasksPlanChangedSinceTaskEvidenceBlockers` and per-task `freshness_inputs.tasks_plan_hash`) MUST key on the structural hash so that narrow target_files-only edit does not stale compatible task evidence.

#### Scenario: Scope-only edit rebuilds in place
GIVEN a change at S2 where a task's target_files set changed but the DAG structure did not
WHEN `slipway run` is invoked
THEN the wave-plan is re-materialized in place and execution continues without reopening planning.

#### Scenario: Scope-only edit preserves runtime task evidence
GIVEN runtime task evidence was recorded against a tasks.md whose only later change is `target_files`
WHEN wave execution or execution-summary freshness is evaluated
THEN no `tasks_plan_changed_since_task_evidence` blocker is emitted solely because of that scope-only edit.

#### Scenario: Task contract edit is not treated as scope-only
GIVEN runtime task evidence was recorded for a task
WHEN any task contract field other than target_files changes (including objective, wave, depends_on, task_kind, covers, evidence, acceptance, or checkpoint_type)
THEN the change is structural for recovery purposes, compatible runtime task evidence is not preserved by the scope-only path, and Slipway reopens S1/audit or reports the owning structural recovery action.

### Requirement: Pivot preserves runtime task evidence (#96)
REQ-005: `slipway pivot --reroute` and `--rescope` MUST preserve runtime task evidence (`evidence/tasks/`), clearing only derived/stale state, consistent with the reopen primitive.

#### Scenario: Reroute keeps completed task evidence
GIVEN a change with recorded runtime task evidence
WHEN `slipway pivot --reroute` runs
THEN the runtime task evidence directory still exists afterward.

### Requirement: Generated execution evidence regenerates on source change (#97)
REQ-006: execution-summary and wave-plan MUST be regenerated when their source inputs changed, never reused merely because the file is readable.

#### Scenario: Stale execution-summary is rebuilt, not reused
GIVEN an execution-summary whose source tasks.md/structural hash changed
WHEN the change is advanced or repaired
THEN the execution-summary is regenerated from current source rather than the stale file being accepted.

#### Scenario: S2 repair gap routes to rebuild or re-walk
GIVEN wave-plan.yaml and execution-summary.yaml are readable but no longer match their current source inputs
WHEN `slipway repair --json` or `slipway run --json` is invoked
THEN the next public action is an executable rebuild or automatic re-walk path, not manual file editing, Tier restamp, source inspection, or pivot/rescope evidence loss.

### Requirement: The recover/restamp escape-hatch does not exist
REQ-007: There MUST be no `slipway recover` command, dependency-ordered recovery graph, Tier-2 attested restamp, or Tier 0/1/2 user-facing recovery vocabulary; `slipway evidence restamp` MUST be removed as a normal recovery path and `repair` MUST route stale-digest drift to `slipway run`.

#### Scenario: No recover/restamp surface
GIVEN the built CLI and generated skills/commands
WHEN `slipway --help` and the generated command reference are inspected
THEN no `recover` command and no `evidence restamp` recovery path are present, and stale-digest remediation points to `slipway run`.

#### Scenario: Commit-before-done staleness is review rerun only
GIVEN a diff-class review becomes stale because the operator commits before finalization
WHEN README/docs/generated skills describe the recovery
THEN they say to rerun the owning review stage through `slipway run`, not to restamp evidence or edit engine-owned digests.

### Requirement: Read-only and mutating surfaces agree
REQ-008: `next`, `validate`, `status`, and governance-blocked `CLIError` MUST report the same root authority and the same next public action that mutating `run` will perform.

#### Scenario: next matches run
GIVEN a stale state
WHEN `slipway next --json` and the subsequent `slipway run --json` are compared
THEN both name the same reopen target authority and next action.

### Requirement: Public CLI/JSON contract changes are guarded
REQ-012: Because this change alters externally consumed Slipway CLI/JSON and generated host-tool surfaces, public recovery behavior MUST be protected by contract tests, docs/generated-surface updates, rollback notes, and external-contract review evidence. Existing additive JSON fields (`recovery.primary_command`, `primary_action`, `recovery_class`, and `steps[]`) MUST remain stable unless an intentional breaking/removal is explicitly tested and documented.

#### Scenario: Recovery JSON remains contract-compatible
GIVEN a stale governed state
WHEN `next --json`, `validate --json`, `status --json`, and a governance-blocked `CLIError` are rendered
THEN the recovery object preserves its documented field shape and reports intentional command/vocabulary changes as `slipway run` recovery, with tests documenting any removed `evidence restamp` behavior.

#### Scenario: Recovery token rename is documented
GIVEN the stale recovery reason code changes from planning-only to generalized evidence recovery
WHEN README, CLAUDE/AGENTS agent instruction files, generated command references, and generated skills are inspected
THEN they document the preserved `recovery` object fields and the intentional reason-code/token rename without stale `stale_planning_recovery_available` guidance.

#### Scenario: External contract review is required
GIVEN S2 implementation changes public CLI/JSON or generated host-tool guidance
WHEN S3 review evidence is recorded
THEN spec-compliance evidence includes `layer:R3=pass` and code-quality evidence includes `layer:IR3=pass` before closeout can be treated as ready.

### Requirement: Guardrail domains fail closed to rerun/review
REQ-009: For a guardrail-domain change, recovery MUST be re-run/review only; it MUST NOT be satisfiable by any restamp or force-close bypass.

#### Scenario: Sensitive change cannot restamp
GIVEN a change with a non-empty guardrail_domain and stale evidence
WHEN recovery is attempted
THEN the only path is re-running the owning stage; no restamp/attest path is offered.

### Requirement: Certified-input coverage fails closed
REQ-010: The earliest-affected computation MUST be backed by executable assertions that every governed artifact is covered by at least one certified-input builder and that downstream skills include the upstream artifacts they rely on, or the omission is explicitly exempted by a test-backed rationale.

#### Scenario: Missing certified input is caught by tests
GIVEN a governed artifact is used by a downstream authority
WHEN the certified-input coverage test evaluates the skill input builders
THEN the artifact appears in the expected upstream/downstream skill input set, or the test fails with the missing artifact and skill name.

#### Scenario: S1 validate substep is ordered explicitly
GIVEN the model includes S1 plan substeps beyond the normal forward research/bundle/audit sequence
WHEN stale authority ordering is tested
THEN the implementation either orders the substep explicitly or proves it is a transition-only substep that cannot be a stale reopen target.

### Requirement: Skills stay aligned to the lifecycle
REQ-011: Each stale case MUST route back to the owning stage skill; there MUST be no `recover` route in any skill and no special recover skill. Skill wording MUST say stale inputs mean rerun the owning stage.

#### Scenario: Skills route to the owning stage
GIVEN the generated skills after this change
WHEN they are inspected for recovery guidance
THEN every stale case routes to the owning stage skill via `slipway run`, with no recover command referenced.

### Requirement: Agent instruction files are black-box lifecycle contracts
REQ-013: `CLAUDE.md` and `AGENTS.md` MUST exist as principle-only agent instruction surfaces. They MUST NOT contain detailed Slipway usage recipes, command walkthroughs, JSON examples, or duplicated lifecycle mechanics. They MUST require agents to treat the current worktree's Slipway CLI as the lifecycle authority, run the flow as a black box using the latest in-worktree code, and classify any point that requires guessing as a Slipway product/usability defect to fix immediately through the self-optimization loop.

#### Scenario: Agent docs avoid command recipes
GIVEN `CLAUDE.md` and `AGENTS.md` in this repository
WHEN their Slipway guidance is inspected
THEN they contain no concrete command tutorial or JSON classification example and instead require black-box execution through the current worktree CLI surfaces, with guess-required nodes treated as defects rather than operator training.

### Requirement: Slipway self-dogfoods through the current worktree binary
REQ-014: During this change's implementation and verification, Slipway lifecycle progression MUST be exercised through the current worktree's latest code (`go run .` in this repository, or the equivalent in-worktree binary) as a black-box caller. A node that requires source-reading, digest/timestamp edits, command guessing, or undocumented sequencing to continue is a product bug that MUST be fixed in code/skills/docs before proceeding.

#### Scenario: No guessing during lifecycle execution
GIVEN this change is advanced through its governed lifecycle
WHEN `run`, `next`, `validate`, `health`, review, and closeout surfaces are used
THEN the returned public JSON/text surfaces are sufficient to decide the next action without source inspection, manual digest edits, or undocumented recovery steps.

### Requirement: Governed skills and Slipway skills match current CLI behavior
REQ-015: All governed host skills, Slipway's workflow/command skills, generated command surfaces, and repo docs MUST be regenerated or updated to match the current command registry, public JSON fields, recovery reason vocabulary, and lifecycle semantics after this change. Stale skill guidance that points to retired commands, retired recovery tokens, or old S3/S4-only behavior is a blocking drift.

#### Scenario: Generated skills align with CLI contracts
GIVEN the implementation updates recovery behavior, command availability, or public JSON fields
WHEN `go run . init --refresh --tools all` runs with the current worktree binary and the resulting project-visible diff is checked
THEN generated skills, command references, README/docs, and agent instruction files agree with the current CLI and contain no stale recovery path.
