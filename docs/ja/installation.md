# インストール

```bash
go install github.com/signalridge/slipway@latest
slipway install --tool claude
```

コンテナイメージには Git が含まれます。Linux で mount した worktree に capability や journal を書く場合は、ホストの UID/GID を渡します。

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:<version> install --tool claude
```

`claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` をサポートします。`--tool` は複数指定でき、`--tool all` も使えます。省略時は検出したホストだけを対象にします。

```bash
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

Refresh と uninstall は ownership manifest のハッシュが一致するファイルだけを変更します。ユーザー変更、未知、marker-only、範囲外 path、symlink は保持または安全に拒否されます。version 1 manifest は旧版の未変更ファイルを安全に削除するためだけに読み取ります。SessionStart や prompt-submit の自動入口は導入しません。
