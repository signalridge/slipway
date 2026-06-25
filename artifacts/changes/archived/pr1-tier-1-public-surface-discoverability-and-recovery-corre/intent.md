# Intent

## Summary
PR1 / Tier 1 — public-surface discoverability and recovery correctness, bundling
two issues into one PR.

**#315** (suggestion): `.slipway.yaml` config keys are not discoverable — only 1 of
~26 strict-decoded keys is named anywhere in CLI help. Add a `slipway config`
command (list/get/set) whose key catalog is derived from the
`internal/model/config.go` struct so it cannot drift (contract-tested, mirroring
the `--list-focuses` pattern); surface 3 behavior-affecting env vars in the
relevant `--help`; add a back-pointer from `run --help`; and warn (not silently
swallow) on unknown top-level config keys.

**#324** (confirmed bug): in S2_IMPLEMENT, stale wave-orchestration evidence makes
`slipway run` recommend `slipway fix`, but `fix` only runs in S3_REVIEW, so it
returns `fix_state_invalid`. The recovery must instead point to a state-valid S2
command/remediation.

## Complexity Assessment
complex
<!-- Rationale -->
Two issues shipped as one PR. #315 adds a new `slipway config` command with three
subcommands (list/get/set), a struct-derived + contract-tested key catalog, a
mutation surface (`set` validates and persists `.slipway.yaml`), env-var help
surfacing across 3 sites, a `run --help` back-pointer, and a load/save behavior
change (warn on unknown top-level keys). #324 corrects subtle state-dependent
recovery routing. Multiple files across `cmd/` and `internal/`.

## Guardrail Domains
<!-- none detected -->
None. `config set` writes the local, user-owned, fully reversible `.slipway.yaml`
under write-validation; it is not Auth/Credentials/Financial/Schema/Irreversible/
External-API. The #324 fix stays fail-closed (corrects the recommended command
only; does not weaken or bypass the stale-evidence gate or S3 review).

## In Scope
**#315 — config discoverability**
- NEW `cmd/config.go`:
  - `slipway config` / `slipway config list [--json]` — enumerate every
    `.slipway.yaml` key: `name · type · default · allowed-values · scope`.
  - `slipway config get <key> [--json]` — print the resolved effective value
    (merged `.slipway.yaml` + defaults).
  - `slipway config set <key> <value>` — validate via the same strict decode and
    persist to `.slipway.yaml`.
- Key catalog derived from the `internal/model/config.go` config struct and
  contract-tested so adding a struct field without a catalog entry fails CI
  (mirror the existing `--list-focuses` discoverability pattern).
- Surface 3 behavior-affecting env vars in the relevant `--help`:
  - `SLIPWAY_GITHUB_API_URL` (`slipway tool ... --backend api`, `cmd/tool_github.go`)
  - `SLIPWAY_CONTEXT_WINDOW_TOKENS` (`slipway next` context budget, `cmd/next_context_budget.go` / `cmd/context_pressure_hook.go`)
  - `SLIPWAY_SESSION_OWNER` (handoff attribution, `internal/state/handoff.go`)
- Back-pointer from `slipway run --help` to the `slipway config` surface.
- Warn (not silently swallow) on unknown **top-level** config keys in
  `internal/model/config.go` (sub-finding A) — currently stored in
  `UnknownTopLevel` and round-tripped on save with no signal.

**#324 — recovery routing**
- Fix the S2_IMPLEMENT stale wave-orchestration evidence recovery so the
  recommended command is runnable in S2 (NOT `slipway fix`, which is S3-only).
  Touch the recovery routing (`internal/model/recovery.go`) and/or the wave
  stale-evidence emit (`internal/engine/progression/wave_sync.go`) so a stale
  wave-orchestration artifact in S2 routes to a state-valid command/remediation.
- Regression test asserting the S2 stale-wave-evidence recovery is state-valid.

## Out of Scope
- **#315's R1–R4 items** (a different defect class — recovery-routing/next-command
  gaps; #315 itself says R1 likely warrants its own issue). In particular R1 ("six
  durable wave safety-net codes have NO `blockerRemediations` entry") is distinct
  from #324, which is a *state-invalid* remediation (`fix` recommended in S2), not
  a *missing* one. The broad R1 six-code sweep, R2 (review verdict has no recovery
  object), R3 (`change_is_done` info-blocker), and R4 (`--focus` → `--list-focuses`
  pointer) are their own issues.
