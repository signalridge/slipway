# Development reference

See the top-level [contribution guide](../CONTRIBUTING.md) for the collaboration flow.

## Repository layout

```text
cmd/                 Cobra commands and protocol output
internal/autopilot/  Action contract and scheduler
internal/runstore/   Git-common-dir journals
internal/adapter/    host generation and ownership
internal/tmpl/       embedded capabilities
internal/fsutil/     atomic and transactional filesystem primitives
internal/testlint/   repository test analyzer
docs/acceptance/     prompt-level release evaluations
```

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

Also exercise built-binary help and JSON behavior after public changes. Adapter changes should test every host path, current-only manifest rejection, marker-only no-op behavior, modified-file and settings preservation, traversal, symlinks, and rollback. Run changes should test transition tables, idempotency, stale Actions, budget exhaustion, stop/resume, linked worktrees, journal crashes, and concurrent submissions.
