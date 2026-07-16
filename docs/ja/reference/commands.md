# コマンドリファレンス

Slipway には7つの public command があります。使用中の binary で `slipway <command> --help` を確認してください。Package channel には古い command generation が含まれる場合があります。

| Command | 目的 |
| --- | --- |
| `install` | 選択した AI coding host 向けに capability を生成する。 |
| `uninstall` | Pristine な Slipway-managed host file を削除する。 |
| `list` | Adapter detection と install 状態を表示する。 |
| `doctor` | Repository、adapter、GitHub tooling、Run storage の状況を診断する。 |
| `run` | Ad-hoc または issue-backed Run を開始し、最初の Action を返す。 |
| `status` | Run を一覧、または1件を inspect する。 |
| `stop` | Recovery data を削除せずに Run を停止する。 |

全 command は `--help` を受け付けます。JSON 生成 command は `contract_version` を含み、machine consumer は documented version を検証し、human prose を parse してはなりません。

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--surface ide|cli] [--refresh] [--json]
```

`--tool` 省略時は detected host を選びます。複数 host は `--tool` を繰り返します。Kiro の初回 install では `--surface` が1つ必須です。Mixed selection では `--surface` は Kiro のみに適用され、Kiro が選択されていない場合だけ無効です。`--tool all --surface ide` と `--tool all --surface cli` は有効です。

新規 install は作成した file だけを claim します。`--refresh` は一致する Slipway-owned file を更新し、欠落した pristine file を再作成します。Modified や unknown content は上書きされず、報告されます。

JSON は selected host、transaction outcome、written/removed path、preserved content、recovery artifact、warning を報告します。Non-committed transaction は計画した write/removal を完了とは報告しません。

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

Hash が一致する managed file だけを削除します。Modified file と host settings は残ります。Run journal は削除されません。
`--tool` 省略時は ownership manifest を持つすべての host を選び、1つも install されていなければ失敗します。`--tool` を繰り返すと removal を指定 host に限定できます。

## `slipway list`

```text
slipway list [--root PATH] [--json]
```

10個の adapter target の detection、installation、refresh、capability 情報を一覧します。Malformed または unsupported ownership manifest は該当 host の read-only 結果だけを degrade し、file を変更せず、他 host も非表示にしません。

## `slipway doctor`

```text
slipway doctor [--root PATH] [--json]
```

Repository discovery、host adapter、generated file、Run-storage durability、GitHub CLI/auth/repository permission、retired-state residue を検査します。GitHub や residue の advisory finding は Run を変更しません。認証 response と token は report に書き込まれません。

`doctor` は観察結果を説明するだけで、project test を実行したり、code が ready か判定したりしません。

## `slipway run`

```text
slipway run <goal> [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json]
```

Run を作成し、最初の `orient` Action を返します。Action budget はデフォルト 8、範囲は 1–1000 です。`--no-review` は advisory Review を無効にします。それ以外でも、Slipway が Action 後に code change を観測した場合だけ Review を issue します。

`--source-file` 省略時は ad-hoc Run です。指定時、CLI は1つの bounded GitHub Change source envelope を開いて検証し、accepted section を pin して file を閉じます。CLI 自体は GitHub を fetch せず、host publication warning を表示しません。これらは generated host instruction が行います。

Canonical machine invocation はすべての flag を `--` の前に置き、goal をその後に置きます。

```bash
slipway run --budget 8 --json --root /absolute/repository -- "small private fix"
slipway run --budget 8 --json --root /absolute/repository \
  --source-file /private/temp/change-envelope.json -- "implement the Change"
```

この command は Action を返すだけで、code 変更を実行しません。

## `slipway status`

```text
slipway status [run-id] [--root ROOT] [--json]
```

ID 省略時は Git common directory 内の Run を一覧します。Current worktree の Run は replay され、別 linked worktree の Run は `workspace_foreign` マーク付き read-only header として表示されます。完全な inspect と mutation は owning worktree が必要です。

`status` は filesystem に対して read-only です。Run namespace や lock file の作成、permission の変更、中断した journal tail の修復は行いません。Replay できない local recovery directory も JSON の `unavailable_runs` に残り、その ID を指定すると `run_journal_invalid`、存在しない ID なら `run_not_found` を返します。

ID 指定時は現在の Run projection と fresh 派生の structured `next` を返します。空リストは有効な出力です。

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

Run を停止し、journal を保存します。ID 省略時は list の active/paused entry を数え、1つだけの場合に進みます。読めない local recovery directory が1つでもあれば、無視せず explicit ID を要求します。Active/paused `workspace_foreign` stub は暗黙に選択しません。Stopped Run は resume できます。Ended Run はできません。

## Hidden host 操作

Generated adapter は versioned `_machine` 操作で Outcome 提出、Action の answer/skip、Run resume、pinned material 読み取りを行います。これらは top-level help に意図的に表示されず、第2の user workflow でもありません。

Prose から hidden command を組み立てず、CLI が返す structured `next` variant を使ってください。詳細は[マシンプロトコル](machine-protocol.md)を参照してください。
