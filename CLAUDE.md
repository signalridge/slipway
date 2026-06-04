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

Only **omitted** fields safe-degrade. An **explicit but invalid** classification
is rejected (exit 2, `error_code=invalid_classification`) with an actionable
remediation listing the valid set and a nearest-match suggestion, rather than
being silently dropped — this is fail-closed for the guardrail signal. Invalid
means: an unrecognized `guardrail_domain` or `complexity` token, or a non-empty
`guardrail_domain` paired with `needs_discovery=false` or a `complexity` below
`complex`. `slipway new --help` lists the accepted tokens. (A flaky *inferred*
classification, by contrast, still safe-degrades.)

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
- `slipway run --json` — advance to next step and return the handoff-only JSON surface (the only state-mutating execution surface); when S3/S4 planning evidence is stale, `run` can reopen S1 plan-audit, clear stale planning/downstream verification evidence, and preserve runtime execution evidence for ordered refresh
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

Codebase maps under `artifacts/codebase/` are git-tracked by default; durable
brownfield context is reviewed and shared, not hidden as local-only state.
Existing repositories auto-migrate the next time `slipway new`,
`slipway codebase-map`, or `slipway init` rewrites the managed `.gitignore`
block. `evidence/`, `events/`, `verification/`, and `.worktrees/` stay ignored.

`verification/evidence-digests.yaml` is engine-owned local state, not host skill
evidence. Slipway writes it when mutating advancement accepts passing skill
evidence, then read-only commands compare the stored input digests with current
content. Each digest entry records the accepted verification verdict timestamp
so a newer re-run verdict can replace a stale digest during mutating
advancement; that timestamp is not the freshness signal. Do not hand-edit
`verification/<skill>.yaml` to add digest fields. Legacy changes without the
digest file are backfilled only after the verdict timestamp safety gate passes;
otherwise the skill must be re-run. Diff-class review digests certify the
current working diff (`git diff HEAD` plus non-ignored untracked reviewable
files, excluding Slipway governed/runtime artifacts under
`artifacts/changes/**`), so committing reviewed changes before finalization can
make read-only commands report the review stale until mutating advancement
restamps or the review is run again at the new diff boundary.

`wave-plan.yaml` `generated_at` is display/audit materialization time. Planning
freshness keys on semantic `tasks_plan_hash`, and runtime task evidence records
that hash in `freshness_inputs` so old task evidence is not reused after a real
`tasks.md` plan change.

`next --json` and `run --json` carry `input_context.codebase_map_status` (and
per-doc `input_context.codebase_map_doc_states`) in the default compact handoff,
without `--diagnostics`. Values mirror the `codebase-map` assessment (`missing`,
`scaffold_only`, `baseline`, `partial`, `populated`); a missing map reports
`"missing"` with per-doc `missing` states rather than an omitted field. Treat
`scaffold_only`/`baseline` maps as non-durable. When research-orchestration or
plan-audit is the next skill and the status is `scaffold_only` or `baseline`,
`warnings` carries a non-blocking consume-time codebase-map advisory. For
discovery-scoped changes (`needs_discovery`), a `missing` map at those same
planning skills also carries a non-blocking discovery advisory routing the host
to the `slipway-codebase-mapping` skill; both advisories share the
`codebase_map_advisory:` prefix, at most one fires, and neither blocks
progression. Non-discovery changes never receive the discovery advisory. Inspect
`codebase_map_doc_states` for `partial` maps, which get no whole-map advisory.
