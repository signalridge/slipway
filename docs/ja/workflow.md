# ガバナンス付きワークフロー

Slipway は作業をガバナンス付きのライフサイクルに沿って進めます。

1. `S0_INTAKE`: 意図、スコープ、未解決の論点、初期エビデンスを記録します。
2. `S1_PLAN`: リサーチ、要件、決定事項、タスク、プラン監査の各成果物を作成します。プラン監査は S2 の開始を許可するレビューであり、対象はプランバンドルそのものであって、固定化されたウェーブキャッシュではありません。
3. `S2_IMPLEMENT`: 算出されたウェーブを実行します。Slipway は、現在の `tasks.md` にある各タスクの宣言済み依存関係とターゲットファイルから、ウェーブスケジュールをその場で算出します。作成者がウェーブ番号を宣言することはありません。依存関係がなくファイルが互いに重ならないタスクは同じウェーブにまとめられ、既定で並行ディスパッチされます。`slipway next --json` はそうしたウェーブを `parallel: true` と示します。順次実行に切り替えるには、`.slipway.yaml` で `execution.parallelization: off` を設定します。
4. `S3_REVIEW`: 実装を成果物と照合して検証し、選定されたレビューチェックを実行し、別のサブエージェントを通じてフィードバックを修正します。その後、単一の終端ゲートである `ship-verification`（権威ある完全なスイートを 1 回、受け入れ証跡、現在性の再チェック、`assurance.md` の証明、レビュアー独立性の証明を実施）を実行し、done-ready の結果を生成します。

アクティブなライフサイクル状態は `artifacts/changes/<slug>/change.yaml` に保存されます。バンドルローカルのライフサイクルイベントは `artifacts/changes/<slug>/events/` 配下に、スキル検証レコードは `artifacts/changes/<slug>/verification/` 配下に置かれます。ウェーブ実行中に記録されるランタイムのタスクエビデンスは `.git/slipway/runtime/changes/<slug>/evidence/...` 配下にあります。

<div align="center" markdown>

![Slipway のガバナンス付きライフサイクル: new、S0 Intake、S1 Plan、S2 Implement、S3 Review、done-ready、done。明示的なライフサイクルコマンドと、ショートカットとしての run を示す](../assets/diagrams/lifecycle.svg)

</div>

## 変更を作成する

```bash
slipway new "refresh governance docs" --preset standard
```

JSON の標準入力を使えば、AI の呼び出し側が分類を直接指定できます。

```bash
echo '{"guardrail_domain":"","needs_discovery":true,"complexity":"complex","test_cmd":"go test ./...","build_cmd":"go build ./...","languages":["Go","Markdown"]}' \
  | slipway new --json "refresh governance docs"
```

分類を省略した場合、Slipway は保守的な既定値を使います。

- `guardrail_domain=""`
- `needs_discovery=true`
- `complexity="complex"`

## 進行スタイル

ハンドオフを明示的に制御するには `next` を使います。

```bash
slipway next --json
# complete the surfaced skill or resolve blockers
slipway run --json
slipway next --json
```

オペレーターに対応を求める停止点まで Slipway に進めさせたい場合は `run` を使います。

```bash
slipway run --json --diagnostics
```

`run` は、提示されたスキル、ブロッカー、または done-ready の結果で停止します。

## 独立性証明トークン

review、ship-verification、wave-orchestration の各ステージは、検証レコードの `references` に（`slipway evidence skill --reference ...` を通じて）エンジンが消費するいくつかのトークンを記録します。各トークンは `standard`/`strict` ではエラー重大度のブロッカー、`light` では助言のみです（Pattern-A の省略として実現されます。すなわち `light` ではゲートが単にブロッカーを返さず、このシームに別途の助言チャネルはありません）。これらはいずれも現在性や最終判定の自己スタンプではありません。タイムスタンプと run バージョンのスタンプを行う唯一の主体はエンジンのままです。

