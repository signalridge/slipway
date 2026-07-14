# Issue workflow（非規範ガイド）

> このローカライズページは non-normative です。完全な[中国語製品契約](../../zh/reference/product-contract.md)と [machine schema](../../reference/machine-protocol.schema.json) が authority です。

複数の独立 delivery が必要な場合だけ Objective を作ります。Change は唯一の issue-backed Run source で、parent body や comments を runtime inheritance せず、実行に必要な Requirements をすべて含みます。小規模、機微、緊急、GitHub unavailable、または Issue を作らない選択では `slipway run --budget N --json --root ABSOLUTE_ROOT -- GOAL` を使います。

## Marker、label、body

Objective body の先頭は `<!-- slipway-level: objective/v1 -->`、直後に `<!-- slipway-publication-operation: UUID -->`、次に `<!-- slipway-publication-item: UUID -->` です。Title は `[Objective] ...`、label は正確に `level:objective` と1個の `kind:*`。H2 は Problem、Outcome、Requirements、Shared constraints、Non-goals、Changes です。

Change draft は `change/v2` marker と manifest を持たず、body は `<!-- slipway-publication-operation: UUID -->` と `<!-- slipway-publication-item: UUID -->` の receipt markers だけです。Final Change の最初の line は `<!-- slipway-level: change/v2 -->`、続いて唯一の strict `slipway-manifest` JSON fence、その fence 後に同じ operation/item markers を保持します。Manifest の ordered array は各 chapter の stable key/role/title を GitHub comment node ID、database ID hint、exact body digest に束縛します。参照 comment の first non-empty line は `<!-- slipway-section:v1 key=KEY -->` で、その後が exact normalized Markdown です。Profile は Outcome、Requirements、Acceptance examples、Constraints、Non-goals の各 role を最低一つ含み、一 role を複数 chapter に分けられます。

Title は `[Change] ...`、label は正確に `level:change` と1個の `kind:*`、任意で `ready-for-agent`。Manifest だけが authority で、comment 表示順、timestamp、unreferenced discussion は使いません。Missing・extra・duplicate・cross-Issue・field-inconsistent・minimized・in-place edited・hash mismatch chapter は fail closed です。Unmarked Issue は manual normalization、別の confirmed linked Change、bounded ad-hoc Run の3択です。

## Self-containment、relationship、limits

Decompose は適用可能な Objective requirements/constraints を各 child の manifest-addressed chapter として物化し、Kind は継承しません。Discussion の判断は replacement chapter と新 manifest を publish してから snapshot に入れます。Objective→Change の1階層 native sub-issues はちょうど100 children まで許可します。Change dependency の blocking と blocked-by は独立した方向で、それぞれちょうど50まで許可します。書き込みが limit を超える場合だけ停止・報告し、overflow を prose や gate に変換しません。

`gh --version` を検出し、`gh >= 2.94.0` は first-class relation operation、それ以外は既存認証で official REST API fallback、または `environment_unavailable`。Local authority は作りません。同じ `github.com` 内の transfer/redirect だけを信頼し、repository/Issue node IDs、labels、parent、dependencies、canonical URL を refetch、旧 URL alias を保存し、marker/revision 比較を続けます。Cross-host redirect は信頼しません。

## Publication と reconciliation

Objective は single-stage publication です。Exact title、full body、labels、relations、operation UUID、item UUID を一つの preview で示し、その精確な外部書き込みに一回だけ current confirmation を得ます。Mutable target を refetch し、`--body-file` で create、曖昧なら exact marker pair を paginated non-search API で reconcile し、identity、URL、title/body/digest、markers、labels、relations を完全 readback します。Chapter comment/manifest、second commit confirmation、Implement はありません。

Change は remote comment ID が create 前に存在しないため二段階 confirmation のままです。最初に full chapter drafts、同一 operation UUID、stable item UUID、body digests、section order/roles、labels/relations、expected parent revision を示し、非 authority draft 作成を確認します。新 Change draft は receipt markers だけで `change/v2` marker はなく、amendment は accepted body を変更しません。Comments を作成・refetch 検証後、observed IDs を含む exact final manifest を示します。Content-identical replacement を含むすべての amendment manifest は preview で確認した pinned revision を `parent_requirements_revision` に設定し、initial manifest は省略します。二回目の current confirmation 後、commit 直前に head/parent drift を再確認し、Issue body marker/manifest を最後に更新します。同じ receipt markers は manifest fence 後に保持します。Unreferenced comments は draft のままで authority ではなく、accepted chapter は in-place edit しません。

GitHub create は exactly-once でなく body CAS もありません。Timeout-after-success、partial relation failure、duplicate marker、index delay、ambiguous response では paginated non-search API で照合し、item/label/relation ごとに `created|matched|failed|ambiguous` を返します。Blind retry や成功済み rollback は禁止。0 match は再 preview/confirmation、1 は converge、複数は user pause です。

Public repository Issue に private switch はありません。Requirements、goal、answers、command summaries が機微であり得ると警告し、private repo、実際の脆弱性だけ enabled private vulnerability reporting、既存 security channel、または ad-hoc Run を使います。認識した credential value を command identity を保って redact し、token、unreferenced discussion、env dump、full transcript、hidden reasoning は収集しません。Source import では exact Issue body と manifest 参照 raw comment fields だけを一時取得し、raw envelope は CLI consume にだけ渡して temporary file を削除し、accepted normalized materials と bounded catalog/provenance だけを保存します。
