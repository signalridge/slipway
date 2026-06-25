# コマンドリファレンス

ルーティングされるほとんどのコマンドは、構造化された出力が役立つ場面で `--json` をサポートします。例外が 2 つあります。`slipway init` はセットアップ専用（`--tools`/`--refresh`、`--json` なし）で、`slipway validate` は JSON のみを出力します（その `--format` フラグはメインレポートではなく `--list-focuses` の出力を整形します）。

生成されるホストコマンドサーフェスは、ホストプロンプトを利用するよう登録されたコマンドを対象とします。`slipway tool` のような CLI 専用のヘルパー名前空間は、ここおよびサーフェスマニフェストに登録されますが、`$slipway-tool` やホストプロンプトのラッパーは生成しません。生成されたスキルはヘルパーサブコマンドを直接呼び出します。

## コアライフサイクル

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway new [description]` | mutation | インテークから開始する統制された変更を作成する。 |
| `slipway intake` | mutation | S0 のインテーク明確化と認可を実行する。 |
| `slipway plan` | mutation | S1 の計画アーティファクト作成、または同一インテント変更の修正を実行する。 |
| `slipway implement` | mutation | S2 の実装ウェーブオーケストレーションを実行する。 |
| `slipway review` | mutation | S3 のレビュー収束とレビュアーフィードバックの修復を実行する。 |
| `slipway fix` | mutation | S3 レビュー指摘に対するフレッシュコンテキストの修正をディスパッチする。 |
| `slipway done` | mutation | done-ready な変更をファイナライズしてアーカイブする。 |
| `slipway next` | query | 状態を進めずに、次に実行可能なスキルやブロッカーを確認する。 |
| `slipway run` | mutation | スキル、ブロッカー、または done-ready の結果が現れるまで、現在のライフサイクルステージをショートカット駆動する。 |
| `slipway status` | query | ライフサイクルの状態、ブロッカー、進捗、次のアクションを表示する。 |

統制された変更にエビデンスの陳腐化があると、`slipway next` は読み取り専用のまま復旧ガイダンスを報告します。状態が分かっている場合は、明示的な現在ステージのコマンド（`intake`、`plan`、`implement`、`review`、`done`）を優先してください。`run` は現在のステージに委譲する自動駆動のショートカットで、JSON では `delegated_to` を報告します。同一インテントのスコープ変更は現在の変更内での修正であり、インテントの衝突は新しい統制された変更を開始します。計画のフレッシュネスは構造的なタスク計画ハッシュをキーとします。プラン監査は S2 が開始できる前に計画バンドルをレビューしますが、`wave-plan.yaml` を計画の権威として認証するものではありません。`wave-plan.yaml` は現在の `tasks.md` から具現化された S2 実行用の投影／キャッシュであり、その `generated_at` はフレッシュネスの権威ではなく、表示・監査用の具現化時刻です。`slipway next --json` の `input_context.wave_plan` フィールドは別個の診断専用の投影で、永続化された `wave-plan.yaml` キャッシュが定義しない表示専用フィールド（`wave_count`、`advisories`）を含むため、決してキャッシュにコピーしてはなりません。`wave-plan.yaml` はツール（`slipway repair`）が再生成するエンジン所有のキャッシュで、手編集はしません。読み取れない場合は、`tasks.md` を編集するのではなく `slipway repair` で再生成してください。

`slipway fix` は S3 のレビュー指摘修復サーフェスです。レビュアーの指摘とアライメントブロッカーを検出し、`repair_batch_id` とフレッシュコンテキスト修復サブエージェント向けのコントラクトを返します。通常の検出はライフサイクルの状態を進めません。`slipway fix --start-reexecution` は、S2 を再オープンして実装修復のための新しい実行ランの境界を具現化する、明示的なレビュー駆動モードです。ホストはまず選択されたレビューバッチの指摘を収集し、根本原因ごとに 1 つの修復ブリーフへ統合します。他の選択されたレビュアーがまだ報告中の段階で、指摘をインラインまたは 1 件ずつ修復してはなりません。サブエージェントがコード、アーティファクト、テスト、または同一インテントのスコープエビデンスを変更した後は、影響を受けた選択レビュアーを再実行し、`slipway review` がバッチをクローズする前に `context_origin:stage=review=<handle>` と `context_origin:stage=fix=<handle>` の両方を記録します。`slipway repair` は引き続きローカルの整合性のみを扱います。

## 作成オプション

```bash
slipway new "add install docs" --preset standard
slipway new "docs-only change" --profile docs
slipway new --from-doc docs/installation.md "refresh install docs"
slipway new "small fix" --trivial
slipway new "auth refactor" --discuss   # carry open questions forward into context
slipway new "schema migration" --full   # force fresh ship-verification evidence before ship
```

プリセットはゲートの厳格さを制御します。`light`、`standard`、`strict` のいずれかです。

ワークフロープロファイルはチェック内容を形作ります。`code`、`docs`、`research`、`config`、`meta` のいずれかです。

`--discuss` は未解決のグレーゾーンを実行前にコンテキストへ永続化します。`--full` はシップゲートの前にリフレッシュ済みの `ship-verification` パスを要求します。

## ディスカバリ

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway codebase-map` | mutation | `artifacts/codebase/` 配下に、リポジトリスコープのアドバイザリなコンテキストを作成または更新する。 |

