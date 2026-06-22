# First Governed Change

In this tutorial you will run a small docs-only change through Slipway. The goal
is not the README edit; the goal is to see how Slipway exposes lifecycle state,
skill handoffs, evidence, review, and fail-closed recovery.

## What You Will Build

You will add one short "Usage" section to a disposable README.

## Prerequisites

- A working `slipway` binary.
- Git installed.
- An AI coding tool that can read the generated Slipway skill or command
  surface, such as Codex, Claude, Cursor, Gemini, or OpenCode.

If you are working from a Slipway source checkout instead of an installed
binary, replace `slipway` with `go run .` from that checkout.

## Step 1: Create A Tutorial Repo

Use a disposable directory:

```bash
mkdir slipway-first-change
cd slipway-first-change
git init
printf '# Slipway First Change\n\nA tiny repo for trying Slipway.\n' > README.md
git add README.md
git commit -m "chore: initial readme"
```

## Step 2: Initialize Slipway

Pick the adapter for your AI tool:

```bash
slipway init --tools codex
```

Other examples:

```bash
slipway init --tools claude
slipway init --tools opencode
slipway init --tools all
```

Check the repo state:

```bash
git status --short
```

Commit `.slipway.yaml` and generated adapters only if your team wants them
tracked. For this tutorial, it is fine to leave the generated files unstaged
until you inspect them.

## Step 3: Create A Governed Change

```bash
slipway new "add a small README usage note" --profile docs --preset standard
```

Inspect the active change:

```bash
slipway status --json
slipway next --json --diagnostics
```

Read the JSON as the current authority. It will name the next skill, blocker, or
command to run. Do not infer the next stage from memory.

## Step 4: Let The AI Author Intake

Paste this into your AI coding tool from the tutorial repo:

```text
Use the active Slipway change. Inspect `slipway next --json --diagnostics`.
Complete only the surfaced intake or artifact-authoring handoff. The objective
is to add one README Usage section later; do not edit README.md during intake.
Do not edit change.yaml, lifecycle events, verification records, or runtime
evidence by hand.
```

When the AI reports the handoff is complete, inspect again:

```bash
slipway status --json
slipway next --json --diagnostics
```

If Slipway reports a missing artifact, run the command it names. For example:

```bash
slipway instructions requirements --json
```

The instructions command gives the authoring contract. The AI must write the
real artifact content; copying the placeholder template is intentionally
rejected by the gates.

## Step 5: Run Planning

Advance through planning using the CLI surface:

```bash
slipway run --json --diagnostics
```

If this returns another skill handoff, paste this prompt:

```text
Continue the active Slipway change from the current `slipway next --json`
handoff. Author only the required planning artifact. Keep the eventual
implementation scoped to README.md. If the task plan needs target files, use
README.md only.
```

Repeat the read-only inspection after each handoff:

```bash
slipway validate --json
slipway next --json --diagnostics
```

Planning is ready for implementation only after plan-audit gates pass. If
Slipway fails closed, follow the named artifact or review recovery. Do not skip
the planning gate.

## Step 6: Implement The README Change

When Slipway reaches implementation, paste this prompt:

```text
Execute the active Slipway implementation handoff. Change only README.md. Add a
short Usage section with a command example that tells readers to run
`slipway status --json` before relying on lifecycle state. Run any targeted
verification command named by the task. Record task evidence only through the
Slipway command or generated execution skill that owns task evidence.
```

The intended README shape is small:

````markdown
## Usage

Inspect the current governed state before acting:

```bash
slipway status --json
```
````

After the AI finishes, inspect the diff:

```bash
git diff -- README.md
slipway validate --json
slipway next --json --diagnostics
```

If validation reports `scope_contract_drift`, the change touched files outside
the task's `target_files`. Repair the scope or amend the plan through the
surfaced Slipway path; do not hide the file in evidence.

## Step 7: Review And Close

Let Slipway run review:

```bash
slipway run --json --diagnostics
```

If selected review evidence is missing or stale, rerun the selected reviewer
named by `next --json --diagnostics`. If review finds issues, use:

```bash
slipway fix --json
```

Send the returned repair contract to a fresh AI context. After repair, rerun the
affected reviewers.

When the state reports done-ready:

```bash
slipway done --json
```

Then inspect what changed:

```bash
git status --short
find artifacts/changes -maxdepth 3 -type f | sort
```

Commit the README and archived Slipway record together if this was real work.

## What You Learned

- `status`, `next`, and `validate` are read-only authority checks.
- `run` advances only until a skill, blocker, or done-ready state.
- Artifacts are authored from `slipway instructions`, not copied from templates.
- Implementation scope comes from `tasks.md` target files.
- Stale evidence is repaired by rerunning the owning stage or reviewer.
- `done` archives the change only after governed readiness.

## Related

- [Start Here](../start-here.md)
- [Commands](../reference/commands.md)
- [Workflow](../explanation/workflow.md)