| トークン | 証明する内容 | 強制 | ゲートがフェイルクローズドになったときの復旧 |
| --- | --- | --- | --- |
| チェーン参加者をまたぐ `context_origin:stage=<stage>=<handle>`。選定された S3 レビュアーはすべて `stage=review` を使い、レビュー指摘の修正がある場合はそれが `stage=fix` を使う | 各々の担当参加者が、共有ワークトリー上で互いに異なるコンテキストの下で実行されたこと。選定レビュアーはスキル名でキー付けされ、ペアごとに区別可能でなければならない。記録された fix ハンドルは実装ハンドルやレビューハンドルと一致してはならない | standard/strict はエラー、light は助言 | 担当レビュアーまたは修正を新しいネイティブサブエージェントで再実行し、異なる `context_origin` ハンドルを再発行させる |
| ship-verification 上の `closeout:reviewer_independence=pass` | 終端の ship レコードにレビュアー独立性の証明が存在すること（Pattern-A）。欠落時は `ship_verification_reviewer_independence_missing` でフェイルクローズドになる | standard/strict はエラー、light は助言 | **ship-verification** を再実行してトークンを記録する |
| ship-verification 上の `closeout:assurance_complete=pass` | ホストが、終端の ship レコード上で `assurance.md` が完成していることを証明すること。欠落時は `ship_verification_assurance_attestation_missing` でフェイルクローズドになる | standard/strict はエラー、light は助言 | **ship-verification** を再実行してトークンを記録する |
| 終端の順序付け `ship-verification >= 選定された各 S3 ピア`（常時有効、トークンなし） | 終端の ship レコードが、選定された各 S3 レビューピアより前ではなく後にスタンプされ、ゲートが最終のレビュー証跡を観測したこと | すべてのプリセット（常時有効。light の例外なし） | 現在性が失われた選定レビュアーを再スタンプし、その後 **ship-verification** を再実行して、判定タイムスタンプが各ピアと同時刻以降になるようにする |
| wave-orchestration 上の `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` | `degraded_sequential` ディスパッチが、真にツールが利用不可であるという正当化と対になっていること | standard/strict はエラー、light は助言 | 正当化の参照を付けて wave-orchestration エビデンスを再記録するか、実際の並行ディスパッチでウェーブを再実行する |

正当化と対になっていない単独の `degraded_sequential` は、`slipway evidence skill` のパスを含め、ガバナンス付きのウェーブ実行を同期するすべてのパスで拒否されます。advance/next だけではありません。

`context_origin:stage=<stage>=<handle>` は、ガバナンス付きチェーン全体にわたる単一の文法です。S3 の選定レビューセットには、すべてのワークフロープロファイルで spec レビュアーと independent レビュアーが含まれます。code-quality レビューはプロファイルが code-quality レビューを必要とする場合に加わり、security レビューはエンジンが導出したセキュリティコントロールがそれを選定したときに加わります。終端の `ship-verification` ゲートは選定ピアではありません。ピアが収束した後に最後に実行されます。選定されたレビューホストはすべて `context_origin:stage=review=<handle>` を発行します。R2 ラティスは各レビュー参加者を、共有の `review` ステージではなくスキル名でキー付けします。その他の参加者は、S2 ウェーブの `executor`、S1 のプラン監査の `audit_origin`（プランの `plan_origin` 作成者と対比される）、および任意の S3 レビュー指摘修正の `fix` ハンドル（レビュアーのエビデンスに記録される）です。衝突ラティスはシームごとに所有されるため、各エッジはちょうど 1 回だけチェックされます。

| シーム | 所有するもの | エッジ |
| --- | --- | --- |
| プランゲート（S1） | ローカルの `audit_origin != plan_origin` エッジ（プラン監査の作成者 vs 監査者の自己監査）のみ | 1 |
| レビュー権威 | `{executor, fix}` 間の全エッジに加え、選定されたレビュースキルのキー。S1 の `audit_origin` は S3 の現役参加者ではない | ワークフロープロファイル、選定されたセキュリティコントロール、任意の fix ハンドルにより可変 |
| シップ権威 | context-origin エッジの追加なし。終端の `ship-verification` ゲートが、終端の順序付け不変条件に加え、レビュアー独立性と assurance-complete の存在証明を所有する | 0 |

シームがフェイルクローズドになったときは、その所有ステージまたは選定レビュアーを新しいネイティブサブエージェントで再実行し、ステージに異なる `context_origin` ハンドルを再発行させます。判定をスタンプする唯一の主体はエンジンのままであり、エンジンが一致してしまったハンドルを再スタンプすることはありません。

`context_origin` ラティスは **監査/構造ティア** です。ハンドルはホストが発行する文字列であり（executor ディスパッチのハンドルと同じ構造ティア）、チェーンの各ステージを単一の作成コンテキストに統合する際のコストと監査可能性を高めますが、独立性の暗号学的証明では決してありません。偽造不可能で真に区別可能なコンテキスト識別（エンジンが発行するステージごとのナンスやライフサイクルイベント境界、いわゆる「Option B」）は、本変更の制約内では実現困難です。そのため、ここでのゲートが暗号学的な区別コンテキスト証明として過大に喧伝されることはありません。