`codebase-map --json` は、ドキュメントが CLI で検出されたリポジトリ事実のみを含む場合に `status: "baseline"` を報告します。ベースラインのドキュメントは出発点として有用なコンテキストであり、作り込まれたブラウンフィールド分析ではありません。呼び出し側は計画やレビューで頼る前に、ソース裏付けのある所見でこれらを洗練すべきです。

`artifacts/codebase/` 配下のコードベースマップは既定で git 管理対象です。耐久性のあるブラウンフィールドのコンテキストは、ローカル限定の状態として隠すのではなく、レビューし共有するためのものです。既存のリポジトリは、`slipway new`、`slipway codebase-map`、または `slipway init` が管理対象の `.gitignore` ブロックを書き換える次のタイミングで自動移行します（`next`/`run`/`status`/`repair` はこれを調整しません）。バンドルローカルの `events/`、`verification/`、レガシーの変更ごとの `evidence/`、および `.worktrees/` パスは引き続き無視されます。ランタイムのタスクエビデンスは `.git/slipway/runtime/changes/<slug>/evidence/` 配下にあります。

## 状況依存コマンド

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway preset <level>` | mutation | アクティブな変更のプリセットを確認または変更する。 |
| `slipway validate` | query | 状態を進めずに、エビデンスとゲートの準備状況を再計算する。 |
| `slipway abort` | mutation | 変更をアーカイブせずに、アクティブな実行セッションを中断する。 |
| `slipway cancel` | mutation | アクティブな変更をキャンセルし、終端状態をアーカイブする。 |
| `slipway delete` | mutation | 放棄された統制された変更（そのバンドル、ランタイムバインディング、任意のワークトリー、またはアーカイブ済みレコード）を破棄する（既定はドライラン）。 |
| `slipway repair` | mutation | 範囲を限定したローカルの整合性修復を実行する。 |
| `slipway evidence task` | mutation | ウェーブ実行向けに、サポートされるランタイムタスクエビデンスを記録する。 |

`slipway cancel` と `slipway delete` は同じ操作ではありません。`cancel` は**アクティブな**変更を終端状態 `cancelled` へ移し、決定が監査証跡に残るよう `artifacts/changes/archived/<slug>` 配下に**アーカイブ**します。`delete` は代わりに、放棄された・誤って作られた・一部削除された変更について、そのバンドル、ランタイムバインディング、そして（`--worktree` 付きで）バインドされた git ワークトリーといった**ローカルの統制状態を破棄**し、`--archived` 付きでは既にアーカイブ済みのレコードをパージできます。`delete` は既定でドライランです。素の `slipway delete --change <slug>` は削除計画を表示するだけで何も削除しません。実行するには `--yes` を渡します。`delete` はフェイルクローズで動作します。追跡対象の未コミット変更や、生成された Slipway パス外の未追跡ファイルを含むワークトリーは `--force` がない限り削除を拒否し、実装ブランチやプッシュ済みの PR ブランチは決して削除しません。変更が放棄された・壊れた・別のワークトリーにバインドされている場合、`slipway status`/`slipway next` と復旧出力は、正確な `slipway delete --change <slug>` コマンドへ案内します。

## ヘルパー

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway tool <helper>` | mutation | 生成されたスキルが使用するヘルパーツールを実行する。明示的なバックエンドやドメインツールが欠けている場合、ヘルパーはフェイルクローズする。 |

