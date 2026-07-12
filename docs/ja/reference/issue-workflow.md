# Issue workflow（非規範ガイド）

> このローカライズページは non-normative です。完全な[中国語製品契約](../../zh/reference/product-contract.md)と [machine schema](../../reference/machine-protocol.schema.json) が authority です。

複数の独立 delivery が必要な場合だけ Objective を作ります。Change は唯一の issue-backed Run source で、parent body や comments を runtime inheritance せず、実行に必要な Requirements をすべて含みます。小規模、機微、緊急、GitHub unavailable、または Issue を作らない選択では `slipway run "<ad-hoc goal>"` を使います。

## Marker、label、body

Objective の最初の non-empty line は `<!-- slipway-level: objective/v1 -->`。Title は `[Objective] ...`、label は正確に `level:objective` と1個の `kind:*`。H2 は Problem、Outcome、Requirements、Shared constraints、Non-goals、Changes です。

Change の例：

```markdown
<!-- slipway-level: change/v1 -->

## Outcome
単一の observable result。

## Requirements
この delivery に必要な全 behavior。

## Acceptance examples
具体的な observable example。

## Constraints
product/technical boundary。

## Non-goals
明示的な除外。

## Implementation checklist
任意の実行 note。revision には含めない。
```

Title は `[Change] ...`、label は正確に `level:change` と1個の `kind:*`、任意で `ready-for-agent`。最初の厳密な marker が Level authority です。title/label drift は warning で、確認後だけ修復でき、marker-valid Run を block しません。marker missing/conflict/Objective/unknown は Change source ではありません。Unmarked Issue は manual normalization、別の confirmed linked Change、bounded ad-hoc Run の3択です。

## Self-containment、relationship、limits

Decompose は適用可能な Objective requirements/constraints を各 child に物化し、Kind は継承しません。Comment の判断は Change body に戻してから snapshot に入れます。Objective→Change は1階層の native sub-issues（最大100）、Change dependency は native blocked-by（blocking/blocked-by 各50）です。Limit で停止・報告し、prose graph に隠しません。

`gh --version` を検出し、`gh >= 2.94.0` は first-class relation operation、それ以外は既存認証で official REST API fallback、または `environment_unavailable`。Local authority は作りません。同じ `github.com` 内の transfer/redirect だけを信頼し、repository/Issue node IDs、labels、parent、dependencies、canonical URL を refetch、旧 URL alias を保存し、marker/revision 比較を続けます。Cross-host redirect は信頼しません。

## Publication と reconciliation

Write 前に full draft と operation UUID、stable item UUID、repository、canonical body SHA-256、exact labels/relations、expected revisions の plan を示し、その exact plan を確認します。Typed markers を level marker 直後に置き、body file を使い、write 前 refetch と write 後 readback を行います。

GitHub create は exactly-once でなく body CAS もありません。Timeout-after-success、partial relation failure、duplicate marker、index delay、ambiguous response では paginated non-search API で照合し、item/label/relation ごとに `created|matched|failed|ambiguous` を返します。Blind retry や成功済み rollback は禁止。0 match は再 preview/confirmation、1 は converge、複数は user pause です。

Public repository Issue に private switch はありません。Requirements、goal、answers、command summaries が機微であり得ると警告し、private repo、実際の脆弱性だけ enabled private vulnerability reporting、既存 security channel、または ad-hoc Run を使います。認識した credential value を command identity を保って redact し、token、raw comments、env dump、full transcript、hidden reasoning を収集しません。
