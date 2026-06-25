# はじめに

このページは、「リポジトリがある」状態から「Slipway が実際の変更を 1 つ統制している」状態までの最短ルートです。

Slipway は、いくつかの永続的なプロジェクト記録を中心に構築されています。

| Slipway のサーフェス | 役割 |
| --- | --- |
| 統制された変更 | `artifacts/changes/<slug>/` 配下にある、境界が定められた 1 つの作業単位。 |
| コードベースマップ | ブラウンフィールド作業向けに `artifacts/codebase/` 配下で共有されるリポジトリのコンテキスト。 |
| タスクエビデンス | `.git/slipway/runtime/changes/<slug>/evidence/` 配下にあるランタイムの証跡。 |
| レビューエビデンス | 現在のワークトリーと一致しなければならない、新鮮な検証記録。 |
| AI アダプター | エージェントを Slipway CLI へ戻すよう経路づける、生成されたホストファイル。 |

権威は CLI にあります。AI ツールはアーティファクトの作成やステージの実行を支援できますが、ライフサイクルの状態を勝手に作り出したり、エビデンスを手作業で編集したりしてはいけません。

## ルートを選ぶ

| 状況 | ここから始める |
| --- | --- |
| Slipway は初めてで、小さなエンドツーエンドの実行を試したい。 | [最初の統制された変更](tutorials/first-governed-change.md) |
| すでに振る舞いを持つリポジトリに Slipway を追加する。 | [既存コードベースのオンボーディング](tutorials/onboarding-existing-codebase.md) |
| インストールとアダプターのコマンドだけが必要。 | [インストールとアダプターの更新](how-to/install-and-refresh-adapters.md) |
| 変更が行き詰まったり、古くなったり、分かりにくくなったりしている。 | [リカバリーとトラブルシューティング](how-to/recover-and-troubleshoot.md) |
| 設計を評価している。 | [設計](explanation/design.md) と [ワークフロー](explanation/workflow.md) |

具体的な導入パターンについては、[実世界のシナリオ](real-world-scenarios.md)を参照してください。

## 最初のインストール

利用中のプラットフォームに対応したインストール方法を選びます。代表的な選択肢は次のとおりです。

```bash
brew install --cask signalridge/tap/slipway
go install github.com/signalridge/slipway@latest
```

続いて、バイナリが認識されているか確認します。

```bash
slipway --help
```

完全なプラットフォーム対応表、リリースアーカイブのパス、チェックサムの検証、ソースからのビルド手順は[インストール](installation.md)に記載しています。

## リポジトリを初期化する

リポジトリのルートで次を実行します。

```bash
slipway init --tools codex
```

実際に使用するツール ID を指定します。

```bash
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`slipway init` は `.slipway.yaml` と、必要に応じて生成される AI ツールアダプターを書き出します。アダプターは利便性のためのサーフェスであり、権威は引き続き CLI にあります。

## 統制された変更を 1 つ始める

Slipway を手作業で動かす必要はありません。AI ツールのセッションで、変更内容を平易な言葉で記述します。

> README に短い使い方メモを追加して。

`slipway init` が生成したアダプターが、その要求を統制されたライフサイクルへ経路づけます。エントリースキルが変更を引き継ぎ、エージェントがあなたに代わって `slipway` の各ステージ（インテーク、プランニング、実装、レビュー、done ゲート）を実行します。Slipway がスキルのハンドオフ、チェックポイント、ブロッカー、または done-ready 状態を提示し、あなたの対応が必要になったときだけ停止します。

変更が実際に完了したかを判断するのはエージェントではなく Slipway です。エージェントが読んでいるのと同じ状態を確認したいときは、読み取り専用のサーフェスを使います。

```bash
slipway status --json
slipway next --json --diagnostics
```

### 自分で動かす（任意）

ライフサイクルを手作業で実行したい場合は、同じ変更を平易なコマンドで作成して進められます。

```bash
slipway new "add a short usage note to README" --profile docs --preset standard
slipway run --json --diagnostics
```

`slipway run` は Slipway が所有するステージだけを進め、オペレーターと向き合う各境界で停止します。スキルのハンドオフが返ってきた場合は、そのハンドオフを AI ツールで完了させ、続行する前に読み取り専用コマンドを再実行してください。

## フェイルクローズしたとき

フェイルクローズの出力は機能のひとつです。Slipway が証跡の欠落や陳腐化を検知し、次に取るべき安全なアクションを示したことを意味します。

次の順序で進めます。

```bash
slipway status --json
slipway validate --json
slipway next --json --diagnostics
slipway health --doctor --json
```

その後、示されたリカバリーコマンドに従います。`change.yaml`、検証 YAML、タスクエビデンス、ライフサイクルのタイムスタンプを手作業で編集してはいけません。エビデンスが陳腐化している場合は、それを所有するステージ、レビュアー、またはタスクエビデンスのパスを再実行し、Slipway が現在のワークトリーから新鮮さを再導出できるようにします。

## さらに進める

- コピー＆ペーストで試せる最初の実行は、[最初の統制された変更](tutorials/first-governed-change.md)に従ってください。
- リポジトリにエージェントが学ぶべき慣習がすでにある場合は、[既存コードベースのオンボーディング](tutorials/onboarding-existing-codebase.md)に従ってください。
- 正確なコマンドと JSON サーフェスの詳細が必要なときは、[コマンド](reference/commands.md)を使ってください。
