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
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `gemini` | `.gemini/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `/slipway-<command>` |

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

Commands that opt into host prompts generate command surfaces. Codex exposes
them as command skills; other hosts expose prompt or command files.

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
$slipway-checkpoint
$slipway-evidence
$slipway-learn
$slipway-stats
$slipway-health
$slipway-instructions
$slipway-init
```

`slipway tool` is CLI-only. It has no generated host command wrapper; generated
skills call helper subcommands directly when required.

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
