# AI ツールアダプター

このページは、生成される AI ツールアダプターに関する Diataxis のリファレンス入口です。
詳細な従来版アダプターリファレンスは
[AI ツールアダプター](../ai-tools.md)で引き続き参照できます。

`slipway init --tools` は、AI コーディングツールが現在のプロジェクトで Slipway の
コマンドと統制されたスキル指示を発見できるようにするホストファイルを書き出します。
これらのファイルは CLI へ処理を戻すものであり、独立した統制エンジンではありません。

## サポートされるツール ID

| ツール ID | 生成されるスキルパス | 呼び出し方法 |
| --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `$slipway-<command>` または `/skills` |
| `copilot` | `.github/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `kilo` | `.kilocode/skills/slipway-*/SKILL.md` | `/slipway:<command>` |
| `kiro` | `.kiro/skills/slipway-*/SKILL.md` | `@slipway:<command>` またはホストのスキルピッカー |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `/slipway-<command>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `/slipway-<command>` またはホストのスキルピッカー |
| `windsurf` | `.windsurf/skills/slipway-*/SKILL.md` | `/slipway-<command>` |

## 生成または更新

```bash
slipway init --tools codex
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --refresh
slipway init --tools all --refresh
```

更新時には Slipway が生成したマーカーを検出し、生成されたアダプターディレクトリの
隣にあるユーザー所有ファイルを保持します。

## 生成されるコマンド面

ホストプロンプトを利用するコマンドは、コマンド面を生成します。Codex、Kiro、Qwen は
それらをコマンドスキルとして公開します。その他のホストはプロンプト、コマンド、または
ワークフローファイルとして公開します。

- Claude: `.claude/commands/slipway/<id>.md`
- Copilot: `.github/prompts/slipway-<id>.prompt.md`
- Cursor: `.cursor/commands/slipway-<id>.md`
- Kilo: `.kilocode/workflows/slipway-<id>.md`
- OpenCode: `.opencode/commands/slipway-<id>.md`
- Pi: `.pi/prompts/slipway-<id>.md`
- Windsurf: `.windsurf/workflows/slipway-<id>.md`

生成されたアダプターファイルは `slipway` CLI へ処理を戻します。ホスト面は独立した
ライフサイクル、レビュー、エビデンスのエンジンを実装しません。

Codex のコマンドスキルトークン:

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
$slipway-handoff
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

`slipway tool` は CLI 専用です。生成されるホストコマンドのラッパーはなく、生成された
スキルが必要に応じてヘルパーサブコマンドを直接呼び出します。

## 設定と所有権

設定対応のアダプターは、エージェントに生成ファイルを手動編集させるのではなく、ホスト設定を
マージします。

- Claude は、インラインの素の `slipway hook ...` 設定コマンドを登録します。
- Pi は `.pi/settings.json` を `enableSkillCommands=true` で書き込み、`./skills` と
  `./prompts` を登録します。さらに `.pi/extensions/slipway-hooks.ts` を生成し、
  `session-start` フックを pi の `session_start` / `before_agent_start` 拡張イベントへ
  ブリッジします。pi は `.pi/extensions/` を自動検出しますが、プロジェクトが信頼された後にだけ
  プロジェクトローカル拡張を読み込みます。`context-pressure` フックはブリッジされません
  （pi の拡張 `exec` には stdin がありません）。
- Qwen は `.qwen/settings.json` を書き込み、セッション開始フックを登録します。
- Codex は `SessionStart` と `UserPromptSubmit` 向けの `.codex/config.toml` フックを
  書き込みます。これらのフックは、リポジトリと各フックがユーザーによって信頼されるまでは
  無効であり、Slipway がグローバルな Codex の信頼設定を編集することはありません。

各アダプターは、アダプタールートの `slipway/` ディレクトリ配下にある Slipway 生成の
センチネルと所有権マニフェストによって追跡されます。Copilot はその管理状態を
`.github/copilot/slipway` 配下に保存するため、更新時に `.github` の残りの部分を
Slipway 所有とは扱いません。

## 安全ルール

- 現在のワークトリーの CLI 出力を権威として扱います。
- コマンド、スキル、フックの契約が変わったら、生成されたアダプターを更新します。
- 隣接する AI ツールディレクトリにあるユーザー所有ファイルを保持します。
- プロジェクトのデフォルトを共有すべき場合は `.slipway.yaml` をコミットします。
- 生成されたアダプターファイルは、コミット前にリポジトリのポリシーに従ってレビューします。

## 詳細情報

生成されるフックの詳細、OpenCode に関する注記、設定対応ホスト、従来版のクリーンアップ動作に
ついては、[AI ツールアダプター](../ai-tools.md)を参照してください。
