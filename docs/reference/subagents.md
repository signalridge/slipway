# Subagent Configuration

`subagents` is repository policy in `.slipway.yaml`. It tells Slipway which
host-facing delegation target to surface for each governance slot. Slipway still
owns lifecycle state, readiness, evidence, and blockers; the configured target
only changes how the current AI host is asked to run the delegated session.

The default provider is `native`. Configure `mcp` or `skills` only when the host
has a hub or adapter that knows how to execute that named target.

## Schema

```yaml
subagents:
  default:
    type: native
    name: default-agent
    session_instructions: Use the host's default fresh session behavior.
    timeout: 30m

  plan_audit:
    name: plan-auditor
    session_instructions: Audit only planning artifacts. Do not edit files.

  executor:
    type: mcp
    name: sliphub-executor
    session_instructions: Execute planned wave tasks and record task evidence.

  review:
    type: skills
    name: sliphub
    session_instructions: Run the selected read-only reviewers in parallel and return separate findings.
    timeout: 45m

  fix:
    name: review-repairer
    session_instructions: Collect all selected reviewer findings before editing files.

  verify:
    name: ship-verifier
    session_instructions: Verify terminal readiness without modifying files.
```

Each slot accepts the same fields:

| Field | Meaning |
| --- | --- |
| `type` | Provider family: `native`, `mcp`, or `skills`. Empty means `native`. |
| `name` | Provider-owned target name. For `native`, this is the host agent name when the host supports one. For `mcp` and `skills`, this is the hub/tool/skill entry chosen by that provider. |
| `session_instructions` | Natural-language instructions for the delegated session. This is not a provider profile and is not inherited as a model prompt. |
| `timeout` | Optional host-facing timeout hint. Slipway validates whitespace only; the host/provider decides how to interpret it. |

`mcp` and `skills` require a non-empty effective `name`. If a slot changes
`type` from the configured `default`, set `name` on that slot too; names do not
inherit across provider families.

## Slots

| Slot | JSON surface | Notes |
| --- | --- | --- |
| `default` | inherited by other slots | Shared fallback. If `subagents` is absent, Slipway emits no delegation directive. |
| `plan_audit` | `next_skill.subagent` for `plan-audit` | Plan authoring itself stays in the main session; only plan audit is delegated. |
| `executor` | `input_context.wave_plan.executor_subagent` | S2 wave execution. A provider may fan out internally, but Slipway still audits task evidence and changed files. |
| `review` | `next_skill.subagent` and `review_batch.subagent` for selected S3 reviewers | One slot covers the selected review batch. Do not configure per-reviewer provider families. |
| `fix` | `slipway fix --json` `contract.subagent` | Fresh repair session for S3 review findings. |
| `verify` | `next_skill.subagent` for `ship-verification` | Terminal read-only verification. |

There is intentionally no `plan` slot and no substep-level configuration.
Planning is high-context authoring and remains in the main session. Subagent
configuration begins where Slipway has a clear independence or dispatch boundary.

## What Is Not Configurable

Provider-specific tool permissions, model settings, and arbitrary provider
arguments are not user-facing Slipway config. Slipway and the selected provider
decide the necessary tool boundary for the current slot. If a hub needs routing
detail, put the operational intent in `session_instructions` and let that
provider interpret it.

This keeps `.slipway.yaml` stable while still allowing `mcp` and `skills`
providers to support very different internal options behind their named target.

## Config Command Examples

Set `name` before switching a slot to `mcp` or `skills`, because the config file
is validated after every `set`:

```bash
slipway config set subagents.review.name sliphub
slipway config set subagents.review.type skills
slipway config set subagents.review.session_instructions "Run selected reviewers in parallel and return separate findings."
slipway config set subagents.review.timeout 45m
```

File-authored YAML is still the clearest way to configure several slots at once.

## Regenerate Host Surfaces

Run `slipway init --refresh` after changing subagent configuration so generated
adapter surfaces and hooks match the current CLI contract.
