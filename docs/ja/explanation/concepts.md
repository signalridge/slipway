# コア概念

Slipway は user intent、ホスト execution、durable Run state を分離します。Machine protocol を読む前に、以下の用語だけで product を理解できます。

![Slipway Run lifecycle: explicit start から1回1 Actionの Action/Outcome loopに入り、userは skip、pause、stop、resumeでき、endedはautomatic Action queueが空であることだけを表す。](../../assets/diagrams/lifecycle.svg)

## Run

**Run** は1つの Git worktree で goal を実行する、1回の interruptible attempt です。Bounded Action budget、recovery history、immutable start workspace identity を持ちます。1つの task に複数 Run を作れますが、1つの Run の primary source は最大1つです。

Run の開始は常に explicit です。Slipway は chat を監視せず、ordinary conversation を work に変換せず、ambient hook から開始しません。

## Action と Outcome

CLI は1回に1つの **Action** を返します。

| Action | Host responsibility |
| --- | --- |
| `orient` | Repository fact、Git state、convention を調べ、次の useful step を選ぶ。 |
| `clarify` | Repository fact で解決できない genuine human decision を1つだけ質問する。 |
| `implement` | Bounded technical change を行い、actual activity と file を報告する。 |
| `review` | Observed change を read-only で確認し、intent/quality finding を報告する。 |
| `summarize` | Observed change、activity、finding、known issue、不確実性をまとめる。 |

Review は default で有効ですが、Slipway が code change を観測した後だけ issue されます。`--no-review` はその Run の Review を無効にします。

ホストは structured **Outcome** で回答します。Slipway は Outcome を validate/record し、Git を独立観測して次の Action を選びます。Reported activity は成功の証明ではなく、observed diff は原因となった process の証明ではありません。

## Source

Run の source は2種類です。

- **Ad hoc:** User-provided goal が source。
- **GitHub Change:** Host が self-contained Change Issue を import し、accepted section を digest で pin。

Source snapshot は Run 中に silently change しません。Refresh 後に accepted content が異なる場合、pinned snapshot を保つか candidate を adopt するか明示的に選びます。History fork には新しい Run が必要です。

## Objective と Change

**Objective** は複数の independently deliverable Change が必要な outcome の optional planning structure で、executable ではありません。

**Change** は1つの coherent result を持つ self-contained work item です。Execution に必要な requirements をすべて含み、Run は parent Objective を runtime に読んで補いません。

Repository の owner が personal account か Organization かは、この モデルに影響しません。[GitHub Issue ワークフロー](../guides/github-issues.md)を参照してください。

## Budget と pause

Action budget は CLI が issue できる Action 数を制限するだけで、time estimate や quality score ではありません。Run は human decision、ソース選択、environment unavailable、exact destructive confirmation でも pause します。

ユーザーは Action の skip、Run の stop/resume、reorder、take over ができます。Skip に理由は不要です。Resume は original worktree identity を再検証し、protocol に従って stale pending work を無効化します。

## Review と completion

Review は advisory/read-only です。Finding は summary に入り、自動 repair/re-review loop は開始しません。

`ended` は automatic Action queue が空であることだけを示します。Test pass、finding なし、branch protection、PR approval、release readiness を意味しません。Slipway が事実を報告し、user と リポジトリ policy が次を決めます。

## Local state

Recovery data は `<git-common-dir>/slipway/runs/` にあります。Append-only ジャーナル が transition を記録し、replaceable projection が read を補助し、accepted Issue section は content-addressed material として保存されます。[Run、復旧、プライバシー](../guides/runs-and-recovery.md)を参照してください。

Exact JSON shape は[マシンプロトコル](../reference/machine-protocol.md)にあります。
