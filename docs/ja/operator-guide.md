# オペレーターガイド

このガイドは、Slipway ワークスペースを保守する人とエージェントを対象としています。

## 状態の権威

| パス | 役割 | Git ポリシー |
| --- | --- | --- |
| `.slipway.yaml` | リポジトリローカルの Slipway 設定。 | プロジェクトの既定値が変わったときにコミットする。 |
| `artifacts/changes/<slug>/change.yaml` | アクティブな変更における現在のライフサイクルとルーティングの権威。アーカイブされたスナップショットは所有ワークスペースに残り、マシンローカルの `worktree_path` を省略し、アーカイブローカルのアーティファクトパスを使う。 | 追跡対象のプロジェクト記録。 |
| `artifacts/changes/<slug>/*.md` | 意図、調査、要件、決定、タスク、保証。 | 追跡対象のプロジェクト記録。 |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | ライフサイクルを変化させるイベントの追記専用トレース。 | 既定ではローカル限定の生証跡。 |
| `artifacts/changes/<slug>/verification/*.yaml` | スキルおよび検証の証跡。 | 既定ではローカル限定の生証跡。 |
| `.git/slipway/runtime/changes/<slug>/evidence/**` | ウェーブ実行と鮮度診断が消費するランタイムタスク証跡。 | Git 内部のローカルランタイム状態。 |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | アクティブな変更について、新しい AI セッション向けの任意の補足的な継続メモ。ライフサイクルの権威でも、ガバナンス証跡でも、鮮度入力でも、ゲートでもない。 | Git 内部のローカルランタイム状態。 |
| `.git/slipway/locks/change-create.lock`、`.git/slipway/locks/repair.lock` | 変更作成と修復のための、ワークスペース/スコープレベルの調整ロック。対応するクリティカルセクションは安定した変更単位ロックの前または外側で始まるため、これらはグローバルなまま残る。 | Git 内部のローカルランタイム状態。 |
| `artifacts/codebase/**` | `slipway codebase-map` が生成する補足的なコードベースマップ。 | 追跡対象のプロジェクト記録。既定で git 追跡される（既存リポジトリは次回の管理ブロック再書き込み時に自動移行する）。 |
| `.worktrees/<slug>` | 専用のガバナンス対象ワークトリーチェックアウト。 | 既定ではローカル限定。 |

`events/lifecycle.jsonl` を `change.yaml` の代替として扱わないこと。これは監査証跡にすぎません。
ランタイムタスク証跡をアクティブなバンドルに書き込まないこと。`slipway evidence
task` がそれを `.git/slipway/runtime/changes/<slug>/evidence/tasks/` 配下に記録します。
継続メモが役立つ場合は、新しい `status` または `next` の出力から `<slug>` を解決したうえで
`.git/slipway/runtime/changes/<slug>/handoff.md` に書き込んでください。ライフサイクル状態、スキルの選択、
鮮度を handoff のテキストから導き出さないこと。
`slipway init`、`slipway new`、`slipway codebase-map` は、Slipway のローカル状態用 `.gitignore` ブロックを冪等に保守します。

## ワークトリー

ガバナンス対象の作業は、`.worktrees/<slug>` 配下の専用ワークトリーに紐付けられる場合があります。アクティブなガバナンス対象の差分を所有するワークトリーを使ってください。

```bash
git status --short --branch
go run . status --json
```

`main...HEAD` だけから準備状態を判断しないこと。ブランチ比較は、直接のワークトリー状態と差分チェックと組み合わせてください。
`artifacts/codebase/**` 配下の永続的なコードベースマップは、
スコープ契約の変更ファイル計上から除外されます。これらのコンテキストファイルだけが変更されている場合、
それらは `scope_contract.changed_files` と
`scope_contract.out_of_scope_files` に含まれず、`scope_contract.status` は `pass` のままになります。
このフィルタリングを `git diff` の不一致から推測させるのではなく可視化するため、
除外されたファイルは `slipway validate --json`、
`slipway status --json`、`slipway review --json` が公開する
`scope_contract.exempt_context_files` フィールドで開示されます。
`slipway done` の後、Git で安全にアーカイブされた記録は所有ワークトリーに残ります。そのワークトリーを削除する前に、それらをコミットまたはマージしてください。
ワークトリーに紐付いた変更に未コミットのソースや非アクティブな
ガバナンス変更が残っている場合でも、`done --json` はアーカイブを実行し、ブロックしない
`worktree_dirty_warning` を `worktree_dirty_files` 付きで返すので、オペレーターは
それらのファイルをアーカイブ済みバンドルと一緒にコミットできます。`done` はワークトリーを削除せず、
`git worktree remove` は dirty なワークトリーの削除を拒否するため、この
アドバイザリで十分です。アクティブな `artifacts/changes/<slug>/` バンドルは、`done` がそれを
`artifacts/changes/archived/<slug>/` に書き換えるため、アドバイザリから除外されます。
dirty な兄弟バンドルやアーカイブ済みバンドルはアドバイザリに列挙されます。

