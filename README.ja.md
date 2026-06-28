<div align="center">

<img alt="Slipway - AI 支援によるソフトウェアデリバリーのためのガバナンス CLI" src="docs/assets/brand/slipway-wordmark.svg" width="480">

<br/>
<br/>

<p>
  <a href="https://github.com/signalridge/slipway/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/ci.yml?branch=main&style=for-the-badge&logo=github&label=CI"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/actions/workflows/docs.yml"><img alt="Docs" src="https://img.shields.io/github/actions/workflow/status/signalridge/slipway/docs.yml?branch=main&style=for-the-badge&logo=astro&label=Docs"></a>&nbsp;
  <a href="https://github.com/signalridge/slipway/releases"><img alt="Release" src="https://img.shields.io/github/v/release/signalridge/slipway?style=for-the-badge&logo=github"></a>&nbsp;
  <a href="https://pkg.go.dev/github.com/signalridge/slipway"><img alt="Go Reference" src="https://img.shields.io/badge/Go-Reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
</p>

<p>
  <a href="docs/installation.md"><img alt="Homebrew Cask" src="https://img.shields.io/badge/Homebrew_Cask-FBB040?style=flat-square&logo=homebrew&logoColor=black"></a>
  <a href="docs/installation.md"><img alt="Scoop" src="https://img.shields.io/badge/Scoop-00BFFF?style=flat-square&logo=windows&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="AUR" src="https://img.shields.io/badge/AUR-1793D1?style=flat-square&logo=archlinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Nix" src="https://img.shields.io/badge/Nix-5277C3?style=flat-square&logo=nixos&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Docker" src="https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="deb" src="https://img.shields.io/badge/deb-A81D33?style=flat-square&logo=debian&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="rpm" src="https://img.shields.io/badge/rpm-EE0000?style=flat-square&logo=redhat&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="apk" src="https://img.shields.io/badge/apk-0D597F?style=flat-square&logo=alpinelinux&logoColor=white"></a>
  <a href="docs/installation.md"><img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
</p>

