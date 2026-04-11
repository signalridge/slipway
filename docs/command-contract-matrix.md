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

- 15 adapter-visible surfaced commands
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
- `GEN`: generated command entries, prompts, and adapter skill templates
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
| `next` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `run` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `status` | query | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `done` | mutation | surfaced / core | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `review` | mutation | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `validate` | query | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
| `validate-requirements` | query | surfaced / situational | `CLI + REG` | `ROOT + GEN + DOC` | green |
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
| `Situational` | `review`, `validate`, `validate-requirements`, `pivot`, `abort`, `cancel`, `repair`, `checkpoint` |
| `Diagnostics` | `stats`, `health` |
| `Setup` | `init` |

Notes:

- `codebase-map` stays registry-tier `diagnostics`, but root help renders it
  under `Discovery` because it is the main durable discovery entrypoint for
  brownfield work. It remains CLI-only today: no generated adapter skill or
  command entry is emitted for it.
- `stats` and `health` remain surfaced diagnostics commands with
  registry-backed descriptions, but they are also CLI-only today and do not
  generate adapter skills or command entries.
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
