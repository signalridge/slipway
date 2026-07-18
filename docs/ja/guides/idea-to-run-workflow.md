# アイデアから Run までのワークフロー

`slipway-workflow` は明示的に呼び出す host-side coordinator であり、Slipway の Issue workflow が定義する機能をつなぎます。Rough idea、既存の Objective または Change、既存の Run のいずれからでも、次にユーザーが所有する capability または明示的な no-further-action outcome へ案内できます。Host にインストールされた skill 群を汎用 pipeline に変えず、永続 workflow state を追加せず、Run scheduler を複製しません。

この guide では host 固有の invocation syntax ではなく capability 名を使います。各 entry point は [host adapter](../reference/adapters.md) を参照してください。

## 1 回の invocation が許可する範囲

Workflow は current fact を調査し、decision interview を行い、現在の段階で必要なら work-item draft を合成できます。ただし、次の capability boundary は越えず、ユーザーが no further action を選んだときに boundary を発明しません。

| Function | AI がここで自律的に行えること | ユーザーが所有し続けること |
| --- | --- | --- |
| Orient と draft | Repository と lifecycle の fact を読み、interview を整理し、Change または Objective を選択して complete draft を合成する | Genuine な product decision または risk decision |
| Publish | 想定される publication shape と route を説明する | 別個の `slipway-propose` invocation と、その exact external-write plan に対する current confirmation |
| Decompose | Objective に self-contained child Change が必要な理由と route を説明する | 別個の `slipway-decompose` invocation と、その exact operation plan に対する current confirmation |
| Execute または resume | Source、Run、budget の route を説明する | 別個の `slipway-run` invocation |

「Stateless」とは、workflow が Slipway Run、journal、cross-stage cursor を作らないことだけを意味します。Conversation、document、tracker artifact、prototype、code change は依然として state または side effect です。Workflow 自身は read-only です。既存の external/unmanaged artifact は non-authoritative planning input にできますが、valid managed Objective、Change、Run record は contract-defined な routing/source authority を保持します。Artifact の新規作成や変更は別個の明示的な detour です。

## 最短の valid route

Workflow は観測した starting point を調べ、不要な stage を飛ばします。

| Starting point | 直後の owner または terminal outcome | 条件付き downstream route |
| --- | --- | --- |
| Rough idea、clarified conversation、spec、PRD、map、ticket list | Complete な Change または Objective draft を作った後の `slipway-propose` | Published Change → `slipway-run`; published Objective → `slipway-decompose` → selected child Change → `slipway-run` |
| Endpoint が draft ではなく bounded summary である、明示的な decision-only clarification request | `slipway-clarify` | Clarify は stateless のままで、materialize も execute もしない |
| Structurally valid な Objective | `slipway-decompose` | Child Change の publication が成功した後に `slipway-run` |
| Structurally valid かつ self-contained な `change/v2` Issue | `slipway-run` | Run が Orient、必要時の Clarify、Implement、advisory Review、Summary を所有する |
| 明示的に private、tiny、urgent、offline、または intentionally untracked な bounded goal | `slipway-run` | Sharpened goal で new ad-hoc Run を開始し、Issue source を暗黙に仮定しない |
| Active Run | `slipway-run` | Exact current Action と submit/skip variant を使う。Stop は public command、take-over/reorder は先に stop して control を返す |
| Paused または stopped Run | `slipway-run` | 現在の structured recovery variant を使い、prose から再構築しない |
| Failed、partial、ambiguous な Propose/Decompose publication | 元の `slipway-propose` または `slipway-decompose` owner | 利用可能な receipt/operation/item/revision fact をすべて返し、owner が same-receipt reconciliation または contract-required な fresh preview/confirmation を決める |
| Ended Run または Review finding | No further action を含む provenance-aware な genuine choice を 1 つ | Stop して capability なし、new tracked scope → Propose、同じ accepted Change scope → fresh-fetch/attest 後の new issue-backed Run、Change なし → new ad-hoc Run |

ユーザーが continuation を選んだ場合だけ、Workflow は直後の capability をちょうど 1 つ示し、自身では呼び出しません。Ended work、advisory finding、またはユーザーが abandoned した publication attempt では、no further Slipway action が valid terminal outcome です。Exact remaining state を報告し、capability を発明しません。Failed、partial、ambiguous な publication は元の Propose/Decompose owner に戻り、Decompose または Run へ進みません。Workflow は operation を blind retry、restart、invent せず、owner が same receipt または contract-required な fresh preview/confirmation を使います。

ユーザーが求める endpoint が draft または publication ではなく bounded decision summary だけなら、standalone `slipway-clarify` を利用できます。Workflow はそれを drafting route の必須 stage にしません。Direct `slipway-implement` と `slipway-review` は、Run attribution や pinned Issue source を伴わない standalone path をユーザーが意図的に望む場合にだけ利用できます。Managed な Change-to-Run route の shortcut ではありません。Active Run はすでに Action loop を所有します。Action variant は submit と skip だけで、stop は public `slipway stop`、take-over/reorder は先に stop して control を返します。

