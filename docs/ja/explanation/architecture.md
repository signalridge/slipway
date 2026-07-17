# アーキテクチャ

Slipway は制御 loop を local CLI に置き、モデル 固有の作業を生成された ホスト アダプターに置きます。この境界により、CLI は モデルや GitHub 認証情報を持たずに state を検証できます。

![Slipway process architecture: ユーザーが AIコーディングツール の generated capability を明示的に呼び出し、ホストが model、repository、認可済み GitHub work を担い、versioned JSON で local CLI と durable Run store に接続する。](../../assets/diagrams/architecture.svg)

## Process boundary

```text
User
  └─ 生成された capability を明示的に呼び出す
       └─ AIコーディングツール
            ├─ リポジトリ を読み、変更する
            ├─ モデル と開発 tool を呼び出す
            ├─ 認可された場合 GitHub data を fetch/publish する
            └─ Slipway と versioned JSON を交換する
                 └─ local CLI と Run store
```

Run state engine は モデルや GitHub API を呼び出しません。Host 提供の ソースエンベロープ を検証し、1度に1つの Action を schedule し、Git を独立観測し、recovery state を保存します。Public `doctor` コマンドは コマンド layer の診断上の例外で、authentication と リポジトリ permission の確認にユーザー環境の `gh` を呼び出す場合があります。Generated ホスト instruction は ホストが調査・公開・実装・報告する方法を定義しますが、第2の state engine ではありません。

## Package 方向

Production dependency は architecture test で制約されます。

```text
cmd ───────────────→ adapter
 │                   ├─→ tmpl
 │                   ├─→ fsutil
 │                   └─→ jsonstrict
 ├─────────────────→ autopilot
 │                   ├─→ runstore
 │                   │    ├─→ fsutil
 │                   │    └─→ jsonstrict
 │                   └─→ jsonstrict
 └─────────────────→ recoverycmd
```

| Package | 責務 |
| --- | --- |
| `cmd` | Cobra command、human/JSON output、root discovery、exit behavior。 |
| `internal/autopilot` | Action/Outcome validation、routing、source candidate、budget、structured recovery。 |
| `internal/runstore` | Journal replay、projection、locking、material storage、Git observation。 |
| `internal/adapter` | Host registry、generated file、ownership manifest、transactional install/remove。 |
| `internal/tmpl` | 複数 ホスト 共通の embedded capability instruction。 |
| `internal/fsutil` | Anchored path、no-follow operation、transaction、sync、platform safety。 |
| `internal/jsonstrict` | Protocol、source、store、アダプター 境界で共有する strict JSON decoding。 |
| `internal/recoverycmd` | すでに structured な argv を人間向けに render する。 |

下位 package は コマンド や host-policy layer を逆 import しません。GitHub publication は core 内の network provider にはならず、generated ホスト instruction に残ります。

## Run 開始と リポジトリ 観測

新規 Run は worktree root、per-worktree Git directory、Git common directory の3つの canonical path を発見します。それらの framed identifier が Run をその worktree に紐付けます。Slipway は worktree を作成・切替・削除しませんが、別 worktree identity からの Run 変更は拒否します。

Initial Git observation は exact index/porcelain-v2 output の fingerprint と、dirty path の bounded metadata/fingerprint を保存します。Raw Git stream や file content は保存しません。後続 observation は diff-first routing と、中立的な「since start changed」報告を支え、誰が変更を引き起こしたかは主張しません。

## Run storage

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl   append-only transition record
├── run.json        replaceable projection
├── run.lock        validated coordination artifact
└── materials/      content digest で保存した accepted source section
```

Unix では opened Run directory 上の OS lock が writer を直列化し、Windows では named mutex が担います。可視の `run.lock` は検証と診断に使われますが、唯一の writer guard ではありません。

Mutation は参照される material を先に書き、その後で ジャーナル event が参照できます。Journal を sync してから projection を置き換えます。Journal commit 後に projection refresh が失敗した場合、error は committed mutation と stale projection を報告し、rollback は主張しません。

## Source boundary

Issue-backed work では、trusted ホストが Issue と manifest 参照 comments を fetch し、temporary strict envelope を渡します。CLI は内部整合性と stable ID を検証できますが、ホストが GitHub から誠実に取得したことを暗号論的に証明はできません。

Accepted section は content-addressed で、local material reader 経由で利用できます。Action は revision と bounded catalog だけを持ち、大きな requirements が Action context に入らず、offline 復旧も可能です。

設計理由と却下した代替案は [ADR-0001](../../../adr/0001-source-bundle-v2.md) に記録されています。issue #434 の完全な契約と versioned schema が規範であり、runtime test は現在の実装に対する executable evidence です。

## Security boundary

![Slipway の trust boundary: Issue content と working tree は untrusted data であり、shell 権限の付与、認証情報 の開示、confirmation の回避、destructive scope の拡大はできません。AIコーディングツール は行為を託され、すべての 認証情報を保持します。Local CLI は厳密な JSON・サイズ・identity・digest を検証しますが 認証情報 は一切持たず、ホストが GitHub を誠実に fetch したことは証明できません。](../../assets/diagrams/trust-boundary.svg)

Slipway は、同じ account の process、root、malware、compromised ホストがその保護を超え得ると仮定します。その境界内で次を行います。

- filesystem operation を anchor し、unsafe symlink traversal を拒否する。
- strict JSON、size、identity、digest を検証する。
- 認証情報を Slipway storage に保存せず、GitHub fetch/publication を Run core の外に置く。
- One-shot destructive grant と自然言語 answer を分離する。
- user-modified な generated file を保持する。
- platform durability limitation を報告する。

Issue content は data であり ホスト instruction ではありません。Generated capability は Issue 内の command、link、認証情報 request を権限として扱ってはなりません。

## 意図的に扱わないこと

Slipway は次を行いません。

- Hosted service や project tracker の実行。
- Model-provider や GitHub 認証情報 の管理。
- worktree の作成・管理。
- merge、deployment、release readiness の認定。
- test、finding、label、Issue state を汎用 リポジトリ policy にすること。
- Review finding の自動修正。
- user-modified な アダプター file の上書き。

外部の branch protection、CI、組織 policy、人間による review は独立したままです。

[コア概念](concepts.md)、[マシンプロトコル](../reference/machine-protocol.md)、[Run、復旧、プライバシー](../guides/runs-and-recovery.md)も参照してください。
