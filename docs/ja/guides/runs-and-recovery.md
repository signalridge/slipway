# Run、復旧、プライバシー

Run は stop や resume が可能ですが、journal は secret vault や completion certificate ではありません。この guide はユーザーが inspect し retention できる内容を説明します。

## Run を inspect する

```bash
slipway status
slipway status <run-id>
slipway status <run-id> --json
```

ID 省略時、`status` は repository の Git common directory を走査します。Current worktree の Run は完全に replay されます。別の linked worktree が作成した Run は `workspace_foreign` マーク付きの read-only header stub として表示され、完全な内容は owning worktree で確認・復旧します。

Inspect は recovery directory や lock を作成せず、journal byte も修復しません。Local Run を安全に replay できない場合、list JSON はそれを隠さず、`unavailable_runs` に ID と診断を保持します。Mutation の前に journal を確認または復旧してください。

ID を指定すると、status には現在の state と fresh 派生の structured `next` operation が含まれます。Generated host は表示用 shell command を parse せず、この object に従います。

## Stop、skip、resume

```bash
slipway stop <run-id>
```

`stop` は pending work を無効化しますが、journal を残します。ID 省略時は list の local active/paused entry を数え、`workspace_foreign` stub は除外します。読めない local recovery directory があれば explicit ID を要求します。Stopped Run は resume できます。Ended Run はできません。

Skip は Action level の制御で、理由は不要です。Reorder や take over は queue を書き換えず、automatic loop を止めます。明示的な resume の後だけ続行します。

Resume は original worktree identity を再検証し、fresh work を生成します。Stale Action ID、source candidate、answer、destructive grant は machine protocol に従って reject または無効化されます。

Resume で `--budget` を省略すると、remaining budget が正なら保持し、0なら initial budget と3の大きい方まで補充します。`--budget N` を指定すると remaining budget を `N` に置き換えます。Replacement は operation が実際に Run を resume した場合だけ適用されます。

## Issue source の復旧

Issue-backed Run では、generated host が有効な source 選択肢を提示します。

- 現在の Change を fetch して比較する。
- Refresh できない、または不要な場合は pinned snapshot を明示的に継続する。
- 変更を検出した場合、pinned snapshot を保つか exact current candidate を adopt する。

Refresh の省略は Issue が unchanged であることの証明になりません。別 Issue identity や source history fork には新しい Run が必要です。

## Storage layout

Run data は repository の Git common directory にあり、current worktree の文字通りの `.git` path とは限りません。

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl
├── run.json
├── run.lock
└── materials/
```

- `journal.jsonl` は復旧用の append-only transition record。
- `run.json` は journal から再構築可能な projection。
- `materials/` は accepted Issue section を content digest で保持。
- `run.lock` は validated な coordination artifact。実際の writer serialization は Unix では OS directory lock、Windows では named mutex を使います。

Load と mutation のたびに、canonical worktree root、per-worktree Git directory、Git common directory を再確認します。Path が別 worktree に再利用されたり、Git metadata が retarget されたりすると、journal 変更前に失敗します。

## 保存され得る内容

Run は次を記録する場合があります。

- Goal と source identity。
- Accepted requirements material と digest。
- User answer と source choice。
- Action、Outcome、summary、finding、不確実性。
- 報告された test、type-check、build、lint command と exit code。
- Start と現在 worktree を比較するための bounded Git observation。

Slipway は GitHub token、credential store、environment dump、unrelated file content、unreferenced Issue comment、full conversation transcript、hidden reasoning を意図的には収集しません。Generated host には、認識した credential value を redact しつつ truthful command identity を保つことが指示されます。

これらの保護は絶対ではありません。Goal、requirements、answer、filename、summary、command argument 自体が機微になり得ます。Run directory は private local data として扱い、secret を貼り付けないでください。

## File 観測

Git observation は fingerprint と bounded metadata を保持し、file content は保持しません。小さな regular dirty/untracked file は full streamed digest になります。大きな file は size と固定 sampling region を使い、sampling 外の同長編集を見逃す場合があります。Symlink は追跡しません。

Run start 以降の観測された変更は、誰またはどの process が原因かを証明しません。Review と summary はこの attribution 不確実性を維持します。

## Durability と platform

Unix では journal/projection の file sync と、対応環境では directory sync を行います。Windows は file flush しますが、新規や renamed directory entry に対する等価な directory-fsync 保証がなく、`doctor` が利用可能な durability level を報告します。

Slipway は unsafe symbolic-link や reparse-point mutation path を拒否します。これらの制御は偶発的・並行的な破損リスクを減らしますが、root、malware、同じ account で継続的に競合する process までは防ぎません。

## Retention と removal

`<git-common-dir>/slipway/runs/<run-id>/` の削除は、その Run の Slipway local recovery data を削除します。GitHub content、Git history、backup、filesystem snapshot、log、すでに他へ複製されたデータは消えず、secure erase でもありません。

Adapter removal は別物です。

```bash
slipway uninstall --tool claude
```

これは pristine generated file だけを削除し、Run data は変更しません。

## トラブルシューティング

まず次を実行します。

```bash
slipway doctor
slipway status <run-id> --json
```

`journal.jsonl` や `run.json` を手動で編集しないでください。元の worktree を保持し、structured error output を保存し、`next` が返す exact recovery option を使います。Source や workspace identity を安全に復旧できない場合は、古い state を無理に操作せず新しい Run を開始してください。

詳細は[コマンドリファレンス](../reference/commands.md)と[アーキテクチャ](../explanation/architecture.md)を参照してください。