## S3 レビューのディスパッチ

`S3_REVIEW` では、エンジンが 1 つの選定レビューセットを解決し、そのセットをコマンドサーフェスを通じて公開します。spec レビューと independent レビューはすべてのワークフロープロファイルで選定されます。code-quality レビューはプロファイルが code-quality レビューを必要とする場合にのみ加わり、security レビュアーはエンジンが導出したセキュリティコントロールが選定されたときにのみ加わります。`slipway next` は選定されたセットを公開し、ホストアダプターはそれらのレビュアーを並行ネイティブサブエージェントとしてファンアウトします。従来式の単一主要スキルは、本当にそれを必要とするサーフェス向けの互換性投影にすぎず、レビューの順序を意味しません。終端の `ship-verification` ゲートは、このピアセットが収束した後にディスパッチされ、決してそのメンバーにはなりません。

選定されたレビュアーは **順序のないピア** です。いずれも他をブロックせず、必須性、レビュー権威、シップ権威、古い証跡からの復旧は、すべて同じ選定セットを参照します。選定された各レビュアーは、それぞれ異なるハンドルで `context_origin:stage=review=<handle>` を記録します。R2 ラティスはそれらのハンドルをスキル名の参加者キーの下で比較するため、ワイヤートークンのステージラベルが共有されていても、重複したレビュアーハンドルはフェイルクローズドになります。レビュー指摘が `slipway fix` を通じて修正されると、該当レビュアーは再レビュー時に `context_origin:stage=fix=<repair-handle>` も記録します。記録された fix ハンドルは、いずれも同じ区別コンテキストラティスに参加します。選定レビュアー証跡の欠落は必須スキルブロッカーが所有します。整形式の `stage=review` ハンドルを持たない合格扱いの選定レビュー証跡は `context_origin_handle_invalid` でフェイルクローズドになり、衝突は `cross_stage_context_not_distinct` で失敗します。ディスク上の未選定のセキュリティ証跡は無言であり、隠れた参加者になることは決してありません。

選定レビュアーの現在性は、現在の diff、計画成果物、`run_summary_version` を基準とします。ピアセットが参照する共有のスイート結果キーストーンはありません。権威ある完全なスイート（および任意のガードレール SAST ベースライン）は、ピアが収束した後に終端の `ship-verification` ゲートが 1 回実行し、レビュアーと共有するレコードではなく、自身の証跡レコードに記録します。

## 読み取り専用サーフェス

これらのコマンドは、ライフサイクル状態を変更することなく状態を検査します。

- `slipway next`
- `slipway status`
- `slipway validate`

機械可読の出力には `--json` を使います。ゲートの詳細、成果物の準備状況、遷移トレース、コンテキスト予算の診断が必要な場合は、`next` または `run` に `--diagnostics` を使います。

## Open Questions のセマンティクス

`intent.md` には正規の `## Open Questions` セクションを含められます。エンジンが判定するのは **散文ではなく構造** です。未チェックのチェックリスト項目だけが intake をブロックします。

以下は解決済みとして読まれます（intake は `S0_INTAKE/confirm` へ進みます）。

```markdown
## Open Questions
(none)
```

```markdown
## Open Questions
- None requiring research — the page model is already specified.
```

```markdown
## Open Questions
- [x] Installer path resolved by research.
```

未チェックの `- [ ]` 項目だけがブロックします（`S0_INTAKE/research` へルーティングされます）。

```markdown
## Open Questions
- [ ] Which installer path should be documented?
```

自由形式の散文や裸の箇条書きは **ドキュメントであり、ブロッカーには決してなりません**。あるものが真の未解決論点かどうかの判断は `intake-clarification` スキルが所有するセマンティックな判定であり、このスキルは実際の未知事項を `- [ ]` 項目として記録します。エンジンが intent の散文を解析することはありません。これにより、未知事項のない変更（`None`、定型的な一文、または空のセクション）が黙ってリサーチへ迂回するのを防ぎつつ、成果物が `- [x]` で論点の履歴を保持できるようにします。実際に項目がブロックする場合は、`slipway run` が該当する `- [ ]` 行を名指しするため、ルーティングが無言になることはありません。

## 完了

ガバナンス付き状態が done-ready になったら、次のようにします。

```bash
slipway done --json
```

`done` はアクティブな変更を確定し、終端状態をアーカイブします。中断後にローカル状態が不整合に見える場合は、まず `slipway health --doctor` で検査し、提案された修復が問題に合致していれば `slipway repair` を実行します。
