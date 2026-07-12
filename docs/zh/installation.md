# 安装

```bash
go install github.com/signalridge/slipway@latest
slipway install --tool claude
```

容器镜像包含 Git。Linux 上向挂载 worktree 写入 capability 或 journal 时，请传入宿主 UID/GID：

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:<version> install --tool claude
```

支持 `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf`；可以重复 `--tool` 或使用 `--tool all`。不指定时只安装检测到的宿主。

```bash
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

Refresh 和 uninstall 只修改 ownership manifest 中哈希仍匹配的文件。用户修改、未知、marker-only、路径越界或 symlink 表面会被保留或安全拒绝。version 1 manifest 只读，用于安全删除旧版本仍保持原样的文件。不会安装 SessionStart 或 prompt-submit 自动入口。
