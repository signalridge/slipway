# はじめての統制された変更

このチュートリアルでは、ドキュメントのみの小さな変更を Slipway で実行します。目的は README の編集そのものではありません。Slipway がライフサイクル状態、スキルの引き継ぎ、エビデンス、レビュー、そしてフェイルクローズドな復旧をどう公開するかを体験することが目的です。

## このチュートリアルで作るもの

使い捨ての README に、短い「Usage」セクションを 1 つ追加します。

## 前提条件

- 動作する `slipway` バイナリ。
- Git がインストールされていること。
- 生成された Slipway のスキルまたはコマンドサーフェスを読み取れる AI コーディングツール（Codex、Claude、Cursor、OpenCode など）。

インストール済みのバイナリではなく Slipway のソースチェックアウトから作業する場合は、`slipway` をそのチェックアウトでの `go run .` に置き換えてください。

## ステップ 1: チュートリアル用リポジトリを作成する

使い捨てのディレクトリを使います。

```bash
mkdir slipway-first-change
cd slipway-first-change
git init
printf '# Slipway First Change\n\nA tiny repo for trying Slipway.\n' > README.md
git add README.md
git commit -m "chore: initial readme"
```

## ステップ 2: Slipway を初期化する

使っている AI ツールに合わせてアダプターを選びます。

```bash
slipway init --tools codex
```

その他の例:

```bash
slipway init --tools claude
slipway init --tools opencode
slipway init --tools all
```

リポジトリの状態を確認します。

```bash
git status --short
```

`.slipway.yaml` と生成されたアダプターは、チームでバージョン管理したい場合にのみコミットしてください。このチュートリアルでは、内容を確認するまで生成ファイルをステージせずに残しておいて構いません。

## ステップ 3: 統制された変更を作成する

```bash
slipway new "add a small README usage note" --profile docs --preset standard
```

アクティブな変更を確認します。

```bash
slipway status --json
slipway next --json --diagnostics
```

JSON を現在の権威として読み取ってください。次に実行すべきスキル、ブロッカー、コマンドが示されます。次のステージを記憶から推測してはいけません。

## ステップ 4: AI にインテークを作成させる

チュートリアル用リポジトリから、次の内容を AI コーディングツールに貼り付けます。

```text
Use the active Slipway change. Inspect `slipway next --json --diagnostics`.
Complete only the surfaced intake or artifact-authoring handoff. The objective
is to add one README Usage section later; do not edit README.md during intake.
Do not edit change.yaml, lifecycle events, verification records, or runtime
evidence by hand.
```

AI が引き継ぎ完了を報告したら、もう一度確認します。

```bash
slipway status --json
slipway next --json --diagnostics
```

Slipway がアーティファクトの欠落を報告した場合は、示されたコマンドを実行します。例:

```bash
slipway instructions requirements --json
```

instructions コマンドは作成のコントラクトを提供します。AI は実際のアーティファクトの内容を書かなければなりません。プレースホルダーのテンプレートをコピーしただけのものは、ゲートによって意図的に拒否されます。

## ステップ 5: プランニングを実行する

CLI サーフェスを使ってプランニングを進めます。

```bash
slipway run --json --diagnostics
```

これがまたスキルの引き継ぎを返した場合は、次のプロンプトを貼り付けます。

```text
Continue the active Slipway change from the current `slipway next --json`
handoff. Author only the required planning artifact. Keep the eventual
implementation scoped to README.md. If the task plan needs target files, use
README.md only.
```

各引き継ぎの後に、読み取り専用の確認を繰り返します。

```bash
slipway validate --json
slipway next --json --diagnostics
```

プランニングが実装の準備完了となるのは、plan-audit のゲートを通過した後だけです。Slipway がフェイルクローズドになった場合は、示されたアーティファクトまたはレビューの復旧手順に従ってください。プランニングのゲートをスキップしてはいけません。

## ステップ 6: README の変更を実装する

Slipway が実装段階に達したら、次のプロンプトを貼り付けます。

```text
Execute the active Slipway implementation handoff. Change only README.md. Add a
short Usage section with a command example that tells readers to run
`slipway status --json` before relying on lifecycle state. Run any targeted
verification command named by the task. Record task evidence only through the
Slipway command or generated execution skill that owns task evidence.
```

意図する README の形は小さなものです。

````markdown
## Usage

Inspect the current governed state before acting:

```bash
slipway status --json
```
````

AI が完了したら、差分を確認します。

```bash
git diff -- README.md
slipway validate --json
slipway next --json --diagnostics
```

検証が `scope_contract_drift` を報告した場合は、タスクの `target_files` 以外のファイルに変更が及んでいます。示された Slipway のパスを通じてスコープを修正するか、プランを修正してください。エビデンスにファイルを隠してはいけません。

## ステップ 7: レビューしてクローズする

Slipway にレビューを実行させます。

```bash
slipway run --json --diagnostics
```

選択されたレビューのエビデンスが欠落していたり古くなっていたりする場合は、`next --json --diagnostics` が示すレビュアーを再実行します。レビューで問題が見つかった場合は次を使います。

```bash
slipway fix --json
```

返された修正コントラクトを、新しい AI コンテキストに渡します。修正後、影響を受けたレビュアーを再実行してください。

状態が done-ready を報告したら:

```bash
slipway done --json
```

続いて、何が変わったかを確認します。

```bash
git status --short
find artifacts/changes -maxdepth 3 -type f | sort
```

これが実際の作業だった場合は、README とアーカイブされた Slipway レコードをまとめてコミットします。

## このチュートリアルで学んだこと

- `status`、`next`、`validate` は読み取り専用の権威チェックである。
- `run` はスキル、ブロッカー、または done-ready 状態に達するまでだけ進める。
- アーティファクトは `slipway instructions` から作成するものであり、テンプレートからコピーするものではない。
- 実装のスコープは `tasks.md` のターゲットファイルから決まる。
- 古いエビデンスは、所有するステージまたはレビュアーを再実行して修復する。
- `done` は統制された準備が整った後にのみ変更をアーカイブする。

## 関連

- [ここから始める](../start-here.md)
- [コマンド](../reference/commands.md)
- [ワークフロー](../explanation/workflow.md)
