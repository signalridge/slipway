# Subagent 設定

`subagents` は `.slipway.yaml` に置くリポジトリポリシーです。各ガバナンス
slot に対して、AI ホストへどの委譲先を提示するかを決めます。ライフサイクル状態、
readiness、証拠、blocker の権威は引き続き Slipway が持ちます。この設定は、現在の
AI ホストが委譲セッションをどう起動するかだけを変えます。

既定 provider は `native` です。`mcp` または `skills` は、その名前を実行できる
hub や adapter がホスト側にある場合だけ設定します。

## Schema

```yaml
subagents:
  default:
    type: native
    name: default-agent
    session_instructions: Use the host's default fresh session behavior.
    timeout: 30m

  plan_audit:
    name: plan-auditor
    session_instructions: Audit only planning artifacts. Do not edit files.

  executor:
    type: mcp
    name: sliphub-executor
    session_instructions: Execute planned wave tasks and record task evidence.

  review:
    type: skills
    name: sliphub
    session_instructions: Run the selected read-only reviewers in parallel and return separate findings.
    timeout: 45m

  fix:
    name: review-repairer
    session_instructions: Collect all selected reviewer findings before editing files.

  verify:
    name: ship-verifier
    session_instructions: Verify terminal readiness without modifying files.
```

各 slot は同じフィールドを受け取ります。

| Field | Meaning |
| --- | --- |
| `type` | Provider family: `native`, `mcp`, `skills`。空なら `native`。 |
| `name` | provider 側が所有するターゲット名。`native` ではホストが対応する agent 名、`mcp` / `skills` では provider が選ぶ hub、tool、skill entry です。 |
| `session_instructions` | 委譲セッションへの自然言語指示。provider profile でも model prompt 継承でもありません。 |
| `timeout` | 任意のホスト向け timeout hint。Slipway は前後空白だけを検証し、解釈は host/provider に任せます。 |

`mcp` と `skills` では、有効な `name` が必須です。slot が `default` と異なる
provider family に `type` を変更する場合、その slot にも `name` を設定してください。
名前は provider family をまたいで継承されません。

## Slots

| Slot | JSON surface | Notes |
| --- | --- | --- |
| `default` | 他の slot に継承 | 共有 fallback。`subagents` がない場合、Slipway は delegation directive を出しません。 |
| `plan_audit` | `plan-audit` の `next_skill.subagent` | plan authoring 自体は main session に残ります。委譲されるのは plan audit だけです。 |
| `executor` | `input_context.wave_plan.executor_subagent` | S2 wave execution。provider が内部で fan out しても、Slipway は task evidence と changed files を監査します。 |
| `review` | 選択された S3 reviewers の `next_skill.subagent` と `review_batch.subagent` | 1 つの slot が選択済み review batch 全体をカバーします。reviewer ごとの provider family は設定しません。 |
| `fix` | `slipway fix --json` の `contract.subagent` | S3 review findings の fresh repair session。 |
| `verify` | `ship-verification` の `next_skill.subagent` | 終端の read-only verification。 |

`plan` slot と substep 単位の設定はありません。Planning は高コンテキストの authoring なので
main session に残ります。Subagent 設定は、Slipway に明確な独立性または dispatch boundary が
あるところから始まります。

## 設定しないもの

Provider 固有の tool permission、model 設定、任意の provider 引数は、Slipway の
ユーザー向け config ではありません。現在の slot に必要な tool boundary は Slipway と
選択された provider が決めます。hub に routing detail が必要な場合は、
`session_instructions` に運用意図を書き、その解釈を provider に任せます。

## config コマンド例

slot を `mcp` または `skills` に切り替える前に `name` を設定してください。`set` のたびに
config 全体が検証されるためです。

```bash
slipway config set subagents.review.name sliphub
slipway config set subagents.review.type skills
slipway config set subagents.review.session_instructions "Run selected reviewers in parallel and return separate findings."
slipway config set subagents.review.timeout 45m
```

複数 slot をまとめて設定する場合は、YAML を直接書くのがいちばん明確です。

## ホスト面の再生成

subagent 設定を変更した後は `slipway init --refresh` を実行し、生成済み adapter
surface と hook を現在の CLI contract に合わせてください。
