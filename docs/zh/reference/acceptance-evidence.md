# 验收与证据

本页说明证据收集，不是 release verdict 或 CLI 路由依据。[中文产品契约](product-contract.md)定义全部 35 场景；[可执行 acceptance matrix](../../../tests/acceptance/README.md)记录当前 artifacts 与真实缺口。

证据标签互补：C=deterministic Go contract/property/race/静态模板；S=调用构建后二进制的 Shell；G=隔离 live GitHub.com user-owned fixture；H=脱敏 Claude/Codex/Pi transcript+evaluator notes；W=native Windows cmd.exe+PowerShell；R=docs/website/package/release validation。

C 不能证明宿主自主行为 H；local fake endpoint 或 deterministic publication fault harness 是可复现 H/G-adjacent，不是 live G；Windows cross-build 不是 native W。缺失证据标 `not collected` 或 `external`，永不控制 Run routing、Issue status、Review、delivery 或 CLI exit。

本地 publication harness 无 credential 模拟 timeout-after-success、partial relation failure、duplicate markers、index delay 与零/一/多匹配。Live G 必须使用受保护测试账号/repository，fork PR 不暴露 secret。Transcript 按 `tests/acceptance/transcripts/` 脱敏格式记录，不复制 raw conversation、不伪造模型运行。

CI matrix 会在 `windows-latest` 构建 `slipway.exe`，并分别执行 native PowerShell 与 `cmd.exe` 资产；workflow wiring 本身仍只是 collector。[run 29197908671 / Windows job 86664073429](https://github.com/signalridge/slipway/actions/runs/29197908671/job/86664073429) 已针对 source `4c1741ae35b42d903fa1ccc4ec5ae32469aaca47` 完成两个资产，因此 matrix 为该 source、binary 与 assets 记录 W；后续相关修改必须重新完成一次收集。R checker 使用 `tests/acceptance/` 下的 stdlib link/release-artifact validator，覆盖 built-site route、archive LICENSE bytes、Scoop、AUR 与 package paths。

所有可执行验收资产只位于 `tests/acceptance/`，不能放入 `scripts/`。本地可用命令与当前状态以 matrix 为准。
