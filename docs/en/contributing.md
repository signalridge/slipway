# Development reference

See the top-level [contribution guide](../../CONTRIBUTING.md) for the collaboration flow.

## Repository layout

```text
cmd/                    seven public commands, hidden machine protocol, rendering
internal/autopilot/     Action/Outcome unions, source envelopes/revisions, routing
internal/runstore/      Git-common-dir append-only journals
internal/adapter/       host generation and ownership-safe transactions
internal/tmpl/          six embedded explicit capabilities
internal/fsutil/        atomic/rooted transaction, Git discovery, symlink defense
internal/recoverycmd/   POSIX/cmd/PowerShell display rendering from argv
internal/jsonstrict/    shared strict-JSON structural scanner
internal/testlint/      repository test analyzer
docs/                   authoritative documentation (website syncs from here)
acceptance/             black-box E2E, fixtures, and host acceptance assets
```

Dependency direction is fixed and enforced: `cmd → autopilot → runstore`, `cmd → adapter → tmpl`, `cmd → recoverycmd`, and the lower packages depend only on `fsutil` primitives as needed. See [Architecture](explanation/architecture.md).

## Local checks

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go test -timeout=20m ./... -race -count=1
go build ./...
git diff --check
```

When available:

```bash
golangci-lint run --timeout 5m
(cd website && npm ci && npm run build)
just acceptance
```

Also exercise built-binary help and JSON behavior after public-surface changes.

## Test focus by change type

| Change type | What to cover |
| --- | --- |
| Adapter | Every host path, current-only manifest rejection, marker-only no-op, modified-file and settings preservation, traversal, symlinks, and rollback. |
| Run / autopilot | Transition tables, idempotency, stale Actions, budget exhaustion, stop/resume, linked worktrees, journal crashes, and concurrent submissions. |
| Source / journal | Strict parser, duplicate keys, oversize, bad UTF-8, symlink/reparse source files, and byte-identical replay. |
| Public surface | Built-binary help, JSON envelopes, and generated capability fixtures. |

## Design constraints

- The user explicitly starts the soft autopilot and retains control throughout.
- Repository facts are investigated before any human decision is requested.
- Technical activities remain part of implementation reporting; Review is read-only.
- Run journals exist only for recovery and may contain sensitive text.
- Generated files are deterministic, transactional, symlink-safe, and ownership-aware.
- Do not add aliases, readers, or compatibility modes for the retired runtime.

See [architecture](explanation/architecture.md), [machine protocol](reference/machine-protocol.md), and [acceptance scenarios](../../acceptance/README.md).
