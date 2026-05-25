# Workflow Feedback

## 2026-05-25T06:43:34Z

- Symptom: after `run --json --diagnostics` advanced the change to `S0_INTAKE/research`, both fresh `next --json --diagnostics` and the run response still returned `next_skill.name=intake-clarification`, while `required_actions` said `complete research.md`.
- Finding: current implementation intentionally maps `S0_INTAKE/research` to `intake-clarification`; true `research-orchestration` is selected only at `S1_PLAN/research`.
- Friction: the operator-facing `required_actions` wording implies a `research.md` artifact before S1 planning, but no `research.md` exists in the bundle at S0 and the current host remains intake clarification.
- Suggested fix: distinguish S0 intake research wording from S1 plan research wording, or rename the S0 substep/action so agents do not infer a missing `research-orchestration` handoff.

## 2026-05-25T06:46:53Z

- Symptom: `slipway codebase-map` reported all seven codebase documents as created, but the generated files contained only headings/placeholders.
- Finding: the command is useful as a path manifest, but it did not provide the grounded brownfield context expected by `research-orchestration`.
- Friction: `next` stopped warning about missing codebase-map documents after generation even though the docs did not contain substantive research context.
- Suggested fix: either populate codebase-map with real repository facts or mark placeholder-only maps as incomplete/stale so research hosts do not over-trust them.

## 2026-05-25T06:48:21Z

- Symptom: `validate --json` reported `research_structure_invalid` because `research.md` lacked top-level `## Unknowns`, even though `research-orchestration` asks agents to present `### Unknowns Resolved` under `## Research Findings`.
- Finding: artifact schema requires `## Alternatives Considered`, `## Unknowns`, `## Assumptions`, and `## Canonical References`.
- Friction: host instructions and artifact schema are not aligned, causing a predictable validation failure after following the host output format.
- Suggested fix: update `research-orchestration` to include the artifact schema headings directly, or relax schema validation to accept the host's generated research summary structure.

## 2026-05-25T06:41:02Z

- Symptom: malformed verification evidence with structured `references` maps caused `validate` and `run` to fail parsing.
- Finding: `model.VerificationRecord.References` is `[]string`, and generated examples use string references.
- Friction: the runtime error was correct, but the schema is easy to misread when evidence needs rich citations.
- Suggested fix: keep the verification evidence schema prominently documented as string-only references, or introduce a typed reference schema deliberately.

## 2026-05-25T07:02:04Z

- Symptom: `validate --json` reported `tasks_checklist_invalid_format` after adding task metadata fields `evidence` and `acceptance` to the plan.
- Finding: `internal/engine/wave/parse.go` only accepts `wave`, `depends_on`, `target_files`, `task_kind`, `covers`, and `checkpoint_type` metadata keys under task checkboxes.
- Friction: `slipway-plan-audit` asks tasks to name their evidence shape, but the strict task parser rejects a natural `evidence:` metadata field. Agents need to know this hidden whitelist before authoring audit-friendly tasks.
- Suggested fix: either add supported metadata keys such as `evidence` and `acceptance` to the task contract, or update plan-audit guidance to show the exact parser-compatible way to record acceptance detail.

## 2026-05-25T07:02:50Z

- Symptom: after `S1_PLAN/research` advanced to `S1_PLAN/bundle`, `validate --json` clearly required plan-audit evidence, but `next --json --diagnostics` returned `next_skill: null` and a `no_skill_required: S1_PLAN` blocker.
- Finding: the bundle phase appears to be a hostless authoring phase even though the next operationally necessary governed host is `slipway-plan-audit`.
- Friction: an agent following only `next_skill.name` has no host path to load, while status/gate diagnostics still require a specific skill evidence file.
- Suggested fix: return `next_skill.name=plan-audit` when `G_plan` is blocked solely by missing/failed plan-audit evidence, or add a bundle-phase action contract that says to author bundle artifacts and then run plan-audit.

## 2026-05-25T07:16:43Z

