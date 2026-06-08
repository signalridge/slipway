# AI Tool Adapters

`slipway init --tools` exports host-tool files that let AI coding tools invoke Slipway commands and load governed skill instructions from the current project.

<div align="center" markdown>

![Slipway tool adapters: slipway init --tools generates per-tool adapter bundles for Claude, Codex, Cursor, Gemini and OpenCode plus the .slipway.yaml runtime config; each adapter's generated skills and commands route governed actions to the slipway CLI](assets/diagrams/tool-adapters.svg)

</div>

## Supported Tools

| Tool ID | Skills path | Command path | Invocation style |
| --- | --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `.claude/commands/slipway/*.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `$CODEX_HOME/prompts/slipway-*.md` | `$slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `.cursor/commands/*.md` | `/slipway-<command>` |
| `gemini` | `.gemini/skills/slipway-*/SKILL.md` | `.gemini/commands/slipway/*.toml` | `/slipway-<command>` |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `.opencode/commands/slipway-*.md` | `/slipway-<command>` |

Codex command prompts are global because Codex consumes prompt files from its home directory. If `CODEX_HOME` is unset, Slipway uses `~/.codex`.

## Generate Adapters

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools all
slipway init --tools none   # initialize runtime layout only, no adapter files
```

Refresh managed files:

```bash
slipway init --tools all --refresh
```

Refresh auto-detected managed adapters:

```bash
slipway init --refresh
```

Slipway detects adapters by its generated markers, not by a bare `.claude`, `.codex`, `.cursor`, `.gemini`, or `.opencode` directory alone.

## Generated Command Surface

Core prompt-backed commands:

- `new`
- `next`
- `run`
- `status`
- `done`

Situational prompt-backed commands:

- `init`
- `cancel`
- `delete`
- `review`
- `validate`
- `checkpoint`
- `preset`
- `pivot`
- `abort`
- `repair`
- `evidence` (the wave-orchestration host records task evidence via `slipway evidence task ...`)

Diagnostics prompt-backed commands:

- `learn`
- `stats`
- `health`
- `codebase-map`
- `instructions`

Every CLI command ships a prompt-backed surface, so an agent never has to fall
back to guessing one; the workflow skill's command reference indexes them all.

## OpenCode Notes

OpenCode stores project commands as Markdown files under `.opencode/commands/`. Slipway generates flat OpenCode command files under:

```text
.opencode/commands/
```

The command file name becomes the OpenCode command ID. For example:

```text
.opencode/commands/slipway-new.md
```

is invoked as:

```text
/slipway-new
```

Some OpenCode builds display project commands with a project prefix in the command picker. The generated file path remains the stable Slipway contract.

Generated OpenCode skills live under:

```text
.opencode/skills/
```

and the advisory session hook is:

```text
.opencode/hooks/slipway-session-start.sh
```

## Safety Rules

- Do not edit generated Slipway adapter files unless you are intentionally customizing local host behavior.
- Use `slipway init --refresh` to update generated files after Slipway changes.
- Preserve user-owned files in adjacent AI-tool directories.
- Commit `.slipway.yaml` when the repository should be initialized for all contributors; review generated adapter files according to the repository's policy before committing them.
