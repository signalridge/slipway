# AI Tool Adapters

This page is the Diataxis reference entry for generated AI-tool adapters. The
detailed legacy adapter reference remains available at
[AI Tool Adapters](../ai-tools.md).

`slipway init --tools` exports host files that let AI coding tools discover
Slipway commands and governed skill instructions in the current project. Those
files route back to the CLI; they are not separate governance engines.

## Supported Tool IDs

| Tool ID | Generated skill path | Invocation style |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `$slipway-<command>` or `/skills` |
| `copilot` | `.github/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `gemini` | `.gemini/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `kilo` | `.kilocode/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `kiro` | `.kiro/skills/slipway-*/SKILL.md` | `@slipway:<command>` or host skill picker |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `/slipway-<command>` or host skill picker |
| `windsurf` | `.windsurf/skills/slipway-*/SKILL.md` | `/slipway-<command>` |

## Generate Or Refresh

```bash
slipway init --tools codex
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --refresh
slipway init --tools all --refresh
```

Refresh detects Slipway generated markers and preserves user-owned files beside
generated adapter directories.

## Generated Command Surface

Commands that opt into host prompts generate command surfaces. Codex, Kiro, and
Qwen expose them as command skills. Other hosts expose prompt, command, or
workflow files:

- Claude: `.claude/commands/slipway/<id>.md`
- Copilot: `.github/prompts/slipway-<id>.prompt.md`
- Cursor: `.cursor/commands/slipway-<id>.md`
- Gemini: `.gemini/commands/slipway/<id>.toml`
- Kilo: `.kilocode/workflows/slipway-<id>.md`
- OpenCode: `.opencode/commands/slipway-<id>.md`
- Pi: `.pi/prompts/slipway-<id>.md`
- Windsurf: `.windsurf/workflows/slipway-<id>.md`

Generated adapter files route to the `slipway` CLI. The host surfaces do not
implement separate lifecycle, review, or evidence engines.

Codex command-skill tokens:

```text
$slipway-new
$slipway-intake
$slipway-plan
$slipway-implement
$slipway-review
$slipway-fix
$slipway-done
$slipway-next
$slipway-run
$slipway-status
$slipway-codebase-map
$slipway-preset
$slipway-validate
$slipway-abort
$slipway-cancel
$slipway-delete
$slipway-repair
$slipway-evidence
$slipway-health
$slipway-instructions
$slipway-init
```

`slipway tool` is CLI-only. It has no generated host command wrapper; generated
skills call helper subcommands directly when required.

## Settings And Ownership

Settings-capable adapters merge host settings instead of asking agents to
hand-edit generated files:

- Claude and Gemini register bare inline `slipway hook ...` settings commands.
- Pi writes `.pi/settings.json` with `enableSkillCommands=true` and registers
  `./skills` and `./prompts`.
- Qwen writes `.qwen/settings.json` to register the session-start hook.

Each adapter is tracked by a Slipway generated sentinel and ownership manifest
under the adapter root's `slipway/` directory. Copilot stores that managed
state under `.github/copilot/slipway` so refresh does not treat the rest of
`.github` as Slipway-owned.

## Safety Rules

- Treat the current-worktree CLI output as authority.
- Refresh generated adapters after command, skill, or hook contracts change.
- Preserve user-owned files in adjacent AI-tool directories.
- Commit `.slipway.yaml` when project defaults should be shared.
- Review generated adapter files according to repo policy before committing
  them.

## Full Detail

See [AI Tool Adapters](../ai-tools.md) for generated hook details, OpenCode
notes, settings-capable hosts, and legacy cleanup behavior.
