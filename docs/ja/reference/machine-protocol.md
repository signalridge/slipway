# マシンプロトコル

現在の `contract_version` は **2** です。規範 JSON Schema は [`machine-protocol.schema.json`](../../reference/machine-protocol.schema.json) です。未知 version・field、重複 key、不正 UTF-8、BOM、末尾データ、1 MiB を超える Outcome は拒否されます。

開始・resume コマンドは固定です。

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json
slipway _machine resume RUN [--budget N]
slipway _machine resume RUN (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
```

新規 Run の `--source-file` は任意です。指定時、CLI は最大 16 MiB の regular/no-follow file を一度だけ開き、Source Bundle v2 を検証します。Issue body の `change/v2` marker に続く唯一の manifest が、5–64 個の chapter key/role を GitHub comment node ID と body digest に明示的に束縛します。Valid manifest の envelope は参照 comment だけを含み、CLI は通常の discussion comment を走査しません。Refresh head に parse 可能な v2 manifest がなければ、宿主は初期化済みの空 `comments` array を使い、無関係な discussion を収集せず core に invalid candidate を分類させます。Raw observation は最大100 labels、pinned source は過去の transfer URL alias を最大64件保持し、超過時は新しい Run 用の structured `start-with-source` recovery を返します。1 chapter は 256 KiB、全 payload は 4 MiB までで、missing・extra・duplicate・minimized・edited・hash mismatch は fail closed です。Manifest revision は comment node/database ID の binding を commit し、requirements revision は provenance を除外します。Accepted payload は journal が参照する前に `0600` content-addressed blob として fsync され、journal/status/Action は catalog、provenance、domain-separated revision のみを保持し、Markdown、raw body、file path は保持しません。Replay はすべての pinned manifest head から accepted-comment identity ledger を導出します。Retire しても node/database identity は忘れられず、再参照時は最初に accepted された section と一致しなければなりません。

最初の resume 形式は ad-hoc 専用で source flag をすべて拒否します。Issue-bound Run は candidate がなければ fresh `--source-file` または明示的 `--use-pinned-source` の一つを必須とし、candidate があれば完全一致する `--source-choice pinned|adopt --candidate ID` だけを受け付けます。invalid candidate は pinned のみです。別 Issue は mutation 前に拒否し、repository/number/URL transfer は旧 URL alias を記録した上で amendment 比較を続行します。

Manifest revision が同じ refresh（同一・projection-only・その他 non-material drift）は旧 Action/queue/authorization を原子的に void して fresh Orient を発行します。Content-identical replacement を含む新しい manifest revision、または invalid body は path-free candidate を保存し、`decision_required` で pause し、その呼出しの budget を適用せず `budget_applied:false` を返します。`pinned` は accepted manifest・Requirements・section content を保持しつつ、candidate の同一 Issue repository/number/canonical-URL/alias/parent projection を適用し、`adopt` だけが valid candidate snapshot を導入します。`adopt` で `requirements_revision` が変わる場合だけ旧 revision 由来 answer を active context から外して履歴に残し、manifest-only replacement では active のままです。同じ `(candidate_id,choice)` retry は event も Action も増やしません。明示 budget は 1..1000、未指定なら正の残量を維持し、枯渇時は `max(initial,3)` にしてから Orient が一つ消費します。状態応答は安全な `pinned_source`、`source_candidate`、`resume_operation`、`budget_applied` を公開し、source-file path は公開しません。

error、pause、stop、ended、full status は shell string ではなく構造化 `next={operation,workspace_identity,variants}` を返します。`next.workspace_identity` は path ではなく安定した小文字 `sha256:<64 hex>` ID です。各 variant は `id`、固定の完全な `base_argv`、non-null `inputs` を持ち、一つだけの `--root` の直後に Run の元の canonical absolute worktree root を保持します。input type は `string|path|enum|digest` のみで、schema 順に `flag` と shell 解釈しない生の値を別 argv element として追加します。必須 input が未解決なら型説明だけを表示し、入力なし/解決済み argv だけを POSIX/cmd.exe/PowerShell の端で render します。rendered text は journal に入りません。

Run 初期化時には version 1 workspace identity（canonical absolute worktree root、その worktree 固有の Git directory、Git common directory、三つの path を length-framed SHA-256 にした ID）を保存します。Linked worktree は Git directory が異なるため別 identity です。Load/status recovery と submit/answer/skip/stop/resume mutation の前に shell を使わず三つを再発見して全 field を比較します。root の再利用、別 linked worktree、Git metadata の移動・redirect は journal write 前に `workspace_identity_mismatch`、`next.operation:none`、空 variants で失敗します。

Version 1 Git observation は HEAD、`git ls-files --stage -z` の正確な bytes の `index_fingerprint`、`git status --porcelain=v2 -z --untracked-files=all` の正確な bytes の `status_fingerprint`、dirty-path 全体の `path_count` と `path_fingerprint`、bounded で sorted/non-null な `dirty_files`/`path_observations` prefix、明示的な `details_truncated`、保持 field と全体 fingerprint を覆う snapshot hash を持ちます。ordinary、rename/copy（origin を含む）、unmerged、untracked path の space/Unicode を保持します。保持された各 path は category/state、既知 size、readable content fingerprint を記録します。16 MiB 以下の regular file は raw content を保持せず full streamed SHA-256 を計算します。より大きい file は `oversize` と明示し、size と固定した先頭・中央・末尾 sample の bounded/domain-separated fingerprint を計算します。size または sampled region の変更は検出しますが、unsampled region 内だけの同一 size edit は見逃す可能性があります。Symlink は追跡せず link target だけを hash し、missing/non-regular/unreadable は明示され全体を失敗させません。詳細 prefix が bounded projection を超える場合も、省略数と全体 `path_fingerprint` は残ります。`initial_git` は replay 中も不変です。

Active Action は `submit-outcome-file|submit-outcome-stdin|skip-action`、decision は `answer-decision|skip-action`、destructive は current digest 固定の `confirm-destructive`、text 必須の `decline-or-feedback`、`skip-action` を返し、environment pause も `skip-action` を返します。resume は `resume-ad-hoc`、`refresh-source|use-pinned-source`、または candidate の `keep-pinned|adopt`（invalid は keep のみ）です。ended は `operation:none` と空配列です。Outcome submit は `--outcome-file FILE|--outcome-stdin` の一方を明示し、idempotency は再 marshal した意味比較ではなく original payload bytes の SHA-256 を使います。answer は action/text/confirm/scope の canonical digest を使います。

## Journal commit error

`.git/slipway/runs/<run-id>/journal.jsonl` だけが復旧 authority で、`run.json` は置換可能な projection、`run.lock` は Run authority ではなく coordination artifact です。immutable initialization inspection は lock を作成しないため、初期 event が欠落・破損している directory や foreign workspace の directory は変更されません。初期 event によって有効な local Run と確認された後は、locked replay または mutation が欠落 lock を再作成してから続行できます。storage mutation の machine error は安定した `details.phase`、`details.committed`、`details.projection_stale`、`details.namespace_detached`、`details.ambiguous` を持ちます。`mutation_committed_projection_stale` は journal event の fsync 後に projection が失敗した状態、`mutation_outcome_ambiguous` は inode write 後に durability または namespace membership を証明できない状態です。どちらも `next.operation:"none"` を返すため、復旧前に journal を inspect/replay し、blind retry は絶対に行いません。write 前の失敗は `mutation_not_committed` です。

対応する Unix 系 system は `file_and_directory_fsync` を提供します。Windows は `file_fsync_only`、`directory_sync:false`、limitation `directory_fsync_unsupported` を安定して報告します。file content は fsync されますが、新規作成・rename した directory entry の crash durability は保証されません。

Active 応答は `contract_version`、`run_id`、`action_id`、`kind`、`goal`、`brief`、`context`、`remaining_budget` を持つ Action です。kind は `orient|clarify|implement|review|summarize` のみです。Ad-hoc Action は `source` と `requirements` の両方を省略します。Issue-bound Action は source/manifest/requirements revision、ordered section catalog、current root/run/action に束縛した structured `_machine material` reader を持ち、Markdown を複製しません。context は 128 KiB、brief は 8 KiB、Action 全体は 256 KiB で、stdout と同じ non-HTML-escaping encoder で測定します。

Requirements payload は別の非切詰め local material で、context に複製しません。Reader は digest、byte count、section revision を検証し、current non-void Action だけが読めます。Completed、replaced、stopped、その他 stale Action は拒否されます。Context は active confirmed decision と non-void Outcome projection だけで、優先順は最新 active decision、他の active decisions（新→旧）、最新 Outcome summary とその known issues、残り Outcomes（新→旧）です。新しい decision は `pause.supersedes_answer_action_id` を明示し、user が回答した場合だけ指定した旧 answer を inactive にします。skip では inactive にせず、他の supersession を prose から推測しません。旧 requirements revision の decision は履歴に残して除外し、destructive confirmation attestation は product decision ではなく、別の decline-or-feedback branch の非空 text だけが active feedback になり得ます。Candidate は CRLF/CR を LF に正規化して UTF-8 を検証し、class 内で時系列に描画します。収まらない item は code-point 境界で切り、`...[truncated original_bytes=N sha256=HEX]` marker を付け、省略は `[omitted CLASS: N]` で記録します。同じ journal replay は byte-identical で 128 KiB を超えません。

構造化確認済みの Implement だけが `destructive_authorization` を持てます。`--confirm-destructive` は trusted host による current user confirmation の attestation であり、偽造不能な human-presence proof ではありません。shell 権限を持つ悪意ある process は flag を偽装できます。target は非空・重複なしで `(kind,value)` の byte 順に整列し、kind は `path|git_ref|external_resource|data_domain` のみです。CLI は canonical scope の SHA-256 を再計算します。`yes` を含む自然言語は破壊権限を与えず、拒否/feedback を記録して request/grant を消去し、権限なしの fresh Orient を発行します。`--confirm-destructive --scope-sha256 DIGEST` が current request と完全一致した場合だけ scope を field-by-field copy した fresh Implement を一つ発行し、target/impact の拡大には新しい request が必要です。

Host Outcome は次の全 field を必ず明示します。

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

`action_kind` は必須で、current Action の `kind` と完全一致しなければなりません。欠落、未知値、不一致は拒否され、推論や legacy fallback はありません。

配列は省略も `null` も不可です。Host status は `completed|needs_input|partial|error` のみで、`skipped` は CLI の `_machine skip` event です。`needs_input` には pause が必須で、他 status は `pause:null` です。Host pause reason は `decision_required|destructive_confirmation_required|environment_unavailable` のみで、`budget_exhausted` は CLI 専用です。破壊 request は Implement の破壊 pause にだけ許可されます。

Orient は `completed|partial|error|needs_input`、Clarify は `completed|error|needs_input` を許可し、Clarify の `partial` は不正です。非 pause の Orient/Clarify は `clarify|implement|summarize` を最大一つ提案でき、提案なしなら Summary へ進みます。`needs_input`、Implement、Review、Summary の提案配列は常に空です。

Implement の `implementation` は `result`、`files_changed`、`activities`、`uncertainties`、正の `attempts` を必須とします。`completed` は `applied|not_needed`、`partial` は `partial`、`error` は `unable`、`needs_input` は `implementation:null` です。activity kind は `test|typecheck|build|lint` のみです。activity がゼロなら最終報告に次をそのまま出します。

```text
No test, typecheck, build, or lint activity was reported.
```

Review の `review` は `result`、`findings`、`uncertainties` を必須とします。`completed` は `no_findings_reported|findings_reported`、`partial` は `inconclusive`、`error` は `error` です。Review は `needs_input` を使わず、修正を提案・自動 dispatch しません。User が Review を skip した場合、action history の `outcome` は空のまま、CLI が `review_projection.result:"not_run"` を記録します。Host Outcome の `not_run` は拒否されます。

Routing は決定的です。Orient/Clarify/Implement の完了時、CLI は Git を再観測します。前回 snapshot から新しい revision があり Review が有効なら、host suggestion より先に Review を発行し、その pending suggestion を破棄して通常どおり Summary へ進みます。Review override がなければ有効な suggestion を処理し、それもなければ Summary です。このため停止・再開後の後続 revision は新しい Review を受け、snapshot が変わらなければ loop しません。Review skip は Summary、Summary skip は最小の事実 summary を書いて終了します。skip は destructive state を消去し、activity exit code と Review finding は routing 条件ではありません。

Start-to-current difference を観測するたび、事実 `observed_since_start` と `attribution_uncertainty` を記録します。並行 user edit、別 Run、tool が寄与した可能性があり、CLI は host や Run に差分を帰属させません。二方向とも中立な report discrepancy です：`applied|partial` report だが diff なし、`not_needed|unable` report だが diff あり。Routing は diff-first のままで、Review brief と final Summary は attribution uncertainty と Run 開始時から dirty だった path の structured observation を保持します。

## Public JSON envelope と Doctor advisory

すべての JSON success/error は top-level `contract_version:2` object です。Install/uninstall は常に `{contract_version,hosts,written,removed,preserved,warnings}` で配列を省略せず、list は `{contract_version,hosts:[...]}`、ID なし status は `{contract_version,runs:[...]}` で、空でも `{"contract_version":2,"runs":[]}` です。Single Run status は flat Run projection のまま top-level `contract_version` と fresh `next` を必須とします。Doctor は `{contract_version,checks:[...]}` で、各 check は `{code,status,host_id,name,detail}` を持ち、`runstore_durability_full|runstore_durability_limited` だけはさらに `durability:{level,file_sync,directory_sync,limitation?}` を持ちます。normative schema の全 object は `additionalProperties:false` です。Repository/adapter code は `repository_ok`、`adapter_manifest_unreadable`、`adapter_not_detected`、`adapter_not_installed`、`adapter_refresh_required`、`adapter_modified`、`adapter_healthy` です。

GitHub code は `github_cli_unavailable|github_cli_version_unknown|github_cli_rest_fallback_required|github_cli_compatible`、`github_auth_unavailable|github_auth_available`、`github_issue_permissions_ok|github_issue_permissions_limited|github_issue_permissions_unknown` です。command は timeout 付き・shell なしで、`gh <2.94.0` は公式 REST fallback が必要です。raw auth/API output や token は report しません。

Legacy code は `legacy_runtime_residue`、`legacy_cache_residue`、`legacy_scope_root_residue`、`legacy_scopes_residue`、`legacy_locks_residue`、`legacy_processes_residue`、`legacy_repair_backups_residue`、`legacy_unknown_residue` です。Doctor は runstore を開かず Git common dir の top-level name を Lstat し、current `runs` を除外し、residue を読取・migration・削除しません。old binary を止め backup 後に必要なら手動 cleanup します。warning は doctor を block せず ad-hoc Run health に影響しません。
