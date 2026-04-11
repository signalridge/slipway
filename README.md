# Slipway

Slipway is a governance-first CLI for routing work into intake, governed execution, and evidence-backed closeout inside a local repository.

## Command surface

Slipway is organized around an intake-first lifecycle plus a smaller set of
setup, diagnostics, and repair commands.

### Core lifecycle

- `slipway new [description]`: create a governed change starting at intake (S0_INTAKE)
- `slipway preset <light|standard|strict>`: confirm or update the governance preset for the active change
- `slipway next`: validate evidence, advance state when ready, and surface the next required skill
- `slipway run`: advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced
- `slipway status`: inspect lifecycle state, blockers, and next actions
- `slipway done`: finalize a done-ready change and archive it

If `slipway new` is run without a description in a TTY, Slipway opens an
interactive intake prompt with inferred project context.

Creation refinements:

- `slipway new --preset <light|standard|strict>`: choose the governance preset during creation
- `slipway new --from-doc <path>`: seed governed work from an existing document
- `slipway new --discuss`: persist unresolved gray areas into context before execution
- `slipway new --full`: require refreshed final-closeout evidence before ship
- `slipway new --trivial`: force complexity=trivial for lightweight changes

## Governed progression

After governed work exists, use either the single-step surface (`next`) or the
loop surface (`run`) to advance the lifecycle.

- `slipway next`: validate evidence, advance at most one governed step, and surface the next required skill
  - Use `--preview` to inspect the next skill context without advancing state
  - Use this when you want step-by-step control
- `slipway run`: keep advancing until Slipway hits an operator-facing stop condition
  - Stop conditions are: next skill surfaced, blockers surfaced, checkpoint surfaced, or done-ready reached
  - Use `--resume` to continue resumable non-checkpoint execution from the latest incomplete wave
  - Use `--resume-response <value>` to resume from an active checkpoint
  - Use this when you want Slipway to keep moving until explicit operator input is needed
- `slipway status`: inspect governed status, blockers, artifact readiness, and next actions
- `slipway done`: finalize a done-ready change and archive it

### Situational commands

- `slipway review`: explicit bidirectional artifact-code alignment review (available from S2_EXECUTE onward); not part of every progression cycle
- `slipway validate`: read-only evidence and gate check for the active change; useful when blocked or when you want a non-mutating readiness check
- `slipway validate-requirements`: validate the active change's `requirements.md` contract (read-only)
- `slipway pivot`: reroute (`--reroute`) or rescope (`--rescope`) an active change (available from S2_EXECUTE onward)
- `slipway abort`: abort only the active execution session without archiving the change
- `slipway cancel`: cancel the active change and archive it as terminal
- `slipway checkpoint`: pause wave execution for user input by task id (available at S2_EXECUTE)
- `slipway repair`: run bounded local integrity repairs (stale locks, interrupted archives, corrupt config)

### Diagnostics and observability

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
slipway next --preview     # optional: inspect without advancing state
slipway next               # surface the next required skill
# execute the surfaced skill
slipway next               # repeat after each completed skill
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
- `slipway next --preview`: inspect the next skill context without mutating state

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

Slipway now splits governed state by concern instead of treating one file as the source of all post-execution truth.

| Surface | Role |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | Lifecycle and routing authority only: slug, workflow position, worktree binding, preset, checkpoint |
| `artifacts/changes/<slug>/runtime-state.yaml` | Non-lifecycle runtime projection: artifact state, non-task evidence refs, auto-pass history, review intent-drift history |
| `artifacts/changes/<slug>/verification/execution-summary.yaml` | Frozen execution outcome authority |
| `artifacts/changes/<slug>/verification/*.yaml` | Skill verification authority |
| `artifacts/changes/<slug>/{intent,requirements,decision,tasks,assurance}.md` | Intent and contract authority |
| Computed governance readiness | Read-only command projection used by `status`, `validate`, `next`, `review`, `done`, and `stats`; never persisted as authority |

`status`, `validate`, and `next --preview` are read-only surfaces. They recompute readiness and projection in-process and do not rewrite `change.yaml` or artifact runtime state during inspection.

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
- `artifacts/changes/`: governed change bundles (`change.yaml`, `runtime-state.yaml`, bundle artifacts, verification evidence)
- `artifacts/codebase/`: durable repo-scoped brownfield maps

## Notes

- Repo config lives in `.slipway.yaml`
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
