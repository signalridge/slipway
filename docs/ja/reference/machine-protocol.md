# マシンプロトコル

このページは adapter や integration author 向けです。通常ユーザーは生成された host capability を呼び出し、[コマンドリファレンス](commands.md)を使います。

現在の JSON contract version は **2** です。

- [machine-protocol.schema.json](../../reference/v2/machine-protocol.schema.json) は public command、Action、Outcome、status、error、recovery の shape を定義します。
- [source-envelope.schema.json](../../reference/v2/source-envelope.schema.json) は GitHub source transport shape を定義します。

実行可能な開始から終了までの host exchange は[マシンプロトコル v2 チュートリアル](../guides/machine-protocol-v2.md)を参照してください。

Schema は serialization shape を定義します。Runtime は JSON Schema で表現しきれないルールも検証します。Embedded manifest syntax、ordering、hash、cross-field identity、idempotency、workspace state、filesystem safety などです。Schema を検証し、prose から Go validator を再実装せず、CLI error を保持してください。

## Process boundary

Host は model を呼び出し、repository を読み、tool を実行し、要求されたときに GitHub credential を使います。この protocol exchange の Run/source path は local かつ deterministic です。Message を検証し、Run を記録し、Git を観測し、model や GitHub を呼び出さずに次の operation を返します。独立した public `doctor` command は read-only diagnosis のためにユーザー環境の `gh` を呼び出す場合があります。

Host は通常、各 step で JSON を使います。

```text
slipway run --budget N --json --root ROOT [--no-review] [--source-file FILE] -- GOAL
slipway protocol submit --run RUN --action ACTION --root ROOT (--outcome-file FILE | --outcome-stdin)
slipway protocol answer --run RUN --action ACTION --root ROOT --text TEXT
slipway protocol answer --run RUN --action ACTION --root ROOT --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway protocol skip --run RUN --action ACTION --root ROOT
slipway protocol resume RUN --root ROOT [--budget N]
slipway protocol resume RUN --root ROOT (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway protocol material --run RUN --action ACTION --root ROOT --section KEY
```

Protocol 操作は versioned host interface です。Documented であり help にも表示されますが、代替の end-user command sequence ではありません。CLI が返す structured `next` variant から実行してください。

## Run の開始

Canonical invocation はすべての flag を1つの `--` separator の前に置き、literal goal をその後に置きます。

```bash
slipway run --budget 8 --json --root /absolute/worktree -- "one goal"
```

Ad-hoc Run は source field を省略します。Issue-backed Run は private temporary `--source-file` を指定します。CLI は1度だけ消費し、その後 file や GitHub に依存しません。

Start response には Run state、初期 `orient` Action、structured `next` operation が含まれます。

## Action

Active Run には null でない Action が1つ含まれます。

```json
{
  "contract_version": 2,
  "run_id": "...",
  "action_id": "...",
  "kind": "orient",
  "goal": "...",
  "brief": "...",
  "context": "...",
  "remaining_budget": 7
}
```

`kind` は `orient`、`clarify`、`implement`、`review`、`summarize` のいずれかです。

Issue-backed Action はさらに次を含みます。

- source、manifest、requirements revision。
- 順序付き bounded section catalog。
- Structured `protocol material` reader。
- 現在 Action に必要な section key。

Requirements Markdown は `context` に複製されません。Material reader は current non-void Action にだけ有効で、content を返す前に digest、byte count、section revision を検証します。

これらの key の versioned field は `requirements.required_for_action` です。Protocol v2 では `requirements.sections` にある全 key の ordered list と厳密に等しく、host がより小さい subset を推論してはいけません。

`context` は active answer と以前の Outcome summary の bounded projection であり、完全な journal、source、conversation、hidden model reasoning ではありません。

## Outcome

Outcome はちょうど1つの入力から提出します。

```text
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-file FILE
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-stdin
```

公開 Outcome field はすべて必須です。空の集合も array のままで、該当しない object branch は JSON `null` にします。

