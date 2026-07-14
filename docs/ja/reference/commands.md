# コマンドリファレンス

公開コマンドは七つだけです。

> このページは non-normative です。[中国語製品契約](../../zh/reference/product-contract.md)と [machine schema](../../reference/machine-protocol.schema.json) が authority です。Workflow は issue-first で issue-gated ではありません。[Issue workflow](issue-workflow.md)を参照してください。

- `install`: 六つのホスト capability を導入。
  初回は `--refresh` 不要です。current ownership manifest が既にある場合、通常の `install` は管理ファイルを変更せず、修復・更新には `--refresh` を明示します。marker だけで current manifest がない場合は no-op safety warning を返し、non-current manifest は mutation 前に失敗します。
- `uninstall`: 未変更の管理ファイルだけを削除。
- `list`: ホストの検出・導入・`needs_refresh`・capability 状態を表示。non-current manifest は fail closed です。`needs_refresh` は drift を示すだけで user edit の上書きを許可しません。Missing pristine content は refresh で再作成できますが、modified file/sentinel はユーザーが明示的に扱うまで保持されます。
- `doctor`: アダプターと実行環境を診断し、コード変更は検査しません。non-current manifest は `adapter_manifest_unreadable`、不完全な current surface は `needs_refresh`/warning になります。
- `run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json`: run を開始。Action budget のデフォルトは `8` で、明示値は `1..1000` です。source 付きでは strict Source Bundle v2 envelope を一度だけ読み、manifest が参照する chapter を local content-addressed material として固定し、journal には catalog/provenance だけを永続化します。
- `status [run-id] [--root ROOT] [--json]`: Run を一覧・表示。ID なしでは current repository の全 Run を列挙します。Current canonical workspace の Run は full replay し、別 linked worktree の Run は `FirstEvent` header stub としてだけ表示し、JSON は `workspace_foreign:true`、human output は `foreign=true` と owning workspace を示します。Foreign stub は discovery 専用で、Load・resume・mutation は引き続き元の workspace からだけ実行できます。
- `stop [run-id] [--root ROOT] [--json]`: journal を残して停止。

```bash
slipway run "small private fix" --json
slipway run "bounded Change を実装" --source-file C:\safe\temp\change-envelope.json --json
```

Source import 前に accepted Requirements、goal、later answers、command summaries が機微であり得ると警告します。Raw body/comments/token/env は journal input ではありません。[Privacy](../explanation/runs-and-privacy.md)を参照してください。

ホストは非表示のプロトコルコマンドを使います。

```text
slipway _machine submit --run ID --action ID (--outcome-file FILE | --outcome-stdin)
slipway _machine answer --run ID --action ID --text TEXT
slipway _machine answer --run ID --action ID --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway _machine skip --run ID --action ID
slipway _machine resume ID [--budget N]
slipway _machine resume ID (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway _machine material --run ID --action ID --section KEY
```

最初の resume 形式は ad-hoc Run 専用です。Issue-bound resume は fresh `--source-file`、明示的な `--use-pinned-source`、または current candidate ID に対する `pinned|adopt` のどれか一つを必須とします。Skip に理由は不要です。pause・error・status JSON は元の absolute `--root` を持つ構造化 `next` を含み、未解決の必須 input がない argv だけを human command として render します。

`install/uninstall --json` は常に `{contract_version,hosts,transaction_outcome,written,removed,preserved,recovery_artifacts,warnings}` を返し、空の配列も省略しません。`transaction_outcome` は `committed|rolled_back|not_committed|ambiguous`、`preserved` は通常の user-modified ownership content だけです。Transaction/quarantine recovery path は `recovery_artifacts` に分離し、committed でない結果は planned `written`/`removed` を主張しません。`list --json` は `{contract_version,hosts:[...]}`、ID なしの `status --json` は `{contract_version,runs:[...]}` で、空リストも versioned object です。ID あり status は flat Run projection のまま、必須の top-level `contract_version` と fresh `next` を持ちます。すべての error に `contract_version` が必須です。

`doctor --json` は `{contract_version,checks:[{code,status,host_id,name,detail}]}` を返し、status は `ok|warning|error` のみです。Repository/adapter の stable code は `repository_ok`、`adapter_manifest_unreadable`、`adapter_not_detected`、`adapter_not_installed`、`adapter_refresh_required`、`adapter_modified`、`adapter_healthy` です。GitHub の stable code は `github_cli_unavailable|github_cli_version_unknown|github_cli_rest_fallback_required|github_cli_compatible`、`github_auth_unavailable|github_auth_available`、`github_issue_permissions_ok|github_issue_permissions_limited|github_issue_permissions_unknown` で、`gh <2.94.0` では公式 REST fallback が必要です。Legacy code は `legacy_runtime_residue`、`legacy_cache_residue`、`legacy_scope_root_residue`、`legacy_scopes_residue`、`legacy_locks_residue`、`legacy_processes_residue`、`legacy_repair_backups_residue`、`legacy_unknown_residue` です。Doctor は top-level metadata だけを調べ、内容を読まず migration/delete もしません。GitHub/legacy warning は ad-hoc Run を block せず、それだけなら doctor は成功終了します。
