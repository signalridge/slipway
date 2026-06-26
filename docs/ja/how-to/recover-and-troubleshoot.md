# リカバリーとトラブルシューティングの方法

Slipway がブロッカー、古い証跡、不足するアーティファクト、アダプターのドリフト、
あるいは紛らわしいローカル状態を報告したときは、このガイドを使ってください。

ルールはシンプルです。まず調査し、その後に示されたリカバリーパスをたどります。
ライフサイクルの権威、証跡の判定結果、タイムスタンプ、ランタイムのタスク証跡を
手で編集してはいけません。

## 変更せずに調査する

これらのコマンドは、リカバリー作業における主要な診断 JSON サーフェスです。

ガバナンス対象のワークトリーから実行します。

```bash
git status --short --branch
slipway status --json
slipway validate --json
slipway next --json --diagnostics
```

ライフサイクルのスナップショットには `status`、ゲートの準備状況には `validate`、
実行可能なブロッカーやスキルのハンドオフには `next --diagnostics` を使います。

## Doctor 出力を実行する

ローカル状態に矛盾があるように見えるとき:

```bash
slipway health --doctor --json
```

`applied_repairs`、`unrepaired_drift`、そして名前付きの `next_action` フィールドを
読みます。Doctor の出力が実際に見えている問題と一致する場合にのみ、repair を
実行してください。

```bash
slipway repair --json
```

`repair` は範囲が限定されたローカルの整合性問題のためのものです。変更を
ライフサイクルゲートに無理やり通すための手段ではありません。

## タスク証跡の不足または陳腐化

症状は通常、ランタイムのタスク証跡の不足、実行サマリーの陳腐化、あるいは
現在の入力との不一致として、`validate --json` または `next --json --diagnostics` に
現れます。

安全なリカバリー:

1. JSON 出力から、対象のタスクと必要な証跡パスを特定する。
2. 担当する実装または wave-orchestration のハンドオフを再実行する。
3. 担当する Slipway コマンドまたは生成されたスキルを通じてタスク証跡を記録する。
4. `slipway validate --json` を再実行する。

`.git/slipway/runtime/changes/<slug>/evidence/` 配下のファイルを手で書いては
いけません。

## アーティファクトの実質的内容の不足

ガバナンス対象のアーティファクトが不足している、プレースホルダーのみ、または
構造的に無効な場合は、リカバリー出力が示す作成サーフェスを使います。

```bash
slipway instructions requirements --json
slipway instructions decision --json
slipway instructions research --json
slipway instructions tasks --json
slipway instructions assurance --json
```

コマンドはテンプレートと品質基準を提供します。アーティファクトの実際の内容は、
現在の目的とソースの事実をもとに、作成スキルまたは人間が書かなければなりません。
テンプレートをそのままコピーしたものは却下されます。

## レビューの指摘

レビューで対処すべき問題が見つかった場合、同じコンテキストでレビューと修復を
混在させてはいけません。修復サーフェスを使います。

```bash
slipway fix --json
```

返された修復コントラクトを、新しいコンテキストで動く修復エージェントに渡します。
修復後、影響を受けた選定レビュアーを再実行し、続けて次を再実行します。

```bash
slipway review --json
slipway validate --json
```

選定レビュアーの証跡は、現在の差分、計画アーティファクト、実行サマリーの入力と
一致していなければなりません。唯一の権威ある完全スイートは、ピアが収束した後に
終端の `ship-verification` ゲートが実行するものであり、ピア間で共有される
キーストーンから得るものではありません。

## スコープのドリフト

`scope_contract` がタスクの `target_files` 外の変更ファイルを報告した場合は、
安全なパスを 1 つ選びます。

- 誤って加えた変更であれば、自分で取り消すか移動する。
- 同じ意図のタスクまたはアーティファクトを、提示された Slipway の計画または
  レビューのパスを通じて修正する。
- 目的が変わったのであれば、新しいガバナンス対象の変更を開始する。

証跡を編集して変更ファイルを隠してはいけません。

## Done 後の汚れたワークトリー

`slipway done --json` は、コミットがまだ必要な非アクティブファイルについて
`worktree_dirty_warning` を返しつつ、done-ready の変更をアーカイブできます。

安全なリカバリー:

```bash
git status --short
git diff --check
```

意図した実装差分を、アーカイブされた Slipway の記録とともにコミットします。
アクティブなバンドルは `artifacts/changes/archived/<slug>/` に書き換えられます。

## アダプターのドリフト

生成された AI ツールのコマンドやスキルが古く見える場合:

```bash
slipway init --refresh
```

その後、差分を調べます。

```bash
git status --short .claude .codex .cursor .opencode
```

生成されたアダプターはハンドオフの補助です。アダプターの挙動と CLI の挙動が
食い違う場合は、現在のワークトリーの CLI 出力を信頼し、生成ファイルを更新します。

## リカバリー クイックリファレンス

| 症状 | 調査 | 安全なアクション |
| --- | --- | --- |
| 次に何をすべきか分からない | `slipway next --json --diagnostics` | 返されたスキル、ブロッカー、コマンドに従う。 |
| ゲートが古い証跡だと報告する | `slipway validate --json` | 担当するステージ、レビュアー、タスク証跡パスを再実行する。 |
| ローカル状態が壊れて見える | `slipway health --doctor --json` | 名前付きの限定的な修復に対してのみ `slipway repair --json` を実行する。 |
| アーティファクトがプレースホルダーのみ | `slipway instructions <artifact> --json` | 実際の内容を作成し、検証を再実行する。 |
| レビューで問題が見つかった | `slipway fix --json` | 新しいコンテキストで修復し、影響を受けたレビュアーを再実行する。 |
| アダプターファイルが古い | `slipway init --refresh` | 生成された差分を調べ、ユーザー所有のファイルを保持する。 |

## 関連

- [コマンド](../reference/commands.md)
- [ワークフロー](../explanation/workflow.md)
- [オペレーターガイド](../operator-guide.md)
