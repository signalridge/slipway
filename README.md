# Slipway

Slipway is a governance-first CLI for routing work into intake, governed execution, and evidence-backed closeout inside a local repository.

## Command surface

Slipway is organized around an intake-first lifecycle plus a smaller set of
setup, diagnostics, and repair commands.

### Core lifecycle

- `slipway new [description]`: create a governed change starting at intake (S0_INTAKE)
- `slipway preset <light|standard|strict>`: confirm or update the governance preset for the active change
- `slipway next`: query the next required skill, blockers, and current action without advancing state
- `slipway run`: advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced
- `slipway status`: inspect lifecycle state, blockers, and next actions
- `slipway done`: finalize a done-ready change and archive it

If `slipway new` is run without a description in a TTY, Slipway opens an
interactive intake prompt with inferred project context.

Unless JSON stdin supplies explicit `guardrail_domain`, `needs_discovery`, or
`complexity` metadata, `slipway new` starts from conservative safe-degrade
classification defaults and reports that degradation in command output.

Creation refinements:

- `slipway new --preset <light|standard|strict>`: choose the governance preset during creation
- `slipway new --profile <code|docs|research|config|meta>`: choose the workflow shape separately from preset strictness
- `slipway new --from-doc <path>`: seed governed work from an existing document
- `slipway new --discuss`: persist unresolved gray areas into context before execution
- `slipway new --full`: require refreshed final-closeout evidence before ship
- `slipway new --trivial`: force complexity=trivial for lightweight changes

Workflow profile is not a second preset. Presets decide how strict the gates
are; profile decides which workflow-specific checks apply. `code` is the
default. `docs` and `research` still require spec-compliance and goal
verification, but they do not require the code-quality-review stage. `config`
and `meta` keep code-quality review active and default to expanded artifacts
because they usually affect rollback, generated surfaces, or governance schema.

## Governed progression

After governed work exists, use either the query surface (`next`) or the
execution surface (`run`) to manage the lifecycle.

- `slipway next`: inspect the next required skill, blockers, and current action without advancing state
  - Use this when you want step-by-step control before deciding whether to advance
- `slipway run`: validate current evidence and keep advancing until Slipway hits an operator-facing stop condition
  - Stop conditions are: next skill surfaced, blockers surfaced, checkpoint surfaced, or done-ready reached
  - Use `--resume` to continue resumable non-checkpoint execution from the latest incomplete wave
  - Use `--resume-response <value>` to resume from an active checkpoint
  - Use this when you want Slipway to keep moving until explicit operator input is needed
- `slipway status`: inspect governed status, blockers, artifact readiness, and next actions
- `slipway done`: finalize a done-ready change and archive it

### Situational commands

- `slipway review`: explicit bidirectional artifact-code alignment review (available from S2_EXECUTE onward); not part of every progression cycle
- `slipway validate`: read-only evidence and gate check for the active change; governed JSON output also includes a `requirements_contract` summary when the bundle can be evaluated cleanly
- `slipway pivot`: reroute (`--reroute`) or rescope (`--rescope`) an active change (available from S2_EXECUTE onward)
- `slipway abort`: abort only the active execution session without archiving the change
- `slipway cancel`: cancel the active change and archive it as terminal
- `slipway checkpoint`: pause wave execution for user input by task id (available at S2_EXECUTE)
- `slipway repair`: run bounded local integrity repairs (stale locks, interrupted archives, corrupt config)

### Diagnostics and observability

- `slipway learn --preview`: aggregate lifecycle evidence into read-only governance improvement proposals
- `slipway stats`: **repo-wide** governance freshness and workflow statistics
- `slipway health`: **repo-local** integrity and repairability findings, with optional governance diagnostics
  - Use `--doctor` to synthesize a prioritized repair/recovery plan without mutating state
- `slipway codebase-map`: create or refresh the durable repo-scoped brownfield map (`artifacts/codebase/`)
- `slipway init`: initialize runtime layout and optional tool artifacts for the current project

## Build and run

```bash
go run . --help
go build ./...
```

Initialize a workspace with tool adapters for your AI runtime:

```bash
slipway init --tools claude          # single tool
slipway init --tools claude,cursor   # multiple tools
slipway init --tools all             # all supported tools
slipway init --tools none            # workspace only, no adapters
```

Omitting `--tools` creates only the tracked config
`.slipway.yaml`, with no adapter generation. Supported tools: `claude`,
`codex`, `cursor`, `gemini`, `opencode`. Use `--refresh` to regenerate
deterministically; if `--tools` is omitted during refresh, previously generated
adapters are auto-detected and refreshed.

Hook-capable adapters register a session-start check, and Claude/Gemini also
register an advisory post-tool context monitor. These generated surfaces are
contract-tested locally so trigger wording, command coverage, and hook
registration do not drift silently.

## Typical workflow

Choose one of the two progression styles below. `review`, `validate`, and
`status` are situational surfaces, not fixed steps in every run.

### Stepwise progression with `next`

```bash
slipway init --tools codex
slipway new "refresh governance docs" --preset standard
slipway next --json        # inspect the current next step (read-only)
# execute the surfaced skill
slipway run                # validate evidence and advance once the skill is complete
slipway next --json        # inspect the newly surfaced next step
slipway done
```

### Continuous progression with `run`

```bash
slipway init --tools codex
slipway new "refresh governance docs" --preset standard
slipway run                # keep advancing until a skill, blocker, checkpoint, or done-ready state is surfaced
# execute the surfaced skill, resolve blockers, or answer the checkpoint
slipway run                # repeat until Slipway surfaces done-ready
slipway done
```

Use the situational commands only when they help with the current stop point:

- `slipway status`: inspect the current state, blockers, and next actions
- `slipway validate`: recompute evidence and gate readiness without advancing state
- `slipway review`: run explicit review after execution evidence exists (S2_EXECUTE onward)
- `slipway next --json`: inspect the next skill context without mutating state

If execution pauses for a checkpoint, inspect the paused task in `status` or
`next --json`, then resume with `slipway run --resume-response <value>`.
If execution looks inconsistent after an interrupted run, start with
`slipway health --doctor`, then run `slipway repair`, and only then retry
`slipway run --resume` or `slipway run --resume-response`.

## Verification

Use the same verification bundle locally that CI now runs:

```bash
go test ./... -count=1
go vet ./...
staticcheck ./...
go test ./... -race -count=1
```

## Authority model

Slipway keeps a single current-state authority and separates that from frozen
evidence and append-only trace records.

| Surface | Role |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | Current lifecycle and routing authority: slug, workflow position, worktree binding, preset, checkpoint, artifact state, evidence refs, auto-pass history, and review intent-drift history |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | Append-only lifecycle trace for mutating lifecycle outcomes; audit data only, never a second state authority |
| `artifacts/changes/<slug>/verification/execution-summary.yaml` | Frozen execution outcome authority |
| `artifacts/changes/<slug>/verification/*.yaml` | Skill verification authority |
| `artifacts/changes/<slug>/{intent,requirements,decision,tasks,assurance}.md` | Intent and contract authority |
| Computed governance readiness | Read-only command projection used by `status`, `validate`, `next`, `review`, `done`, and `stats`; never persisted as authority |

`runtime-state.yaml` is a legacy sidecar name only. Current Slipway versions
load it for migration/repair compatibility and then fold recognized runtime
fields into `change.yaml`.

`status`, `validate`, and `next` are read-only surfaces. They recompute
readiness and projection in-process and do not rewrite `change.yaml` or append
lifecycle trace records during inspection.

`learn --preview` is also read-only. It aggregates `change.yaml` telemetry and
`events/lifecycle.jsonl` traces into deterministic proposals, but it never
applies policy, prompt, skill, or template changes automatically.

Lifecycle traces include mutating state/gate outcomes plus derived audit
events such as `skill.presented`, `control.triggered`, and
`skill.evidence_recorded`. The derived events make handoffs and consumed
verification evidence visible without turning the trace into state authority.

