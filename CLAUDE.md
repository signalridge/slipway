# Slipway

Governance CLI for AI-assisted software delivery.

## Build & Test

```bash
go build ./...
go test ./...
```

## Creating a Governed Change

`slipway new` accepts intent classification via JSON stdin when `--json` is set.
This lets the AI caller provide classification directly instead of Slipway inferring it.

```bash
echo '{"description":"add OAuth login flow","guardrail_domain":"auth_authz","needs_discovery":true,"complexity":"critical","test_cmd":"go test ./...","build_cmd":"go build ./...","languages":["Go"]}' | slipway new --json
```

Positional arg for description also works alongside stdin classification:

```bash
echo '{"guardrail_domain":"","needs_discovery":false,"complexity":"simple"}' | slipway new --json "fix typo in readme"
```

Without classification fields or without stdin, safe fallback applies:
`guardrail_domain=""`, `needs_discovery=true`, `complexity="complex"`.

### `guardrail_domain` values

Classify **intent**, not keyword mention. If the change itself modifies the
behavior described below, use that domain. If text merely mentions the topic
(docs, error messages, test fixtures), use empty string.

| Value | When to use |
|---|---|
| `""` (empty) | No sensitive domain applies |
| `auth_authz` | Modifies authentication, authorization, sessions, RBAC |
| `security_credentials` | Handles credentials, tokens, secrets, key rotation |
| `privacy_pii` | Handles personal data, privacy boundaries, redaction |
| `financial_flows` | Alters payment, billing, ledger, money movement |
| `schema_data_migration` | Alters schema shape, migration behavior |
| `irreversible_operations` | Introduces hard deletes, permanent purge, destructive ops |
| `external_api_contracts` | Alters externally consumed API or webhook contracts |

### `complexity` values

| Value | When to use |
|---|---|
| `trivial` | Tiny and low-risk (typo, one-liner) |
| `simple` | Bounded implementation, low coordination |
| `complex` | Multi-step, ambiguous, or coordination-heavy |
| `critical` | High-risk, severe impact if wrong |

### `needs_discovery`

Set `true` when the codebase is unfamiliar, the scope is uncertain, or the
change requires investigation before planning. When `guardrail_domain` is
non-empty, prefer `true`.

### Project context fields (optional)

Provide caller-owned project metadata for `slipway new --json`. Omitted fields
remain empty; Slipway does not auto-infer missing project context on JSON
surfaces.

| Field | Type | Description |
|---|---|---|
| `tech_stack` | string | e.g. `"Go"`, `"Go, Node.js"` |
| `test_cmd` | string | e.g. `"go test ./..."` |
| `build_cmd` | string | e.g. `"go build ./..."` |
| `languages` | string[] | e.g. `["Go", "TypeScript"]` |
| `recent_work` | string | Recent commit summary |

### Control overrides (optional)

| Field | Type | Description |
|---|---|---|
| `disabled_controls` | string[] | Controls to disable for this change |

Available control IDs: `clarification`, `research`, `domain_review`,
`independent_review`, `worktree_isolation`, `rollback_required`.

Guardrail-domain protections (`domain_review`, `rollback_required` for
sensitive domains) are fail-closed and cannot be disabled via this field.

## Routed Commands

All routed commands support `--json` for structured output:

- `slipway review --json` — review current change
- `slipway validate --json` — validate governance state
- `slipway repair --json` — repair local integrity issues
- `slipway codebase-map --json` — create or refresh advisory repo-scoped context under `artifacts/codebase`
- `slipway next --json` — query next step as a handoff-only JSON surface (read-only, does not advance state)
- `slipway next --json --no-auto-pass` — query next step, reporting `auto_pass_eligible` instead of auto-passing
- `slipway next --json --diagnostics` — include governance/readiness diagnostics in the next-step view
- `slipway run --json` — advance to next step and return the handoff-only JSON surface (the only state-mutating execution surface)
- `slipway run --json --diagnostics` — include transition traces and governance/readiness diagnostics in the returned run view
- `slipway status --json` — check current state

JSON output from `next` includes `next_skill.name` as the governed host
handoff. The caller derives its own SKILL.md path using local tool
conventions such as `.claude/skills/slipway-{name}/SKILL.md`. Default `next`
JSON is intentionally compact; use `--diagnostics` for gate status, artifact
status, skill evidence, context-budget details, wave plans, and transition
traces.

`codebase-map --json` can report `status: "baseline"` with `baseline_docs`
when documents contain only CLI-detected repository facts. Treat baseline docs
as a starting point awaiting source-backed verification, not as completed
brownfield analysis.
