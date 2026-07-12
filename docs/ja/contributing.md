# コントリビュート

`go.mod` 指定の Go を使い、提出前に次を実行してください。

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

CLI、マシンプロトコル、生成 capability、文書、テストを同時に整合させます。Adapter 変更は十ホスト、ownership version 1 の安全移行、ユーザー変更保持、settings の精密 cleanup、path traversal、symlink、rollback をテストします。Run 変更は transition、冪等性、古い Action、budget、stop/resume、linked worktree、journal interruption、concurrency をテストします。
