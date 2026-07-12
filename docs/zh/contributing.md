# 贡献

开发需要 `go.mod` 指定的 Go 版本。提交前运行：

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

CLI、机器协议、生成能力、文档与测试必须保持一致。Adapter 变更要覆盖十个宿主、ownership version 1 安全切换、用户修改保留、settings 精确清理、path traversal、symlink 和事务回滚。Run 变更要覆盖转移、幂等、旧 Action、预算、stop/resume、linked worktree、journal 中断与并发。
