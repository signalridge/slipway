# 参与贡献

Slipway 是一个 Go CLI，配套生成 AI 工具接入面（AI-tool surfaces）和受治理的产物工作流。请把改动限定在你实际修改的模块或契约范围内。

## 仓库结构

| 路径 | 用途 |
| --- | --- |
| `cmd/` | Cobra 命令接入面，以及 CLI 的 JSON/文本视图。 |
| `internal/model/` | 持久化的领域类型、工作流状态和配置 schema。 |
| `internal/state/` | 文件系统布局、工作区配置、change 持久化以及归档辅助逻辑。 |
| `internal/engine/` | 推进（progression）、gate、治理、产物、上下文和 wave 逻辑。 |
| `internal/toolgen/` | AI 工具适配器生成，以及冻结的接入面契约。 |
| `internal/tmpl/templates/` | 内嵌的命令、skill、hook 和产物模板。 |
| `docs/` | 文档源页面（唯一可信源）。 |
| `website/` | Astro Starlight 站点，把 `docs/` 渲染并发布到 GitHub Pages。 |
| `artifacts/changes/` | Slipway 创建的受治理 change 产物包。 |

## 开发命令

```bash
go run . --help
go test ./... -count=1
go build ./...
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
```

需要最终验证时，使用与 CI 相同的超时设置：

```bash
go test -timeout=20m ./... -count=1
```

## 测试质量准则

测试要证明行为，而不是验证实现细节或机器时序。不要为了让 CI 安静下来而跳过有问题的测试。请在同一个 PR 里删掉它们，并换成确定性的行为覆盖。

- **空洞测试**：删掉那些只是执行了一遍代码、检查硬编码常量，或者断言 mock 被调用过却没有约束任何用户可见行为的测试。
- **源码 grep 测试**：删掉那些读取 `.go` 文件、对源码文本做 `strings.Contains` 断言的测试。改用能真正触发导出行为、解析结果、状态转换或渲染输出的测试——也就是这段源码本应保护的东西。
- **耗时测试**：删掉那些用 `time.Since`、时长阈值、sleep 或调度器时序去断言挂钟耗时的测试。改用确定性同步、fake clock、受控的 context 或显式事件。
- **真实竞态测试**：删掉那些指望 goroutine 按某种特定顺序交错、以此“证明”竞态的测试。改用同步屏障、race 检测器覆盖，或直接对状态机做断言。

当文本本身就是产品行为时，文本断言依然成立：生成的接入面、golden-output fixture，以及 CLI/API 契约测试都可以断言精确文本或必需的子串。把这类 fixture 放在被测行为附近，并给测试起一个让评审者一眼能看出“文本即契约、而非实现 grep”的名字。

`internal/testlint` 分析器覆盖了本地最有价值的几项策略检查：读取 `.go` 文件并断言 `strings.Contains` 的源码 grep 测试，以及基于 `time.Since` 或时长比较的耗时断言。直接运行：

```bash
go run ./internal/testlint/cmd/testlint ./...
```

## 文档

文档放在 `docs/`（唯一可信源），由 `website/` 中的 Astro Starlight 站点渲染。`website/scripts/sync-docs.mjs` 在构建时把 `docs/**` 转换成 Starlight 的内容集合，所以千万不要手改 `website/src/content/docs/`。新增或移动页面时，记得同步更新 `website/astro.config.mjs` 里的侧边栏。

本地构建或预览：

```bash
cd website
npm install
npm run build   # runs sync-docs, then `astro build`
npm run dev     # local preview with content synced from docs/
```

文档的 CI 工作流运行的是同一条 `npm run build`，然后部署到 GitHub Pages。

## 适配器契约

当命令元数据、生成路径、hook 或 prompt 接入面发生变化时，代码和测试要一起改：

```bash
go test ./internal/toolgen -count=1
```

生成的接入面都有契约测试覆盖，包括支持的工具 ID、命令路径、Codex 命令 skill 与遗留 prompt 清理、OpenCode 扁平命令，以及字节级稳定性。

## 治理契约

当生命周期、产物或 gate 语义发生变化时：

- 在所属的包里加一个聚焦的回归测试。
- 把共享语义收敛到一个 helper 里，不要重复 Markdown 或状态解析逻辑。
- 工具契约变化时，更新生成的 skill 或文档。
- 在当前受治理的 worktree 内用 `go run . validate --json` 验证。

## 治理内核覆盖率门禁

治理内核——`internal/engine/gate`、`internal/engine/governance` 和 `internal/engine/progression`（readiness 解析器位于 `progression/readiness.go`）——由一道“不可回退”的覆盖率门禁保护。任何内核包的语句覆盖率一旦跌破其已提交的下限，CI 就会失败。这道门禁是 fail-closed 的：它绝不会自动下调下限，也没有任何跳过、强制或软通过的路径。

- **基线**：`coverage-baseline.json`（仓库根目录）记录每个受门禁约束的包的下限（百分比，保留一位小数）。它由 `covergate` 工具生成，绝不手改。
- **CI 任务**：`Kernel Coverage Gate` 任务在 `-coverpkg` 限定到内核包的前提下跑完整套件，然后运行 `covergate -check`。覆盖率只在单一操作系统（ubuntu）上测量，以保证基线是确定的。
- **并集语义**：在多包运行下，`-coverpkg` 会在每个测试二进制里发出同一个 block；`covergate` 会对它们取并集（一个 block 只计一次，只要任意一次运行覆盖到了就算覆盖），与 `go tool cover` 保持一致。
- **模式选择**：`covergate` 必须显式指定 `-check` 或 `-write`；不带模式调用会被拒绝，这样门禁调用就不会意外软通过。`-check` 始终原样使用已提交的基线；像 `-exclude` 这类仅在 write 时使用的标志，在 check 模式下会被拒绝。

本地运行门禁：

```bash
just coverage-gate
```

当 CI 报告覆盖率回退时，常规修法是补测试把覆盖率补回来。如果这次下降是有意为之且经过评审（比如删掉了死代码），就上调（ratchet）基线并提交这段差异，让改动在评审中可见：

```bash
just coverage-baseline   # regenerates coverage-baseline.json from current coverage
```

向下调整基线永远不是自动的——它会出现在 PR 差异里，必须经过评审。提升覆盖率之后，用同一条命令把下限抬高。

**排除列表**：只在 `-write` 时（即确定受门禁约束的集合时）给 `covergate` 传 `-exclude <prefix[,prefix...]>`。它保留给非内核、生成的或仅测试用的前缀，仅在 include 集合扩大时使用；它无法移除任何必须纳入的治理内核包下限。请把受门禁约束的集合限定在治理内核范围内。

CI 任务在默认分支上变绿之后，维护者应把 `Kernel Coverage Gate` 加入分支保护的必需状态检查，这样一旦出现回退就会标红并阻止合并。