- A generated `docs/reference/config.md` as an *alternative* to the command
  (#315 said "and/or"); we ship the command. A generated doc can follow later.
- Any change to what config keys *do* (their semantics/behavior). This change is
  purely surfacing, discoverability, and recovery-recommendation correctness.

## Constraints
- Catalog MUST derive from the struct so it cannot drift, guarded by a contract
  test — not a hand-maintained list that silently re-rots (the exact failure #315
  reports). (Reflection-walk vs. contract-tested explicit table is a plan-stage
  design decision.)
- `config set` MUST validate with the same strict decode (`KnownFields(true)`),
  reject invalid values with a clear error, and not corrupt or drop existing
  `.slipway.yaml` content on write.
- #324 fix MUST stay fail-closed: correct only the recommended command for the S2
  stale-evidence state; do not add a bypass, weaken the gate, or skip S3 review.
- Follow repo conventions: cobra command registration, existing `--json` output
  shape, `BuildRecovery` contract, and the recovery contract-test pattern.

## Acceptance Signals
1. `slipway config list` (and `--json`) enumerates every `.slipway.yaml` key with
   type · default · allowed-values · scope; a contract test asserts every
   `internal/model/config.go` struct field has a catalog entry (a new field with
   no entry fails the test).
2. `slipway config get execution.auto` (and `--json`) prints the resolved
   effective value (merged `.slipway.yaml` + defaults).
3. `slipway config set execution.auto true` writes `.slipway.yaml`, the value
   round-trips via `config get`, and an invalid `set` (e.g. `execution.auto nope`)
   is rejected with a clear error and no file corruption.
4. All three env vars are discoverable from `--help`: `SLIPWAY_GITHUB_API_URL`,
   `SLIPWAY_CONTEXT_WINDOW_TOKENS`, `SLIPWAY_SESSION_OWNER`.
5. `slipway run --help` references the `slipway config` surface.
6. Loading a `.slipway.yaml` with an unknown top-level key emits a warning rather
   than silently swallowing it.
7. **#324**: with stale wave-orchestration evidence in S2_IMPLEMENT,
   `slipway run --json` / `status --json` / `next --json` `recovery.primary_command`
   is runnable in S2 and does NOT recommend `slipway fix`; a regression test covers it.
8. Full `go test ./...` is green; `gofmt -s` and golangci-lint are clean.

## Open Questions
None requiring a research stage — the two issues provide exact source maps (line
references), so the remaining choices (catalog reflection-vs-table,
`config set` write-path comment preservation, and the precise #324 emit-vs-route
fix) are plan-stage design decisions resolvable from the mapped source, not
discovery unknowns.

## Deferred Ideas
- `config set` advanced UX (comment/format-preserving YAML round-trip, `--unset`,
  array/map key editing) beyond basic validated scalar/section set.
- Generated `docs/reference/config.md` reference page derived from the same catalog.
- The full R1 six-code `blockerRemediations` sweep and R2–R4 (separate issues).

## Approved Summary
<!-- User-confirmed final summary + confirmation timestamp -->
Confirmed by user on 2026-06-24 (intake hard-gate, fresh confirmation).

PR1 / Tier 1 bundles two issues into one PR — public-surface discoverability
(#315) and a recovery-routing correctness fix (#324).

**What it does**
- **#315 — config discoverability.** Add a `slipway config` command with three
  subcommands: `list [--json]` (enumerate every `.slipway.yaml` key with
  name · type · default · allowed-values · scope), `get <key> [--json]` (print the
  resolved effective value), and `set <key> <value>` (validate via the same strict
  `KnownFields(true)` decode, then persist to `.slipway.yaml`). The key catalog is
  DERIVED from the `internal/model/config.go` struct and contract-tested so a new
  struct field without a catalog entry fails CI (mirroring `--list-focuses`).
  Surface three behavior-affecting env vars in the relevant `--help`
  (`SLIPWAY_GITHUB_API_URL`, `SLIPWAY_CONTEXT_WINDOW_TOKENS`,
  `SLIPWAY_SESSION_OWNER`), add a `run --help` back-pointer, and warn (not silently
  swallow) on unknown top-level config keys.
- **#324 — recovery routing.** In S2_IMPLEMENT, stale wave-orchestration evidence
  makes `slipway run` recommend the S3-only `slipway fix` (→ `fix_state_invalid`).
  Correct the recovery so the recommended command is runnable in S2. Stays
  fail-closed: only the recommended command changes; the stale-evidence gate and S3
  review are not weakened or bypassed.

**Scope boundaries**
- IN: `cmd/config.go` (new), struct-derived contract-tested catalog, `config set`
  write path, 3 env vars in `--help`, `run --help` back-pointer, unknown-top-level
  warn, and the #324 S2 recovery fix + regression test.
- OUT: #315's R1–R4 (separate issues; R1 ≠ #324), config-key semantics, and a
  generated `docs/reference/config.md` (the command ships instead).

**Primary acceptance signal**
`slipway config list/get/set` works end to end (invalid `set` rejected with no file
corruption); a contract test asserts catalog↔struct parity; all three env vars are
discoverable from `--help`; with stale S2 wave evidence the recovery
`primary_command` is runnable in S2 and is not `slipway fix` (regression-tested);
`go test ./...` green with `gofmt -s` and golangci-lint clean.
