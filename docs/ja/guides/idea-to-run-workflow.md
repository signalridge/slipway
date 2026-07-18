# アイデアから Run までのワークフロー

`slipway-workflow` は、rough idea から Slipway work item draft までをつなぐ、明示的に呼び出す host-side bridge です。AI は調査、interview の構造化、draft の合成を自律的に行えますが、それらを永続的な governance pipeline にしません。CLI command、Run state、quality gate、automatic repair loop は追加されません。

以下では capability 名だけを示します。各 host での呼び出し方は [host adapter](../reference/adapters.md) を参照してください。

## 権限の境界

1 回の明示的な `slipway-workflow` 呼び出しが許可するのは最初の行だけです。

| Phase | AI が自律的に行えること | ユーザーが保持する権限 |
| --- | --- | --- |
| 調査と draft | repository fact の確認、限定された unknown の調査、interview の構造化、Change/Objective の選択、draft の合成 | 本当の product/risk decision、別途依頼された artifact write |
| Publish | 自動では何もしない | `slipway-propose` の別個の呼び出しと、正確な external-write plan への current confirmation |
| Decompose | 自動では何もしない | publish 済み Objective に対する別個の `slipway-decompose` 呼び出し |
| Execute | 自動では何もしない | 別個の `slipway-run` 呼び出し、source choice、initial budget |

ここでいう stateless は、Slipway Run、journal、phase 間 cursor を作らないという意味だけです。Conversation、document、tracker Issue、prototype、code change は依然として state または side effect です。通常の idea-to-draft path は read-only です。Artifact を生成する detour は、ユーザーの元の依頼がその artifact と scope を既に許可している場合だけ行い、必ず報告し、それ自体を executable source にしません。

## 自律的な前半

Host は最初に current Git state、関連 code/test、repository の verification convention を確認します。調査できる fact をユーザーに質問しません。本当に人間だけが決められる事項が残る場合は、一度に 1 つだけ質問し、観測した fact に基づく推奨、代替案、trade-off を示して回答を待ちます。Request が完全なら質問は 0 件です。

Matt の installed かつ model-invocable な `/grilling` primitive は、decision interview の optional accelerator です。Model-reachable なので workflow から `/grilling` skill を実行できますが、その場合も one-question-at-a-time と shared-understanding confirmation の規則を守ります。存在しなくても workflow は止まらず、install も開始しません。

Artifact-producing primitive は read-only shortcut ではありません。`/domain-modeling` は glossary/ADR material、`/research` は Markdown report、`/prototype` は throwaway code を書きます。ユーザーの元の依頼がその exact artifact と scope を別途許可している場合だけ呼び出し、それ以外は直接調査して残る uncertainty を報告します。未解決の decision map が 1 context に安全に収まらない場合、host はそれを黙って永続化せず、bounded map と次の推奨を返します。利用可能なら人間が `/wayfinder` を別途呼び出すか、新しい明示的な workflow continuation を開始します。

## Matt Pocock の方法との対応

Matt の `/grill-me`、`/grill-with-docs`、`/wayfinder`、`/to-spec`、`/to-tickets`、`/implement`、`/ask-matt` front door は user-invoked であり、別の skill から起動できません。そのため Slipway は有用な discipline を内在化し、それらの command は optional wizard path として残します。`code-review` の invocation setting に関係なく、handoff 後の execution と Review は Slipway Run が所有するため、この workflow からは呼び出しません。

| Matt の方法または出力 | Slipway workflow での意味 |
| --- | --- |
| `grill-me` / `grill-with-docs` | Installed なら model-invocable `/grilling` primitive を再利用可能。Front door は human-only のままで、document write には別の authorization が必要 |
| `wayfinder` の destination / Issue map | 独立 delivery が複数必要なら 1 Objective。Multi-session map は別途呼び出す external workflow |
| `to-spec` の spec/PRD | 下記 Change/Objective section に正規化する planning input |
| `to-tickets` の tracer bullet | Objective の provisional Changes。その後、明示的な `slipway-decompose` が marker-valid child Change を作成 |
| `implement` / `code-review` | Handoff 前は使用しない。以後の Implement と advisory Review は Slipway Run が所有 |

`slipway-workflow` は `/grill-me`、`/grill-with-docs`、`/wayfinder`、`/to-spec`、`/to-tickets`、`/implement`、`/code-review`、`/ask-matt` を決して呼び出しません。ユーザーは user-only command を別の multi-command wizard として個別に呼び出せます。

## 適切な draft level

**Change** は、独立して delivery、verification、revert ができ、安全な repository state を残し、おおむね 1 fresh Agent context に収まる結果です。5 つの independently addressable role を持ちます。

- Outcome
- Requirements
- Acceptance examples
- Constraints
- Non-goals

**Objective** は、独立して有用な delivery が複数必要な大きな destination です。Planning 専用で、次を持ちます。

- Problem
- Outcome
- Requirements
- Shared constraints
- Non-goals
- Changes（provisional tracer-bullet slice と blocker edge を含む）

Issue-backed Run を開始できるのは self-contained で marker-valid な `change/v2` Issue だけです。Objective は開始できません。純粋な調査は `kind:research` Change とし、code ではなく evidence-backed conclusion を delivery します。

## Publication と source の handoff

Workflow は complete draft と intended publication shape を返して停止します。Approved exact publication plan を作ったとは主張しません。Repository refetch、operation identity、exact body/digest、relation revision、preview、その plan への 1 回の current confirmation は `slipway-propose` だけが所有します。[GitHub Issue workflow](github-issues.md) を参照してください。

通常の spec/tracker Issue（`to-spec` や `to-tickets` の出力を含む）は non-authoritative planning input であり、直接 Run に渡せません。Change publish 後、host は canonical URL と number を報告します。その後ユーザーが別途 `slipway-run` を明示的に呼び出すと、host が正確な Change を fetch/attest し、temporary Source Bundle envelope を組み立て、`--source-file` で local CLI に渡します。CLI は GitHub を fetch せず、bare Issue number は CLI source ではありません。

小さく、private、urgent、offline、または意図的に untracked な作業では、workflow は sharpened goal に対する別個の ad-hoc `slipway-run` を推奨できます。これは explicit source choice であり、workflow が execution を開始する抜け道ではありません。

## 「自律的に実行する」の正確な意味

Issue-backed Run は `1..1000` の範囲で deliberate budget を指定して開始します。Pinned Requirements と利用可能 budget の範囲で Action を 1 つずつ進め、`paused`、`stopped`、`ended` のいずれかで止まります。Run Clarify は genuine decision のために停止できます。良い draft はその可能性を下げますが、無効化はしません。`budget_exhausted` は正常で resume 可能な pause です。Resume で明示的な `--budget N` は remaining budget を `N` に置き換え、省略時は正の残量を保持し、0 を `max(initial_budget, 3)` まで補充します。Replacement は mutation が実際に Run を resume した場合だけ適用されます。

Review は read-only で advisory です。Finding は automatic Implement/re-review loop を作りません。`ended` は automatic Action queue が空であることだけを意味し、correct、complete、shippable の証明ではありません。Finding は new Change にするか、同じ Change に別の Run を開始できます。[Run、recovery、privacy](runs-and-recovery.md) を参照してください。
