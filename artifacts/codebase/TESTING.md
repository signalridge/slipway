# Testing

- Env source coverage: extend `internal/model/env_catalog_test.go`, which
  already enforces `EnvCatalog()` coverage for production env reads, to require
  structured contract fields for env vars with accepted tokens, format
  constraints, or non-obvious unset behavior.
- Config output coverage: extend `cmd/config_test.go` so
  `config list --env --json` exposes structured wiring metadata and
  `config list --env` text includes human-readable contract fields for
  `SLIPWAY_HOST_CAPABILITIES`.
- Capability behavior coverage: existing capability resolver tests in
  `internal/engine/capability/resolver_test.go` already prove the runtime
  behavior for `subagent`, `delegation`, `none`, unknown, and fallback modes.
  New catalog tests should align the public contract with those semantics
  instead of duplicating runtime resolution logic.
- Documentation coverage: add or update docs tests/manifest checks only if the
  repo already enforces the new doc path. At minimum, focused tests should make
  the command/catalog output self-sufficient even if prose docs drift.
- Verification stack for this change: focused `go test ./internal/model
  ./cmd ./internal/engine/capability -count=1`, then full `go test ./...`, plus
  governed `validate`/`next` evidence before closeout.