`slipway tool` は意図的に CLI 専用です。`$slipway-tool` も生成されたホストプロンプトのラッパーもありません。生成されたスキルは特定のヘルパーサブコマンドを直接呼び出します。

## 診断

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway health` | query | リポジトリローカルの整合性と修復可能性の所見を表示する。 |
| `slipway instructions <artifact>` | query | テンプレート、品質基準、そして変更内であれば解決済み出力パスと依存グラフを、統制されたアーティファクトまたはコードベースマップのドキュメントについて表示する。 |

`slipway instructions <artifact>` は、作成スキルが実ファイルを直接書けるよう、アーティファクトのテンプレートと品質基準を提供します。エンジンが構造を、スキルが内容を所有し、置き換えるべきシード本文はありません。統制された変更の内部では、解決済みの出力パス、依存・解放グラフ、そしてスキルが遵守すべきだがアーティファクトには決してコピーしないタグ付きの背景情報（`context`/`rules`）も返します。6 つの統制バンドルアーティファクト（`intent`、`requirements`、`decision`、`research`、`tasks`、`assurance`）と、リポジトリスコープのコードベースマップドキュメント（`stack`、`architecture`、`structure`、`conventions`、`integrations`、`testing`、`concerns`）を対象とします。`--json` では、`context_is_baseline: true` が、作成するドキュメントへ保持・拡張すべきコードベースマップのベースラインコンテキストを示します。これが存在しないか false の場合、`context` は遵守すべきだがコピーしない背景情報です。

`next --json` と `run --json` は、既定の非 `--diagnostics` ハンドオフに `input_context.codebase_map_status`（およびドキュメントごとの `input_context.codebase_map_doc_states`）を含め、参照されるマップが耐久性のあるものか呼び出し側が判断できるようにします。値は `slipway codebase-map` の評価（`missing`、`scaffold_only`、`baseline`、`partial`、`populated`）を反映します。マップが欠けている場合は、フィールドを省略するのではなく `"missing"` を報告し、各ドキュメントを `missing` とします。マップを消費する計画スキル（research-orchestration または plan-audit）が次に来て、ステータスが `scaffold_only` または `baseline` のとき、`warnings` は非ブロッキングのコードベースマップアドバイザリを伝えます。

## セットアップ

| コマンド | クラス | 目的 |
| --- | --- | --- |
| `slipway init` | mutation | `.slipway.yaml`、リポジトリローカルのランタイムレイアウト、および任意の AI ツールアダプターを初期化する。 |
| `slipway config [list|get|set]` | mutation | リポジトリレベルの `.slipway.yaml` 設定キーを確認・更新する。CLI 専用で、生成されたアダプターのプロンプトサーフェスはありません。 |

`docs/SURFACE-MANIFEST.json` は、アダプター、コマンド、スキル、JSON、ドキュメントの各行についてコミットされた生成サーフェスのインベントリです。マニフェストは Slipway 所有の Go 権威から再構築され、CI 向けの Go テストでチェックされます。

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go run ./internal/toolgen/cmd/gen-surface-manifest --write
```

