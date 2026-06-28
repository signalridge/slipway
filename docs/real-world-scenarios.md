# Real-World Scenarios

Use this page to pick the right Slipway path for the work in front of you. Each
scenario keeps one rule: move forward through current-worktree evidence, not
through private memory or manual state edits.

## Scenario Map

| Scenario | Use when | Main Slipway value |
| --- | --- | --- |
| 1. First governed change | You want to learn the lifecycle on a small safe edit. | See the full evidence loop once. |
| 2. Adopt an existing project | The repo already has conventions and risk areas. | Make real codebase context durable before planning. |
| 3. Ship a product feature | Work touches code, tests, docs, and review. | Keep scope, tasks, evidence, and review aligned. |
| 4. Repair review findings | S3 review found actionable issues. | Consolidate fixes through fresh-context repair. |
| 5. Recover a stale or stuck change | Evidence, tasks, or artifacts drifted. | Fail closed with named recovery commands. |
| 6. Roll out adapters to a team | Multiple AI tools need the same Slipway surface. | Generate host files from one CLI authority. |

## 1. First Governed Change

Use this when you want a low-risk way to learn the lifecycle.

Starting prompt for an AI coding tool:

```text
Use Slipway for one small docs-only change. Keep the scope to README.md,
inspect status and next before each mutating command, and stop if Slipway
reports stale evidence or out-of-scope files.
```

Workflow:

1. Initialize adapters with `slipway init --tools <tool-id>`.
2. Create the change with `slipway new "add a short README usage note" --profile docs`.
3. Use `slipway next --json --diagnostics` to see the current handoff.
4. Let the returned skill author the required artifact or implementation step.
5. Run `slipway validate` after implementation.
6. Run `slipway done` only after the state is done-ready.

Done means:

- The intended file changed.
- The artifact bundle explains why.
- Current validation accepts the evidence.
- The archived record exists after `done`.

## 2. Adopt An Existing Project

Use this when the codebase already has real behavior, but conventions live in
source patterns, old PRs, scattered docs, or reviewer memory.

Starting prompt:

```text
This is an existing repo. Do not refactor yet. Use Slipway to create or refresh
the codebase map, then identify the smallest governed change that would prove
the map is useful. Cite files for every convention you record.
```

Workflow:

1. Run `slipway init --tools <tool-id>`.
2. Run `slipway codebase-map --json`.
3. Author or refine `artifacts/codebase/` docs using `slipway instructions stack`,
   `slipway instructions architecture`, `slipway instructions testing`, and the
   other codebase-map instruction subjects.
4. Create one small governed pilot change.
5. During planning, check that `next --json` reports the map status in
   `input_context.codebase_map_status`.
6. Review the pilot outcome and update the map only with source-backed findings.

Guardrails:

- Record only conventions supported by current files.
- Delete speculative rules that cannot be traced to code or docs.
- Treat a baseline-only map as advisory until it has authored substance.
- Do not make broad cleanup part of the onboarding task.

Done means:

- `artifacts/codebase/` contains reviewed context.
- The first governed pilot used that context.
- The map did not become a dumping ground for guesses.

## 3. Ship A Product Feature

Use this when the work has implementation, tests, documentation, and review
requirements.

Starting prompt:

```text
Use Slipway for this feature. First clarify scope and acceptance criteria. Keep
target files explicit in tasks.md, run targeted tests for each task, and treat
review findings as a separate S3 repair batch.
```

Workflow:

1. Create the change with `slipway new "<feature>" --preset standard`.
2. Let intake and planning produce real `intent.md`, `requirements.md`,
   `decision.md`, `research.md`, and `tasks.md`.
3. Confirm every task has concrete `target_files`.
4. Execute through `slipway implement --json` or `slipway run --json`.
5. Record task evidence through the generated wave execution path.
6. Run S3 review and close only when selected reviewers are fresh.

Done means:

- Requirements map to implementation and tests.
- Task evidence matches the current run version.
- Selected review and closeout evidence pass.
- `done` archives the change without hiding dirty work.

## 4. Repair Review Findings

Use this when S3 review reports actionable issues.

Starting prompt:

```text
Use Slipway fix for the selected review findings. First consolidate confirmed
findings by root cause. Make one repair pass, rerun the affected reviewers, and
do not repair findings inline while review is still reporting.
```

Workflow:

1. Inspect `slipway review --json` or `slipway next --json --diagnostics`.
2. Run `slipway fix --json`.
3. Send the returned repair contract to a fresh-context repair agent.
4. Rerun the affected selected reviewers.
5. Continue review only after fix and review context-origin evidence is fresh.

Done means:

- The fix addresses the selected findings by root cause.
- Review evidence was refreshed after the repair.
- No stale selected reviewer is silently ignored.

## 5. Recover A Stale Or Stuck Change

Use this when `next`, `status`, or `validate` reports stale evidence, missing
task proof, scope drift, or inconsistent local state.

Starting prompt:

```text
Diagnose this Slipway blocker without editing state by hand. Run status,
validate, next with diagnostics, and health doctor. Follow only the named safe
recovery command or explain why none applies.
```

Workflow:

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
slipway health --doctor --json
```

If health names a bounded local repair, run:

```bash
slipway repair --json
```

If a stage or reviewer is stale, rerun that owning stage or reviewer. If an
artifact is missing substance, run `slipway instructions <artifact>` and author
the real artifact.

Done means:

- The original blocker is gone for the current worktree.
- Freshness was regenerated by the owning command or skill.
- The recovery did not forge timestamps, verdicts, or lifecycle state.

## 6. Roll Out Adapters To A Team

Use this when multiple people or tools need the same command and skill surface.

Starting prompt:

```text
Refresh Slipway adapters for the tools this repo actually uses. Preserve
user-owned files near the generated directories, inspect the diff, and do not
make generated host files authoritative over the CLI.
```

Workflow:

```bash
slipway init --tools claude,codex,opencode
slipway init --refresh
```

Use `--tools all --refresh` only when the repo intentionally supports every
adapter Slipway generates.

Done means:

- `.slipway.yaml` reflects repo defaults.
- Generated adapter files match the current CLI.
- User-owned host config was preserved.
- The team knows to use `slipway next`, `status`, and `validate` for authority.
