# ホストアダプター

`slipway install` は6つの explicit capability 向けに host-native entry point を生成します。

```text
slipway-run  slipway-clarify  slipway-propose
slipway-decompose  slipway-implement  slipway-review
```

`run` は recoverable Run を駆動します。`clarify` は standalone/stateless な decision 会話です。`propose` と `decompose` は GitHub work item を準備します。`implement` は technical work を行います。`review` は read-only です。

![Slipway host adapters: install が10個の対応host向けnative entry pointを書き込み、各entryが同じ6個のexplicit capabilityを公開し、versioned JSONでlocal CLIへrouteする。](../../assets/diagrams/tool-adapters.svg)

## Generated target

下表は generated file と意図される invocation を示します。外部 host の実際の動作はその host version に依存し、repository test は generation と protocol text を検証するもので、すべての host UI の E2E 検証ではありません。

| ID | Generated target | 意図される invocation |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `slipway-<name>` skill を呼び出す。 |
| `codex` | `.codex/skills/slipway-*/SKILL.md` と各 skill の `agents/openai.yaml` | `$slipway-<name>` |
| `copilot` | `.github/agents/slipway-<name>.agent.md` | custom agent を選択する。 |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `slipway-<name>` skill を呼び出す。 |
| `kilo` | `.kilo/commands/slipway-<name>.md` と `.kilocode/slipway/capabilities/` | `/slipway-<name>` |
| `kiro` IDE | `.kiro/steering/slipway-<name>.md` と `.kiro/slipway/capabilities/` | 手動で `#slipway-<name>` を含める。 |
| `kiro` CLI | `.kiro/agents/slipway-<name>.json` と `.kiro/slipway/capabilities/` | `kiro-cli chat --agent slipway-<name>` |
| `opencode` | `.opencode/commands/slipway-<name>.md` と `.opencode/slipway/capabilities/` | `/slipway-<name>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/skill:slipway-<name>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `slipway-<name>` skill を呼び出す。 |
| `windsurf` | `.windsurf/workflows/slipway-<name>.md` と `.windsurf/slipway/capabilities/` | `/slipway-<name>` |

Copilot agent は self-contained です。Kilo、Kiro、OpenCode、Windsurf は generated capability body を指す thin native entry を使います。Skill-native host は capability body を `SKILL.md` に持ちます。

Kiro は初回 install で `--surface ide` または `--surface cli` が必要です。選択は記録され、通常の refresh では切り替わりません。

## 明示的な invocation

Adapter は session-start hook、prompt-submit hook、launcher、global router を install しません。Host settings は adapter ownership の外です。ユーザーが明示的に capability を呼び出します。明示的に開始した `slipway-run` 内では、host は各通常 step のたびに authorization を求めずに bounded Action loop を進められます。

Codex policy file は各 generated capability で implicit model invocation を無効化します。他の target は各々の native explicit-entry surface と共通 instruction を使います。

## CLI と host の責務

CLI：

- Run を validate して記録する。
- 次の Action を選ぶ。
- Git と workspace identity を観測する。
- Source envelope と Outcome を validate する。
- Structured recovery を返す。

Host：

- Repository を読み、technical work を行う。
- Model を呼び出す。
- Issue-backed work をユーザーが要求したときに GitHub credential を使う。
- Temporary source envelope を構築する。
- Publication preview、confirmation、reconciliation instruction に従う。

したがって `propose` と `decompose` は host が GitHub API をどう使うかを記述し、Go CLI は GitHub publication transaction を提供しません。[GitHub Issues を使う](../guides/github-issues.md)を参照してください。

## Install と refresh

```bash
slipway install --tool claude
slipway install --tool kiro --surface ide
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

初回 Kiro と `--tool all` の注意点は[インストール](../installation.md)を参照してください。

## Ownership safety

各 host root には repository-relative path と SHA-256 を記録した Slipway ownership manifest があります。Refresh と uninstall は記録 hash と一致する file だけを変更します。

User-modified capability、unknown file、modified sentinel、malformed manifest、path escape、duplicate claim、unsafe symlink は決して静かに managed content になりません。操作は preserve または reject し理由を報告します。Transaction recovery artifact は通常の preserved user file とは別に報告されます。

Generated sentinel は installation health を示すだけで、ownership ではありません。Managed-file 変更を authorize できるのは manifest だけで、unsupported manifest version は mutation 前に失敗します。
