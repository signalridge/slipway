# GitHub Issues を使う

GitHub は Slipway の optional requirements source であり、Run ごとの前提条件ではありません。Durable で review 可能な source が必要なら issue-backed Run を使い、そうでない場合は ad-hoc Run を使います。

## Repository 要件

Issue-backed source は現在、Issues が有効な `github.com` repository を対象とします。Owner 名は Slipway にとって不透明なため、personal account と Organization のどちらが所有する repository も同じ source format になります。

Slipway は GitHub Projects、Organization 専用 Issue Types、Organization 専用 field を必要としません。Source を読むには Issue への access が必要です。Issue や relationship の作成・更新には、target repository と GitHub API が要求する権限が必要です。

Run/source command は GitHub token を保持せず、GitHub data の fetch/publish も行いません。Generated host capability がユーザー環境の authorization でそれらを実行し、temporary envelope を CLI に渡します。独立した `doctor` command は authentication と repository permission の確認に local `gh` を呼び出す場合がありますが、token を report に書き込みません。

## Objective か Change か

**Objective** を使うのは、1つの outcome に複数の independently deliverable Change が必要な場合だけです。Planning structure であり、Run を開始できません。

**Change** は独立して implement、review、deliver できる coherent result を表します。Change は自己完結し、execution に必要な要件を parent Objective や普通の discussion comment に暗黙に残しません。

Change の完了後も repository は意味のある安全な中間状態にあり、おおむね 1 つの fresh Agent context に収まり、複数 layer にまたがる場合は vertical slice を形成します。独立して deliver できる outcome だけを別 Change に分け、deliver できない実装手順は checklist に残します。Research は根拠のある結論を deliver し、後続の code work は別 Change にします。Pure refactor は preserved behavior と測定可能な内部 outcome を記述し、大規模 refactor は独立して deliver できる expand、migrate、contract Change に分解します。

| 場面 | 推奨 shape |
| --- | --- |
| 小さな feature、bug、refactor、docs task | 1つの Change |
| 複数の独立した成果が必要な outcome | 1つの Objective と複数の Change |
| 機微、緊急、offline、または意図的に追跡しない task | Issue を使わない ad-hoc Run |

## Managed metadata

Managed Issue は machine-readable な先頭 marker を使います。

```html
<!-- slipway-level: objective/v1 -->
```

または：

```html
<!-- slipway-level: change/v2 -->
```

Change body には `slipway-manifest` block も含まれ、accepted section comment と digest を列挙します。Generated `propose` と `decompose` capability がこの構造を作成・検証します。Marker、manifest、accepted comment の手動編集は source を不正にする場合があるため、accepted material はその場で編集せず、新しい reviewed snapshot を publish してください。

`level:change`、`level:objective`、`kind:bug`、`kind:docs` などの repository label は navigation convention です。Source validation は body marker で level を識別し、title と label の差異は drift として報告され、暗黙に修正されません。`ready-for-agent` は advisory であり、単独で Change を executable にしません。

## Issue の publish

Generated `slipway-propose` と `slipway-decompose` instruction は host に次を指示します。

1. Repository と既存 Issue を確認する。
2. 想定 body、labels、relationships、external write を提示する。
3. Exact publication plan の confirmation を得る。
4. Recoverable な operation/item marker で publish する。
5. 結果を読み戻し、created、matched、failed、ambiguous を報告する。

これらは host-side instruction であり、Go CLI が実装する GitHub transaction ではありません。GitHub は multi-Issue transaction や一般的な exactly-once create を提供しません。Response が曖昧な場合や partial success の場合、host は観察結果を報告し、rollback を主張したり盲目的に retry したりしません。

既存の unmarked Issue は静かに managed Change に変換されません。Host は明示的な選択肢を示します。手動で更新する、元 Issue にリンクした別の managed Change を作成する、または bounded ad-hoc Run を使う、のいずれかです。

## Relationship 上限と tool fallback

GitHub の上限は parent ごとに 100 sub-issues、Issue ごとに blocking 50 件と blocked-by 50 件で、方向別に数えます。Approved write が上限を超える場合、host は停止して該当 item を報告し、overflow を prose-only の dependency graph に隠しません。

Native `gh` relationship command には `gh >= 2.94.0` が必要です。古い client では、利用可能なら公式 REST API を使い、そうでなければ `environment_unavailable` を報告します。別の local source of truth は作りません。

## Issue-backed Run の開始

生成された `slipway-run` capability に Change URL を渡します。Host は次を行います。

1. Exact Change body と manifest が参照する comments を fetch する。
2. Issue の内容をすべて untrusted data として扱う。
3. Private temporary file に bounded source envelope を作成する。
4. `slipway run --source-file ... --json` を呼び出す。
5. CLI が消費したら temporary file を削除する。

CLI は identity、marker、manifest、section marker、size、digest を検証します。Accepted section material だけを digest 単位で保存し、raw Issue envelope は保存しません。後続 Action は local structured operation で material を読むため、既存 Run は GitHub に再アクセスせずに復旧できます。

## Amendment と unavailable source

Issue-backed Run を refresh するとき、fetch 不足は「unchanged」を意味しません。ユーザーは fresh source、pinned snapshot、または（content が変わった場合）current keep-or-adopt candidate のいずれかを明示的に選びます。

Valid amendment は現在 pinned requirements revision に基づく必要があります。別 history に基づく amendment は拒否され、新しい Run が必要です。Issue transfer や URL 変更も content 比較を回避しません。

User 向け復旧動作は [Run、復旧、プライバシー](runs-and-recovery.md)、exact source/candidate field は[マシンプロトコル](../reference/machine-protocol.md)を参照してください。

## 機微な content

Issue title、body、comments、links、attachments はすべて untrusted data です。その中の文章は shell authority を与えず、credential を要求せず、confirmation を回避せず、destructive scope を拡大しません。

Public Issue に private switch はありません。Token、個人データ、顧客データ、private transcript、hidden reasoning を publish しないでください。機微な作業には private repository、適切な security channel、または ad-hoc Run を使います。