コマンド、スキル、JSON 出力コントラクト、またはドキュメント向けサーフェスを追加するときは `--write` を実行し、マニフェスト行のドキュメントトークンを指定されたドキュメントファイル内に残してください。マニフェストが陳腐化したり、ドキュメントトークンが欠けたりすると `go test ./internal/toolgen` が失敗します。

## 出力およびハイドレーションのフラグ

クエリ系とレビュー系のコマンドは、リバースなフラグコントラクトテストによって CLI と整合した、一貫した出力・ハイドレーションのサーフェスを共有します。

- `--format <text|yaml|json>` — `status` は全セットをサポートします。`review`、`validate`、`repair`、`health` は `--list-focuses` 出力（`text|json`）の整形にのみ `--format` を使います。`--json` はサポートされる場面で `--format json` の短縮形です。
- `--hydrate` / `--hydrate-ref <skill-id>/<name>` — `status`、`review`、`health` は選択されたハイドレート参照本文をテキスト出力に追記します。`--hydrate-ref` はハイドレーションを指定した参照に限定します（繰り返し指定可）。
- `--focus <alias>` / `--list-focuses` — `status`、`health`、`review`、`validate`、`repair` は公開フォーカスの上書きを受け付けます。列挙するには `<command> --list-focuses` を実行します。既知のエイリアス: `status`/`health` → `incident`、`review` → `sast`、`calibration`、`validate` → `sast`、`property`、`mutation`、`spec-trace`、`repair` は現在いずれも公開していません。
- `status --root` は正規の Slipway スコープルートを表示します。`status --stats` はワークスペースの診断情報（アクティブ件数、陳腐化したサマリー、整合性の問題）を表示します。
- `next --no-auto-pass` は自動パスの代わりにスキルの適格性を報告します。`next --context-guard` はコンテキスト予算のガードメッセージをフック形式で出力します。
- `done --all-ready` は、現在 done-ready な全アクティブ変更をアーカイブします。
- 同一インテントのスコープ変更は、現在のステージコマンドによって変更修正として扱われます。所有するアーティファクトとエビデンスを更新し、そのまま前進してください。エグゼキューターエージェントは、宣言されたタスクスコープの外へ黙って書き込んではなりません。修正を提案するか、ブロッカーを返します。目的が変わった場合は、新しい統制された変更を開始してください。

## 役立つ JSON 呼び出し

```bash
slipway new --json "refresh docs"
slipway intake --json
slipway plan --json
slipway implement --json
slipway fix --json
slipway next --json --diagnostics
slipway run --json --diagnostics
slipway status --json
slipway validate --json
slipway handoff show --json
slipway config --json
slipway evidence task --result-file task-result.json [--result-file next-task-result.json ...] --json
slipway health --doctor --json
```

JSON コントラクトのカバレッジ向けの安定したマニフェストトークン:

| コントラクト | トークン |
| --- | --- |
| abort JSON | `slipway abort --json` |
| cancel JSON | `slipway cancel --json` |
| codebase-map JSON | `slipway codebase-map --json` |
| config JSON | `slipway config --json` |
| delete JSON | `slipway delete --change <slug> --json` |
| done JSON | `slipway done --json` |
| evidence skill JSON | `slipway evidence skill --skill <name> --verdict pass --json` |
| evidence task JSON | `slipway evidence task --result-file task-result.json [--result-file next-task-result.json ...] --json` |
| fix JSON | `slipway fix --json` |
| handoff JSON | `slipway handoff show --json` |
| health JSON | `slipway health --json` |
| implement JSON | `slipway implement --json` |
| instructions JSON | `slipway instructions <artifact> --json` |
| intake JSON | `slipway intake --json` |
| new JSON | `slipway new --json` |
| next JSON | `slipway next --json` |
| plan JSON | `slipway plan --json` |
| preset JSON | `slipway preset <level> --json` |
| repair JSON | `slipway repair --json` |
| review JSON | `slipway review --json` |
| run JSON | `slipway run --json` |
| status JSON | `slipway status --json` |
| validate JSON | `slipway validate --json` |

