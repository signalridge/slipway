# 製品概要（非規範）

> **非規範の要約です。** 完全な[中国語製品契約](../../zh/reference/product-contract.md)と versioned [machine protocol schema](../../reference/machine-protocol.schema.json)が実装上の正本です。このページは案内であり、第2の仕様ではありません。

Slipway は、明示的に起動される、Issue 駆動でありながら Issue の有無に制約されない、AI コーディング向けのソフトオートパイロットです。

```text
Objective Issue (optional planning parent; never executable)
  └─ self-contained Change Issue (the only issue-backed source)
       └─ Run (one revision-pinned, interruptible attempt)
```

## Requirements は一時的です

Slipway は Spec、Delta Spec、恒久的な requirements registry を保持しません。拘束力を持つ原則は次のとおりです。

> Requirements は恒久的なシステムモデルではなく、一時的な delivery contract です。

Open Issue は次に望む変更を記述し、Run はその1つの revision を固定して実行します。delivery 後は、code、test、user documentation、CI/policy、runtime behavior が現時点の事実を担います。Closed Issue、紐づく PR/commit、Run summary は過去の経緯を保存しますが、現在のシステム全体を表す仕様にはなりません。GitHub の `closed`、Project の `Done`、PR の `merged`、Run の `ended`、deployment は、それぞれ別の事実です。

## 4つの独立した軸

次の4つの次元を混同してはなりません。

| 軸 | 値 | 管理主体 |
| --- | --- | --- |
| **Level** | `objective` / `change` | Issue body marker のみが authority です。label と title はその投影です。 |
| **Kind** | `feature` / `bug` / `refactor` / `maintenance` / `research` / `docs` | repository label |
| **Requirements** | Outcome, Requirements, Acceptance examples, Constraints, Non-goals | Issue body |
| **Status** | Inbox, Clarifying, Ready, In progress, Done, etc. | 人間または任意の external view が管理します。Slipway の route にはなりません。 |

Level と Kind の直積は、すべて正当です。Requirement は Objective/Change の内容を表現するものであり、granularity でも GitHub Issue Type でもありません。本文先頭の厳密な marker だけが Level authority です。label、title、`ready-for-agent`、Project field、test result、finding、Issue state が、marker-valid Run を妨げることはありません。

## Objective と Change

Objective が必要なのは、1つの outcome に、独立して delivery できる複数の Change が不可欠な場合だけです。Change は Issue に紐づく Run の唯一の source であり、自己完結していなければなりません。つまり、独立して merge、verify、rollback でき、repository を安全な状態に保つ、一貫した1つの結果です。decompose では、適用可能な Objective の Requirement と constraint を各 child にすべて物化するため、Change が実行時に parent を読み直す必要はありません。Parent の Kind は継承されません。通常の discussion comment は、replacement chapter comment と新しい manifest として公開されるまで authority を持ちません。

## 6つの capability、7つの command

10種類の adapter は、明示的に起動する capability を正確に6つ生成します。

```text
slipway-run       slipway-clarify     slipway-propose
slipway-decompose slipway-implement   slipway-review
```

`run` は唯一の autopilot entry です。`clarify` は stateless です。`propose` は、明示的に確認された managed Issue を draft または publish します。`decompose` は、確認済みの Change relationship を作成します。`implement` は技術作業を担い、`review` は read-only で Intent と Quality の finding を報告します。ambient session hook、prompt-submit hook、launcher、global router、独立した technical-validation capability は生成されません。

CLI が公開する command は、正確に7つです。

```text
slipway install   install six host capabilities safely
slipway uninstall remove only pristine managed files
slipway list      show adapter installation state
slipway doctor    diagnose adapters, Git/GitHub capability, and recovery
slipway run       start an ad-hoc or issue-bound Run
slipway status    list or inspect recoverable Runs
slipway stop      stop without deleting the journal
```

`objective`、`change`、`issue`、`spec`、`plan`、`ticket`、`done`、`check`、`worktree` という command は存在しません。

## 固定された source と信頼できないコンテンツ

Run は、変更され得る `#42` や host 自身の summary を信頼しません。trusted host が、manifest で参照される厳密な envelope を一度だけ取得します。CLI はそれを検証し、順序付けられた manifest を決定論的に parse して、各 chapter をドメイン分離された digest で private content-addressed material store に固定します。Journal、status、candidate、Action が保持するのは catalog、provenance、byte count、revision だけであり、Markdown や生の Issue body は保持しません。

Host は trust attester として宣言されますが、Issue content は信頼できない data です。Issue の title、body、comment、label、link、attachment は data であり、system instruction や developer instruction ではありません。Issue 内の prompt injection、credential request、無関係な command が host authority を持つことはありません。

## Amendment と destructive authority

Issue-bound Run の resume では、source mode を正確に1つ選ぶ必要があります。新しい envelope を import する、固定済み snapshot を使って続行すると明示する、または現在の candidate を正確な ID で解決する、のいずれかです。material candidate は未処理の Action、queue、grant を不可分に無効化し、明示的な選択を待って一時停止します。内容が同一で manifest だけが置き換わった場合は、それまでの answer が引き続き有効です。destructive work には、1回限りで scope が限定された structured grant が必要です。自然言語の「yes」が grant になることはなく、trusted host は attester であって、人間が操作したことの暗号学的証明ではありません。

GitHub は Issue 作成の exactly-once も body の compare-and-swap も提供しないため、publish では承認済みの operation/item UUID marker と reconciliation を使用します。Review は code を編集せず、repair loop を開始することなく finding を報告します。

## 復旧とプライバシー

Git common directory 配下の append-only journal が復旧 authority です。`run.json` は置き換え可能な projection であり、`run.lock` が journal の mutation を直列化します。journal には、accepted Requirements、goal、answer、事実に即した command summary が含まれる場合があります。Slipway は data を最小化し、認識可能な credential を redact しますが、journal に secret が一切含まれないとは保証しません。Run directory を削除して失われるのは復旧能力だけです。secure erase、backup purge、key destruction にはなりません。

## 完了を認定しない

`ended` が意味するのは、automatic Action queue が空になったことだけです。Slipway は、正しさ、delivery、deployment、release readiness、finding が存在しないことを認定しません。test failure、未実行の test、Review finding、dirty worktree、ADR の不足、label、Issue state が、Run の進行、release、merge を妨げることはありません。

続いて、[Issue workflow](issue-workflow.md)、[コマンド](commands.md)、[マシンプロトコル](machine-protocol.md)、[Windows の動作](windows-rendering-and-durability.md)、[acceptance evidence](acceptance-evidence.md)を参照してください。
