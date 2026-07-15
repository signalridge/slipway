# マシンプロトコル v2 チュートリアル

このチュートリアルでは、Run の開始、Orient と Implement Outcome の送信、Summarize による終了まで、ローカルプロトコルのライフサイクルを一通り実行します。対象はホストおよびアダプターの作者です。通常、これらの hidden operation は生成されたホスト capability が実行します。エンドユーザー向けの別ワークフローではありません。

正規の契約は、バージョン付きパスの [machine protocol schema](../../reference/v2/machine-protocol.schema.json) と [source envelope schema](../../reference/v2/source-envelope.schema.json) です。URL のバージョンは JSON の `contract_version` または `source_version` とそろえてください。従来のバージョンなし schema URL は v2 の互換エイリアスとして残りますが、新しい統合では `/reference/v2/` を使用してください。

## 前提条件

`slipway`、`git`、`jq` をインストールし、すべての snippet を同じ shell session で順に実行してください。このチュートリアルは実際の Run journal を作成し、追跡対象ファイルを変更するため、使い捨てディレクトリ内で動作します。

```bash
TUTORIAL_DIR=$(mktemp -d)
WORKSPACE="$TUTORIAL_DIR/workspace"
mkdir -p "$WORKSPACE"
cd "$TUTORIAL_DIR"
git -C "$WORKSPACE" init -q
git -C "$WORKSPACE" config user.name 'Protocol Tutorial'
git -C "$WORKSPACE" config user.email tutorial@example.invalid
printf '# Protocol tutorial\n' > "$WORKSPACE/README.md"
git -C "$WORKSPACE" add README.md
git -C "$WORKSPACE" commit -qm initial
```

## 1. Run を開始する

すべての flag を `--` セパレーターの前に置き、goal を 1 つのリテラル引数として渡します。`--no-review` により、この短いライフサイクルは Implement から Summarize へ直接進みます。

```bash
slipway run \
  --budget 4 \
  --json \
  --root "$WORKSPACE" \
  --no-review \
  -- "add one tutorial line to README.md" > start.json

jq -e '
  .contract_version == 2 and
  .state == "active" and
  .action.kind == "orient" and
  .next.operation == "action"
' start.json

RUN_ID=$(jq -r '.run_id' start.json)
ORIENT_ID=$(jq -r '.action.action_id' start.json)
```

本番ホストでは、レスポンス全体を machine protocol schema で検証してください。ここでの `jq` 式は、チュートリアル上の重要な検証点を明示するだけです。`next.variants[].base_argv` は引数配列のまま保持し、表示用のコマンド文字列を解析しないでください。

## 2. Orient Outcome を送信する

公開 Outcome の全フィールドを含めます。該当しない branch は JSON `null`、空の collection は配列のままにします。

```bash
jq -n --arg action "$ORIENT_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "orient",
  status: "completed",
  summary: "Repository facts observed.",
  observations: ["README.md is the only tracked file."],
  known_issues: [],
  suggested_actions: [{
    kind: "implement",
    brief: "Append the requested tutorial line."
  }],
  pause: null,
  implementation: null,
  review: null
}' > orient-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$ORIENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file orient-outcome.json > implement.json

jq -e '.contract_version == 2 and .action.kind == "implement"' implement.json
IMPLEMENT_ID=$(jq -r '.action.action_id' implement.json)
```

`action_id` と `action_kind` は outstanding Action と一致しなければなりません。以前のレスポンスの ID を再利用したり、次の Action をホスト側で作り出したりしてはいけません。

## 3. 実装して結果を報告する

観測可能な変更を行い、チェックを実行し、正確な activity と終了コードを報告します。

```bash
printf 'Protocol v2 lifecycle completed.\n' >> "$WORKSPACE/README.md"
git -C "$WORKSPACE" diff --check

jq -n --arg action "$IMPLEMENT_ID" \
  --arg check_command "git -C \"$WORKSPACE\" diff --check" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "implement",
  status: "completed",
  summary: "Appended the requested README line.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: {
    result: "applied",
    files_changed: ["README.md"],
    activities: [{
      kind: "test",
      command: $check_command,
      exit_code: 0,
      summary: "No whitespace errors."
    }],
    uncertainties: [],
    attempts: 1
  },
  review: null
}' > implement-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$IMPLEMENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file implement-outcome.json > summarize.json

jq -e '.action.kind == "summarize"' summarize.json
SUMMARIZE_ID=$(jq -r '.action.action_id' summarize.json)
```

`files_changed` はホストの報告であり、変更の帰属を証明するものではありません。Slipway は別途、bounded Git observation を記録し、ユーザーや他のツールによる並行変更の不確実性を保持します。

## 4. Run を終了する

```bash
jq -n --arg action "$SUMMARIZE_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "summarize",
  status: "completed",
  summary: "The requested README update is complete and git diff --check passed.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: null,
  review: null
}' > summarize-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$SUMMARIZE_ID" \
  --root "$WORKSPACE" \
  --outcome-file summarize-outcome.json > ended.json

jq -e '
  .contract_version == 2 and
  .state == "ended" and
  .next.operation == "none" and
  (.next.variants | length) == 0
' ended.json

rm -rf "$TUTORIAL_DIR"
```

完全に同じ Outcome byte の再送は冪等です。完了済みの同一 Action に異なる byte を送ると `outcome_conflict` になり、古い Action ID は fail closed になります。message の文字列ではなく、構造化 error の `code` で分岐してください。

## Issue-backed 拡張

Issue-backed Run では、trusted host が source envelope を検証して private temporary file に書き、`--source-file` で一度だけ渡します。レスポンスには `pinned_source`、`action.source`、`action.requirements`、構造化された `_machine material` reader が追加されます。Manifest が参照するコメントだけを取得し、通常の discussion comment を requirement として扱わないでください。Authority と publication model は[マシンプロトコルリファレンス](../reference/machine-protocol.md)および [ADR-0001](../../../adr/0001-source-bundle-v2.md)を参照してください。
