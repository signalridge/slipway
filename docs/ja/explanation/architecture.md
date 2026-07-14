# アーキテクチャ

> 日本語版は非規範です。完全な[中国語製品契約](../../zh/reference/product-contract.md)を参照してください。

Slipway は、範囲を限定した復旧可能な AI 支援作業のための小規模な control plane です。AI coding tool、project tracker、Git の代替ではありません。Host が作業を実行する一方で、Slipway は versioned Action を1つずつスケジュールし、Git を独立に観測し、source を digest で固定して、復旧履歴を保存します。

## 依存関係の方向

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil (only required low-level primitives)
```

依存関係の方向は固定されており、architecture guard test で強制されます。`autopilot` は構造化された `next` value だけを生成し、`recoverycmd` には依存しません。`recoverycmd` は journal を読まず、route を決定することもありません。

## パッケージ

| パッケージ | 責務 |
| --- | --- |
| `cmd` | 7つの public command、hidden versioned machine command、text/JSON rendering。 |
| `internal/autopilot` | 厳密な Action/Outcome union、source envelope/revision/candidate、budget、routing、destructive authorization、構造化された `next` value。 |
| `internal/runstore` | canonical Git identity を検出し、anchor 付き append-only journal と置き換え可能な projection を管理します。 |
| `internal/adapter` | 10種類の host に対して、ownership-safe な capability generation を計画します。 |
| `internal/tmpl` | 正確に6つの明示的な capability と、attribution を付けた `grill-me` reference を embed します。 |
| `internal/fsutil` | root を限定した atomic transaction、Git discovery、symlink/reparse defense、rollback 後の state validation。 |
| `internal/recoverycmd` | 完全な argv だけを受け取り、POSIX/cmd/PowerShell の表示用 command を render します。journal を読まず、route を決定することもありません。 |
| `internal/jsonstrict` | duplicate key、valid JSON、trailing data の拒否に使う共通 structural scanner。 |
| `internal/testlint` | repository の test policy analyzer。 |

## Run の開始と Git の観測

Run の開始時に、CLI は immutable workspace identity と Git fingerprint を保存します。fingerprint には、正確な index と porcelain-v2 の byte 列に加え、既存の dirty/untracked path すべてについて、sort 済みの metadata/digest が含まれます。復旧時には、load や mutation より前に identity を再検証します。root の再利用、別の linked worktree、移動または retarget された Git metadata は、journal を変更する前に `workspace_identity_mismatch` で失敗します。

開始時点から観測された差分は、safety 側の Review routing に使用されますが、その変更を Run が引き起こしたことの証明にはなりません。ユーザーによる同時編集、別の Run、ほかの tool が差分に寄与している可能性があります。Slipway は事実としての `observed_since_start` と `attribution_uncertainty` を記録し、差分を特定の host や Run に帰属させることはありません。

## Host と GitHub

Go binary は provider token を保持しません。host が attest した raw Change envelope を厳密に検証し、normalized pinned snapshot を journal に記録します。GitHub の読み書きは、ユーザー自身が認証した tool を用いて host 側で行います。publish には、repository Change runtime ではなく、承認済みの operation/item UUID marker と reconciliation を使用します。

model provider、old-state reader、compatibility alias、dual runtime、ambient activation、required-command registry、Spec/artifact lifecycle、worktree binding、automatic review-repair loop はありません。historical data と legacy namespace は変更せず、そのまま無視します。

## 導入しないもの

```text
internal/change   internal/spec   internal/plan
internal/lifecycle   internal/gate   internal/tracker runtime
```

[製品概要](../reference/product-overview.md)、[マシンプロトコル](../reference/machine-protocol.md)、[manifest-addressed source bundle を採用する決定](../../decisions/0001-source-bundle-v2.md)も参照してください。
