# Contributing to Slipway

Thanks for improving Slipway. Use a focused fork-and-pull-request workflow and keep implementation, tests, generated capability contracts, and documentation aligned.

## Development setup

Slipway requires the Go version declared in `go.mod`. Optional release and documentation tooling includes `golangci-lint`, GoReleaser, Node.js, and npm.

```bash
git clone https://github.com/<you>/slipway.git
cd slipway
go mod download
go test ./... -count=1
go build ./...
```

## Before opening a pull request

Run the checks relevant to your change; changes to journals, locks, concurrency, or filesystem mutation should include the race suite.

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
just acceptance
git diff --check
```

When available:

```bash
golangci-lint run --timeout 5m
(cd website && npm ci && npm run build)
goreleaser check
```

## Design constraints

- The user explicitly starts the soft autopilot and retains control.
- Repository facts are investigated before human decisions are requested.
- Technical activities remain part of implementation reporting.
- Review is read-only and reports findings without creating an automatic repair loop.
- Run journals exist only for recovery.
- Generated files are deterministic, transactional, symlink-safe, and ownership-aware.
- Do not add aliases or readers for the retired runtime.

See [architecture](docs/en/explanation/architecture.md), [machine protocol](docs/en/reference/machine-protocol.md), and [acceptance scenarios](tests/acceptance/README.md).

## Commits and pull requests

Use Conventional Commits, for example `feat: add resume diagnostics` or `fix(adapter): preserve modified capability`. Add `!` or a `BREAKING CHANGE:` footer when appropriate. Keep a pull request focused and explain observable behavior, tests run, and remaining uncertainty.

Contributions are licensed under the repository's [BSD 3-Clause License](LICENSE).
