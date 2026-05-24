# Agent Contracts

This document defines the coherent agent surface after the Phase 5
formalization work.

## Authority

- Built-in governance agent set shipped with Slipway
- Built-in default governance skill mappings shipped with Slipway
- Override surface: scope-root `.slipway.yaml` under `agents.mappings`
- Public runtime handoff: `next --json` exposes `next_skill.name`, not agent
  identity or tool-resolved prompt paths
- Bounded context handoff: `next --json` exposes project metadata and durable
  read references under `input_context`, not expanded artifact bodies
- Validation surface: `slipway health` and `slipway health --doctor`

The scope-root config is authoritative. Workspace-local `.slipway.yaml` files
inside sibling worktrees are visibility mirrors, not a second source of truth.

## Skill Layer Boundaries

Slipway deliberately separates the runtime governance skill registry from the
larger embedded template library:

- Runtime governance skills are the only skills that progression may surface
  through `next --json` as `next_skill.name`, and the only skills whose
  verification records can satisfy state-machine evidence gates.
- Exported support templates under `internal/tmpl/templates/skills/` provide
  specialist procedures for hosts. They can be invoked by users or suggested by
  command surfaces, but their existence does not add workflow states or bypass
  governance gates.
- `worktree-preflight` is an exported governance-adjacent handoff. Progression
  can surface it when a dedicated worktree must be established, but the
  corresponding authority is the worktree validation gate, not a generic skill
  verdict.

## Override Example

```yaml
agents:
  mappings:
    intake-clarification: slipway-planner
    research-orchestration: slipway-researcher
    wave-orchestration: slipway-orchestrator
    spec-compliance-review: slipway-reviewer
```

Rules:

- Keys must be known governance skills.
- Values must be governance agent names from the built-in governance agent set.
- Invalid mappings are surfaced by `health` as `agent_contract` findings.
- `next --json` does not expose internal agent identity directly; overrides
  affect the internal governance model and health validation, not a public
  exported agent path.
- Callers derive their own skill prompt path from `next_skill.name` using
  local tool conventions such as `.claude/skills/slipway-{name}/SKILL.md`.

## Runtime Handoff Context

`next --json` includes bounded handoff metadata under
`input_context.handoff_context`:

- `workflow_profile`: effective workflow shape (`code`, `docs`, `research`,
  `config`, or `meta`)
- `context_policy`: currently `bounded_references_only`
- `trace`: read-only correlation ID plus append-only lifecycle trace path
- `context_budget`: compact-mode inline budget hint
- `read_refs`: typed artifact/config/trace paths with reasons
- `policy_packs`: bounded summaries of configured advisory policy packs
- `risk`: guardrail domain, active controls, and profile-specific risk hints
- `change_authority`: the canonical `change.yaml` path
- `lifecycle_event_log`: append-only audit trace path
- `config_path`: canonical `.slipway.yaml` path
- `required_reads`: small set of durable paths the host should read before
  executing the next skill

Project metadata supplied at creation is exposed as
`input_context.project_context`. Hosts may inject it into prompts, but the
state machine does not infer missing project metadata on JSON surfaces.

Policy pack summaries are advisory-only handoff context. They may surface
project-local advisory rules, artifact requirements, recommended reviewers, and
terminology, but they do not create blocking controls and cannot weaken built-in
fail-closed guardrail domains.

## Default Governance Skill Mappings

| Governance skill | Default agent | Activation condition |
|---|---|---|
| `intake-clarification` | `slipway-planner` | active change is in `S0_INTAKE` and planning intent must be clarified |
| `research-orchestration` | `slipway-researcher` | `S1_PLAN` requires discovery research |
| `plan-audit` | `slipway-auditor` | `S1_PLAN` audit step is due |
| `wave-orchestration` | `slipway-orchestrator` | `S2_EXECUTE` requires governed wave execution |
| `tdd-governance` | `slipway-orchestrator` | `S2_EXECUTE` task is in a guardrail domain that requires TDD-governed orchestration |
| `spec-compliance-review` | `slipway-reviewer` | `S3_REVIEW` stage 1 review is due |
| `code-quality-review` | `slipway-reviewer` | `S3_REVIEW` stage 2 review is due after spec compliance passes for profiles that require code review |
| `goal-verification` | `slipway-verifier` | `S4_VERIFY` requires goal-backward verification |
| `final-closeout` | `slipway-closer` | optional closeout refresh is requested or required before ship |

Reconciliation note:

- The old `slipway-clarifier` drift is gone. `intake-clarification` now maps
  to the generated `slipway-planner` agent.

## Full Generated Agent Set

| Agent | Status | Runtime-bound | Activation condition |
|---|---|---|---|
| `slipway-analyst` | manual-only helper | yes | use for discovery-required worktree preflight and baseline verification before governed execution |
| `slipway-auditor` | governance-mapped | yes | activated by `plan-audit` |
| `slipway-closer` | governance-mapped | yes | activated by `final-closeout` |
| `slipway-debugger` | manual-only helper | no | use when failures require hypothesis-driven debugging and root-cause investigation |
| `slipway-executor` | manual-only helper | yes | spawned by `slipway-orchestrator` to execute one task at a time inside a wave |
| `slipway-mapper` | manual-only helper | no | use for architecture mapping, dependency tracing, and blast-radius discovery |
| `slipway-orchestrator` | governance-mapped | yes | activated by `wave-orchestration` and `tdd-governance` |
| `slipway-planner` | governance-mapped | no | activated by `intake-clarification` to decompose work into governed artifacts |
| `slipway-researcher` | governance-mapped | yes | activated by `research-orchestration` |
| `slipway-reviewer` | governance-mapped | yes | activated by `spec-compliance-review` and `code-quality-review` |
| `slipway-verifier` | governance-mapped | yes | activated by `goal-verification` |

## Manual-Only Helpers

Manual-only means the agent remains part of the embedded governance model but
is not selected by the governance skill registry as a direct public runtime
handoff.

- `slipway-analyst`: worktree bootstrap and baseline verification helper
- `slipway-debugger`: failure investigation helper
- `slipway-executor`: subagent implementation worker dispatched by the
  orchestrator
- `slipway-mapper`: architectural discovery helper

## Validation And Generation

- `slipway health` validates that mapped agent names exist in the built-in
  governance agent set and that exported host skill surfaces exist for active tools.
- `slipway health --doctor` includes agent-contract problems in the same
  prioritized repair/recovery view as runtime-state issues.
- `slipway init --tools ...` resolves the canonical scope root first, loads the
  authoritative config there, and mirrors the effective `.slipway.yaml` into
  the active workspace when needed.
- `slipway init --tools ...` no longer exports adapter-visible agent files;
  host skill prompts remain the only exported prompt surface.