## ヘルスと修復

変更する前に検査する。

```bash
slipway health --doctor --json
slipway validate --json
slipway status --json
```

修復は、doctor の出力が観測された問題と一致する場合にのみ実行する。

```bash
slipway repair --json
```

修復は、stale なロック、保持されていないロックアンカー、中断されたアーカイブ、破損した設定、
修復可能なレイアウトドリフトといった、範囲が限定されたローカル整合性の問題を対象としています。
`.git/slipway/runtime/handoff.md` のようなレガシーのリポジトリレベルのランタイム handoff ファイルを報告するので、
オペレーターは削除する前に有用なコンテキストを現在の変更単位 handoff パスへ移行できます。
JSON 出力では、`applied_repairs` が実施された修正を列挙し、`unrepaired_drift` が
オペレーターの対応をまだ必要とするドリフトを、対象、理由、次のアクションとともに列挙します。
鮮度フィールドやタイムスタンプを手で編集しないこと。代わりに、名前の付いた証跡を再生成するか、
ソースアーティファクトに同一意図の修正（amendment）を加えてください。
ランタイムタスク証跡の方が新しいという理由だけで stale になっている ready な実行サマリについては、
修復が現在のウェーブ裏付けのタスク証跡からサマリを再構築できます。
計画ソースのドリフトは修復されないまま残り、代わりに計画やレビューの
証跡更新へと差し戻されます。

ヘルスの所見にはアクティブな変更への影響が含まれます。コードベースマップの警告は
既定では補足的なものであり、現在のゲートに対しては非ブロッキングとして扱い、
マップの再構築が必要なときは更新パスまたはコマンドを示すべきです。

## 診断 JSON

実行鮮度の診断は、ハッシュベースではなく構造ベースです。現在の
実行サマリは、`change_id`、`run_summary_version`、`task_id`、`guardrail_domain` といった
タスク鮮度入力を記録します。古いハッシュのみのサマリは stale として扱われ、再生成が必要です。

`next --json --diagnostics`、`run --json --diagnostics`、`validate --json`、
`status --json` は、stale なソース/証跡のペア、最初の stale 原因、下流の証跡チェーン、
期待値/現在値のタスク入力値、権威あるバンドルおよびランタイムのパス、安全な次のアクションとともに
鮮度の失敗を公開します。
タスク証跡欠落のブロッカーには、ランタイムタスク証跡ディレクトリ、
`record_command=slipway evidence task --result-file <path> --json`、そしてコンパクトな
エグゼキューター結果スキーマが含まれます。
`task_id,verdict,evidence_ref,changed_files,blockers,session_id`
`--result-file` を繰り返すと複数のタスク結果ファイルをアトミックにインポートできます。Slipway は
バッチ全体をプリフライトし、いずれかのファイルが無効、または別のタスク ID と重複する場合は、
タスク証跡を一切書き込みません。
このディレクトリはアクティブな変更については `.git/slipway/runtime/changes/<slug>/evidence/tasks/` です。
バンドルローカルの `events/` と `verification/` は `artifacts/changes/<slug>/` 配下のままです。