`next --json` は AI ツールへのハンドオフ向けに `next_skill.name` を含みます。ホストツールは、自身のアダプター規約からローカルの `SKILL.md` パスを導出します。

診断が有効な場合、レビュー状態のハンドオフ JSON には次も含まれることがあります。

- 概念上のステージが、実行可能な欠落スキルと異なる場合の `next_skill.display_name`、`next_skill.blocking_name`、`next_skill.resolution_reason`。
- `next_skill.review_context.required_artifact_layers` と `next_skill.review_context.required_implementation_layers`。これらは `layer:R0=pass`、`layer:R3=pass`、`layer:IR1=pass`、`layer:IR3=pass` といった正確なゲートトークンに対応します。
- トップレベルの `confirmation_requirement`。これは、ハードストップに新たなユーザー確認が必要か、事前認可で十分か、人間向けの散文としての次のオペレーターアクション（`next_action`）、機械可読な `next_action_kind`（`skill_handoff` | `review_batch` | `preset_confirmation` | `command` | `blocker_resolution` | `confirmation` | `none`）、そしてそのまま実行可能な場合に実行する正確な `next_command` を報告します。`next_action_kind`/`next_command` で分岐し、`next_action` は表示用の散文としてのみ扱ってください。
- `freshness_diagnostics`。陳腐化したソース／エビデンスの組、フィールドレベルの実行入力の不一致、パスの権威、次の再生成アクションを報告します。

`run --auto` / `run --no-auto` は、1 回の呼び出しに限り `execution.auto` を上書きします。設定レベルの `execution.auto` は `intake`、`plan`、`implement` にも適用されます。これらのステージコマンドに上書きフラグはありません。auto は純粋なペーシングの境界のみを越えます。`security-review` の境界、センシティブ／ガードレールの確認、インテークの Approved Summary、エビデンスゲートは引き続き停止点です。

`validate --json` はアクティブな準備状況の権威です。現在の統制状態が今すぐ前進できるかに答え、実行可能なレビューハンドオフを `actionable_next_skill` を通じてミラーします。これには、実行可能なスキルが供給すべき正確なレイヤー参照のための `required_tokens` が含まれます。`run --json` は変更を伴う遷移サーフェスです。`advanced` はこの呼び出しが何を変更したかを報告し、`blockers` は遷移後の現在の停止条件を報告します。したがって前進の成功に続いて、次の必須スキルに対するエラー重大度のブロッカーが報告されることがあります。`health --governance --json` は診断的なヘルスフィードバックです。コントロールやトレーサビリティの詳細を確認するために使い、`run` が今前進したかを判定するライフサイクルの権威としては使わないでください。

`status --json` は、実行エビデンスが陳腐化していると分かっている場合に `freshness_diagnostics` を含め、各 `artifact_dag` ノードに `blocking` と `blocking_reason` を付与します。これにより、ドラフトの計画アーティファクトが現在のレビューブロッカーと取り違えられないようにします。

`validate --change <slug>` は明示的なアクティブ変更を選択します。slug がアーカイブ済みの終端変更を指す場合、コマンドは `archived_change_not_validatable` で失敗し、汎用の no-active 診断の代わりに終端状態とアーカイブ済みの `change.yaml` パスを返します。これはアクティブな準備状況のコントラクトです。`validate` は `done` の前に現在アクティブな統制状態を証明するものであり、凍結されたバンドルに対するアーカイブ後の監査サーフェスではありません。

