# 既存コードベースのオンボーディング

このチュートリアルでは、アプリケーションの挙動を変えずに既存リポジトリへ Slipway を導入します。目的は、AI エージェントに機能開発の計画を依頼する前に、ソースに裏付けられた永続的なリポジトリコンテキストを用意することです。

## 作成するもの

`artifacts/codebase/` 配下にコードベースマップを作成（または更新）し、そのマップを使った小さなパイロット用のガバナンス対象変更を 1 件実行します。

## 前提条件

- 動作する `slipway` バイナリ。
- 既存の Git リポジトリ。
- ファイルを読み取り、Slipway CLI を実行できる AI コーディングツール。

既存リポジトリで作業を始めます。

```bash
cd path/to/existing-repo
git status --short --branch
```

すでにリポジトリが dirty な場合は、その変更がオンボーディング作業に属するものかどうかを判断してください。無関係な編集は保持します。

## ステップ 1: Slipway を初期化する

```bash
slipway init --tools codex
```

チームで使うアダプター ID を指定します。

```bash
slipway init --tools claude,codex,opencode
```

生成された内容を確認します。

```bash
git status --short
```

## ステップ 2: ベースラインのコードベースマップを構築する

```bash
slipway codebase-map --json
```

これにより、次の場所にリポジトリスコープの永続的なコンテキストが作成されます。

```text
artifacts/codebase/
```

生成されるベースラインは検出された事実であって、最終的に作り込まれた分析ではありません。ファイルを確認します。

```bash
find artifacts/codebase -maxdepth 1 -type f | sort
```

## ステップ 3: AI にソース裏付けのあるコンテキストを作成させる

次のプロンプトを AI コーディングツールに貼り付けます。

```text
Use Slipway's codebase-map instructions to refine artifacts/codebase/. Preserve
real baseline facts from `slipway codebase-map`, but add only source-backed
conventions and risks. Cite file paths for every convention. Do not refactor or
edit application code during onboarding.

Start with:
- slipway instructions stack --json
- slipway instructions architecture --json
- slipway instructions testing --json
- slipway instructions concerns --json
```

結果はコードと同じようにレビューしてください。現行のファイル、テスト、ビルドスクリプト、設定ファイル、既存ドキュメントに結びついていないルールはすべて削除します。

## ステップ 4: 小さなパイロット変更を作成する

マップが役立つことを証明できる、最小限で有用な変更を選びます。良いパイロットの例:

- 既存ヘルパー周辺に不足しているテストを追加する。
- ドキュメント 1 ページを現行のコマンドに合わせて更新する。
- 再現手順がわかっている小さなバグを修正する。
- リポジトリのルーティングとテストのパターンがすでに明確な場合に限り、ヘルスチェックエンドポイントを追加する。

ガバナンス対象の変更を作成します。

```bash
slipway new "pilot change using the codebase map" --preset standard
```

ハンドオフを確認します。

```bash
slipway next --json --diagnostics
```

`input_context.codebase_map_status` フィールドは、Slipway がマップを missing、scaffold-only、baseline、partial、populated のいずれと見なしているかを示します。baseline のみで、かつタスクが規約に依存する場合は、計画に入る前にいったん止めてマップを充実させてください。

## ステップ 5: マップをコンテキストにして計画する

次のプロンプトを貼り付けます。

```text
Continue the active Slipway change. During intake and planning, use
artifacts/codebase/ as advisory repo context. Do not invent conventions that are
not in the map or supported by current files. Keep the pilot small enough that
one task can verify whether the map improved planning.
```

各ハンドオフのあとに実行します。

```bash
slipway validate --json
slipway next --json --diagnostics
```

計画スキルがコードベースマップは欠落している、または baseline のみだと警告した場合は、マップを充実させるかタスクを絞り込むかを判断してください。AI が以前のセッションでリポジトリを覚えていると当て込んで先に進めてはいけません。

## ステップ 6: パイロットを実行してレビューする

実装とレビューは Slipway に任せます。

```bash
slipway run --json --diagnostics
```

実装がタスクエグゼキューターに到達したら、次のプロンプトを使います。

```text
Execute the active Slipway task using the codebase map as context. Touch only
the target files declared in tasks.md. Run the task's verification command. If
the map contradicts current source, stop and report the discrepancy instead of
guessing.
```

実装後に確認します。

```bash
git diff --stat
slipway validate --json
slipway next --json --diagnostics
```

レビューで見つかった指摘は、レビューと修正を同じコンテキストで混在させず、`slipway fix --json` を通じて修正してください。

## ステップ 7: 有用な学びを定着させる

パイロットによって永続的な規約が判明した場合は、対応する `artifacts/codebase/` のファイルをソース裏付けのある表現で更新します。範囲は狭く保ちます。

- 良い例: 「HTTP ルートのテストは `internal/http/*_test.go` で `httptest.NewRecorder` を使う。」
- 悪い例: 「常に網羅的なテストを書くこと。」

最後に読み取り専用のチェックを実行します。

```bash
slipway validate --json
```

done-ready になったら実行します。

```bash
slipway done --json
```

パイロットの差分とアーカイブされたガバナンス記録をまとめてコミットします。

## 学んだこと

- `slipway codebase-map` は永続的なブラウンフィールドコンテキストを作成する。
- `slipway instructions <codebase-map-doc>` はマップ精緻化のための作成コントラクトである。
- ベースラインのコンテキストも役立つが、作り込んだソース裏付けのあるコンテキストの方が強力である。
- 計画は想定した規約ではなく、現行のコードを根拠にすべきである。
- 小さなパイロットは、チーム全体への展開前にマップが有用かどうかを明らかにする。

## 関連項目

- [実世界のシナリオ](../real-world-scenarios.md)
- [復旧とトラブルシューティング](../how-to/recover-and-troubleshoot.md)
- [設計](../explanation/design.md)
