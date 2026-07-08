# Stack

## Languages And Runtime
- Go 1.26 module `github.com/signalridge/slipway` (`go.mod`).
- CLI entry points are Cobra commands under `cmd/`; domain logic lives under `internal/`.

## Build And Test Tooling
- Primary verification is `go test ./...`.
- Focused command and model tests live in package-local `_test.go` files, with shared command fixtures in `cmd/lifecycle_commands_test.go` and `cmd/evidence_task_test.go`.

## Change-Relevant Dependencies
- `github.com/spf13/cobra` owns CLI command surfaces.
- `gopkg.in/yaml.v3` is used for engine-owned state such as wave plans and verification records.
- `github.com/stretchr/testify` backs assertions in the affected tests.