`artifacts/codebase/**` 配下の耐久性あるコードベースマップは、スコープコントラクトの変更ファイル計上から免除されます。それらのコンテキストファイルのみがダーティな場合、`scope_contract.changed_files` と `scope_contract.out_of_scope_files` に含まれず、`scope_contract.status` は `pass` のままです。リフレッシュされたコードベースマップ単独ではスコープコントラクトのドリフトを引き起こしません。このフィルタリングを `git diff` の不一致から推測させるのではなく可視にするため、免除されたファイルは `scope_contract.exempt_context_files` フィールドで明示的に開示され、`slipway validate --json`、`slipway status --json`、`slipway review --json` で表示されます。

`slipway evidence task` は、wave-orchestration の同期のために、フラットなランタイムタスク JSON を `.git/slipway/runtime/changes/<slug>/evidence/tasks/` 配下に書き込みます。既定の S2 コーディネーターパスは `--result-file <path>` で、コーディネーターが 1 つのアトミックなバッチインポートを行いたいときは繰り返します。各エグゼキューターの結果 JSON には `task_id`、`verdict`、`evidence_ref`、`changed_files`、`blockers`、および任意の `session_id` が含まれます。バッチはすべてのファイルをプリフライトし、重複する `task_id` エントリを拒否し、いずれかのメンバーが無効ならタスクエビデンスを一切書き込みません。エグゼキューターの結果ファイルには、ledger 所有のフィールド（`run_summary_version`、`task_kind`、`target_files`、`captured_at`、`freshness_inputs`、`input_hash`）を含めてはなりません。Slipway はそれらをアクティブなウェーブ計画と現在のタスクエビデンスランから導出します。手動フラグモードはホスト内部または復旧フォールバック用に引き続き利用可能です。現在のフラグコントラクトは `slipway evidence task --help` を参照してください。コマンドは `freshness_inputs` を計算し、タスク種別／verdict／ブロッカーを検証し、手書き JSON に頼る代わりに未知またはパス安全でないタスク ID を拒否します。`freshness_inputs` には現在のタスク由来の `tasks_plan_hash` が含まれるため、`tasks.md` が意味的に変わった後にタスクエビデンスを再利用できません。

`slipway evidence skill --skill wave-orchestration` は execution-summary エビデンスのための S2 ブートストラップです。`execution-summary.yaml` が存在する前に、現在のフラットなタスクエビデンス ledger からウェーブのランバージョンを導出し、すべてのタスクエビデンスが単一の有効な `run_summary_version` を使うことを要求し、その ledger から wave-orchestration のダイジェストをスタンプします。`spec-compliance-review`、`code-quality-review`、終端の `ship-verification` ゲートといった後続のラン-サマリー連動スキルは、既存の execution summary を引き続き要求し、それが無い場合は `evidence_skill_run_summary_missing` でフェイルクローズします。

受理された統制スキルのエビデンスは、さらに `verification/evidence-digests.yaml` によってバインドされます。これはエンジン所有のローカルファイルで、各パスしたスキルが認証した入力のコンテンツダイジェストを記録します。エントリには受理された検証 verdict のタイムスタンプも保存されるため、より新しいホストの再実行 verdict が、変更を伴う前進中に陳腐化したダイジェストを置き換えられます。読み取り専用コマンドは保存されたダイジェストと現在の入力を比較するだけです。変更を伴う前進パスは、パスしたエビデンスが受理されたときにファイルをスタンプします。diff クラスのレビューダイジェストは現在の作業 diff（`git diff HEAD` に加え、`artifacts/changes/**` 配下の Slipway 統制／ランタイムアーティファクトを除く、無視されないレビュー可能な未追跡ファイル）を認証します。そのため、レビューとファイナライズの間のコミットによって、所有するレビューステージが新しい diff 境界に対して `slipway run` で再実行されるまで、読み取り専用の投影がレビューを陳腐化と報告することがあります。必要なダイジェストエビデンスが欠落または陳腐化している場合、所有する統制スキルは陳腐化と報告され、再実行が必要です。

