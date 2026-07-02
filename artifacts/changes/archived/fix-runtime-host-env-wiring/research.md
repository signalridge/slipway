# Research

## Alternatives Considered

### Architecture
- Affected modules: `internal/model/env_catalog.go`, `cmd/config.go`,
  `cmd/config_test.go`, `internal/model/env_catalog_test.go`, docs under
  `docs/`, and generated skill source templates that mention host capability
  preconditions.
- Dependency chains: production env reads flow from command/runtime code into
  behavior (`cmd/next.go`, `cmd/context_pressure_hook.go`,
  `cmd/next_context_budget.go`, `cmd/tool_github.go`,
  `internal/state/handoff.go`); `internal/model/env_catalog.go` is the curated
  public env authority; `cmd/config.go` projects that authority through
  `slipway config list --env`; docs and command help point users back to that
  command.
- Blast radius: additive model/output/docs/tests. No lifecycle state file,
  capability resolver, subagent dispatch, or credential destination semantics
  need to change.
- Constraints: JSON output is public, so the design must add `omitempty` fields
  without removing or renaming existing fields. Secrets must remain env-only.
  Workflow skills should keep capability prerequisites, not become host manuals.

Env contract audit:

| Env var | Hidden contract found | Current public surface | Fix decision |
| --- | --- | --- | --- |
| `SLIPWAY_HOST_CAPABILITIES` | Accepted tokens `subagent`, `delegation`, `none`, `unavailable`; unset/empty means unknown and triggers a continuable authorization prerequisite; non-matching declared tokens mean unavailable. | Catalog only says "tokens"; skills say "host with subagent capability"; no public token mapping. | Fix in env catalog JSON/text and host-env doc. |
| `SLIPWAY_HOST_CAPABILITY_FALLBACKS` | Accepted values are the fallback modes declared on selected host-capability contracts, e.g. `manual_plan_audit`, `manual_independent_review`, `manual_security_review`, `manual_spec_compliance_review`, `manual_code_quality_review`, `manual_ship_verification`, `same_context_degraded`. | Catalog only says "fallbacks"; skills mention modes locally. | Fix in env catalog JSON/text and host-env doc. |
| `SLIPWAY_CONTEXT_WINDOW_TOKENS` | Positive integer only; malformed, zero, or negative values are ignored; default is 200000 tokens. | `next --help` documents this; catalog omits syntax/default behavior. | Add catalog structured syntax/unset behavior for single-source discovery. |
| `SLIPWAY_CONTEXT_METRICS_PATH` | Optional JSON metrics file path for context-pressure hook. Metrics may use `tokens_used` + `context_window`, `used_pct`, `used_percentage`, or `remaining_percentage`; stale metrics older than 60s are ignored; unset falls back to sanitized session temp files and transcript tail. | Catalog only names path. | Add catalog/doc metadata for path, accepted JSON fields, and unset fallback. |
| `SLIPWAY_SESSION_OWNER` | First non-empty owner wins from `SLIPWAY_SESSION_OWNER`, `USER`, `USERNAME`, hostname, then `unknown`. | `handoff --help` and catalog mention part of this. | Add precise catalog unset behavior. |
| `USER` / `USERNAME` | Ambient fallback only for handoff owner. | Catalog names fallback. | No structural accepted values needed; add unset behavior only if schema requires consistency. |
| `SLIPWAY_GITHUB_API_URL` | HTTPS GitHub REST/GraphQL base URL, normalized; env > file > default; override must be allowlisted. | GitHub helper help and README explain this; catalog has default/file key. | Add value syntax/unset behavior to catalog, but no runtime change. |
| `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS` | Comma/space/semicolon/newline/tab separated HTTPS API base URLs, normalized; env list overrides file allowlist and confirms token egress for file-configured override URLs. | GitHub helper help and README explain this. | Add value syntax/unset behavior to catalog. |
| `SLIPWAY_GITHUB_API_TOKEN` | Env-only token sent only to allowlisted override host; ambient tokens are not sent to override hosts. | GitHub helper help and README explain this. | Add catalog unset behavior; preserve secret handling. |
| `GH_TOKEN` / `GITHUB_TOKEN` | Ambient tokens used only for default `https://api.github.com`; `GH_TOKEN` wins over fallback. | GitHub helper help and README explain this. | Add catalog unset behavior; preserve secret handling. |

### Patterns
- Existing config catalog pattern: `internal/model/config_catalog.go` uses
  `AllowedValues []string` for constrained `.slipway.yaml` keys and
  `cmd/config.go` renders those values in text output. Env catalog currently has
  no equivalent.
- Existing env coverage pattern:
  `internal/model/env_catalog_test.go:TestEnvCatalogCoversEveryGetenv` scans
  production source literals and fails if a public env var is read without a
  catalog entry. This is the right test to extend from "listed" to "listed with
  contract detail when needed".