```json
{
  "contract_version": 2,
  "action_id": "...",
  "action_kind": "orient",
  "status": "completed",
  "summary": "observed facts",
  "observations": [],
  "known_issues": [],
  "suggested_actions": [],
  "pause": null,
  "implementation": null,
  "review": null
}
```

`action_kind` は outstanding Action と一致する必要があります。Host status は `completed`、`needs_input`、`partial`、`error` で、skip は Outcome status ではなく CLI operation です。

- Orient または Clarify は最大1つの `clarify`、`implement`、`summarize` Action を提案できます。
- Non-paused Implement は `implementation` branch を使い、実際の file、attempt、uncertainty、test/type-check/build/lint activity と exit code を報告します。
- Non-paused Review は `review` branch を使い finding を報告し、repair work を提案しません。
- Summary とすべての `needs_input` Outcome は suggested Action を持ちません。

### 正当な Outcome 組み合わせ

| Action | Host status | 必須 result branch | 許可される pause | 許可される suggestion |
| --- | --- | --- | --- | --- |
| Orient | `completed` / `partial` / `error` | `implementation=null`、`review=null` | なし | Clarify、Implement、Summarize のいずれかを 0 または 1 件 |
| Orient | `needs_input` | `implementation=null`、`review=null` | decision または environment | なし |
| Clarify | `completed` / `error` | `implementation=null`、`review=null` | なし | Clarify、Implement、Summarize のいずれかを 0 または 1 件 |
| Clarify | `needs_input` | `implementation=null`、`review=null` | decision または environment | なし |
| Implement | `completed` | `implementation.result=applied\|not_needed`、`review=null` | なし | なし |
| Implement | `partial` | `implementation.result=partial`、`review=null` | なし | なし |
| Implement | `error` | `implementation.result=unable`、`review=null` | なし | なし |
| Implement | `needs_input` | `implementation=null`、`review=null` | decision、destructive、environment | なし |
| Review | `completed` | `review.result=no_findings_reported\|findings_reported`、`implementation=null` | なし | なし |
| Review | `partial` | `review.result=inconclusive`、`implementation=null` | なし | なし |
| Review | `error` | `review.result=error`、`implementation=null` | なし | なし |
| Summarize | `completed` / `error` | `implementation=null`、`review=null` | なし | なし |

Clarify には意図的に `partial` の正当な組み合わせがありません。1つの Action は1つの decision だけを扱います。Review は `needs_input` や Implement suggestion を使えず、`not_run` は CLI が生成する Review-skip projection だけに属します。

`needs_input` Outcome の pause reason は `decision_required`、`destructive_confirmation_required`、`environment_unavailable` のいずれかです。`budget_exhausted` は CLI だけが生成します。

Destructive confirmation は exact current Implement request と scope digest にだけ有効です。「yes」などの自然言語は authorization ではなく feedback です。Action 変更、resume、scope 拡大、mismatch は grant を無効化します。

Outcome input は上限 1 MiB、有効 UTF-8 で、BOM、duplicate/unknown field、trailing data を含んではなりません。

## Structured `next`

継続可能な success/error には typed `next` object が含まれます。

- `operation`：`action`、`answer`、`resume`、`start`、`command`、`none`。
- 元の `workspace_identity`。
- `id`、`base_argv`、typed input を持つ variant が0個以上。

Input type は `string`、`path`、`enum`、`digest` です。Consumer は1つの variant を選び、入力値を schema 順に独立 argv element として挿入します。Display command を parse/連結してはなりません。

すべての required input が解決された variant だけが human shell command に render できます。POSIX、`cmd.exe`、PowerShell rendering は presentation であり、machine value は structured argv です。

Windows display command に `cmd.exe` では安全に保持できない expansion-sensitive な `%` または `!` が含まれる場合、renderer は PowerShell UTF-16LE `EncodedCommand` trampoline を使用します。これは copyable display form だけを変え、decode 後の process argv は structured variant と byte-for-byte で等価でなければなりません。

Ended Run は `operation: "none"` と空の variant list を使います。