Ended Run は terminal であり、resume しません。Finding は advisory で、ユーザーは accept して no further action を選べます。New issue-backed attempt を選んだ場合、ended Run の pinned snapshot を new source evidence として再利用せず、canonical Change を fresh-fetch/attest します。Change を持たない ended ad-hoc Run または standalone Review finding は、retry を選んだ場合だけ sharpened goal で new ad-hoc Run を使います。New/changed tracked scope は Propose を通ります。

## Skill catalog routing ではなく decision interview

Workflow は最初に current Git state、関連 code/test、repository の verification convention を確認します。調査できる fact はユーザーに質問しません。Genuine な human decision が残る場合は、一度に 1 つだけ質問し、推奨、根拠、代替案、trade-off を示します。Complete request なら interview は不要です。

Workflow は self-contained です。すでにインストール済みで model-invocable な `/grilling` primitive だけを optional external accelerator として利用できます。利用する場合も one-question-at-a-time と shared-understanding の規則を維持します。その confirmation が確認するのは draft に使う理解であり、publication、decomposition、implementation、Run の authority は付与しません。`/grilling` がなくても workflow は止まらず、installation も開始しません。

Workflow は host の他の skill を discover、rank、invoke しません。User-only front door は human-only のままであり、external implementation/review skill が Slipway Run を置き換えることもありません。別の ADR、report、prototype、persistent wayfinding map が必要なら、workflow は detour を説明して停止します。ユーザーはその tool を別途呼び出し、後で output を non-authoritative planning input として戻せます。

## 適切な work-item level を選ぶ

**Change** は、独立して delivery、verification、revert ができ、安全な repository state を残し、おおむね 1 fresh Agent context に収まる 1 つの結果です。5 つの independently addressable role を持ちます。

- **Outcome**
- **Requirements** — behavior と contract を優先し、exact path、format、example 自体が必要な constraint なら保持する
- **Acceptance examples** — 客観的に検証可能にする。User-facing work では external behavior を優先し、refactor/maintenance work では preserved behavior と measurable internal outcome を組み合わせられ、research では delivery する evidence と conclusion を扱う
- **Constraints**
- **Non-goals**

**Objective** は、独立して有用な delivery を必然的に複数必要とする destination です。Planning-only で、次を含みます。

- **Problem**
- **Outcome**
- **Requirements**
- **Shared constraints**
- **Non-goals**
- **Changes** — provisional tracer-bullet slice と blocker edge を含む

Issue-backed Run を開始できるのは、manifest-addressed chapter が source validation を通る、structurally valid かつ self-contained な `change/v2` Issue だけです。Objective は executable ではありません。純粋な調査は `kind:research` Change とし、evidence-backed conclusion を delivery します。その後の code は別の Change で扱います。

## Publication と source の handoff

Workflow が返すのは complete draft と intended publication shape であり、Propose の approved publication plan ではありません。Repository refetch、operation/item identity、exact body/digest、relation revision、preview、reconciliation、その plan への current confirmation は `slipway-propose` だけが所有します。Child Change の対応する operation は `slipway-decompose` が所有します。

Change publication の成功後、host は canonical URL と number を報告します。Objective の場合、decomposition の成功後にすべての child URL を報告し、advisory unblocked frontier を説明し、ユーザーの選択権を保ったまま 1 つの Change を推奨します。その後にだけ、ユーザーは selected Change に対して `slipway-run` を明示的に呼び出します。

Host はその exact Change を fetch/attest し、temporary Source Bundle envelope を組み立て、`--source-file` で local CLI に渡します。CLI は GitHub を fetch せず、bare Issue number は CLI source ではありません。

New Run の開始時にユーザーが initial budget override を指定しない場合、host は contract default の `8` を明示して使います。明示的な override は `1..1000` です。より大きな値を推奨する場合は理由が必要で、completion は約束しません。Resume は下記の distinct remaining-budget rule を使います。Tiny、private、urgent、offline、または意図的に untracked な work では、workflow は sharpened goal に対する explicit ad-hoc `slipway-run` へ案内できます。

Workflow は追加の governance gate を導入しません。各 external-write operation は operation-scoped current confirmation を保持し、Run start も別個の explicit action のままです。

## 「自律的に実行する」の正確な意味

明示的に開始した Run は pinned Requirements と budget の範囲で Action を 1 つずつ進め、`paused`、`stopped`、`ended` のいずれかで止まります。Run Clarify は genuine decision のために停止できます。`budget_exhausted` は正常で resume 可能な pause です。Explicit resume の `--budget N` は残量を置き換え、省略時は正の残量を維持し、0 の場合は mutation が実際に Run を resume するときだけ `max(initial_budget, 3)` まで補充します。

Review は read-only で advisory です。Finding は automatic Implement/re-review loop を作りません。`ended` は automatic Action queue が空であることだけを意味し、correctness、completion、ship readiness の証明ではありません。
