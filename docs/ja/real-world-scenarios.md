# 実践シナリオ

このページを使って、目の前の作業に合った Slipway の進め方を選んでください。どのシナリオも 1 つのルールを守ります。プライベートな記憶や手動でのステート編集ではなく、現在のワークトリーが生み出す証跡をもとに前進することです。

## シナリオ一覧

| シナリオ | こんなときに | Slipway の主な価値 |
| --- | --- | --- |
| 1. 初めての統制された変更 | 小さく安全な編集でライフサイクルを学びたい。 | 証跡のループ全体を一度通して体験する。 |
| 2. 既存プロジェクトへの導入 | リポジトリに既存の規約やリスク領域がある。 | 計画前に、実際のコードベースの文脈を恒久的なものにする。 |
| 3. プロダクト機能のリリース | コード、テスト、ドキュメント、レビューにまたがる作業。 | スコープ・タスク・証跡・レビューの整合を保つ。 |
| 4. レビュー指摘の修正 | S3 レビューが対応すべき問題を検出した。 | 新しいコンテキストで修正し、対応を集約する。 |
| 5. 古くなった・行き詰まった変更の復旧 | 証跡・タスク・成果物がドリフトした。 | 名前の付いた復旧コマンドでフェイルクローズする。 |
| 6. チームへのアダプター展開 | 複数の AI ツールに同じ Slipway サーフェスが必要。 | 1 つの CLI 権威からホストファイルを生成する。 |

## 1. 初めての統制された変更

ライフサイクルを低リスクで学びたいときに使います。

AI コーディングツールへの起点プロンプト:

```text
Use Slipway for one small docs-only change. Keep the scope to README.md,
inspect status and next before each mutating command, and stop if Slipway
reports stale evidence or out-of-scope files.
```

ワークフロー:

1. `slipway init --tools <tool-id>` でアダプターを初期化する。
2. `slipway new "add a short README usage note" --profile docs` で変更を作成する。
3. `slipway next --json --diagnostics` で現在のハンドオフを確認する。
4. 返されたスキルに、必要な成果物または実装ステップをオーサリングさせる。
5. 実装後に `slipway validate` を実行する。
6. ステートが done-ready になってから、`slipway done` を実行する。

完了の条件:

- 意図したファイルが変更されている。
- 成果物バンドルがその理由を説明している。
- 現在の検証が証跡を受理している。
- `done` の後にアーカイブ記録が存在する。

## 2. 既存プロジェクトへの導入

コードベースにはすでに実際の挙動があるものの、規約がソースのパターン、古い PR、散在するドキュメント、レビュアーの記憶に埋もれているときに使います。

起点プロンプト:

```text
This is an existing repo. Do not refactor yet. Use Slipway to create or refresh
the codebase map, then identify the smallest governed change that would prove
the map is useful. Cite files for every convention you record.
```

ワークフロー:

1. `slipway init --tools <tool-id>` を実行する。
2. `slipway codebase-map --json` を実行する。
3. `slipway instructions stack`、`slipway instructions architecture`、`slipway instructions testing` などのコードベースマップ向け instructions の各テーマを使って、`artifacts/codebase/` のドキュメントをオーサリングまたは洗練する。
4. 小さな統制されたパイロット変更を 1 つ作成する。
5. 計画中に、`next --json` が `input_context.codebase_map_status` でマップの状態を報告しているか確認する。
6. パイロットの結果をレビューし、ソースに裏付けられた知見のみでマップを更新する。

ガードレール:

- 現在のファイルで裏付けられる規約だけを記録する。
- コードやドキュメントにたどれない推測ルールは削除する。
- ベースラインのみのマップは、実質的な内容がオーサリングされるまで参考扱いとする。
- 大規模なクリーンアップを導入タスクに含めない。

完了の条件:

- `artifacts/codebase/` にレビュー済みの文脈が含まれている。
- 最初の統制されたパイロットがその文脈を使った。
- マップが推測の寄せ集めになっていない。