## Source envelope

Source envelope は上限 16 MiB で、repository/Issue node ID で1つの `github.com` Issue を識別します。Valid Change では次を満たします。

- body の最初の非空行は `<!-- slipway-level: change/v2 -->`。
- 次の非空 block は厳密に1つの `slipway-manifest` JSON fence。
- ordered manifest は5〜64の section entry を持ち、outcome、requirements、acceptance examples、constraints、non-goals role を含みます。
- envelope は参照された comments をちょうど含みます。
- 各 comment は exact section marker で始まり、宣言 digest と一致します。

Normalized section は最大 256 KiB、完全 section payload は最大 4 MiB、manifest は最大 256 KiB です。Missing、extra、duplicate、minimized、edited、oversized、hash-mismatched reference は拒否されます。

Top-level source schema は invalid refreshed head で空または whitespace-only の Issue/comment body と空 comments array を許可します。これにより、marker 不足、空の referenced section、digest mismatch は host が envelope を先に拒否したり無関係な議論を収集したりせず、CLI によって分類されます。Embedded manifest string と semantic digest check は runtime が検証し、top-level schema 単独ではありません。

CLI は stable identity、provenance、byte count、revision、content-addressed accepted material を保存し、raw envelope、title label、source-file path、unreferenced comment を journal に書きません。

## Refresh と candidate

Issue-backed resume は次のいずれかを明示的に行います。

- fresh envelope を import して比較する。
- pinned snapshot を継続する。
- exact current candidate で keep pinned または adopt valid candidate を選ぶ。

Source option 省略は「unchanged」でも暗黙の network access でもありません。別 Issue identity や異なる parent requirements revision の amendment は Run を変更せずに拒否されます。Candidate ID と choice は stale-safe idempotency を持ちます。

成功した resume は必要に応じて stale outstanding work を無効化し、workspace を再検証し、通常は fresh Orient を返します。`--budget` 省略時は正の remaining budget を保持し、0なら `max(initial_budget, 3)` まで補充します。明示的な `--budget N` は `N` に置き換えます。Replacement は実際に Run を resume する mutation でのみ適用されます。

## Workspace と Git 観測

Workspace identity には canonical worktree root、per-worktree Git directory、Git common directory が含まれます。Load や mutation のたびにこれらの path を再発見・比較してから journal を変更します。

Repository-wide `status` は filesystem read-only の例外です。Namespace/lock の作成、permission 変更、journal byte の修復を行いません。別 linked worktree の Run は `workspace_foreign` 付き FirstEvent header stub として表示され、owning worktree 外では完全に replay しません。読めない local Run directory は list JSON の `unavailable_runs` に identity を残し、targeted read は corruption と absence を区別します。

Git observation は index、porcelain status、dirty path の hash と bounded metadata を記録し、file content は記録しません。差異は Run start 以降の変更の証拠であり、現在の host が原因であることの証明ではありません。

## Idempotency と順序

- Outcome idempotency は元の accepted input bytes で hash を計算し、意味的に同じでも serialization が異なる JSON は conflict します。
- Answer、skip、resume、candidate operation はすべて current ID に紐付き、stale/conflicting retry を拒否します。
- 各 Run の writer は同時に1つで、platform lock implementation が強制します。
- Journal order が recovery record で、`run.json` は再構築可能です。

## Error と互換性

Machine error には `contract_version`、stable `code`、human `message`、`exit_code` が含まれ、回復可能なら structured recovery も付きます。Consumer は code/version で分岐し、message text で分岐してはなりません。

`journal_record_too_large` は厳密な `context`、`size`、`limit` detail field を持ち、Run ID が既知ならその Run への read-only `status` recovery variant も持ちます。Oversized record の拒否は persistent Run を終了させず、無効にもしません。

未知の contract version と field は拒否されます。Version 2 は、それ以前の未公開開発 format との互換性を約束しません。将来の非互換変更は新しい明示的な version を使うべきで、暗黙の alias を加えてはなりません。
