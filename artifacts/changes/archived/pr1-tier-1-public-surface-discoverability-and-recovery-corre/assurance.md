# Assurance

## Scope Summary
PR1 (Tier 1) bundles two public-surface improvements as one change:

- **#315 — config discoverability (read-write).** A new `slipway config`
  command (`list`/`get`/`set`) backed by a `.slipway.yaml` key catalog that is
  *derived by reflection* from the `internal/model` `Config` struct (not a
  hand-maintained list), plus a contract test that fails CI when a strict-decoded
  struct field has no catalog entry. Discoverability is rounded out by surfacing
  three behavior-affecting env vars in `--help`, a `run --help` back-pointer to
  `config`, and a load-time warning that names unknown top-level `.slipway.yaml`
  keys instead of silently swallowing them.
- **#324 — S2 recovery routing.** When wave-orchestration evidence is stale in
  `S2_IMPLEMENT`, recovery previously recommended the S3-only `slipway fix`
  (→ `fix_state_invalid`). The early `staleEvidenceRepairTarget` site now mirrors
  the S2 guard the sensitive-evidence site already uses, so recovery recommends
  the state-valid `slipway run`. The fix is fail-closed: only the recommended
  command changes; the stale gate and S3 review are untouched.
- **Round-trip correctness fix folded into `config set` (REQ-004).** A bug was
  found in `internal/model/config.go` `Config.ToYAML`: it emitted `governance`
  only when certain keys were set and `context` from a hand-maintained field
  list, so an isolated `governance.auto_provision_worktree` (including an
  explicit `false`) or a `context.recent_work` value was silently dropped on
  save — exactly the corruption `config set` must not cause. `ToYAML` now emits
  `governance` whenever `AutoProvisionWorktree != nil` and emits `context`
  whenever `!Context.IsZero()` (reusing the single empty-context authority rather
  than a drifting field list). RED→GREEN round-trip tests were added
  (`internal/model/config_test.go`, `cmd/config_test.go`).

Delivered across 4 tasks (t-01..t-04) plus the R3 ToYAML correctness fold, in
dependency-ordered waves. 15 files changed; scope contract `pass` (changed files
== planned targets). `config` is registered via `makeConfigCmd()` in
`newRootCmd()` (`cmd/root.go`) and listed in the root help groups, so it is
discoverable from `slipway --help`.

## Verification Verdict
Overall verdict: **pass**. All 4 planned tasks recorded passing task
evidence at `run_summary_version=1`; the wave-orchestration skill evidence was
recorded last. Per-wave changed-file scope audit was clean (no parallel overlap,
no scope escape). The cross-package contract test
`TestReasonAndErrorContractTestsDoNotTextMatchMessageProse` initially caught a
t-03 violation (asserting on `CLIError.Message` prose); it was corrected by
carrying the offending key in stable `CLIError.Details` and asserting that
stable field, then re-verified green.

The R3 consolidation round folded a `Config.ToYAML` correctness fix (no longer
drops isolated `governance.auto_provision_worktree` or `context.recent_work` on
save) directly into PR1, since it is the corruption mode REQ-004's `config set`
must avoid. This stage's terminal ship-verification re-derived the full
authoritative suite and lint on a fresh checkout: `go test ./...` reports all
packages with tests `ok` / 0 FAIL, and `golangci-lint run ./...` reports
`0 issues.` (full combined output captured at
`verification/logs/ship-suite.txt`). All four adversarial S3 review peers are
certified `pass` at the current digest with distinct review handles.

## Evidence Index
- `verification/execution-summary.yaml` — per-task verdicts (t-01..t-04 all
  `pass`), changed/target files, `tasks_plan_hash`
  `31ef9875…aaade790`, `run_summary_version=1`.
- `verification/wave-orchestration.yaml` + `-notes.md` — wave plan (W1 parallel
  t-01∥t-02; W2 t-03; W3 t-04), dispatch handles, per-wave scope audit and
  integration gates.
- `verification/plan-audit.yaml` + `-notes.md` — G_plan audit (PASS) with
  distinct author/auditor context-origin handles.
