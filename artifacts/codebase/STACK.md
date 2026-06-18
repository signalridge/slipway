# Stack

- Languages: Go; module path `github.com/signalridge/slipway`.
  Evidence: `go.mod:1-3`.
- Frameworks and runtimes: Cobra CLI via `github.com/spf13/cobra`;
  command entry starts at `main.go` and delegates to `cmd.Execute`.
  Evidence: `go.mod:8`, `main.go:7-17`.
- Build and test tooling: `go build ./...` and `go test ./...` are the
  baseline commands emitted by `slipway codebase-map --json`.
- Key dependencies: file locking (`gofrs/flock`), UUIDs (`google/uuid`),
  Cobra/pflag, testify, x/term, x/tools, and YAML. Evidence: `go.mod:5-13`.
- Notes: this change is template- and hook-surface oriented; no new runtime
  dependency should be needed.