理由コードの `code` 値は、ブロッカー、回復ルーティング、JSON コンシューマー、生成スキルのための
安定したマシン契約です。正規の列挙は
`internal/model/reason_code.go` の
`canonicalReasonDefinitions` のキー集合であり、`internal/model/reason_code_contract_test.go` が
スナップショットテストでその集合と各コードの重大度を凍結します。`message` は
表示用の散文として扱うこと。理由/エラーペイロードのテストとスキルロジックは、メッセージテキストの一致ではなく、
`code`、`detail`、`error_code`、`category`、`exit_code` といった
安定したフィールドや構造化された詳細をアサートしなければなりません。リポジトリローカルの AST リントは、
構文的に認識可能な理由/エラーペイロードの面（`ReasonCode`、
`CLIError`、`HealthFinding`、既知のコンストラクター/ヘルパー、ブロッカー/理由
コレクション）についてそのルールを強制します。`Message` という名前の他のフィールドは、
その理由/エラーペイロードの面の一部になるまではレビュー管理のままです。生成元が
認識できないトークンを出力した場合、正規化は `unknown_reason_code` へとフェイルクローズし、
元のトークンを `detail` に保持するので、生成元を修正して正規の列挙に追加できます。CLI エラーから
理由ペイロードへのブリッジは、正規の理由コードのみを直接保持しなければなりません。理由ドメイン外の
`error_code` 値は、単独の理由コードとして正規化するのではなく、正規のラッパー理由の detail に
載せて運ぶ必要があります。

レビューの handoff は厳密なレイヤートークンを使います。仕様準拠の証跡は
`layer:R0=pass` を、ガードレールドメインが要求する場合は `layer:R3=pass` を記録します。
コード品質の証跡は `layer:IR1=pass` を、要求される場合は
`layer:IR3=pass` を記録します。`layer:CORRECTNESS=pass`、
`layer:SAFETY=pass`、`layer:QUALITY=pass` のようなトークンは、ゲートを満たす
代替にはなりません。

ステータスのアーティファクト DAG エントリには `blocking` と `blocking_reason` が含まれます。ドラフトの
計画アーティファクトは、ライフサイクルが計画ゲートを過ぎた後では情報提供的なものになり得ます。このフラグは
現在のゲートシグナルとして扱ってください。

## 検証スタック

実装中はピンポイントなチェックを使う。

```bash
go test ./internal/stringutil ./internal/engine/progression ./internal/engine/governance -run 'TestHasBlockingOpenQuestions|TestFirstBlockingOpenQuestion|TestAdvanceIntake_OpenQuestionsUseChecklistStructure|TestOpenQuestionsRoutingNoteNamesEntryAndEscapeHatch|TestTraceability.*OpenQuestions|TestGovernanceReadinessUsesTraceabilitySnapshot' -count=1
```

クローズアウト前には完全な証明を使う。

```bash
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
(cd website && npm run build)
```

ドキュメントビルド（Astro Starlight）は、Node の依存関係がローカルで利用できるときにのみ実行してください。
まず `cd website && npm install` を実行します。CI は検証として同じドキュメントビルドを実行します。

## アダプターの更新

テンプレートやコマンド契約を変更した後は、生成された AI ツールの面を更新する。

```bash
slipway init --tools all --refresh
```

コミットする前に、生成されたパスの変更を確認してください。Codex のコマンドの面は
`.codex/skills/slipway-<command>/SKILL.md` 配下にあります。Codex の更新は
ホストグローバルの `$CODEX_HOME/prompts` ファイルにはもう触れません。

## クローズアウト

`done` の前に行うこと。

1. `go run . validate --json` が、対象となるアクティブな変更のゲートを承認済みと
   報告することを確認する。これはアーカイブ前の鮮度/準備状態ゲートであり、
   `done` の後に同じアーカイブ済みバンドルを再検証できるという約束ではない。
2. 現在のラン version に対してタスク証跡が fresh であることを確認する。
3. `git diff --check` を確認する。
4. 意図したファイルのみをステージする。
5. `git diff --cached --check` を確認する。
6. 変更が done-ready になったら `slipway done --json` を実行する。アクティブな変更
   バンドルに `done` 前のコミットは不要。
7. `worktree_dirty_warning` が返された場合、変更はすでにアーカイブ済み。ワークトリーを削除する前に、
   列挙された `worktree_dirty_files` をアーカイブ済みバンドルと一緒にコミットする。

`done` の後は、アーカイブされた `change.yaml` とバンドルの内容を凍結された
プロジェクト記録として使ってください。`validate --change <slug>` は、アーカイブ済みのスラグを
`archived_change_not_validatable` で意図的に拒否します。読み取り専用のアーカイブ監査は
別のコマンドの面となります。