[ドキュメント](https://signalridge.github.io/slipway/) |
[ここから始める](docs/start-here.md) |
[クイックスタート](#クイックスタート) |
[インストール](docs/installation.md) |
[リリースノート](CHANGELOG.md)

<br/>

[English](README.md) · [简体中文](README.zh.md) · **日本語**

</div>

# Slipway

**AI 支援によるソフトウェアデリバリーのための、ローカルで動作する Git ネイティブな
ガバナンス CLI。コードを書くのはエージェント、変更が本当に完了したかを判断するのは
Slipway です。**

AI コーディングエージェントは高速ですが、検証を飛ばしたり、計画から逸脱したり、
現在のワークトリーで証明される前に「完了」と報告したりすることがあります。Slipway
は 1 つの作業単位を、ライフサイクル状態・計画成果物・タスク証跡・レビュー証跡、
そしてリポジトリに残る最終アーカイブを備えたガバナンス対象の変更へと変えます。

Slipway はホスティングサービスでもプロジェクト管理ツールでもなく、AI コーディング
ツールの代替でもありません。エージェントの作業を可視化し、フェイルクローズドにする
コントロールプレーンです。

その核心的な強みは、もう 1 つチェックリストを増やすことではありません。Slipway は
現在のワークトリー、生成されたホスト向け指示、ライフサイクル状態、レビューコンテキスト
を分離したうえで、それらがいまも整合しているかを CLI に再計算させます。

## なぜ Slipway なのか

| 機能 | 何が変わるか |
| --- | --- |
| **コンパイルされた完了ゲート** | `slipway done` はアーカイブ前に、現在のレビュー・検証・スコープ・ガードレールの証明を再チェックします。証跡が欠落または陳腐化していると、ファイナライズはブロックされます。 |
| **薄い AI アダプター** | 生成されるホストアダプターファイル（Claude、Codex、Cursor、OpenCode、Copilot、Kilo、Kiro、Pi、Qwen、Windsurf）は、独立したワークフローエンジンになるのではなく、エージェントを CLI へ送り返します。 |
| **平易な言葉での入口** | `slipway init --tools <id>` の後は、変更を普通の言葉で記述できます。生成された入口スキルが、エージェントをガバナンス対象のライフサイクルへ案内します。 |
| **現在のワークトリーが権威** | `status`、`validate`、`next` は、陳腐化したサマリーやアーカイブ済みの記録を信用せず、所有元のワークトリーから状態を再計算します。 |
| **コンテキスト分離チェック** | 計画監査、実装、選定された S3 レビューピア、修復、そして終端の `ship-verification` ゲートは、それぞれ異なるコンテキスト由来の証跡と順序チェックを備えます。 |
| **ワークトリー束縛の実行** | 調査負荷の高い変更は専用の `.worktrees/<branch>` チェックアウトで実行できます。ワークトリーのパスとブランチの束縛は、実行を続ける前に検証されます。 |
| **実際の編集に基づくウェーブ監査** | 依存順に並んだウェーブは並列実行できます。実装後、Slipway は実際に変更されたファイル、エグゼキューターのハンドル、ディスパッチモード、スコープの封じ込めを監査します。 |
| **リポジトリ所有の監査証跡** | `artifacts/changes/`、`.git/slipway/runtime/`、ライフサイクルイベント、アーカイブ済みバンドルにより、セッション終了後も記録を検査できます。 |

## クイックスタート

Slipway をインストールし、リポジトリを初期化して、実際に使っている AI ツール向けの
アダプターを生成します。

```bash
brew install --cask signalridge/tap/slipway
# or
go install github.com/signalridge/slipway@latest

slipway init --tools codex
```

その他のアダプター ID は `claude`、`codex`、`cursor`、`opencode`、`copilot`、
`kilo`、`kiro`、`pi`、`qwen`、`windsurf`、`all`、`none` です。

この一度きりのセットアップがインストールのすべてです。以降は Slipway を手作業で
操作しません。AI ツールのセッションで、変更を平易な言葉で記述します。

> エクスポートコマンドに `--dry-run` モードを追加して。

`slipway init` が生成したアダプターが、そのリクエストをガバナンス対象のライフ
サイクルへ送り込みます。入口スキルが変更を引き取り、エージェントが `slipway` の
インテーク、計画、実装、レビュー、完了ゲートをあなたの代わりに実行します。停止する
のは、Slipway がスキルハンドオフ、チェックポイント、ブロッカー、あるいはあなたの
判断を必要とする完了準備済み状態を返したときだけです。

あなたは平易な言葉のまま、Slipway は変更が本当に完了したかどうかの権威であり続けます。
コマンドの順序を暗記したり、ライフサイクル状態を頭の中に抱え込んだりする必要は
ありません。それはエージェントの仕事であり、CLI が裏付けます。読み取り専用の
サーフェスは、エージェントが見ているものを確認したいときにいつでも使えます。

```bash
slipway status --json
slipway next --json --diagnostics
```

セッションをまたいだ継続には、コマンドが所有するアドバイザリーなハンドオフを使います。

```bash
slipway handoff write
printf '現在の実装コンテキスト...\n' | slipway handoff write --section "現在位置"
slipway handoff show --brief
slipway handoff show
```

`slipway handoff write` は、変更ごとのランタイムハンドオフのスケルトンとマシン
ヘッダーを更新します。ヘッダーが持つのは識別情報と現在性を示すフィールドのみで、ライフ
サイクル状態や次のアクションをスナップショットすることはありません。新しいセッション
では、現在の状態を確認するために引き続き `slipway status --json` と
`slipway next --json` を実行します。

<details>
<summary><strong>コマンド主導のライフサイクル</strong></summary>

自分でライフサイクルを動かしたい場合や、CI でスクリプト化したい場合はどうでしょう。
以下は、エージェントがあなたの代わりに実行しているのと同じコマンドを、直接利用
できるように公開したものです。

```bash
slipway new "refresh governance docs" --preset standard
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway validate
slipway done
```

`slipway run --json --diagnostics` はショートカットドライバーです。現在の主要な
ステージコマンドへ委譲し、オペレーターと向き合う境界で停止します。

### 実行の auto モード

`.slipway.yaml` の `execution.auto` は**デフォルトでオフ**です。オプトインした
場合（または `slipway run --auto` でラン単位で上書きした場合）、`slipway run` は
純粋なペーシング上の一時停止——レビューバッチ、機微でないスキルハンドオフ、
そして現在の入力に合った human_action チェックポイント——を、事前の認可に基づいて、
改めての確認で停止することなく自動で進めます。`slipway run --no-auto` は単一の
ランを手動ペーシングへ戻します（`--no-auto=false` は積極的な上書きではなく、
設定にフォールスルーします）。

設定レベルの `execution.auto` は、ステージコマンド（`slipway intake` /
`slipway plan` / `slipway implement`）にも適用され、`run` と一貫して自動で
進みますが、ステージ単位のフラグは公開しません。ラン単位の `--auto` /
`--no-auto` の上書きは `slipway run` にのみ存在します。

auto モードがガバナンスを緩めることはありません。`security-review` の境界、
機微／ガードレール確認、インテークの Approved Summary、decision および
human_action のチェックポイント、陳腐化または現在性が確認できないチェックポイント、
そしてあらゆる証跡ゲートは、**決して**自動で進められません。これらは常に
ハードストップし、明示的なオペレーター入力と現在の入力に合った証跡を求めます。アップグレード
専用プリセットの自動確認は、ガバナンスの厳格さを引き上げるだけで（決して下げ
ません）、これらのレッドラインには該当しません。

</details>

## 仕組み

<div align="center">
  <img alt="Slipway のガバナンス対象ライフサイクル: new、S0 Intake、S1 Plan、S2 Implement、S3 Review、done-ready、done" src="docs/assets/diagrams/lifecycle.svg" width="920">
</div>

| ステージ | Slipway が期待するもの |
| --- | --- |
| `S0_INTAKE` | 意図、スコープ、未解決の問い、リスク分類、初期認可。 |
| `S1_PLAN` | 調査、要件、決定、タスク計画、計画監査の証跡。 |
| `S2_IMPLEMENT` | 依存順のウェーブ、変更ファイル、タスク証跡。 |
| `S3_REVIEW` | 選定されたピアレビュー、修復の証跡、そして終端の `ship-verification` ゲート（1 つの権威あるフルスイート、受け入れ証明、現在性の再チェック、アシュアランス、レビュアー独立性の証明）。 |
| `done` | `artifacts/changes/archived/<slug>/` 配下の終端アーカイブ。 |

`change.yaml` が現在のライフサイクル権威を所有します。Markdown の成果物が作業を
説明し、YAML の検証記録が個々のステージを証明し、ライフサイクルイベントが変更の
追記専用トレースを与えます。読み取り専用のサーフェス（`status`、`validate`、
`next`）は、セッションを再開したときや変更がわかりにくいときに、まず最初に見る
場所です。主要なミューテーションのサーフェスは `slipway new`、`slipway intake`、
`slipway plan`、`slipway implement`、`slipway review`、`slipway fix`、
`slipway done`、そしてショートカットドライバーの `slipway run` です。

## 設計思想

Slipway は 3 つのプロジェクトルールに従います。

- **唯一の現在の権威。** `change.yaml` がライフサイクル状態を所有します。ログと
  Markdown はそれを補助しますが、決して置き換えません。
- **分離されたコンテキストを、後でチェックする。** 作成、監査、レビュー、修復、
  ship-verification の証跡は、それぞれ別個の参加者として記録されます。ゲートは
  独立性の連鎖が崩れていないかをチェックします。
- **人間が読め、機械が検査できる。** 人は成果物をレビューでき、CLI は構造化された
  入力から現在の入力との一致を再計算します。
- **最小限の有用なコントロールプレーン。** ホストアダプターは薄く保ち、ガバナンスは
  CLI とリポジトリの成果物に宿ります。

短めの説明は [Design](docs/explanation/design.md) と
[Workflow](docs/explanation/workflow.md) を、より掘り下げた旧来の解説は
[Design Philosophy](docs/design.md) と
[Governed Workflow](docs/workflow.md) をご覧ください。

<details>
<summary><strong>深い強制の軸</strong></summary>

ゲートの裏側で、すべてのステージは、エンジンが信用せずに再導出する証跡を所有します。
これらは、偽装された「完了」を失敗させる実装上の軸です。

| 軸 | エンジンの挙動 |
| --- | --- |
| 現在性が証明されたコンテキスト | レビュー、計画監査、修復、クローズアウトの各記録は、異なるコンテキスト由来の証跡と順序チェックを備えます。 |
| 改ざん検知可能な証跡 | 現在の入力との一致は、変更ファイル、成果物、ラン・サマリーのバージョン、終端の `ship-verification` スイート実行、ランタイムのタスク証跡から導出されます。`pass` と書かれたファイルからではありません。 |
| 両面の並列安全性 | 計画上ファイルが互いに素なウェーブの後に、実際に変更されたファイル、エグゼキューターのハンドル、ディスパッチモード、スコープ契約の監査が続きます。 |
| スコープの封じ込め | `target_files` と開示された例外は、実際の差分に対して照合されます。レーン外の編集はフェイルクローズドになります。 |
| ドリフトを認識する回復 | 計画や証跡のドリフトは変更をその場で再オープンし、`slipway next` が前進のための修復コマンドを示します。 |
| ローカルファースト監査 | アクティブおよびアーカイブ済みの記録はリポジトリに残り、ランタイムの証明は `.git/slipway/runtime/` 配下にあります。 |
| リスク階層ガードレール | 機微なドメインでは、出荷承認の前にドメインを意識したレビュー、高リスクチェック、明示的な証跡が必要です。 |

</details>

## 他ツールとの比較

多くの AI ワークフローシステムは、作業を構造化することに長けています。Slipway の
より絞り込んだ賭けは強制です。最終的なライフサイクル権威は、リポジトリの証跡から
状態を再計算する決定論的な CLI に宿ります。

<details>
<summary><strong>隣接ツールとトレードオフ</strong></summary>

| ツール | 操作方法 | 完了の強制 |
| --- | --- | --- |
| [Spec Kit](https://github.com/github/spec-kit) | `/speckit.*` スラッシュコマンド | アドバイザリーなチェックリストとフェーズプロンプト。 |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | `/opsx:*` スラッシュコマンド | 柔軟なスペックワークフロー。検証は任意。 |
| [spec-kitty](https://github.com/Priivacy-ai/spec-kitty) | `/spec-kitty.*` コマンドとオートパイロット | 一部のステータスゲートはあるが、レビューはアドバイザリーのまま。 |
| [GSD Core](https://github.com/open-gsd/gsd-core) | ランタイムサーフェスと `/gsd-*` フェーズコマンド | 強力なフェーズループと新しいコンテキストのオーケストレーション。ただし最終的な証明は依然としてワークフロー成果物を介する。 |
| [superpowers](https://github.com/obra/superpowers) | 自動発火するスキル | 強力なエージェント規律。ただしルールはモデルのコンテキストに宿る。 |
| **Slipway** | 薄いアダプター越しの平易な言葉、または直接の CLI | リポジトリの証跡に裏付けられた、コンパイル済みでフェイルクローズドなゲート。 |

Slipway は幅を犠牲にして権威を取ります。広範なプロンプトフレームワークより
ファーストクラスのアダプターは少ないものの、生成される各サーフェスは同じ CLI へ
送り返します。使い捨ての編集では軽量なプロンプトパックより重いですが、陳腐化した
証跡、スコープのドリフト、リスクの高いドメインを見逃しやすい場面では、はるかに
厳格です。

</details>

## AI ツールアダプター

`slipway init --tools <id>` でホストツールのサーフェスを生成し、`slipway init --refresh`
で管理対象ファイルをリフレッシュします。生成されたファイルは所有権が追跡されている
ため、リフレッシュは隣接するユーザー所有のカスタマイズを削除することなく、Slipway
所有のファイルだけを置き換えられます。

<details>
<summary><strong>ツールごとに生成されるサーフェス</strong></summary>

| ツール | 生成されるサーフェス |
| --- | --- |
| Claude | `.claude/skills/slipway-*/SKILL.md`、`.claude/commands/slipway/*.md`、`.claude/settings.json` のフックエントリ |
| Codex | `.codex/skills/slipway-*/SKILL.md` の入口・コマンド・ガバナンススキル、`.codex/config.toml` の SessionStart および UserPromptSubmit フックエントリ |
| Cursor | `.cursor/skills/slipway-*/SKILL.md`、`.cursor/commands/*.md`、セッション開始フックのランチャー |
| OpenCode | `.opencode/skills/slipway-*/SKILL.md`、`.opencode/commands/slipway-*.md`、セッション開始フックのランチャー |
| Copilot | `.github/skills/slipway-*/SKILL.md`、`.github/prompts/slipway-*.prompt.md`、`.github/copilot/slipway` の管理状態 |
| Kilo | `.kilocode/skills/slipway-*/SKILL.md`、`.kilocode/workflows/slipway-*.md` |
| Kiro | `.kiro/skills/slipway-*/SKILL.md` の入口・コマンド・ガバナンススキル |
| Pi | `.pi/skills/slipway-*/SKILL.md`、`.pi/prompts/slipway-*.md`、`.pi/settings.json` のスキル／プロンプト登録 |
| Qwen | `.qwen/skills/slipway-*/SKILL.md` のコマンドスキル、`.qwen/settings.json` のフックエントリ |
| Windsurf | `.windsurf/skills/slipway-*/SKILL.md`、`.windsurf/workflows/slipway-*.md` |

エクスポートされる生成スキルの行は、公開スキルディレクトリで固定されています。
`slipway/SKILL.md`、`slipway-ci-triage/SKILL.md`、
`slipway-code-quality-review/SKILL.md`、`slipway-codebase-mapping/SKILL.md`、
`slipway-coding-discipline/SKILL.md`、`slipway-context-assembly/SKILL.md`、
`slipway-coverage-analysis/SKILL.md`、`slipway-git-recovery/SKILL.md`、
`slipway-incident-response/SKILL.md`、`slipway-independent-review/SKILL.md`、
`slipway-intake-clarification/SKILL.md`、`slipway-plan-audit/SKILL.md`、
`slipway-research-orchestration/SKILL.md`、
`slipway-root-cause-tracing/SKILL.md`、`slipway-security-review/SKILL.md`、
`slipway-ship-verification/SKILL.md`、
`slipway-spec-compliance-review/SKILL.md`、`slipway-spec-trace/SKILL.md`、
`slipway-tdd-governance/SKILL.md`、`slipway-test-design/SKILL.md`、
`slipway-wave-orchestration/SKILL.md`、
`slipway-worktree-preflight/SKILL.md` です。

Codex は、境界づけられた SessionStart のハンドオフポインタと、陳腐化を条件とする
UserPromptSubmit の書き込みナッジに、リポジトリローカルの `.codex/config.toml`
フックを使います。これらのフックは、リポジトリと各フックがユーザーに信頼される
までは不活性です。Slipway がグローバルな Codex の信頼設定を編集することはありません。

正確なコマンドとスキルのインベントリは、[AI Tool Adapters](docs/reference/ai-tools.md)
および生成された [Surface Manifest](docs/SURFACE-MANIFEST.json) を参照してください。

</details>

## ランタイムファイル

<details>
<summary><strong>Slipway が書き込むリポジトリ状態</strong></summary>

| パス | 役割 |
| --- | --- |
| `artifacts/changes/<slug>/change.yaml` | 現在のライフサイクルおよびルーティングの権威。 |
| `artifacts/changes/<slug>/*.md` | 意図、調査、要件、決定、タスク、アシュアランスの記録。 |
| `artifacts/changes/<slug>/verification/` | 出荷ゲートが消費するスキル検証記録。 |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | 追記専用のライフサイクル変更トレース。 |
| `.git/slipway/runtime/changes/<slug>/evidence/` | Git ローカルのタスク証跡とランタイム証明。 |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | `slipway handoff` が書き込み／読み取りする、変更ごとのアドバイザリーな継続メモ。ライフサイクル権威・証跡・現在性・ゲートのいずれでもありません。 |
| `.git/slipway/locks/change-create.lock`、`.git/slipway/locks/repair.lock` | 変更作成と修復のための、ワークスペース／スコープレベルの調整ロック。安定した変更スラグの前または外で始まる操作を保護するため、意図的に変更ごとではありません。 |
| `artifacts/changes/archived/<slug>/` | `slipway done` 後の終端記録。 |
| `artifacts/codebase/` | ブラウンフィールドの計画とレビューに使う、リポジトリスコープのコンテキストマップ。 |
| `.worktrees/<branch>/` | 変更が分離されているときの、専用のガバナンス対象ワークトリー。 |

AI ツールのセッションは、生成されたホストサーフェスをプロジェクトルートから
読み取ります。ガバナンス対象のワークトリーはコード変更を保持しますが、ルートの
ホストアダプターファイルが各ワークトリーにコピーされることはありません。
`.codex/config.toml` に生成された Codex のフックは、リポジトリが信頼され、各フックが
ユーザーに信頼されるまで不活性です。Slipway がグローバルな Codex の信頼設定を編集
することはありません。
`.git/slipway/runtime/handoff.md` のような旧来のリポジトリレベルのハンドオフ
ファイルは、ローカルランタイムの衛生上の所見として報告され、現在の変更の権威として
使われることはありません。

</details>

## ドキュメント

ドキュメントはタスク別に整理されています。

- [Start Here](docs/start-here.md): インストールから 1 つのガバナンス対象変更までの
  最短経路。
- [Real-World Scenarios](docs/real-world-scenarios.md): 導入パターン。
- [First Governed Change](docs/tutorials/first-governed-change.md): ガイド付き
  チュートリアル。
- [Onboarding Existing Codebase](docs/tutorials/onboarding-existing-codebase.md):
  ブラウンフィールドのセットアップ。
- [Install and Refresh Adapters](docs/how-to/install-and-refresh-adapters.md):
  アダプターの運用コマンド。
- [Recover and Troubleshoot](docs/how-to/recover-and-troubleshoot.md):
  フェイルクローズドな回復。
- [Commands](docs/reference/commands.md): コマンドと JSON サーフェスのリファレンス。
- [AI Tool Adapters](docs/reference/ai-tools.md): 生成されるホストファイルと
  呼び出しスタイル。
- [Design](docs/explanation/design.md) と
  [Workflow](docs/explanation/workflow.md): 概念とその根拠。

## 検証

開発中に役立つローカルチェック。

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go run ./internal/testlint/cmd/testlint ./...
golangci-lint run --timeout=5m
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
(cd website && npm run build)
```

CI は、Markdown／YAML／action のリント、Go のリント、Slipway の testlint、
プラットフォーム横断の Go テスト、レーステスト、カーネルカバレッジ、ビルドチェック、
セキュリティスキャン、Nix チェック、そして docs ワークフローを実行します。

## コントリビュート

コントリビューションはフォーク＆プルリクエストのワークフローを通して行います。
コントリビューションの流れは [CONTRIBUTING.md](CONTRIBUTING.md) を、開発の詳細は
[docs/contributing.md](docs/contributing.md) を参照してください。

## ライセンス

Slipway は [BSD 3-Clause License](LICENSE) の下でライセンスされています。

## リポジトリの状態

![Repobeats 分析画像](https://repobeats.axiom.co/api/embed/20e468225cab8a858d9bc969314a0e9c3d12bddb.svg "Repobeats 分析画像")