The learning surface reports deterministic aggregates such as plan-audit
stall/budget signals, plan-audit iteration distribution, control and evidence
missing frequencies, checkpoint resolution rate, interruption resume success
rate, and guardrail-domain frequency. Proposals remain manual-review only and
carry a date-stamped `proposal_id`.

## Skill surfaces

Slipway has two related but distinct skill layers:

- The governance skill registry is the runtime handoff surface. These skills
  are eligible to appear in `next --json` as `next_skill.name` and are the only
  skills that progression logic treats as state-machine evidence.
- The embedded skill template library under `internal/tmpl/templates/skills/`
  contains support and specialist guidance such as SAST orchestration, threat
  modeling, mutation testing, CI triage, and security review. These templates
  can be exported for host tools, but they do not become progression states just
  because they exist.

`worktree-preflight` is an exported governance-adjacent handoff used by the
worktree gate. It is surfaced by progression when needed, but its validation is
owned by the worktree gate rather than by generic governance skill evidence.

## Repository layout

- `cmd/`: Cobra CLI commands, flag parsing, output rendering, and thin orchestration
- `internal/engine/`: core workflow logic, split by concern:
  - `action/`: workflow path definitions and state routing
  - `artifact/`: artifact schema, scaffold, and reconciliation
  - `context/`: execution context assembly
  - `control/`: control configuration, derivation, and evaluation
  - `gate/`: gate evaluation engine (blocking / advisory)
  - `governance/`: preset policies, readiness snapshots, traceability
  - `intake/`: `--from-doc` parsing, intent seeding, interactive prompt payload
  - `progression/`: state-machine advancement, evidence, skill resolution, validation
  - `review/`: artifact-code alignment review logic
  - `skill/`: skill registry and loader
  - `status/`: governed status projection (progress, evidence inventory, artifact DAG, diagnostics)
  - `wave/`: wave execution planning and task-plan parsing
- `internal/model/`: domain types (Change, WorkflowState, ReasonCode, ExecutionSummary, etc.)
- `internal/state/`: lifecycle authority I/O, bundle paths, runtime sidecar, worktree binding
- `internal/bootstrap/`: workspace initialization
- `internal/toolgen/`: tool adapter generation and workspace tool resolution
- `internal/tmpl/`: embedded templates for artifacts, agents, skills, hooks, and command entries
- `internal/fsutil/`: atomic file writes and advisory file locking
- `internal/stringutil/`: HTML comment stripping, unique-sorted helpers
- `internal/writeutil/`: best-effort user-facing process output helpers
- `artifacts/changes/`: governed change bundles (`change.yaml`, lifecycle events, bundle artifacts, verification evidence)
- `artifacts/codebase/`: durable repo-scoped brownfield maps

## Notes

- Repo config lives in `.slipway.yaml`
- Advisory organization policy packs can be registered under
  `governance.policy_packs`; they are proposal/context inputs only and cannot
  override built-in fail-closed guardrail domains. `next --json` includes
  bounded advisory policy-pack summaries and read refs for configured packs,
  including advisory rules, artifact requirements, recommended reviewers, and
  terminology when present.
- Human-readable artifacts live under `artifacts/`
- Shared runtime control/cache lives under `$(git rev-parse --git-common-dir)/slipway/`
- Commands anchor to the canonical main repo scope by default; explicit nested slipway scopes are registered separately in git metadata
- Shared runtime state stays owned by the resolved scope root even when a discovery-heavy governed bundle relocates into a dedicated worktree
- Nested scopes are independent: if a subtree is initialized as its own slipway scope, it also owns its own `artifacts/` and git-scoped runtime paths
- `slipway new --help`, `slipway next --help`, `slipway run --help`, and `slipway status --help` are the best entry points for day-to-day operation details
- Command contract records live in `docs/command-contract-matrix.md`, `docs/adr-retire-sync-as-product-verb.md`, and `docs/execution-surface-boundary.md`
- Agent activation rules and config override behavior are documented in `docs/agent-contracts.md`
- Worktree/orchestrator promotion is officially deferred and documented in `docs/worktree-orchestrator-deferment.md`
- Historical implementation plans live under `docs/plans/`
