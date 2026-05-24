# Agent Contracts

This document defines the coherent agent surface after the Phase 5
formalization work.

## Authority

- Built-in governance agent set shipped with Slipway
- Built-in default governance skill mappings shipped with Slipway
- Override surface: scope-root `.slipway.yaml` under `agents.mappings`
- Public runtime handoff: `next --json` exposes `next_skill.name`, not agent
  identity or tool-resolved prompt paths
- Default context handoff: `next --json` exposes only the paths and state
  needed to continue the next governed host; diagnostic governance detail
  belongs to `next --json --diagnostics`, `validate`, `status`, or `health`
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

`next --json` and `run --json` are handoff-only surfaces. Their default JSON
answers:

- which change is active: `slug`, `phase`, `current_state`, and lifecycle mode
- which governed host owns the next step: `next_skill.name`
- where evidence belongs: `next_skill.verification_dir`
- short action metadata for that host: `next_skill.skill_constraints` and
  `next_skill.technique_hints`
- which durable paths may be read: `input_context.workspace_root`,
  `input_context.artifact_bundle`, `input_context.codebase_map_dir`, and
  `input_context.codebase_map_docs`
- whether execution is blocked or requires confirmation:
  `blockers`, `warnings`, and `confirmation_required`

Default JSON is an action/status contract, not a context dump. The default
handoff deliberately does not include governance forecasts, gate status,
artifact DAG state, active controls, skill-evidence diagnostics, review-layer
matrices, policy-pack summaries, read-reference shelves, or context-budget
breakdowns. Those fields are diagnostic, not routing contract.

Default handoff fields are classified as:

```text
always_inline:
  slug
  phase
  current_state
  lifecycle_status
  next_skill.name
  next_skill.verification_dir
  blockers
  warnings
  confirmation_required
  input_context.workspace_root
  input_context.artifact_bundle

conditional_inline:
  next_skill.skill_constraints
  next_skill.technique_hints
  input_context.resume_checkpoint
  context_budget only when guard_action is warn/stop
  governance_summary only when status has governance blockers/actions

reference_only:
  change_authority
  artifact_bundle
  codebase_map_dir
  codebase_map_docs
  policy_pack paths
  lifecycle_event_log

diagnostic_only:
  governance_signals
  active_controls
  required_actions
  skill_evidence
  artifact_status
  gate_status
  full context_budget breakdown
  handoff_context.policy_packs prose
  advisory_rules
  artifact_requirements
  recommended_reviewers
  terminology
```

Context budget is self-regulating on default handoffs. When the budget guard is
`ok`, no `context_budget` field is emitted. When it is `warn` or `stop`, the
default JSON emits only `guard_action` and `remaining_percent` plus the warning
needed to recover. Full token estimates, thresholds, and breakdowns remain
diagnostic-only.

Use `slipway next --json --diagnostics` or `slipway run --json --diagnostics`
when a caller explicitly needs the full readiness/governance view. The
diagnostic surface may include `input_context.handoff_context`,
`input_context.gate_status`, `input_context.artifact_status`,
`input_context.wave_plan`, `governance_signals`, `active_controls`,
`skill_evidence`, `artifact_amendments`, and `context_budget`.

Project metadata supplied at creation is part of the diagnostic handoff
context. Hosts may inject it into prompts, but the state machine does not infer
missing project metadata on JSON surfaces.

Policy pack summaries are advisory-only diagnostic context. They may surface
project-local advisory rules, artifact requirements, recommended reviewers, and
terminology, but they do not create blocking controls and cannot weaken built-in
fail-closed guardrail domains.

`status --json` is also a default status contract. It reports lifecycle state,
blockers, progress, and next actions, but it does not inline the raw governance
diagnostic triplet `governance_signals`, `active_controls`, and
`required_actions`. When governance controls create blockers or required
actions, status emits a compact `governance_summary` with blocked controls,
required action text, and authority references. Complete governance diagnostics
belong to `slipway health --governance --json --change <slug>`.

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
