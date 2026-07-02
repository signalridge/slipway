# Structure

- `internal/model/env_catalog.go`
  - Owns `EnvCatalogEntry`, env scopes, and the curated `EnvCatalog()` list
    surfaced by `slipway config list --env`.
- `internal/model/env_catalog_test.go`
  - Owns source-scan coverage for production env reads and well-formedness
    checks for env catalog entries.
- `cmd/config.go`
  - Owns `config list` / `config list --env` JSON and text projections.
- `cmd/config_test.go`
  - Owns command-level assertions for config catalog and env catalog output
    shape.
- `internal/engine/capability/resolver.go`
  - Owns runtime interpretation of host capability tokens and fallback modes.
- `cmd/next.go` and `cmd/validate.go`
  - Own host capability requirement projection into `next` and `validate`
    views; they should consume existing runtime semantics unchanged.
- `cmd/context_pressure_hook.go` and `cmd/next_context_budget.go`
  - Own context metric/window env behavior that the catalog must describe.
- `cmd/tool_github.go`
  - Owns GitHub API env resolution, allowlist parsing, and token destination
    safety.
- `internal/state/handoff.go`
  - Owns handoff session-owner fallback order.
- `docs/reference/commands.md`, `docs/index.md`, and `README.md`
  - Own public navigation into configuration and host environment reference
    docs.
