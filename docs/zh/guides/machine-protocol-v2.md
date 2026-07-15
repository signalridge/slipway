# 机器协议 v2 教程

本教程执行一次完整的本地协议生命周期：启动 Run，提交 Orient 与 Implement Outcome，最后以 Summarize 结束。本文面向宿主与适配器作者。通常由生成的宿主 capability 执行这些隐藏操作；它们不是供终端用户使用的另一套工作流。

规范契约是带版本路径的 [machine protocol schema](../../reference/v2/machine-protocol.schema.json) 与 [source envelope schema](../../reference/v2/source-envelope.schema.json)。URL 中的版本必须与 JSON 的 `contract_version` 或 `source_version` 同步。原有无版本 schema URL 继续作为 v2 兼容别名，但新集成应使用 `/reference/v2/`。

## 前置条件

安装 `slipway`、`git` 与 `jq`，并在同一个 shell 会话中依次运行所有代码片段。本教程在一次性目录中工作，因为它会创建真实的 Run journal，并修改一个已跟踪文件。

```bash
TUTORIAL_DIR=$(mktemp -d)
WORKSPACE="$TUTORIAL_DIR/workspace"
mkdir -p "$WORKSPACE"
cd "$TUTORIAL_DIR"
git -C "$WORKSPACE" init -q
git -C "$WORKSPACE" config user.name 'Protocol Tutorial'
git -C "$WORKSPACE" config user.email tutorial@example.invalid
printf '# Protocol tutorial\n' > "$WORKSPACE/README.md"
git -C "$WORKSPACE" add README.md
git -C "$WORKSPACE" commit -qm initial
```

## 1. 启动 Run

将所有 flag 放在 `--` 分隔符之前，并把目标保留为一个字面参数。`--no-review` 让这个简短生命周期从 Implement 直接进入 Summarize。

```bash
slipway run \
  --budget 4 \
  --json \
  --root "$WORKSPACE" \
  --no-review \
  -- "add one tutorial line to README.md" > start.json

jq -e '
  .contract_version == 2 and
  .state == "active" and
  .action.kind == "orient" and
  .next.operation == "action"
' start.json

RUN_ID=$(jq -r '.run_id' start.json)
ORIENT_ID=$(jq -r '.action.action_id' start.json)
```

生产宿主应使用 machine protocol schema 验证完整响应。这里的 `jq` 表达式只把教程关注的断言明确展示出来。必须将 `next.variants[].base_argv` 保留为参数数组；不要解析渲染后的命令字符串。

## 2. 提交 Orient Outcome

每个公开 Outcome 字段都必须存在。不适用的分支使用 JSON `null`，空集合仍保持为数组。

```bash
jq -n --arg action "$ORIENT_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "orient",
  status: "completed",
  summary: "Repository facts observed.",
  observations: ["README.md is the only tracked file."],
  known_issues: [],
  suggested_actions: [{
    kind: "implement",
    brief: "Append the requested tutorial line."
  }],
  pause: null,
  implementation: null,
  review: null
}' > orient-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$ORIENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file orient-outcome.json > implement.json

jq -e '.contract_version == 2 and .action.kind == "implement"' implement.json
IMPLEMENT_ID=$(jq -r '.action.action_id' implement.json)
```

`action_id` 与 `action_kind` 必须匹配当前待处理 Action。宿主不得复用旧响应中的 ID，也不得自行构造下一个 Action。

## 3. 执行并报告实现

完成可观察的修改，运行检查，并报告准确的 activity 与退出码。

```bash
printf 'Protocol v2 lifecycle completed.\n' >> "$WORKSPACE/README.md"
git -C "$WORKSPACE" diff --check

jq -n --arg action "$IMPLEMENT_ID" \
  --arg check_command "git -C \"$WORKSPACE\" diff --check" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "implement",
  status: "completed",
  summary: "Appended the requested README line.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: {
    result: "applied",
    files_changed: ["README.md"],
    activities: [{
      kind: "test",
      command: $check_command,
      exit_code: 0,
      summary: "No whitespace errors."
    }],
    uncertainties: [],
    attempts: 1
  },
  review: null
}' > implement-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$IMPLEMENT_ID" \
  --root "$WORKSPACE" \
  --outcome-file implement-outcome.json > summarize.json

jq -e '.action.kind == "summarize"' summarize.json
SUMMARIZE_ID=$(jq -r '.action.action_id' summarize.json)
```

`files_changed` 是宿主报告，不是归因证明。Slipway 会另外记录有界的 Git observation，并保留并发用户或工具修改所带来的不确定性。

## 4. 结束 Run

```bash
jq -n --arg action "$SUMMARIZE_ID" '{
  contract_version: 2,
  action_id: $action,
  action_kind: "summarize",
  status: "completed",
  summary: "The requested README update is complete and git diff --check passed.",
  observations: [],
  known_issues: [],
  suggested_actions: [],
  pause: null,
  implementation: null,
  review: null
}' > summarize-outcome.json

slipway _machine submit \
  --run "$RUN_ID" \
  --action "$SUMMARIZE_ID" \
  --root "$WORKSPACE" \
  --outcome-file summarize-outcome.json > ended.json

jq -e '
  .contract_version == 2 and
  .state == "ended" and
  .next.operation == "none" and
  (.next.variants | length) == 0
' ended.json

rm -rf "$TUTORIAL_DIR"
```

再次提交完全相同的 Outcome 字节是幂等的。针对同一个已完成 Action 提交不同字节会返回 `outcome_conflict`；过期 Action ID 会失败关闭。请根据结构化错误 `code` 分支，不要匹配 message 文本。

## Issue-backed 扩展

对于 issue-backed Run，受信任宿主先验证 source envelope 并写入私有临时文件，再通过 `--source-file` 一次性传入。响应会增加 `pinned_source`、`action.source`、`action.requirements` 与结构化 `_machine material` reader。只获取 manifest 引用的评论，绝不能把普通讨论评论视为需求。权威性与发布模型详见[机器协议参考](../reference/machine-protocol.md)和 [ADR-0001](../../../adr/0001-source-bundle-v2.md)。
