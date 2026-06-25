# Tasks

## Task List

<!--
Wave shape (engine assigns waves from depends_on + target_files):
- t-01 (pkg internal/model) and t-02 (pkg internal/engine/progression) have no
  deps → dispatched together in wave 1 (distinct packages = concurrent-compile
  safe).
- t-03 (pkg cmd) depends on t-01 (consumes the model catalog API).
- t-04 (pkg cmd) depends on t-03 ONLY to serialize within package `cmd`: two
  subagents editing different files of the same Go package in one parallel wave
  race on `go build`/`go test ./cmd/...`. This is a real executor constraint
  (see dogfooding friction), not narrative ordering.
-->

- [x] `t-01` Derive a `.slipway.yaml` key catalog from the `internal/model` config struct (name · type · default · allowed-values · scope), with a contract test asserting every strict-decoded struct field has a catalog entry so a new field without one fails CI (mirror the `--list-focuses` completeness pattern). Catalog walks the nested `Config` struct via yaml tags (dotted keys e.g. `execution.auto`, `governance.thresholds.independent_review_blast_radius`); allowed-values/scope/description are enriched per-entry; defaults sourced from `DefaultConfig()`. Expose typed get/set helpers (resolve effective value by dotted key; parse+validate a string value for `set`) for the command layer to consume. Also harden the persistence round-trip the catalog's `set` helper relies on: `Config.ToYAML` previously dropped an isolated `governance.auto_provision_worktree` (omitted from the `hasGovernance` predicate) and `context.recent_work` (omitted from a hand-maintained context predicate), so `config set` on either key silently lost the value on save; emit the governance section when `AutoProvisionWorktree != nil` and gate the context section on `!Context.IsZero()`, with round-trip regression tests in config_test.go.
  - depends_on: []
  - target_files: [internal/model/config_catalog.go, internal/model/config_catalog_test.go, internal/model/config.go, internal/model/config_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-009]

- [x] `t-02` Fix #324: in `S2_IMPLEMENT`, a stale-but-passing wave-orchestration authority routes through `staleEvidenceRepairTarget` → `forwardOnlyStaleEvidenceSummary` (advance_governed.go:80-90) which emits `review_alignment_required` → the S3-only `slipway fix` (→ `fix_state_invalid`). Apply the same S2 guard the `sensitiveEvidenceRepairTarget` site already uses (advance_governed.go:230): when `fromState == StateS2Implement`, surface `target.Blockers` (`required_skill_stale:wave-orchestration:*`, whose remediation is the state-valid `slipway run`) via `blockedAdvanceSummary` instead of the review-alignment path. Stay fail-closed: only the recommended command changes; the stale gate and S3 review are untouched. Add a regression test asserting the S2 stale-wave recovery primary_command is runnable in S2 and is not `slipway fix`.
  - depends_on: []
  - target_files: [internal/engine/progression/advance_governed.go, internal/engine/progression/advance_governed_test.go]
  - task_kind: code
  - covers: [REQ-008, REQ-009]

- [x] `t-03` Add the `slipway config` cobra command consuming the t-01 catalog: `config`/`config list [--json]` (enumerate every key with name · type · default · allowed-values · scope), `config get <key> [--json]` (resolved effective value, unknown key → clear error + non-zero exit), `config set <key> <value>` (validate via the catalog's typed setter + `Config.Validate()`/strict decode, persist through `model.SaveConfig` atomic write, reject invalid value with no file corruption). Register on rootCmd (cmd/root.go) and surface it in the root help command group so the public surface is discoverable from `slipway --help`, not only `slipway help config`. Tests cover list/get/set round-trip, invalid-set rejection + file integrity, `--json` shapes, and a root-help assertion that `config` plus its short description appear in the top-level help (root_help_test.go).
  - depends_on: [t-01]
  - target_files: [cmd/config.go, cmd/config_test.go, cmd/root.go, cmd/root_help_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004, REQ-009]

- [x] `t-04` Discoverability help edits in package `cmd`: surface the three behavior-affecting env vars in the relevant command `--help` (Long sections) — `SLIPWAY_GITHUB_API_URL` in the github tool command (tool_github.go), `SLIPWAY_CONTEXT_WINDOW_TOKENS` in `next` (next.go), `SLIPWAY_SESSION_OWNER` in `handoff` (handoff.go); add a `slipway run --help` back-pointer to `slipway config` (run.go); and emit a warning naming unknown top-level keys when config loads, by iterating the already-captured `cfg.UnknownTopLevel` at the `loadConfigAtRoot` boundary (common.go) instead of silently swallowing them. Verify the `command_description_contract` test (Short==desc) still passes (adding Long is safe) and that emitting the load warning does not break stderr-sensitive tests.
  - depends_on: [t-03]
  - target_files: [cmd/next.go, cmd/handoff.go, cmd/tool_github.go, cmd/run.go, cmd/common.go]
  - task_kind: code
  - covers: [REQ-005, REQ-006, REQ-007, REQ-009]
