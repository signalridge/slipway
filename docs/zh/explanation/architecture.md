# 架构

规范语义见[中文产品契约](../reference/product-contract.md)。

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil（仅需要的低层原语）
```

`cmd` 拥有七个公开命令、隐藏机器协议与 text/JSON rendering；`autopilot` 拥有严格 Action/Outcome、source/revisions/candidates、budget、routing、destructive grant 与结构化 next；`runstore` 拥有 anchored append-only journal 与 projection；`adapter` 为十宿主做 ownership-safe planning；`tmpl` 嵌入精确六能力和保留 MIT attribution 的 `grill-me` reference；`fsutil` 提供 rooted transaction、Git discovery、symlink/reparse defense 与 rollback validation；`recoverycmd` 只消费完整 argv 并渲染 POSIX/cmd/PowerShell，不读 journal、不决定路由，autopilot 不依赖它。

Run start 保存 immutable workspace identity、精确 index/porcelain-v2 bytes 与 pre-existing dirty/untracked path metadata/digests；Load/mutation 前重验 identity。Observed-since-start diff 触发安全侧 Review，但不能证明差异由本 Run 造成。

GitHub publication 在 host side；Go binary 不持 provider token，只验证 host-attested raw Change envelope 并固定 normalized snapshot。Publication 用 approved markers 与 reconciliation，不恢复 repository runtime。

架构不含 model provider、old-state reader、compat alias、dual runtime、ambient activation、required-command registry、Spec/artifact lifecycle、worktree binding 或 automatic review-repair loop。历史数据不触碰、运行时忽略。
