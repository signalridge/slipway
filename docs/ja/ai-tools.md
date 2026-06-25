# AI ツールアダプター

`slipway init --tools` は、AI コーディングツールが Slipway コマンドを呼び出し、現在のプロジェクトからガバナンス対象のスキル指示を読み込めるようにするホストツール用ファイルを出力します。

<div align="center" markdown>

![Slipway ツールアダプター: slipway init --tools は Claude、Codex、Copilot、Cursor、Kilo、Kiro、OpenCode、Pi、Qwen、Windsurf 向けにツールごとのアダプターバンドルと .slipway.yaml ランタイム設定を生成し、各アダプターが生成するスキルとコマンドがガバナンス対象のアクションを slipway CLI へルーティングする](assets/diagrams/tool-adapters.svg)

</div>

## 対応ツール

| Tool ID | スキルのパス | コマンドのパス | 呼び出し方法 |
| --- | --- | --- | --- |
| `claude` | `.claude/skills/slipway-*/SKILL.md` | `.claude/commands/slipway/*.md` | `/slipway:<command>` |
| `codex` | `.codex/skills/slipway-*/SKILL.md` | `.codex/skills/slipway-*/SKILL.md` | `$slipway-<command>`（または `/skills`） |
| `copilot` | `.github/skills/slipway-*/SKILL.md` | `.github/prompts/slipway-<command>.prompt.md` | `/slipway-<command>` |
| `cursor` | `.cursor/skills/slipway-*/SKILL.md` | `.cursor/commands/*.md` | `/slipway-<command>` |
| `kilo` | `.kilocode/skills/slipway-*/SKILL.md` | `.kilocode/workflows/slipway-<command>.md` | `/slipway:<command>` |
| `kiro` | `.kiro/skills/slipway-*/SKILL.md` | `.kiro/skills/slipway-<command>/SKILL.md` | `@slipway:<command>` またはホストのスキルピッカー |
| `opencode` | `.opencode/skills/slipway-*/SKILL.md` | `.opencode/commands/slipway-*.md` | `/slipway-<command>` |
| `pi` | `.pi/skills/slipway-*/SKILL.md` | `.pi/prompts/slipway-<command>.md` | `/slipway-<command>` |
| `qwen` | `.qwen/skills/slipway-*/SKILL.md` | `.qwen/skills/slipway-<command>/SKILL.md` | `/slipway-<command>` またはホストのスキルピッカー |
| `windsurf` | `.windsurf/skills/slipway-*/SKILL.md` | `.windsurf/workflows/slipway-<command>.md` | `/slipway-<command>` |

Codex、Kiro、Qwen のコマンドは、各アダプターのスキルディレクトリ配下に、発見可能なコマンドごとのスキルとして生成されます。プロンプトベースおよびワークフローベースのホストは、代わりにコマンドファイルを生成します。生成されるすべてのコマンドサーフェスは `slipway` CLI を呼び出します。ホストファイルがライフサイクル、レビュー、エビデンスの動作を独自に実装することはありません。Slipway はグローバルな Codex プロンプトファイルを書き込まなくなり、Codex のリフレッシュもホストグローバルなプロンプトディレクトリを削除しません。プロジェクト単位のプロンプトベースアダプターは、リフレッシュ時に Slipway 所有の廃止済みプロンプトファイルを引き続き削除します。

