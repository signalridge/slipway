# コマンドリファレンス

このページは Slipway コマンドの Diataxis リファレンスの入口です。拡張された
オペレーター向けリファレンスは引き続き [コマンド](../commands.md) で参照できます。
このページは、生成されたサーフェスマニフェストを `docs/reference/` 配下に
アンカーしておくためのものです。

ルーティングされるコマンドのほとんどは、構造化された出力が必要なときに `--json` を
サポートします。`slipway validate` と `slipway done` は別個の `--json`
フラグなしで JSON を出力し、`slipway init` はセットアップ専用です。
`slipway config`、`slipway tool`、`slipway hook` は公開 CLI 専用
サーフェスであり、アダプターのプロンプトラッパーは生成されません。

## コマンド一覧

| コマンド | クラス | 用途 |
| --- | --- | --- |
| `slipway new` | mutation | ガバナンス対象の変更を作成する。 |
| `slipway intake` | mutation | インテイクの明確化と承認を実行する。 |
| `slipway plan` | mutation | 計画アーティファクトを作成または修正する。 |
| `slipway implement` | mutation | S2 の実装ウェーブオーケストレーションを実行する。 |
| `slipway review` | mutation | S3 のレビュー収束を実行する。 |
| `slipway fix` | mutation | S3 の指摘に対する修正をディスパッチする。 |
| `slipway done` | mutation | done-ready の変更をアーカイブする。 |
| `slipway next` | query | 前進せずに次のスキルやブロッカーを確認する。 |
| `slipway run` | mutation | 停止条件に達するまで現在のステージを進める。 |
| `slipway status` | query | ライフサイクルの状態と次のアクションを表示する。 |
| `slipway codebase-map` | mutation | 永続的なリポジトリスコープのコンテキストを作成または更新する。 |
| `slipway handoff` | mutation | 変更ごとの参考用継続メモを書き込む、または表示する。 |
| `slipway preset` | mutation | アクティブなプリセットを確認または変更する。 |
| `slipway validate` | query | 前進せずにレディネスを再計算する。 |
| `slipway abort` | mutation | アクティブな実行セッションを中断する。 |
| `slipway cancel` | mutation | アクティブな変更をキャンセルしてアーカイブする。 |
| `slipway delete` | mutation | 放棄されたガバナンス対象のローカル状態を破棄する。 |
| `slipway repair` | mutation | 範囲を限定したローカルの整合性修復を実行する。 |
| `slipway evidence` | mutation | サポートされているタスクまたはスキルのエビデンスを記録する。 |
| `slipway tool` | mutation | 生成されたスキルが使う CLI 専用のヘルパーツールを実行する。 |
| `slipway hook` | mutation | 生成されたアダプター設定が使う CLI 専用のホストフックヘルパーを実行する。 |
| `slipway health` | query | リポジトリローカルの整合性の指摘を表示する。 |
| `slipway instructions` | query | アーティファクトまたは codebase-map のオーサリング契約を表示する。 |
| `slipway init` | mutation | ランタイムレイアウトとオプションのアダプターを初期化する。 |
| `slipway config` | mutation | リポジトリレベルの設定キーを確認・設定する。 |

## JSON サーフェストークン

生成されたサーフェスマニフェストは、すべての JSON 契約がドキュメント内で見つかる
ことを確認します。そのため、以下の例はそのままの形で残しています。

```bash
slipway new --json
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway next --json
slipway run --json
slipway status --json
slipway handoff show --json
slipway validate
slipway done
slipway codebase-map --json
slipway preset <level> --json
slipway abort --json
slipway cancel --json
slipway delete --change <slug> --json
slipway repair --json
slipway evidence task --result-file task-result.json [--result-file next-task-result.json ...] --json
slipway evidence skill --skill <name> --verdict pass --json
slipway evidence skill --skill <selected-review-skill> --verdict pass --refresh-current --reference "context_origin:stage=review=<handle>" --notes-file artifacts/changes/<slug>/verification/<selected-review-skill>-notes.md --json
slipway health --json
slipway instructions <artifact> --json
slipway config list --json
```

ブロッカーの詳細、アーティファクトのレディネスの詳細、トランジショントレースが
必要なときは、`next` や `run` に `--diagnostics` を付けて
ください。

## サブコマンドとモードの要点