- Existing command-help pattern: `cmd/tool_github.go` has rich command-local env
  help for GitHub helpers; `cmd/next.go` and `cmd/handoff.go` include narrower
  env help. The catalog should not depend on every command duplicating the same
  full contract.
- Existing generated-skill pattern: skill frontmatter and remediation name host
  capability requirements/fallback evidence, while resolver behavior and
  next/validate projections own availability. Do not duplicate env knob manuals
  into every skill template.

### Risks
- Technical risk, medium: widening `EnvCatalogEntry` changes public JSON shape.
  Mitigation: add only optional fields and keep existing field names.
- Technical risk, medium: a free-form string-only fix could pass tests but fail
  machine consumers. Mitigation: use structured fields for value syntax,
  accepted values with descriptions, examples, and unset behavior.
- Security risk, low: documenting secrets must not imply they can be persisted
  to `.slipway.yaml`. Mitigation: preserve `secret=true`, keep `file_config_key`
  empty for secret entries, and explicitly describe env-only behavior.
- Guardrail domain: external API contract. The change affects public host
  integration behavior and must fail closed to review and evidence.
- Reversibility: additive catalog/docs/tests are straightforward to revert; no
  persisted user state migration is involved.

### Test Strategy
- Existing coverage: `internal/model/env_catalog_test.go` covers source env
  usage and entry well-formedness; `cmd/config_test.go` covers env text/JSON
  output; `internal/engine/capability/resolver_test.go` covers capability token
  runtime semantics.
- New coverage: require non-empty contract metadata for env vars with accepted
  values or non-obvious behavior; assert `SLIPWAY_HOST_CAPABILITIES` JSON
  includes accepted token mappings; assert text output includes contract fields;
  assert docs mention the concrete subagent declaration knob.
- Verification approach: run focused Go tests for `./internal/model`, `./cmd`,
  and `./internal/engine/capability`, then full `go test ./...` plus governed
  validation.

### Options
- Option A: Add structured env catalog wiring metadata and project it through
  `config list --env`, then add a concise host environment doc.
  - Pros: single authority, machine-readable JSON, human-readable text, catches
    future drift with tests, satisfies #395 without changing runtime behavior.
  - Cons: slightly expands public JSON shape and text output.
- Option B: Add only a host-integration doc.
  - Pros: smallest implementation.
  - Cons: leaves `config list --env --json` incomplete and does not prevent the
    same bug for future env vars.
- Option C: Patch every affected generated skill remediation to name
  `SLIPWAY_HOST_CAPABILITIES`.
  - Pros: users may see the knob near the blocker.
  - Cons: crosses the instruction boundary, duplicates host manuals across
    skills, and still leaves the env catalog incomplete.
- Selected: Option A. The user asked for the optimal design after a thorough
  investigation, and the evidence shows the env catalog is already the intended
  discovery authority. A concise doc can explain host integration, but the
  catalog/output must carry the structured contract.

## Unknowns

- Resolved: Which env vars have hidden accepted values, fallback semantics, or
  behavior mappings? -> See the env contract audit table above. Host capability
  env vars are the highest-impact hidden contract; context and GitHub vars also
  merit structured catalog detail.
- Resolved: Which public surface should own host-integration wiring? ->
  `EnvCatalog()` / `slipway config list --env` should own the machine-readable
  contract; docs should explain integration; per-stage skills should remain
  workflow instructions.
- Resolved: What schema/output design is best? -> Optional additive fields on
  `EnvCatalogEntry`: value syntax, accepted values with behavior descriptions,
  examples, and unset behavior. Text output should expose the same contract
  without removing existing columns.
- Remaining: None.

## Assumptions

- Existing runtime behavior is correct and should not change. Evidence:
  `internal/engine/capability/resolver.go`, `cmd/context_pressure_hook.go`,
  `cmd/next_context_budget.go`, `cmd/tool_github.go`.
- Additive JSON fields are acceptable for public command output. Evidence:
  existing `ConfigCatalogEntry` and `EnvCatalogEntry` structs use `omitempty`
  optional fields, and current tests assert required fields without forbidding
  additional fields.
- English docs are sufficient for this change's primary host-integration
  closure; localized command summaries can remain general unless the repo has a
  generation path for translated prose. Evidence: issue #395 asks for a public
  host-facing surface and acceptance is centered on `config list --env`.

## Canonical References

- `internal/model/env_catalog.go`
- `internal/model/env_catalog_test.go`
- `cmd/config.go`
- `cmd/config_test.go`
- `internal/engine/capability/resolver.go`
- `internal/engine/capability/resolver_test.go`
- `cmd/next.go`
- `cmd/context_pressure_hook.go`
- `cmd/next_context_budget.go`
- `cmd/tool_github.go`
- `internal/state/handoff.go`
- `README.md`
- `docs/reference/subagents.md`