## アダプターの生成

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools all
slipway init --tools none   # initialize runtime layout only, no adapter files
```

管理対象ファイルをリフレッシュし、Slipway 所有の廃止済みアダプター成果物を削除します。

```bash
slipway init --tools all --refresh
```

自動検出した管理対象アダプターをリフレッシュします。

```bash
slipway init --refresh
```

Slipway はアダプターを、生成したセンチネルによって検出します。`.claude`、`.codex`、`.cursor`、`.opencode`、`.pi`、`.qwen`、`.kiro`、`.windsurf`、`.kilocode` といったディレクトリが単に存在するだけでは検出しません。所有権マニフェストはリフレッシュ時に生成済みファイルを保護します。センチネルのみのレガシーアダプターはマニフェスト追跡へ取り込めますが、センチネルのないパス衝突は、既存の内容が生成出力とすでに一致している場合を除き、フェイルクローズドのままです。Copilot はこの管理状態を、共有の `.github` ツリーをアダプター所有として扱うのではなく、`.github/copilot/slipway` 配下に保持します。リフレッシュは、ユーザー所有のフック、プロンプト、ワークフロー、スキルを生成済みファイルとともに保持しつつ、Slipway 所有のレガシーシェルフックランチャーと廃止済みの `bash "<hook>.sh"` フック設定エントリを削除します。

## 生成されるコマンドサーフェス

生成ホストプロンプトを利用するコマンドは、すべてのツールでコマンドサーフェスを提供します。

- Claude、Copilot、Cursor、Kilo、OpenCode、Pi、Windsurf 上のプロンプトファイルおよびワークフローファイル。
- Codex、Kiro、Qwen 上のコマンドごとのスキル。

`slipway tool` のような CLI 専用のヘルパー名前空間は、Slipway バイナリ内では公開されたままですが、ホストコマンドのラッパーは生成しません。生成されたスキルが `slipway tool ...` サブコマンドを直接呼び出します。

生成されるフックは `slipway` バイナリ以外に依存しません。手動のヘルパーコマンドは、明示的に認証済みのバックエンドやドメインツールを利用できます。GitHub ヘルパーは `gh` を優先し、`gh` が利用できない場合や認証エラーを報告した場合はトークン API にフォールバックし、どちらのバックエンドも存在しない場合はフェイルクローズドします。

コアのライフサイクルコマンド:

- `new`（`$slipway-new`）
- `intake`（`$slipway-intake`）
- `plan`（`$slipway-plan`）
- `implement`（`$slipway-implement`）
- `review`（`$slipway-review`）
- `fix`（`$slipway-fix`）
- `done`（`$slipway-done`）
- `next`（`$slipway-next`）
- `run`（`$slipway-run`）
- `status`（`$slipway-status`）

ディスカバリーコマンド:

- `codebase-map`（`$slipway-codebase-map`）

状況依存コマンド:

- `preset`（`$slipway-preset`）
- `validate`（`$slipway-validate`）
- `abort`（`$slipway-abort`）
- `cancel`（`$slipway-cancel`）
- `delete`（`$slipway-delete`）
- `repair`（`$slipway-repair`）
- `evidence`（`$slipway-evidence`。wave-orchestration ホストは `slipway evidence task ...` でタスクのエビデンスを記録します）

ヘルパー:

- `tool` は CLI 専用です。`$slipway-tool` や生成ホストプロンプトのラッパーは存在しません。生成されたスキルが `slipway tool <helper>` を直接呼び出します。

診断コマンド:

- `health`（`$slipway-health`）
- `instructions`（`$slipway-instructions`）

セットアップコマンド:

- `init`（`$slipway-init`）

ワークフロースキルのコマンドリファレンスが、生成されたコマンドサーフェスをインデックス化します。CLI 専用のヘルパーについては、生成されたスキル指示が示す明示的な `slipway tool ...` コマンドを使用してください。

## サーフェスマニフェスト

`docs/SURFACE-MANIFEST.json` は、生成されるアダプター、コマンド、スキル、JSON、ドキュメントの各サーフェスについてコミットされたインベントリです。手動で編集するものではなく、Slipway 所有の Go オーソリティから再生成されます。

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

生成されるツール、コマンド、スキル、JSON コントラクト、ドキュメントサーフェスを追加したら `--write` を実行し、マニフェストの行が指すファイル内に対応するドキュメントトークンを維持してください。

## OpenCode に関する注意

OpenCode はプロジェクトコマンドを `.opencode/commands/` 配下の Markdown ファイルとして保存します。Slipway は次の場所にフラットな OpenCode コマンドファイルを生成します。

```text
.opencode/commands/
```

コマンドファイル名がそのまま OpenCode のコマンド ID になります。たとえば次のファイルは、

```text
.opencode/commands/slipway-new.md
```

次のように呼び出します。

```text
/slipway-new
```

一部の OpenCode ビルドは、コマンドピッカーでプロジェクトコマンドにプロジェクト接頭辞を付けて表示します。生成されるファイルパスは、安定した Slipway のコントラクトとして維持されます。

生成される OpenCode スキルは次の場所に置かれます。

```text
.opencode/skills/
```

また、OpenCode には `settings.json` がないため、アドバイザリのセッションフックはプラットフォームネイティブのランチャーファイルとして生成されます。

```text
.opencode/hooks/slipway-session-start
.opencode/hooks/slipway-session-start.ps1
.opencode/hooks/slipway-session-start.cmd
```

Cursor も同じパターンに従い、`.cursor/hooks/slipway-session-start` と `.ps1`・`.cmd` のコンパニオンを提供します。これらのランチャーは `slipway hook ...` へ委譲するだけで、フックの動作は Slipway バイナリ内にあります。

## アダプターに関する補足

Copilot はコマンドプロンプトを `.github/prompts/` に `.prompt.md` 拡張子で保存し、生成されるスキルを `.github/skills/` に保存します。センチネルと所有権マニフェストは `.github/copilot/slipway` 配下に置かれます。

Pi はコマンドプロンプトを `.pi/prompts/` に、生成されるスキルを `.pi/skills/` に保存します。Slipway は `.pi/settings.json` もマージし、`enableSkillCommands` を `true` に、`./skills` を `skills` に、`./prompts` を `prompts` にそれぞれ記載します。

Qwen と Kiro は、コマンドを別個のプロンプトファイルではなく、生成されたコマンドスキルとして公開します。Qwen はセッションスタートフック用に `.qwen/settings.json` を書き込みます。Kiro のコマンドスキルは `@slipway:<command>` を使用します。

Windsurf と Kilo は、コマンドを `.windsurf/workflows/` および `.kilocode/workflows/` 配下のワークフローファイルとして公開します。Kilo はワークフローファイルが `slipway-<command>.md` という名前であっても、`/slipway:<command>` トリガーを使用します。

## 設定対応ホスト

Claude（`.claude/settings.json`）と Qwen（`.qwen/settings.json`）は、ランチャースクリプトを介さず、自身の設定ファイルにフックをインラインで登録します。Slipway は `settings.json` に `slipway hook ...` コマンドをそのまま書き込みます。

- `PostToolUse` 上の `slipway hook context-pressure`
- `SessionStart` 上の `slipway hook session-start`

Claude は両方のフックを登録します。Qwen は `SessionStart` のみを登録します。これらの設定登録型フックにはランチャーファイルは生成されず、コマンドは `PATH` 上の `slipway` バイナリを解決し、フックの動作はそのバイナリ内にあります。Pi の設定はスキルとプロンプトの登録専用で、フック設定は含みません。

## 安全に関するルール

- 意図してローカルのホスト動作をカスタマイズする場合を除き、生成された Slipway アダプターファイルを編集しないでください。
- Slipway の変更後は、`slipway init --refresh` を使って生成済みファイルを更新し、Slipway 所有の廃止済みアダプターエントリを削除してください。
- 隣接する AI ツールディレクトリ内のユーザー所有ファイルは保持してください。
- リポジトリをすべての貢献者向けに初期化すべき場合は `.slipway.yaml` をコミットしてください。生成されたアダプターファイルは、コミット前にリポジトリのポリシーに従ってレビューしてください。
