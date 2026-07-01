# アダプターのインストールと更新の方法

現在のリポジトリで動作する Slipway バイナリと、生成された AI ツールサーフェスが必要なときは、このガイドを参照してください。

リリースマトリックス、チェックサム、コンテナイメージ、パッケージマネージャーのチャネル、ソースビルドの詳細については、[インストール](../installation.md)を参照してください。

## CLI のインストール

可能な限りリリース提供のチャネルを使ってください。

| プラットフォーム | 推奨パス |
| --- | --- |
| macOS | `brew install --cask signalridge/tap/slipway` |
| Windows | `scoop bucket add signalridge https://github.com/signalridge/scoop-bucket`<br>`scoop install slipway` |
| Linux | [インストール](../installation.md#linux)の `.deb`、`.rpm`、`.apk`、`tar.gz`、AUR、またはコンテナイメージのパスを使います。 |

リリースパッケージが利用できない場合や、意図的に Go 管理のバイナリが欲しい場合は Go install を使います。

```bash
go install github.com/signalridge/slipway@latest
```

インストールを確認します。

```bash
slipway --help
```

AI ツールが未知のレジストリで同名のパッケージを見つけた場合は、いったん作業を止め、インストールする前に所有者を確認してください。

## リポジトリでの Slipway 初期化

リポジトリのルートで実行します。

```bash
slipway init --tools codex
```

よく使うアダプターの選択肢:

```bash
slipway init --tools claude
slipway init --tools claude,codex,opencode
slipway init --tools all
slipway init --tools none
```

`--tools none` は、ホストアダプターファイルを書き込まずに、ランタイムレイアウトと `.slipway.yaml` を初期化します。

生成されたファイルをコミットする前に、差分を確認してください。

```bash
git status --short
git diff -- .slipway.yaml .claude .codex .cursor .opencode
```

リポジトリで Slipway のデフォルトを共有すべきなら `.slipway.yaml` をコミットします。生成されたアダプターファイルは、リポジトリのポリシーに従ってのみコミットしてください。

## 既存アダプターの更新

自動検出された Slipway 管理のアダプターを更新します。

```bash
slipway init --refresh
```

特定のセットだけを更新します。

```bash
slipway init --tools codex,opencode --refresh
```

サポートされているすべてのアダプターを更新します。

```bash
slipway init --tools all --refresh
```

更新は Slipway が生成したマーカーを検出します。素の `.claude`、`.codex`、`.cursor`、`.opencode` ディレクトリを Slipway 所有とは見なしません。

## ユーザー所有ファイルの保護

更新の差分を受け入れる前に、隣接するホスト設定を確認してください。

```bash
git status --short .claude .codex .cursor .opencode
```

生成されたファイルは CLI が管理します。ユーザー所有のホスト設定、ローカルプロンプト、手動コマンド、Slipway 以外のフックはそのまま保持されるべきです。

更新の出力が Slipway 所有のレガシーなランチャーやプロンプトを削除する場合は、コミットする前に新しい生成サーフェスが存在することを確認してください。Codex のコマンドサーフェスは、現在は次の場所にあります。

```text
.codex/skills/slipway-<command>/SKILL.md
```

## コマンドまたはスキルのサーフェスを変更した後

コマンド登録、生成スキル、JSON コントラクト、ドキュメントトークンを変更した場合は、サーフェスマニフェストを更新します。

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go test ./internal/toolgen -run SurfaceManifest -count=1
```

マニフェストは Go の権威ソースとドキュメントトークンから導出されます。ジェネレーター自体を修正する場合を除き、生成された行を手で編集しないでください。

## 関連項目

- [AI ツールアダプター](../reference/ai-tools.md)
- [コマンド](../reference/commands.md)
- [復旧とトラブルシューティング](recover-and-troubleshoot.md)
