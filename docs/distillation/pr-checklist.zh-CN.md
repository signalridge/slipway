# 蒸馏 PR 清单（B0-B7 参考）

该清单保留为 B0-B7 阶段的评审参考。
B8 之后同等约束由 CI gate 自动强制；评审时可把本清单作为失败原因解释模板。

## Schema（等效 `schema-lint`）

- [ ] 前言声明 `skill_id`、`domain`、`function`、`tier`、
      `primary_attachment`、`summary`、`trigger_signals`、`evidence_contract`、
      `bindings`、`provenance_ref`。
- [ ] `skill_id` 与 `internal/tmpl/templates/skills/` 下的目录名一致。
- [ ] `summary` 使用 `Use when ... / Triggers on ...` 短语。
- [ ] `primary_attachment` 与每条 binding 的 `attachment` 属于
      `posture` / `procedure` / `checklist` / `tool-recipe` / `report-schema`。
- [ ] `trigger_signals[]` 仅使用 `schema.md` §4 列出的算子。
- [ ] 主体或 binding 中引用到的类型化模板均在磁盘上存在。

## Size（等效 `size-lint`，按层级）

进入警戒带只算评审提示，不会单独阻塞。真正的阻塞条件是主体超过 tier
hard-max 且缺少所需的 `size_rationale`。

- [ ] T1：`SKILL.md` 主体 ≤ 2.5 KB；2.5-6 KB 警戒；超 6 KB 需理由。
- [ ] T2：≤ 3.5 KB；3.5-8 KB 警戒；超 8 KB 需理由。
- [ ] T3：≤ 1.5 KB；1.5-3 KB 警戒；超 3 KB 必须填写 `size_rationale`。
- [ ] 超出预算的长例、反例与叙事已迁至 `references/` 或有意识删除；
      能留在预算内的 inline tool-recipe 差异可以继续保留在 `SKILL.md`。

## Binding（等效 `binding-compare`）

- [ ] 前言的 `bindings[]` 与 Go 侧能力注册表对应条目一一对应。漂移即阻塞。

## Provenance（等效 `provenance-coverage-scan`）

- [ ] `provenance.yaml` 把计划指派的每个源列入 `extracted` / `dropped` /
      `conflicts_with` 之一。
- [ ] 所有上游叙事要么以规则锚定吸收，要么带原因被丢弃，要么搬入 `references/`。

## 文档同步

- [ ] 若该批次落地，已更新 `docs/distillation/catalog.md` 的状态列。
- [ ] 已更新 `docs/distillation/by-source.md` 中本 PR 触及源的状态列。
- [ ] 本 PR 同步更新 EN 与 zh-CN 两版。

## 内核与边界

- [ ] `ResolveNextSkill` 未改。
- [ ] 不新增既有 10 个 governed host 之外的任何 host。
- [ ] `--mode` / `--view` 行为保持“显式覆盖优先于自动路由”。
- [ ] CI 门自动化（schema/size/binding/provenance）持续由测试强制。
