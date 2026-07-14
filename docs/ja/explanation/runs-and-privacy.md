# Run とプライバシー

復旧データは `.git/slipway/runs/<run-id>/{journal.jsonl,run.json,run.lock}` にありますが、workspace identity は worktree ごとに異なります。ID なしの `status` は current canonical workspace が所有する Run だけを列挙します。Sibling linked worktree の Run は common directory 上で物理的に共有されても一覧には現れず、recovery route の表示や mutation は original workspace からだけ実行できます。`journal.jsonl` だけが append-only authority、`run.json` は再構築可能な projection、`run.lock` は journal mutation だけを直列化します。Unix intent は directory `0700`、leaf `0600`。旧 `events.jsonl` と `runtime/cache/scope-root/scopes/locks/processes/repair-backups` は unowned residue で、Run は無視し、doctor は advisory 用の top-level name だけを見て、task content の read/migrate/alias/delete を行いません。

## 保存データと正確な privacy claim

Journal は original goal、canonical workspace identity、immutable initial Git observation、structured Git delta、bounded issue-source chapter catalog/provenance、Actions/Outcomes、answer/supersession metadata、skip/stop、source choice、destructive request/grant、budget、truthful activity command summaries、known issues、uncertainties を含みます。Accepted chapter Markdown は private content-addressed `materials/` に一度だけ保存し、各 Action や journal event へ複製しません。**Goal、accepted Requirements、user answer、command summary は機微な text を含む可能性があります。** Source import/journal creation 前に警告し、`.git/slipway/runs/` を local private data として扱います。

Slipway は secret-free journal を約束しません。GitHub token、credential store、unreferenced discussion、environment dump、unrelated file content、full transcript、hidden reasoning を意図的に収集しない data minimization を約束します。Issue source import では host が exact Issue body と manifest 参照 raw comment fields だけを一時取得し、private raw envelope を CLI consume にだけ渡して temporary file を削除します。CLI は parser-accepted normalized chapter payloads と bounded catalog/provenance だけを保存し、raw Issue body、comment marker、raw enclosing comment envelope は保持しません。Git path observation は category/state、size、bounded SHA-256 だけで、16 MiB 超過/unreadable は状態だけです。

Generated host は publication/journaling 前に認識した credential value を redact しながら、executable と redacted argument の位置/名前という truthful command identity を残します。認識は完全ではないため secret を入力しないでください。Public repository Issue に private switch はなく、private repository、実際の vulnerability だけ enabled private vulnerability reporting、既存 security channel、または ad-hoc Run を使います。

Action context は 128 KiB に制限され full replay ではありません。Requirements は別 field のまま、active decisions と Outcome summaries/known issues を決定的に選び、newline/UTF-8 boundary truncation に byte count/SHA-256 marker と omission count を付けます。

## Permission、retention、削除

Unix mode は root、backup、malware、same UID process を防ぎません。Windows は current-user ACL intent を使いますが inherited ACL、administrator、backup agent、same-account process がアクセスできる可能性があり、absolute ACL isolation は保証しません。Owner は retention を定義し、ACL と backup を確認し、`.git/slipway/runs/` を publish しないでください。

Namespace mutation の保証はさらに限定されます。Anchored handle と長寿命 identity pin は parent traversal、identity reuse、validation checkpoint で観測した replacement を防御・検出しますが、portable POSIX `unlinkat` は parent descriptor と leaf name で削除し、最後の directory entry と既に開いた leaf handle を compare-and-unlink できません。Exact native primitive がない platform では、最後の validation と unlink の間を継続的に競争する same-UID watcher に対する linearizable exact-object deletion を主張しません。Private randomized quarantine、atomic no-replace relocation、revalidation、post-check で window を縮小し、checkpoint で観測した replacement は保存・報告します。Root、malware、same-account process が最後の syscall gap を競争することは明示的な residual limitation です。

run directory の削除は Slipway recovery capability と projection だけを除きます。Git/source/Issue/deployment、replica、snapshot、cloud backup、filesystem remnant、encryption key は変わりません。Secure erase、backup purge、key destruction ではありません。

## Commit と recovery

Journal bytes と file fsync が成功して初めて committed です。Projection は temp encode/write/fsync、rename、対応 platform の directory sync を使います。Commit 後の projection failure は `mutation_committed_projection_stale` と no retry を返すため、blind retry せず authoritative journal を replay します。Load/status/mutation 前に workspace identity を再検証し、mismatch は journal を変更しません。Interrupted final record は同じ verified handle で修復し、以前の corruption は拒否します。Windows は file flush を行いますが equivalent directory fsync がなく、`file_fsync_only`/`directory_fsync_unsupported` と報告し Unix と同じ crash durability を主張しません。
