# Onboarding An Existing Codebase

In this tutorial you will add Slipway to an existing repository without changing
application behavior. The goal is to create durable, source-backed repo context
before you ask an AI agent to plan feature work.

## What You Will Build

You will create or refresh the codebase map under `artifacts/codebase/`, then
run one small pilot governed change that uses that map.

## Prerequisites

- A working `slipway` binary.
- An existing Git repository.
- An AI coding tool that can read files and run the Slipway CLI.

Start in the existing repo:

```bash
cd path/to/existing-repo
git status --short --branch
```

If the repo is already dirty, decide whether the dirty files belong to your
onboarding work. Preserve unrelated edits.

## Step 1: Initialize Slipway

```bash
slipway init --tools codex
```

Use the adapter IDs for your team:

```bash
slipway init --tools claude,codex,opencode
```

Inspect what was generated:

```bash
git status --short
```

## Step 2: Build The Baseline Codebase Map

```bash
slipway codebase-map --json
```

This creates durable repo-scoped context under:

```text
artifacts/codebase/
```

The generated baseline is detected facts, not final authored analysis. Inspect
the files:

```bash
find artifacts/codebase -maxdepth 1 -type f | sort
```

## Step 3: Ask The AI To Author Source-Backed Context

Paste this prompt into your AI coding tool:

```text
Use Slipway's codebase-map instructions to refine artifacts/codebase/. Preserve
real baseline facts from `slipway codebase-map`, but add only source-backed
conventions and risks. Cite file paths for every convention. Do not refactor or
edit application code during onboarding.

Start with:
- slipway instructions stack --json
- slipway instructions architecture --json
- slipway instructions testing --json
- slipway instructions concerns --json
```

Review the result like code. Remove any rule that is not tied to a current file,
test, build script, config file, or existing doc.

## Step 4: Create A Small Pilot Change

Pick the smallest useful change that can prove the map helps. Good pilots:

- Add a missing test around an existing helper.
- Update one docs page to match current commands.
- Fix a small bug with a known reproduction.
- Add a health check endpoint only if the repo's routing and test patterns are
  already clear.

Create the governed change:

```bash
slipway new "pilot change using the codebase map" --preset standard
```

Inspect the handoff:

```bash
slipway next --json --diagnostics
```

The `input_context.codebase_map_status` field tells you whether Slipway sees
the map as missing, scaffold-only, baseline, partial, or populated. If it is
baseline-only and the task depends on conventions, stop and improve the map
before planning.

## Step 5: Plan With The Map In Context

Paste this prompt:

```text
Continue the active Slipway change. During intake and planning, use
artifacts/codebase/ as advisory repo context. Do not invent conventions that are
not in the map or supported by current files. Keep the pilot small enough that
one task can verify whether the map improved planning.
```

After each handoff:

```bash
slipway validate --json
slipway next --json --diagnostics
```

If a planning skill warns that the codebase map is missing or baseline-only,
decide whether to enrich the map or narrow the task. Do not proceed by assuming
the AI remembers the repo from a previous session.

## Step 6: Execute And Review The Pilot

Let Slipway drive implementation and review:

```bash
slipway run --json --diagnostics
```

When implementation reaches a task executor, use this prompt:

```text
Execute the active Slipway task using the codebase map as context. Touch only
the target files declared in tasks.md. Run the task's verification command. If
the map contradicts current source, stop and report the discrepancy instead of
guessing.
```

After implementation:

```bash
git diff --stat
slipway validate --json
slipway next --json --diagnostics
```

Review findings should be repaired through `slipway fix --json`, not by mixing
review and repair in the same context.

## Step 7: Promote Useful Learnings

If the pilot revealed a durable convention, update the matching
`artifacts/codebase/` file with source-backed wording. Keep it narrow:

- Good: "HTTP route tests use `httptest.NewRecorder` in `internal/http/*_test.go`."
- Bad: "Always write comprehensive tests."

Run a final read-only check:

```bash
slipway validate --json
```

When done-ready:

```bash
slipway done --json
```

Commit the pilot diff and archived governed record together.

## What You Learned

- `slipway codebase-map` creates durable brownfield context.
- `slipway instructions <codebase-map-doc>` is the authoring contract for map
  refinement.
- Baseline context is useful, but authored source-backed context is stronger.
- Planning should cite current code, not assumed conventions.
- A small pilot reveals whether the map is useful before a team-wide rollout.

## Related

- [Real-World Scenarios](../real-world-scenarios.md)
- [Recover and troubleshoot](../how-to/recover-and-troubleshoot.md)
- [Design](../explanation/design.md)