- `verification/intake-clarification.yaml` — locked scope (config `set` included;
  bundle #315+#324 as one PR).
- S3 review evidence (this stage), all `verdict: pass`, `run_version: 1`, with
  distinct `context_origin:stage=review` handles and a shared
  `context_origin:stage=fix=main-session-pr1-r3-consolidation`:
  - `verification/spec-compliance-review.yaml` — `subagent-pr1-spec-compliance-v3`
  - `verification/code-quality-review.yaml` — `subagent-pr1-code-quality-v3`
  - `verification/independent-review.yaml` — `subagent-pr1-independent-v3`
  - `verification/security-review.yaml` — `subagent-pr1-security-v3`
  - `verification/ship-verification.yaml` — terminal gate, handle
    `subagent-pr1-ship-verify-v2` (this round; supersedes the prior
    `subagent-pr1-ship-verify` cert that went stale when the ToYAML fix landed).
- `verification/logs/ship-suite.txt` — terminal ship-verification authoritative
  full `go test ./...` + `golangci-lint run ./...` combined output (this round).
- ToYAML round-trip regression coverage: `internal/model/config_test.go` and
  `cmd/config_test.go` (set isolated `governance.auto_provision_worktree=false`
  / `context.recent_work` then reload and assert the value survives).

## Requirement Coverage
| Requirement | Covered by | Verifying evidence |
|---|---|---|
| REQ-001 config list (name·type·default·allowed·scope, `--json`, bare==list) | t-01, t-03 | `go test ./internal/model/...`, `./cmd/...` |
| REQ-002 catalog derived from struct + contract test | t-01 | catalog completeness/parity contract test |
| REQ-003 `config get` resolved value, unknown→error+non-zero | t-03 | `cmd/config_test.go` get round-trip + unknown-key |
| REQ-004 `config set` strict-validate, atomic persist, invalid→no corruption | t-03 + R3 ToYAML fix | invalid-set leaves `.slipway.yaml` byte-unchanged test; `Config.ToYAML` no longer drops isolated `governance.auto_provision_worktree`/`context.recent_work` (round-trip tests in `internal/model/config_test.go` + `cmd/config_test.go`) |
| REQ-005 three env vars discoverable in `--help` | t-04 | help `Long` edits in tool_github/next/handoff |
| REQ-006 `run --help` back-pointer to `config` | t-04 | `cmd/run.go` Long |
| REQ-007 unknown top-level key warns (STDERR), still loads | t-04 | `warnUnknownTopLevelConfigKeys` at `loadConfigAtRoot`; manual smoke (STDERR warn, STDOUT `--json` valid) |
| REQ-008 S2 stale-wave recovery recommends state-valid command, not `slipway fix` | t-02 | `advance_governed_test.go` regression: `PrimaryCommand != "slipway fix"`, runnable in S2 |
| REQ-009 suite green + `gofmt -s`/golangci-lint clean | t-01..t-04 | post-wave integration gates; final `go test ./...` at ship-verification |

## Residual Risks and Exceptions
In-scope observations assessed by the S3 review set:

1. `config` is intentionally absent from the toolgen `commandRegistry`, so it has
   no generated command surface/skill and `command_description_contract` does not
   cover it. Its `Short` is sourced from `configShortDescription` in `config.go`
   and surfaced in the root help groups (`cmd/root.go`). Confirmed consistent
   with scope (the change ships a command, not a generated
   `docs/reference/config.md`).
2. Env-var help discoverability (REQ-005) and the unknown-top-level-key warning
   (REQ-007) are verified by the root-help contract test and manual smoke; both
   were re-confirmed by the v3 review peers as acceptable for the change's blast
   radius.
3. `config set` is intentionally scalar-only: slice/map leaves (e.g.
   `governance.controls`, `context.languages`) are rejected by `assignScalar`
   rather than parsed from a string, so the strict-decode contract is never
   bypassed. Round-trip persistence of those collections (when set via file) is
   preserved by `ToYAML`; the R3 fix closed the last drop-on-save gap.

Out-of-scope (filed separately, not fixed here): the plan-audit generated
SKILL.md narrates a HARD-GATE "wait for explicit user confirmation" while the CLI
`confirmation_requirement` reported `required=false` once G_plan was approved —
a surface-vs-CLI tension to reconcile in its own change.

## Rollback Readiness
Low blast radius. #315 is purely additive (a new command + help text + a
non-fatal load warning); no schema, persistence-format, or lifecycle-gate change.
#324 changes only the *recommended recovery command string* for one S2 branch and
is covered by a regression test; it does not alter gate decisions or evidence
state. Rollback = revert the branch; no data migration or state cleanup required.

## Archive Decision
Not yet archivable. Final closeout requires the terminal ship-verification gate
(with `closeout:assurance_complete=pass` and `closeout:reviewer_independence=pass`)
plus an active `slipway validate --json` freshness/readiness proof captured in the
current worktree immediately before `slipway done`. This assurance must not be
read as a revalidation of an archived bundle; the active validate gate is the
authority and will be run live before `done`.
