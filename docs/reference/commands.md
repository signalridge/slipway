# Command Reference

This page is the Diataxis reference entry for Slipway commands. The detailed
legacy reference remains available at [Commands](../commands.md); this wrapper
keeps the generated surface manifest anchored under `docs/reference/`.

Most routed commands support `--json` when structured output is useful.
`slipway validate` emits JSON for its main report, and `slipway init` is
setup-only.

## Command Index

| Command | Class | Use |
| --- | --- | --- |
| `slipway new` | mutation | Create a governed change. |
| `slipway intake` | mutation | Run intake clarification and authorization. |
| `slipway plan` | mutation | Author or amend planning artifacts. |
| `slipway implement` | mutation | Run S2 implementation wave orchestration. |
| `slipway review` | mutation | Run S3 review convergence. |
| `slipway fix` | mutation | Dispatch repairs for S3 findings. |
| `slipway done` | mutation | Archive a done-ready change. |
| `slipway next` | query | Inspect the next skill or blocker without advancing. |
| `slipway run` | mutation | Drive the current stage until a stop condition. |
| `slipway status` | query | Show lifecycle state and next actions. |
| `slipway codebase-map` | mutation | Create or refresh durable repo-scoped context. |
| `slipway handoff` | mutation | Write or show per-change advisory continuation notes. |
| `slipway preset` | mutation | Confirm or change the active preset. |
| `slipway validate` | query | Recompute readiness without advancing. |
| `slipway abort` | mutation | Abort the active execution session. |
| `slipway cancel` | mutation | Cancel and archive an active change. |
| `slipway delete` | mutation | Discard abandoned governed local state. |
| `slipway repair` | mutation | Run bounded local integrity repairs. |
| `slipway evidence` | mutation | Record supported task or skill evidence. |
| `slipway tool` | mutation | Run CLI-only helper tools used by generated skills. |
| `slipway health` | query | Show repo-local integrity findings. |
| `slipway instructions` | query | Show artifact or codebase-map authoring contracts. |
| `slipway init` | mutation | Initialize runtime layout and optional adapters. |

## JSON Surface Tokens

These examples are kept literal because the generated surface manifest checks
that every JSON contract remains findable in the docs.

```bash
slipway new --json
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway next --json
slipway run --json
slipway status --json
slipway handoff show --json
slipway validate --json
slipway done --json
slipway codebase-map --json
slipway preset <level> --json
slipway abort --json
slipway cancel --json
slipway delete --change <slug> --json
slipway repair --json
slipway evidence task --result-file task-result.json [--result-file next-task-result.json ...] --json
slipway evidence skill --skill <name> --verdict pass --json
slipway health --json
slipway instructions <artifact> --json
```

Use `--diagnostics` with `next` or `run` when you need blocker detail,
artifact-readiness detail, transition traces, or context-budget diagnostics.

## Read-Only Authority

These commands inspect state without changing lifecycle authority:

```bash
slipway status --json
slipway validate --json
slipway next --json --diagnostics
```

Use them before choosing a mutation command.

## Mutating Stage Commands

These commands can advance or change governed state:

```bash
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway done --json
slipway run --json --diagnostics
```

If a mutation fails closed, rerun the current read-only checks and follow the
named recovery command.

Config-level `execution.auto` applies to `intake`, `plan`, and `implement`.
Those stage commands do not accept per-invocation `--auto` or `--no-auto`
override flags; use `slipway run --auto` or `slipway run --no-auto` when one-run
override behavior is needed.

## Run Auto Mode

`slipway run` can auto-advance pure-pacing pauses so a governed change keeps
moving without a fresh human stop at every routine handoff. Enable it per repo
with the `execution.auto` config, or override it for a single invocation:

```bash
slipway run --auto --json
slipway run --no-auto --json
```

`--auto` and `--no-auto` take precedence over the `execution.auto` config for
that one run. Under auto, Slipway auto-advances pure-pacing pauses (review
batches without `security-review`, non-sensitive/non-security-review skill
handoffs) on prior authorization and auto-confirms a pending workflow-preset
upgrade-only (never downgraded). `security-review` boundaries, sensitive and
guardrail confirmations, the intake Approved Summary, and every evidence gate
still hard-stop and are never auto-advanced.

## Surface Manifest

`docs/SURFACE-MANIFEST.json` is regenerated from Slipway-owned Go authorities:

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

When adding or changing a command, JSON output contract, or docs-facing surface,
keep its token present in the manifest row's `docs` file.

## Full Detail

The detailed command reference remains in [Commands](../commands.md), including
creation options, discovery commands, diagnostics, output flags, and common JSON
invocations.
