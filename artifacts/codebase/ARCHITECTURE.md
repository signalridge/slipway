# Architecture

- Question: Which public surfaces own environment-variable and host-capability
  contracts so host integrators can wire Slipway without reading Go source?
- Environment catalog authority: `internal/model/env_catalog.go` defines
  `EnvCatalogEntry` and `EnvCatalog()`, and `cmd/config.go` projects it through
  `slipway config list --env` text and JSON. This is the right single authority
  for env contract metadata because `internal/model/env_catalog_test.go` already
  scans source literals to ensure every `SLIPWAY_*`, token, and username fallback
  read by production Go code is cataloged.
- Runtime-host consumers: `cmd/next.go` reads `SLIPWAY_HOST_CAPABILITIES` and
  `SLIPWAY_HOST_CAPABILITY_FALLBACKS` for host capability gate projection;
  `cmd/next_context_budget.go` and `cmd/context_pressure_hook.go` read
  `SLIPWAY_CONTEXT_WINDOW_TOKENS`; `cmd/context_pressure_hook.go` reads
  `SLIPWAY_CONTEXT_METRICS_PATH`; `internal/state/handoff.go` reads
  `SLIPWAY_SESSION_OWNER`, `USER`, and `USERNAME`.
- Host capability semantics: `internal/engine/capability/resolver.go` maps
  `subagent` and `delegation` to available subagent capability, `none` and
  `unavailable` to unavailable, empty/unset to unknown, and configured fallback
  modes to explicit degraded operation. Current catalog output does not expose
  those tokens or their behavior.
- Existing partial public surfaces: command help for GitHub API helpers already
  explains `SLIPWAY_GITHUB_API_URL`,
  `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS`, `SLIPWAY_GITHUB_API_TOKEN`, and
  `GH_TOKEN`/`GITHUB_TOKEN`; `next --help` explains
  `SLIPWAY_CONTEXT_WINDOW_TOKENS`; `handoff --help` explains
  `SLIPWAY_SESSION_OWNER`. These are command-local hints, not the complete env
  catalog contract.
- Generated skill boundary: stage skills correctly declare/remediate required
  host capabilities as workflow prerequisites, but they do not name env knobs.
  Per instruction-boundary rules, host wiring belongs in `config list --env` and
  a host-integration doc, not repeated inside every per-stage skill.
- Blast radius: additive model fields, config output, documentation, and tests.
  No lifecycle state schema, capability runtime semantics, or subagent dispatch
  semantics need to change.