- Symptom: after creating a dedicated worktree, passing `go test ./... -count=1`, and writing parser-compatible `verification/worktree-preflight.yaml`, `validate`, `next`, and `run` still reported `Dedicated worktree metadata is missing`.
- Finding: `advance_governed.go` calls `governance.RequiredActionBlockers(...)` before the S2 worktree-preflight evidence path has a chance to call `DeriveWorktreeBlockers` and `ApplyWorktreeMetadata`. The required-action blocker therefore deadlocks the preflight path.
- Friction: the worktree-preflight skill tells the agent to write evidence and then advance, but the runtime blocks before consuming that evidence.
- Suggested fix: in S2 execution with an unbound discovery worktree, consume and validate passing `worktree-preflight` evidence before evaluating required-action blockers, then persist worktree metadata and relocate the governed bundle.

## 2026-05-25T07:38:10Z

- Symptom: after the bundle was relocated into the dedicated worktree, `next --json --diagnostics` selected `next_skill.name=wave-orchestration` but also reported `required_skill_missing: wave-orchestration`.
- Finding: the source repo has `.codex/skills/slipway-wave-orchestration/SKILL.md`, but the generated worktree did not contain that skill directory.
- Friction: the authoritative execution context became the worktree, while the host skill path remained discoverable only from the source checkout. Agents need either an explicit source-skill fallback or a copied skill catalog in the bound worktree.
- Suggested fix: when binding a worktree, either copy/export governed host skills into the worktree or include the canonical skill source path in `next --json` diagnostics.

## 2026-05-25T07:42:29Z

- Symptom: a parallel `status --json --diagnostics` and `next --json --diagnostics` request caused one command to return `state_lock_timeout`.
- Finding: both commands are read-only from the user's perspective, but they still contend on the same advisory state lock.
- Friction: agents are encouraged to parallelize independent reads, and `status`/`next` look like read-only candidates. Lock contention is surprising and easy to misdiagnose as corruption.
- Suggested fix: document state-locking commands as non-parallelizable, or allow read-only status/next projections to share a read lock when no mutation is pending.

## 2026-05-25T08:02:18Z

- Symptom: `go test ./... -count=1` timed out in `cmd` after 10 minutes when run alongside lint/config checks, without surfacing a functional assertion failure.
- Finding: the full `cmd` package was already close to the Go default 10-minute package timeout during the clean worktree baseline, so normal CI load can make the default timeout flaky.
- Friction: the governed workflow asks for full-suite proof, but the repo's slow package makes the default Go timeout a test-environment hazard rather than a product signal.
- Suggested fix: either split/parallelize the slow `cmd` tests, or make repo-documented full-suite commands use an explicit timeout that reflects current suite runtime.

## 2026-05-25T08:22:04Z

- Symptom: the approved execution task `t-08` originally required "required review evidence" even though governed review skills run after S2 execution is complete.
- Finding: putting S3/S4 review evidence into an S2 execution task creates a circular dependency: wave execution cannot finish until review evidence exists, but review cannot start until execution finishes.
- Friction: task plans need a clear boundary between execution-readiness evidence and downstream governed review/closeout evidence.
- Suggested fix: plan-audit should flag task acceptance criteria that require future lifecycle-state evidence before the workflow can legally reach that state.

## 2026-05-25T09:19:38Z

- Symptom: for this discovery-required governed change, `artifacts/changes/<slug>` was created and plan-audited before the dedicated worktree was bound.
- Finding: active bundle path resolution falls back to the project root until `worktree_path` is persisted, while the worktree-preflight handoff currently happens at S2 execution. The runtime then relocates the already-created governed bundle into the worktree after preflight evidence is consumed.
- Friction: the operator experiences two authoritative-looking artifact locations: root-scoped planning artifacts first, then worktree-scoped execution artifacts later. This makes it unclear which path should be reviewed, edited, and cited.
- Suggested fix: for changes that require worktree isolation, bind or create the dedicated worktree before S1 bundle artifact creation. Keep only minimal root-level runtime/index state before worktree binding, then scaffold the governed bundle directly in the canonical worktree path.
- Additional fix detail: after binding the worktree, resolve the worktree-local canonical `artifacts/changes/<slug>` path first. If it does not exist, create it; if it exists and is empty, initialize it; if it exists and is non-empty, validate that it belongs to the same change or fail/repair explicitly. Never blindly overwrite or relocate into a non-empty artifact directory.

