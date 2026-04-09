# Slipway

Slipway is a governance-first CLI for routing work into intake, governed execution, and evidence-backed closeout inside a local repository.

## Entry surface

Slipway's public entry surface is `new`.

- `slipway new`: create a governed change starting at intake (S0_INTAKE)

Refinements:

- `slipway new --from-doc <path>`: seed governed work from an existing document
- `slipway new --discuss`: persist unresolved gray areas into context before execution
- `slipway new --full`: require refreshed final-closeout evidence before ship
- `slipway new --trivial`: force complexity=trivial for lightweight changes

## Governed progression

After governed work exists, `next`, `status`, and `done` drive progression and closeout.

- `slipway next`: validate evidence, advance governed state, and surface the next required skill
- `slipway status`: inspect governed status, blockers, and artifact readiness
- `slipway done`: finalize a done-ready change and archive it

### Situational commands

- `slipway review`: bidirectional artifact-code alignment review (available from S2_EXECUTE onward)
- `slipway validate`: read-only evidence and gate check for the active change
- `slipway sync`: validate the change's requirements.md exists and is well-formed (read-only)
- `slipway pivot`: reroute (`--reroute`) or rescope (`--rescope`) an active change (available from S2_EXECUTE onward)
- `slipway cancel`: cancel the active change and archive it as terminal
- `slipway checkpoint`: pause wave execution for user input (available at S2_EXECUTE)
- `slipway repair`: run bounded local integrity repairs (stale locks, interrupted archives, corrupt config)

### Diagnostics and observability

- `slipway stats`: **repo-wide** quantitative reporting — active change count, archive count, codebase-map freshness
- `slipway health`: **repo-local** integrity and repairability findings — missing codebase maps, ambiguous layouts, worktree problems
- `slipway codebase-map`: create or refresh the durable repo-scoped brownfield map (`artifacts/codebase/`)

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

- `cmd/`: Cobra CLI commands and user-facing orchestration
- `internal/engine/`: workflow progression, gates, routing, and artifact logic
- `internal/state/`: lifecycle authority loading plus bundle-local runtime sidecar management
- `internal/fsutil/`: atomic file and lock primitives
- `artifacts/changes/`: governed change bundles (`change.yaml`, `runtime-state.yaml`, bundle artifacts, verification evidence)
- `artifacts/codebase/`: durable repo-scoped brownfield maps

## Notes

- Repo config lives in `.slipway.yaml`
- Human-readable artifacts live under `artifacts/`
- Shared runtime control/cache lives under `$(git rev-parse --git-common-dir)/slipway/`
- Commands anchor to the canonical main repo scope by default; explicit nested slipway scopes are registered separately in git metadata
- Shared runtime state stays owned by the resolved scope root even when a discovery-heavy governed bundle relocates into a dedicated worktree
- Nested scopes are independent: if a subtree is initialized as its own slipway scope, it also owns its own `artifacts/` and git-scoped runtime paths
- `slipway status --help` and `slipway next --help` are the best entry points for day-to-day operation details
