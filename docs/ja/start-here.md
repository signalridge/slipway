# はじめに

> 日本語は non-normative guide です。完全な[中国語製品契約](../zh/reference/product-contract.md)と [machine schema](../reference/machine-protocol.schema.json) が実装 authority です。

Slipway はユーザーの明示 invocation 後だけ AI coding host を調整し、issue-first ですが issue-gated ではありません。

```text
Objective Issue（任意、実行不可）
  └─ self-contained Change Issue
       └─ Run: orient → 必要なら clarify → implement → observed diff なら review → summarize
```

Change は唯一の issue-backed source で effective Requirements をすべて含みます。Objective は複数の独立 delivery にだけ使います。GitHub unavailable、機微、微小、緊急、または Issue を作らない選択では ad-hoc Run を使います。

```bash
go install github.com/signalridge/slipway@latest
cd your-git-repository
slipway install --tool claude
slipway run --budget 8 --json --root "$PWD" -- "レポートに CSV export を追加"
```

Issue-bound Run は trusted host が strict Change envelope を一度だけ安全に取得します。

```bash
slipway run --budget 8 --json --root /absolute/repository --source-file /safe/temp/change-envelope.json -- "bounded Change を実装"
```

Marker-valid body が Level authority で、title/label drift は warning のみです。[Issue workflow](reference/issue-workflow.md)を確認してください。Public Issue に private switch はないため、機微な作業は private repo、適切な security channel、または ad-hoc Run を使います。

Host は repository facts を先に調査します。Clarify は Matt Pocock `grill-me` に従い、依存順に1つの human decision、recommendation/trade-off を示し、完全な request は0質問、理解が変われば shared understanding を確認し、wrap-up は即停止して何も書きません。自然言語 control に理由は不要です。“skip this” は current skip-action を正確に呼び、“stop” は public `slipway stop`、“take over” はまず stop して Run ID を保持・報告し outstanding Action を実行しません。“reorder”/“do X first” は automatic loop を停止して制御を返し、queue を暗黙変更せず skip に変換しません。明示 resume 後だけ続行します。Review は read-only で repair loop を作りません。`ended` は queue が空であることだけを示します。

10 adapter は正確に6 capability、CLI は7 public command です。Journal は accepted Requirements、goal、answer、command summary を含み得ます。[Privacy](explanation/runs-and-privacy.md)、[Windows](reference/windows-rendering-and-durability.md)、[evidence](reference/acceptance-evidence.md)も参照してください。
