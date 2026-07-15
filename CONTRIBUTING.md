# Contributing to Slipway

Thanks for improving Slipway. Keep pull requests focused, use a fork when you do not have write access, and keep implementation, tests, generated capability contracts, and documentation aligned.

## Development setup

Slipway requires the Go version declared in `go.mod`. Optional release and documentation tooling includes `golangci-lint`, GoReleaser, Node.js, and npm.

```bash
# Replace the URL with your fork when you do not have upstream write access.
git clone https://github.com/signalridge/slipway.git
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
just coverage-gate check
go test -timeout=20m ./... -race -count=1
go build ./...
just acceptance
git diff --check
```

Coverage baseline updates are reviewed artifacts. `just coverage-gate write` intentionally refuses to rewrite the committed Linux baseline on another operating system.

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

See [architecture](docs/en/explanation/architecture.md), [machine protocol](docs/en/reference/machine-protocol.md), and [acceptance scenarios](acceptance/README.md).

## Commits and pull requests

Use Conventional Commits for commits and release-ready pull request titles. CI validates titles against these types; the checker recognizes temporary WIP titles, which must be replaced before the pull request is ready to merge:

- `feat`
- `fix`
- `perf`
- `refactor`
- `deps`
- `security`
- `revert`
- `docs`
- `style`
- `chore`
- `test`
- `ci`
- `build`

The accepted forms are `<type>: <description>` and `<type>(<scope>): <description>`, for example `feat: add resume diagnostics` or `fix(adapter): preserve modified capability`. Mark a breaking change with `!`, such as `feat!: replace the protocol`, and explain it with a `BREAKING CHANGE:` footer when the pull request body needs more detail.

Keep a pull request focused and explain observable behavior, tests run, and remaining uncertainty. A title is release input, not only style: Release Please classifies it into the changelog and version proposal.

## Release automation

Release Please owns `CHANGELOG.md`, `.release-please-manifest.json`, the release pull request, and the `v*` tag. Do not hand-edit a release version or create a release tag as part of an ordinary contribution.

After changes reach `main`, Release Please opens or updates one release pull request. Maintainers review its changelog and manifest together. Merging that pull request creates the matching tag and GitHub Release with the configured automation token; the tag-triggered Release workflow then validates the tag, runs tests, builds and signs artifacts, and performs publication checks. A missing tag or GitHub Release for the manifest version is a failed release state that must be reconciled before another automated release.

Contributions are licensed under the repository's [BSD 3-Clause License](LICENSE).