- Symptom: the dedicated worktree was created as a sibling path (`/Users/yixianlu/ghq/github.com/signalridge/slipway-reference-clinvoker-cicd`) instead of under the source repo's ignored `.worktrees/` directory.
- Finding: the worktree-preflight skill requires a path different from the primary repo and registered in `git worktree list`, but it does not prescribe a default location. The repository already ignores `.worktrees/`, which is a stronger local convention than a sibling checkout.
- Friction: sibling worktrees make the relationship between source checkout, branch, artifacts, and archive harder to discover from the repository itself.
- Suggested fix: default generated worktree paths to `<repo>/.worktrees/<slug>` (or another explicitly configured repo-local worktree root), while still allowing an operator-supplied override for sibling or external worktree layouts.

- Symptom: `artifacts/codebase/*.md` existed and was referenced by plan-audit, but the files contained only scaffold-level placeholders.
- Finding: the codebase-map directory is useful as a durable brownfield context location only when populated with substantive repository facts. Placeholder-only files should not satisfy context or planning evidence expectations.
- Friction: agents and audits can over-trust generated files because they exist on disk, even though the actual context came from direct source inspection and `research.md`.
- Suggested fix: distinguish populated, stale, missing, and scaffold-only codebase-map states. Treat scaffold-only maps as incomplete/advisory, and do not count them as evidence for plan-audit readiness.

- Symptom: execute did have a plan-audit gate before S2, but the review happened before worktree binding and accepted placeholder codebase-map references.
- Finding: the narrow lifecycle order was correct (`plan-audit` before execute), but the audited artifact set was not yet anchored to the final worktree workspace, and part of the cited context was weak.
- Friction: "plan reviewed before execute" is technically true but not strong enough to prove the executable worktree bundle was the reviewed bundle.
- Suggested fix: run plan-audit after the worktree-local governed bundle is canonical, and make the audit fail or warn strongly when cited context artifacts are placeholder-only.

- Symptom: after execute/review edits touched governed artifacts, the workflow rerouted through S1 research/bundle/audit before returning to execution/review.
- Finding: broad stale-evidence invalidation is safe, but it is too coarse for common post-execution edits. Not every `change.yaml` or artifact freshness change requires full research re-entry.
- Friction: review-driven fixes and metadata corrections become unnecessarily expensive because the recovery path does not distinguish plan input changes, implementation changes, review evidence changes, and final closeout wording changes.
- Suggested fix: add targeted recheck routing. Scope/requirement/task changes should re-run affected planning gates; implementation changes should re-run affected wave evidence and review; review-only or assurance-only edits should run freshness/closeout checks without restarting research unless they alter lifecycle authority or accepted scope.

- Symptom: `.codex/skills/slipway/references/catalog` contains catalog artifacts such as `context-assembly.md` and `security-review.md` that duplicate the full instructions from dedicated skills such as `.codex/skills/slipway-context-assembly/SKILL.md` and `.codex/skills/slipway-security-review/SKILL.md`.
- Finding: the root `slipway` skill says it is a CLI entry/router and must not replace CLI progression authority, but its catalog tree currently stores full procedure/checklist copies plus catalog metadata. This is more than a thin dispatch index.
- Friction: duplicated skill content creates drift risk and makes it unclear whether the root `slipway` skill, the catalog artifact, or the dedicated `slipway-*` skill is the authoritative instruction source.
- Suggested fix: keep the root `slipway` skill thin. Either generate catalog metadata from the dedicated skill frontmatter, or move catalog routing to a separate generated manifest. Avoid storing full duplicated skill instructions under `.codex/skills/slipway/references/catalog` unless there is a deliberate packaging/export reason and a drift check.

- Symptom: after `slipway done`, the archived governed bundle moved back to the project root at `artifacts/changes/archived/<slug>` instead of staying in the dedicated worktree where the active bundle lived.
- Finding: active governed bundle paths resolve through the workspace root, but archive paths currently resolve through the project root. The archived `change.yaml` also retains frozen artifact paths that point at the former worktree active bundle location.
- Friction: the final audit package is no longer colocated with the worktree/branch where the governed work happened, and its metadata points back to paths that may no longer contain the archived files.
- Suggested fix: make archive placement consistent with the canonical workspace. Prefer worktree-local archives for worktree-bound changes plus a project-root index/pointer for global discovery, or keep project-root archives only if archived artifact paths are rewritten to archive-local paths and `done` clearly reports the relocation.
