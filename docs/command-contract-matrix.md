# Command Contract Matrix

This is the Phase 0 and Phase 1 authority record for Slipway's current command
surface.

## Scope Note

The optimization plan started from a 17-command baseline:

- 16 surfaced product commands
- 1 hidden product helper command: `root`

The implemented surface is now larger because the plan explicitly promoted two
previously-missing execution surfaces:

- `run`
- `abort`

Current product command count:

- 14 adapter-visible surfaced commands
- 3 surfaced CLI-only diagnostics commands
- 1 hidden product helper command: `root`

`help` remains a Cobra utility surface and is intentionally tracked outside the
product command matrix.

## Product Identity

Slipway is a governance-first workflow runtime with explicit execution
surfaces and config-driven agent coordination.

## Authority Legend

- `CLI`: Cobra command contract in `cmd/*.go`
- `REG`: registry contract in `internal/toolgen/toolgen.go`
- `ROOT`: grouped root help rendering in `cmd/root.go`
- `GEN`: generated command prompt surfaces (inline body partials)
- `DOC`: user-facing docs such as [README.md](/Users/yixianlu/ghq/github.com/signalridge/slipway/README.md) and workflow guides

For surfaced adapter-visible commands, `CLI + REG` is authoritative and `ROOT +
GEN + DOC` are downstream rendered surfaces. For surfaced CLI-only commands,
`CLI + REG` is authoritative and `ROOT + DOC` are the downstream rendered
surfaces.

## Product Commands

| Command | Class | Visibility / tier | Authority | Downstream | Status |
|---|---|---|---|---|---|
| `new` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `preset` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `next` | query | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `run` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `status` | query | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `done` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `review` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `validate` | query | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `pivot` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `abort` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `cancel` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `repair` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `checkpoint` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `init` | mutation | surfaced / situational, shown under Setup in root help | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `stats` | query | surfaced / diagnostics, CLI-only | `CLI + REG` | `ROOT + DOC` | green |
| `health` | query | surfaced / diagnostics, CLI-only | `CLI + REG` | `ROOT + DOC` | green |
| `codebase-map` | mutation | surfaced / diagnostics, CLI-only, shown under Discovery in root help | `CLI + REG` | `ROOT + DOC` | green |
| `root` | query helper | hidden / CLI-only | `CLI` | none | green |

## Root Help Grouping

The root help surface is a progressive-disclosure rendering, not an independent
authority layer. Current group membership is intentional:

| Root help group | Commands |
|---|---|
| `Core lifecycle` | `new`, `preset`, `next`, `run`, `status`, `done` |
| `Discovery` | `codebase-map` |
| `Situational` | `review`, `validate`, `pivot`, `abort`, `cancel`, `repair`, `checkpoint` |
| `Diagnostics` | `stats`, `health` |
| `Setup` | `init` |

Notes:

- `codebase-map` stays registry-tier `diagnostics`, but root help renders it
  under `Discovery` because it is the main durable discovery entrypoint for
  brownfield work. It remains CLI-only today: no generated command prompt
  surface is emitted for it.
- `stats` and `health` remain surfaced diagnostics commands with
  registry-backed descriptions, but they are also CLI-only today and do not
  generate command prompt surfaces.
- `init` stays adapter-visible `situational`, but root help renders it under
  `Setup` because it is workspace bootstrap rather than day-to-day governed
  execution.
- Hidden `root` is intentionally excluded from grouped help.

## Utility Surface

`help` is a Cobra utility surface, not a product contract row.

- Authority: Cobra help handling in `cmd/root.go`
- Role: expose grouped help and command-specific help
- Policy: kept outside the product matrix so adapter registry contracts do not
  have to model Cobra's built-in utility behavior

## Routed Command Surfaces

Routed commands expose a four-class surface taxonomy through the
`SurfacePolicy` authority in `internal/engine/capability/surfaces.go`. Exposure
classes, flags, and discovery commands are:

| Class | User surface | Discovery |
|---|---|---|
| Primary | selected automatically, never by flag | implicit |
| Suggested | rendered via `suggested_capabilities[]` in JSON and a `Suggested:` text block | bounded at 3, disjoint from primary |
| ExplicitFocus | `--focus <alias>` on `review`, `validate`, `repair` | `<command> --list-focuses [--format text\|json]` |
| View | `--view <alias>` on `status`, `health` | `<command> --list-views [--format text\|json]` |

Legacy `--mode` has been removed from all routed commands. Raw skill IDs are
rejected with `unknown_route_mode` (for focus) or `unknown_route_view` (for
views). The public alias registry is:

### `--focus` aliases

| Command | Alias | Backing skill |
|---|---|---|
| `review` | `calibration` | `multi-reviewer-calibration` |
| `review` | `sast` | `sast-orchestration` |
| `validate` | `mutation` | `mutation-testing` |
| `validate` | `property` | `property-testing` |
| `validate` | `sast` | `sast-orchestration` |
| `validate` | `spec-trace` | `spec-trace` |
| `repair` | `sast` | `sast-orchestration` |

### `--view` aliases

| Command | Alias | Backing skill |
|---|---|---|
| `status` | `incident` | `incident-response` |
| `health` | `incident` | `incident-response` |

### Primary routes

| Command | Backing skill |
|---|---|
| `review` | `independent-review` |
| `repair` | `root-cause-tracing` |
| `health` | `incident-response` (change-scoped; requires `ConcreteChangeTarget`) |

`status` and `validate` no longer have primary routes; their default output is
neutral and state-focused. Expert posture is available via `--focus`. `health`
retains its primary route gated on `ConcreteChangeTarget`.

### `suggested_capabilities[]` output contract

Routed commands (`review`, `validate`, `repair`) carry a stable,
user-facing suggestion channel in both JSON and text output. This is
`suggested_capabilities[]` in §4.4 of the route-surface refactor plan:

- JSON output gains an optional `suggested_capabilities` array. Each entry
  is `{ "name": <public-name>, "summary": <prose>, "reason": <prose>, "kind":
  "suggested" | "explicit_focus" }`. `summary` and `reason` are omitted when
  prose is unavailable. `name` is the user-facing surface name (for example an
  explicit-focus suggestion emits `sast`, not `sast-orchestration`). The list
  is capped at three entries, ordered by (clause score desc, skill id asc),
  and disjoint from `Supports`.
- Text output emits a `Suggested:` block after the `Mode:` line whenever
  the list is non-empty. Each line prefers the matched `reason`; if `reason`
  is unavailable, it falls back to `summary`:

  ```
  Suggested:
    - <name> — <reason-or-summary>
  ```

`status` and `health` intentionally do not emit
`suggested_capabilities[]`; they are read-only diagnostic views with no
alternative-capability channel (§4.4).

### Discovery flags

- `review --list-focuses [--format text|json]`
- `validate --list-focuses [--format text|json]`
- `repair --list-focuses [--format text|json]`
- `status --list-views [--format text|json]` (status reuses its existing
  `--format` flag, which also accepts `yaml` for non-discovery paths)
- `health --list-views [--format text|json]`

Discovery flags short-circuit execution before any workspace access and
emit the list of public aliases with their human-readable summaries.
Discovery flags are registered only on the surfaces that own them: using
`review --list-views` or `status --list-focuses` fails at parse time as an
unknown flag, mirroring wrong-surface `--focus` / `--view` handling.