- `slipway handoff write` は stdin から参考用の継続メモを書き込みます。bare 形式では完全な `## Current Position` 本文を pipe し、`--section <name>` を渡すと stdin から指定セクションだけを置き換えます。
- `slipway handoff show --json` は現在の変更ごとの handoff を構造化して出力します。
- `slipway evidence task --result-file <path> --json` はコンパクトな実行タスク結果を取り込みます。原子的なバッチにするには `--result-file` を繰り返します。
- `slipway evidence skill --skill <name> --verdict pass --json` は、そのスキルを所有するステージで統制スキルのエビデンスを記録します。
- `slipway evidence skill --skill <selected-review-skill> --verdict pass --refresh-current --reference "context_origin:stage=review=<handle>" --notes-file artifacts/changes/<slug>/verification/<selected-review-skill>-notes.md --json` は、選択済み S3 レビュースキルの既に current な passing エビデンスを、意図的な再実行として置き換える場合だけに使います。通常の重複エビデンスは引き続き拒否されます。
- `slipway status --stats --json` は、廃止されたトップレベルの `stats` コマンドを戻さずにワークスペース診断を報告します。
- `slipway health --doctor --json` は、ヘルスレポートに修復向けの診断を追加します。
- `slipway tool <helper>` は生成されたスキルから直接呼び出され、生成されたアダプターのプロンプトサーフェスはありません。
- `slipway hook session-start` は生成されたホストフック設定から直接呼び出されます。フックヘルパーは、ホストの自動フックがユーザーをブロックしないよう静かに失敗します。
- `slipway config`、`slipway config list --json`、`slipway config list --env [--json]`、`slipway config get <key> --json`、`slipway config set <key> <value>` は `.slipway.yaml` キーとランタイム/シークレット環境変数サーフェスを確認します。更新できるのはファイル設定キーだけです。`config` は意図的に CLI 専用であり、生成されたアダプターのプロンプトサーフェスはありません。
- `subagents.*` config は `plan_audit`、`executor`、`review`、`fix`、`verify` の slot-based delegation target を制御します。詳しくは [Subagent 設定](subagents.md) を参照してください。

## 読み取り専用サーフェス

以下のコマンドは、ライフサイクル状態を変更せずに状態を確認します。

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
```

mutation コマンドを選ぶ前に、これらを使ってください。

## 状態を変更するステージコマンド

以下のコマンドは、ガバナンス対象の状態を前進または変更できます。

```bash
slipway intake --json
slipway plan --json
slipway implement --json
slipway review --json
slipway fix --json
slipway done
slipway run --json --diagnostics
```

mutation がフェイルクローズドになった場合は、現在の読み取り専用チェックを再実行し、
指定された復旧コマンドに従ってください。

設定レベルの `execution.auto` は `intake`、`plan`、`implement` に適用されます。
これらのステージコマンドは、呼び出しごとの `--auto` や `--no-auto` の
オーバーライドフラグを受け付けません。1 回の実行だけオーバーライドしたいときは、
`slipway run --auto` または `slipway run --no-auto` を使ってください。

## run の自動モード

`slipway run` は、成功した前進の直後にルーティンな
`run_slipway_run_to_advance` コマンド境界を続けて越えられます。これにより、
同じ `slipway run` をもう一度実行してもらうためだけの停止を避けられます。
リポジトリごとに `execution.auto` 設定で有効にするか、1 回の呼び出しだけ
オーバーライドできます。

```bash
slipway run --json --auto
slipway run --json --no-auto
```

`--auto` と `--no-auto` は、その 1 回の実行に限り `execution.auto` 設定より
優先されます。auto では、Slipway はルーティンな run-to-advance コマンド境界だけを
続けて越え、保留中のワークフロープリセットのアップグレードのみを自動で承認します
（ダウングレードは決して行いません）。ガバナンススキルの実行、レビューバッチの
ディスパッチ、エビデンス記録、インテイクの Approved Summary 承認、done-ready 変更の
finalize は行いません。非センシティブなスキルハンドオフやレビューバッチは
`next --json` で `evidence_continuation` として報告され、事前認可で十分と示される
ことがありますが、run/stage ループは host がスキルまたはレビューを実行して
エビデンスを記録するために停止します。`security-review` の境界、機微および
ガードレールの確認、そしてすべてのエビデンスゲートは依然としてハードストップであり、
自動では進められません。

## サーフェスマニフェスト

`docs/SURFACE-MANIFEST.json` は、Slipway が所有する Go の権威ソースから再生成
されます。

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --write
go run ./internal/toolgen/cmd/gen-surface-manifest --check
```

コマンド、JSON 出力契約、またはドキュメントに面したサーフェスを追加・変更する
ときは、そのトークンをマニフェスト行の `docs` ファイルに残してください。

## 詳細

詳細なコマンドリファレンスは引き続き [コマンド](../commands.md) にあり、作成
オプション、ディスカバリーコマンド、診断、出力フラグ、よく使う JSON 呼び出しを
含みます。
