# Host adapters

`slipway install` generates host-native entry points for six explicit capabilities:

```text
slipway-run  slipway-clarify  slipway-propose
slipway-decompose  slipway-implement  slipway-review
```

`run` drives a recoverable Run. `clarify` is a standalone, stateless decision conversation. `propose` and `decompose` prepare GitHub work items. `implement` performs technical work. `review` is read-only.

## Generated targets

The table describes generated files and intended invocation. External host behavior depends on the installed host version; repository tests validate generation and protocol text, not every host's runtime UI.

| ID | Generated target | Intended invocation |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | Invoke the `slipway-<name>` skill. |
| `codex` | `.codex/skills/slipway-*/SKILL.md` plus per-skill `agents/openai.yaml` | `$slipway-<name>` |
| `copilot` | `.github/agents/slipway-<name>.agent.md` | Select the custom agent. |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | Invoke the `slipway-<name>` skill. |
| `kilo` | `.kilo/commands/slipway-<name>.md` plus `.kilocode/slipway/capabilities/` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/slipway-<name>.md` plus `.kiro/slipway/capabilities/` | Manually include `#slipway-<name>`. |
| `kiro` CLI | `.kiro/agents/slipway-<name>.json` plus `.kiro/slipway/capabilities/` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/slipway-<name>.md` plus `.opencode/slipway/capabilities/` | `/slipway-<name>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | Invoke the `slipway-<name>` skill. |
| `windsurf` | `.windsurf/workflows/slipway-<name>.md` plus `.windsurf/slipway/capabilities/` | `/slipway-<name>` |

Copilot agents are self-contained. Kilo, Kiro, OpenCode, and Windsurf use a thin native entry that points to a generated capability body. Skill-native hosts receive the capability body in `SKILL.md`.

Kiro needs `--surface ide` or `--surface cli` on first install. The choice is recorded and cannot be switched by an ordinary refresh.

## Explicit invocation

Adapters do not install session-start hooks, prompt-submit hooks, launchers, or a global router. Host settings remain outside adapter ownership. A user explicitly invokes a capability; within an explicitly started `slipway-run`, the host may then follow the bounded Action loop without asking for authorization before every ordinary step.

Codex policy files disable implicit model invocation for each generated capability. Other targets use their native explicit-entry surface and shared instructions.

## CLI and host responsibilities

The CLI:

- validates and records Runs;
- chooses the next Action;
- observes Git and workspace identity;
- validates source envelopes and Outcomes;
- returns structured recovery.

The host:

- reads repository files and performs technical work;
- calls the model;
- uses GitHub credentials when the user requests issue-backed work;
- constructs temporary source envelopes;
- follows publication previews, confirmations, and reconciliation instructions.

Accordingly, `propose` and `decompose` describe how the host should use GitHub APIs; the Go CLI does not provide a GitHub publication transaction. See [Using GitHub Issues](../guides/github-issues.md).

## Install and refresh

```bash
slipway install --tool claude
slipway install --tool kiro --surface ide
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

See [Installation](../installation.md) for the first-time Kiro and `--tool all` caveat.

## Ownership safety

Each host root contains a Slipway ownership manifest with repository-relative paths and SHA-256 hashes. Refresh and uninstall mutate only files still matching their recorded hash.

A user-modified capability, unknown file, modified sentinel, malformed manifest, path escape, duplicate claim, or unsafe symlink is never silently adopted as managed content. Operations preserve or reject it and report the reason. Transaction recovery artifacts are reported separately from ordinary preserved user files.

A generated sentinel indicates installation health, not ownership. The manifest is the only file that authorizes later managed-file changes, and unsupported manifest versions fail before mutation.
