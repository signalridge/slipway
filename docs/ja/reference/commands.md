# コマンドリファレンス

Slipway には7つの user コマンド と、generated アダプターが呼び出す `protocol` 操作があります。使用中の binary で `slipway <command> --help` を確認してください。Package channel には古い コマンド generation が含まれる場合があります。

| Command | 目的 |
| --- | --- |
| `install` | 選択した AIコーディングツール 向けに capability を生成する。 |
| `uninstall` | Pristine な Slipway-managed ホスト file を削除する。 |
| `list` | Adapter detection と install 状態を表示する。 |
| `doctor` | Repository、アダプター、GitHub tooling、Run storage の状況を診断する。 |
| `run` | Ad-hoc または issue-backed Run を開始し、最初の Action を返す。 |
| `status` | Run を一覧、または1件を inspect する。 |
| `stop` | Recovery data を削除せずに Run を停止する。 |

全 コマンドは `--help` を受け付けます。JSON 生成 コマンドは `contract_version` を含み、machine consumer は documented version を検証し、human prose を parse してはなりません。

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--surface ide|cli] [--refresh] [--json]
```

`--tool` 省略時は detected ホスト を選びます。複数 ホストは `--tool` を繰り返すか、`--tool claude,codex` のようにカンマ区切りの値を1つ渡します。どちらの書き方も同じです。Kiro の初回 install では `--surface` が1つ必須です。Mixed selection では `--surface` は Kiro のみに適用され、Kiro が選択されていない場合だけ無効です。`--tool all --surface ide` と `--tool all --surface cli` は有効です。

新規 install は作成した file だけを claim します。`--refresh` は一致する Slipway-owned file を更新し、欠落した pristine file を再作成します。Modified や unknown content は上書きされず、報告されます。

JSON は selected host、transaction outcome、written/removed path、preserved content、recovery artifact、warning を報告します。Non-committed transaction は計画した write/removal を完了とは報告しません。

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

Hash が一致する managed file だけを削除します。Modified file と ホスト settings は残ります。Run ジャーナル は削除されません。
`--tool` 省略時は ownership manifest を持つすべての ホスト を選び、1つも install されていなければ失敗します。`--tool` を繰り返すと removal を指定 ホストに限定できます。

## `slipway list`

```text
slipway list [--root PATH] [--json]
```

10個の アダプター target の detection、installation、refresh、capability 情報を一覧します。Malformed または unsupported ownership manifest は該当 ホストの read-only 結果だけを degrade し、file を変更せず、他 ホスト も非表示にしません。

## `slipway doctor`

```text
slipway doctor [--root PATH] [--json]
```

Repository discovery、ホスト アダプター、generated file、Run-storage durability、GitHub CLI/auth/リポジトリ permission、retired-state residue を検査します。GitHub や residue の advisory finding は Run を変更しません。認証 response と token は report に書き込まれません。

`doctor` は観察結果を説明するだけで、project test を実行したり、code が ready か判定したりしません。

## `slipway run`

```text
slipway run [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json]
  (--goal-file FILE | --goal-stdin | -- <goal>)
```

Run を作成し、最初の `orient` Action を返します。Action budget はデフォルト 8、範囲は 1–1000 です。`--no-review` は advisory Review を無効にします。それ以外でも、Slipway が Action 後に code change を観測した場合だけ Review を issue します。

`--source-file` 省略時は ad-hoc Run です。指定時、CLI は1つの bounded GitHub Change ソースエンベロープ を開いて検証し、accepted section を pin して file を閉じます。CLI 自体は GitHub を fetch せず、ホスト publication warning を表示しません。これらは generated ホスト instruction が行います。

Goal input はちょうど1つ必要です。Human caller は1つの positional goal、`--goal-file`、`--goal-stdin` のいずれかを使い、これらは相互排他的です。Generated アダプターは private temporary regular file を使い、exact goal を process list に出さず、platform command-line length の制限も避けます。Canonical machine invocation は次のとおりです。

```bash
slipway run --budget 8 --json --root /absolute/リポジトリ \
  --goal-file /private/temp/goal.txt
slipway run --budget 8 --json --root /absolute/リポジトリ \
  --goal-file /private/temp/goal.txt \
  --source-file /private/temp/change-envelope.json
```

CLI が消費した後、ホストは temporary goal/source file を削除します。直接の `-- <goal>` は便利な human form として残ります。

この コマンドは Action を返すだけで、code 変更を実行しません。

## `slipway status`

```text
slipway status [run-id] [--root ROOT] [--json]
```

ID 省略時は Git common directory 内の Run を一覧します。Current worktree の Run は replay され、別 linked worktree の Run は `workspace_foreign` マーク付き read-only header として表示されます。完全な inspect と mutation は owning worktree が必要です。

`status` は filesystem に対して read-only です。Run namespace や lock file の作成、permission の変更、中断した ジャーナル tail の修復は行いません。Run ID を指定した場合、存在しなければ `run_not_found`、local Run が壊れていれば `run_journal_invalid`、writer が bounded inspection timeout の間 commit boundary を保持すれば `run_busy` を返します。Repository-wide JSON は読めない local identity を `unavailable_runs` に残し、各 entry の `code` は `run_journal_invalid`、`run_unavailable`、`run_busy` のいずれかです。`run_not_found` は targeted error 専用で、`unavailable_runs[].code` には現れません。

ID 指定時は現在の Run projection と fresh 派生の structured `next` を返します。空リストは有効な出力です。

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

Run を停止し、ジャーナル を保存します。Stop は現在の Action を取り下げるため、stopped Run は `current_action` も destructive authorization も報告しません。ジャーナル には発行した Action がすべて残ります。Resume は常に新しい Orient を発行します。ID 省略時は list の active/paused entry を数え、1つだけの場合に進みます。読めない local recovery directory が1つでもあれば、無視せず explicit ID を要求します。Active/paused `workspace_foreign` stub は暗黙に選択しません。Stopped Run は resume できます。Ended Run はできません。

## Machine protocol 操作

Generated アダプターは `protocol` 操作で Outcome 提出、Action の answer/skip、Run resume、pinned material 読み取りを行います。これらは実装詳細ではなく公開された contract であるため top-level help に表示されます。contract を隠すことは、それを偽って伝えることになります。

ただし第2の user ワークフローではありません。各操作は既存の Run だけを対象とし、該当する場合は CLI の structured `next` が示す Action、candidate、またはその他の typed identity を使います。Prose からコマンドを組み立てず、その variant を使ってください。`run` と `status` がそれらの variant を生成する入口です。詳細は[マシンプロトコル](machine-protocol.md)を参照してください。
