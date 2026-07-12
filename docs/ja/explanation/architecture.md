# アーキテクチャ（非規範）

> 完全な authority は[中国語製品契約](../../zh/reference/product-contract.md)です。

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil（必要な low-level primitive のみ）
```

`cmd` は7 public command、hidden machine protocol、text/JSON rendering、`autopilot` は strict Action/Outcome、source/revisions/candidates、budget、routing、destructive grant、structured next、`runstore` は anchored append-only journal/projection、`adapter` は10 host の ownership-safe plan、`tmpl` は正確に6 capability と MIT attribution 付き `grill-me` reference、`fsutil` は rooted transaction/Git discovery/symlink-reparse defense/rollback validation、`recoverycmd` は完全 argv の POSIX/cmd/PowerShell display rendering だけを担当します。Renderer は journal や route を読みません。

Run start は immutable workspace identity、exact index/porcelain-v2 bytes、pre-existing dirty/untracked metadata/digest を保存し、Load/mutation 前に identity を再検証します。Observed-since-start diff は safety-side Review signal ですが Run causation の証明ではありません。

GitHub publication は host-side で、Go binary は token を持たず host-attested raw envelope を検証し normalized snapshot を固定します。Approved markers/reconciliation を使い repository runtime を作りません。

Model provider、old-state reader、compat alias、dual runtime、ambient activation、required-command registry、Spec/artifact lifecycle、worktree binding、automatic review-repair loop はありません。Historical data は変更せず無視します。