選択された S3 レビューピア（spec-compliance-review、independent-review、ワークフロープロファイルが要求する場合の code-quality-review、ポリシーで選択された場合の security-review）は、現在の diff、計画アーティファクト、ラン-サマリーバージョンに対して自身の verdict を主張します。これらは共有の suite-result キーストーンを消費しません。1 つの権威ある全スイート実行は、加えてあらゆるガードレール SAST ベースラインも、終端の `ship-verification` ゲートが所有します。ゲートはピアの収束後に一度だけそれを実行し、ピア共有のレコードからは決して実行しません。`slipway evidence suite-result` サブコマンドはありません。ship-verification は単一の終端エビデンスパスの一部として、スイートを自ら実行・記録します。

`repair --json` は `applied_repairs` と `unrepaired_drift` を分離します。applied repairs は実際に実行された、範囲を限定したローカル修正です。unrepaired drift には、Slipway が自動的に変更しなかったエビデンスやアーティファクト作業についての target、reason、`next_action` が含まれます。ランタイムタスクエビデンスが新しいという理由だけで陳腐化した ready な execution summary は、現在のウェーブ裏付けのタスクエビデンスから再構築できます。陳腐化した計画ソースのドリフトは未修復のまま残ります。アーカイブ後のクリーンアップで残った空のオーファンなアクティブバンドルディレクトリは `empty_orphan_bundle` の applied repairs として削除されます。空でないオーファンバンドルはオペレーターレビューが必要な整合性所見として残ります。`.git/slipway/runtime/handoff.md` のようなレガシーのリポジトリレベルのハンドオフファイルは、`.git/slipway/runtime/changes/<slug>/handoff.md` への手動移行のために報告されます。保持されていない空のロックアンカーは `cleaned_lock_anchor` として報告されます。`change-create.lock` と `repair.lock` は、変更ごとのロックではなくワークスペース／スコープレベルの調整ロックのままです。欠落したタスクエビデンスのブロッカーには、ランタイムタスクエビデンスパス、`record_command=slipway evidence task --result-file <path> --json`、コンパクトな結果スキーマ `task_id,verdict,evidence_ref,changed_files,blockers,session_id` が含まれます。アトミックなバッチインポートには `--result-file` を繰り返します。`health --json` の所見には `active_change_blocking` と `active_change_impact` が含まれます。アドバイザリなコードベースマップ警告は、アクティブな変更に対して非ブロッキングとしてマークされます。

`done --json` は、ソースファイルや非アクティブな統制アーティファクトが未コミットでも、done-ready でワークトリーにバインドされた変更をアーカイブし、非ブロッキングの `worktree_dirty_warning` を `worktree_dirty_files` 付きで返します。これにより、オペレーターはそれらのファイルをアーカイブされたバンドルと一緒にコミットできます。`done` はワークトリーを決して削除せず、`git worktree remove` は既にダーティなワークトリーの削除を拒否するため、このアドバイザリがハードブロックを置き換えます。アクティブな `artifacts/changes/<slug>/` バンドルは、`done` がそれを `artifacts/changes/archived/<slug>/` へ書き換えるため、アドバイザリから除外されます。兄弟バンドルやアーカイブ済みバンドルは一覧表示されます。

## 実行の再開

実行セッションが再開可能な場合:

```bash
slipway run --resume --json
```

状態が中断されている、または一貫していないように見えるときは、repair や resume の前に `health --doctor` を使ってください。

`run --resume` は `S2_IMPLEMENT` のような再開可能な実行状態にのみ適用されます。アクティブな変更が既に S3 レビューまたは done-ready の場合、JSON エラーには `current_state`、`resumable_states`、そしてオペレーターを S3 レビュー／done-ready フローへ戻す `next_action` が含まれます。