## 3. プロダクト機能のリリース

実装、テスト、ドキュメント、レビューの要件がある作業に使います。

起点プロンプト:

```text
Use Slipway for this feature. First clarify scope and acceptance criteria. Keep
target files explicit in tasks.md, run targeted tests for each task, and treat
review findings as a separate S3 repair batch.
```

ワークフロー:

1. `slipway new "<feature>" --preset standard` で変更を作成する。
2. インテークと計画に、実体のある `intent.md`、`requirements.md`、`decision.md`、`research.md`、`tasks.md` を生成させる。
3. すべてのタスクに具体的な `target_files` があることを確認する。
4. `slipway implement --json` または `slipway run --json` で実行する。
5. 生成されたウェーブ実行パスを通じてタスク証跡を記録する。
6. S3 レビューを実行し、選定されたレビュアーが現在の入力と一致しているときにのみクローズする。

完了の条件:

- 要件が実装とテストに対応づいている。
- タスク証跡が現在の run バージョンと一致している。
- 選定されたレビューとクローズアウトの証跡が合格している。
- `done` が、汚れた作業を隠さずに変更をアーカイブする。

## 4. レビュー指摘の修正

S3 レビューが対応すべき問題を報告したときに使います。

起点プロンプト:

```text
Use Slipway fix for the selected review findings. First consolidate confirmed
findings by root cause. Make one repair pass, rerun the affected reviewers, and
do not repair findings inline while review is still reporting.
```

ワークフロー:

1. `slipway review --json` または `slipway next --json --diagnostics` を確認する。
2. `slipway fix --json` を実行する。
3. 返された修正コントラクトを、新しいコンテキストで動く修正エージェントに渡す。
4. 影響を受けた選定レビュアーを再実行する。
5. fix とレビューの context-origin の証跡が現在の入力と一致してから、レビューを継続する。

完了の条件:

- 修正が、選定された指摘を根本原因から解消している。
- 修正後にレビュー証跡が更新された。
- 古くなった選定レビュアーが黙って無視されていない。

## 5. 古くなった・行き詰まった変更の復旧

`next`、`status`、`validate` が、古い証跡、欠落したタスク証明、スコープのドリフト、あるいはローカルステートの不整合を報告したときに使います。

起点プロンプト:

```text
Diagnose this Slipway blocker without editing state by hand. Run status,
validate, next with diagnostics, and health doctor. Follow only the named safe
recovery command or explain why none applies.
```

ワークフロー:

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
slipway health --doctor --json
```

health が範囲を限定したローカル修復を示した場合は、次を実行する:

```bash
slipway repair --json
```

ステージまたはレビュアーが古くなっている場合は、その所有ステージまたはレビュアーを再実行する。成果物に実質的な内容が欠けている場合は、`slipway instructions <artifact>` を実行して、実体のある成果物をオーサリングする。

完了の条件:

- 元のブロッカーが、現在のワークトリーで解消されている。
- 所有するコマンドまたはスキルによって現在性が再判定された。
- 復旧の過程で、タイムスタンプ・判定・ライフサイクルステートを偽造していない。

## 6. チームへのアダプター展開

複数の人やツールが、同じコマンドとスキルのサーフェスを必要とするときに使います。

起点プロンプト:

```text
Refresh Slipway adapters for the tools this repo actually uses. Preserve
user-owned files near the generated directories, inspect the diff, and do not
make generated host files authoritative over the CLI.
```

ワークフロー:

```bash
slipway init --tools claude,codex,opencode
slipway init --refresh
```

`--tools all --refresh` は、Slipway が生成するすべてのアダプターをリポジトリが意図的にサポートする場合にのみ使う。

完了の条件:

- `.slipway.yaml` がリポジトリのデフォルトを反映している。
- 生成されたアダプターファイルが現在の CLI と一致している。
- ユーザー所有のホスト設定が保持されている。
- 権威として `slipway next`、`status`、`validate` を使うことをチームが理解している。
