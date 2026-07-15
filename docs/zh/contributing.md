# 开发参考

本页补充根目录[贡献指南](../../CONTRIBUTING.md)，介绍仓库布局和文档验证。

## 仓库布局

| 路径 | 用途 |
| --- | --- |
| `cmd/` | 公开 CLI 命令及 JSON/human presentation。 |
| `internal/autopilot/` | Run routing、protocol validation、source handling 与 recovery choice。 |
| `internal/runstore/` | Journal、projection、locking、material 与 Git observation。 |
| `internal/adapter/` | Host generation 与 ownership-aware filesystem change。 |
| `internal/tmpl/` | 跨宿主共享的 embedded capability instruction。 |
| `internal/fsutil/` | Filesystem safety 与 transaction。 |
| `docs/{en,zh,ja}/` | 范围对等的用户、指南、参考与解释页面。 |
| `docs/reference/` | 与语言无关的 JSON Schema。 |
| `adr/` | 维护者决策历史，不属于用户文档。 |
| `acceptance/` | Black-box script、prompt scenario 与人工 evidence procedure。 |
| `website/` | 从 `docs/` 生成的 Starlight 网站。 |

强制 package direction 见[架构](explanation/architecture.md)。

## 本地检查

普通 Go 修改：

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go build ./...
git diff --check
```

并发、locking、journal 或 filesystem 修改还应运行 race suite：

```bash
go test -timeout=20m ./... -race -count=1
```

工具可用时再运行：

```bash
golangci-lint run --timeout 5m
goreleaser check
```

## 文档检查

仓库 Markdown 是网站来源。除三个 locale splash page 外，不要手工编辑 `website/src/content/docs/` 下的 generated file。

```bash
python3 -I acceptance/link_check.py --self-test
npm --prefix website ci
npm --prefix website run build
python3 -I acceptance/link_check.py --require-site
git diff --check
```

文档规则：

- 描述当前行为，不记录 PR 操作步骤或把可变 planning Issue 当成文档；
- 分开用户指南、integration reference、maintainer architecture、ADR rationale 与 acceptance evidence；
- 英文、中文、日文页面保持范围对等；
- 不让任何一种语言充当独立 implementation contract；
- 精确 machine shape 以 JSON Schema 表达，并明确 runtime-only semantic check；
- 宿主侧指令应写成 host behavior，不能误写为 Go CLI guarantee；
- Stable user page 不保存单次 CI run evidence 或 release history；
- 移动页面时同时更新 source link、website splash link 与 sidebar slug。

## 按修改类型选择测试

| 修改 | 重点 |
| --- | --- |
| CLI flag 或 output | Cobra help、JSON schema test、human rendering 与 command docs。 |
| Action/Outcome routing | Autopilot contract/service test、machine shell acceptance 与 protocol docs。 |
| Journal 或 locking | Replay/adversarial test、race suite、durability diagnostics 与 recovery docs。 |
| Source handling | Strict parser、hash/size/identity test、source schema、Issue guide 与 privacy docs。 |
| Adapter/template | Generator test、ownership test、`acceptance/adapters.sh` 与 adapter docs。 |
| Release channel | GoReleaser check、artifact validation 与 installation compatibility wording。 |
| 文档 | Link checker、markdown lint、website build 与 locale parity review。 |

## 产品约束

修改必须保留显式调用、用户控制、先查事实再提问、如实报告活动、只读 Review、可恢复 journal、ownership-aware generated file，以及不联网且不持有凭据的 core。公开接口变更时，应同步更新 code、schema、generated capability、test 和三种语言文档。
