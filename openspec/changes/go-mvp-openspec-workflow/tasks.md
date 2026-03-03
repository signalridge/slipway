Task policy:
- task IDs are stable within this simplified MVP plan
- this file intentionally replaces prior over-designed governance task set
- completion standard is: behavior + tests + `openspec validate`

## 1. Bootstrap

- [ ] 1.1 Wire CLI entrypoint (`main.go`, `cmd/root.go`) for 11 commands
- [ ] 1.2 Add core dependencies (`cobra`, `yaml.v3`, `gofrs/flock`, DAG helper, `testify`, `uuid`)
- [ ] 1.3 Add deterministic error envelope helpers and exit-code mapper (`2/3/4/5/6`)

## 2. Core Models (`internal/model/`)

- [ ] 2.1 Implement score model (`N/A/I/R/V`) + derived methods (`discovery/control`)
- [ ] 2.2 Implement `AdmissionState` model (request metadata, intake assessment, route snapshot, state/action history)
- [ ] 2.3 Implement `ChangeState` model (slug, artifacts, gates, worktree metadata, state/action history)
- [ ] 2.4 Implement `RunRecord` ledger model (`checks[]`, `human_confirmations[]`, wave summaries, override events; no authoritative lifecycle state mirror)
- [ ] 2.7 Add `RunRecord.history[]` append-only event stream model and persistence helpers
- [ ] 2.8 Normalize check record identity to `check_id` (type inferred by `checks[]` vs `human_confirmations[]`)
- [ ] 2.9 Implement `RunRecord` frozen-summary contract (`wave_summaries[]` append-only + `latest_summary_version` pointer)
- [ ] 2.10 Implement command-check override trace fields (`override`, `override_note`, `override_at`) in `RunRecord.checks[]`
- [ ] 2.5 Implement config model (`defaults.level_mode`, execution knobs, unknown top-level key preservation)
- [ ] 2.6 Implement typed enums and validators (`Level`, statuses, gate status, verdicts)

## 3. Filesystem & State (`internal/fsutil/`, `internal/state/`)

- [ ] 3.1 Implement project-root and required path detection for `.speclane/`
- [ ] 3.2 Implement atomic write utility (`temp -> fsync -> rename -> fsync dir`)
- [ ] 3.3 Implement mutation lock utility (`.speclane/state.lock`) with timeout handling
- [ ] 3.4 Implement stale-lock cleanup path in `speclane repair` only
- [ ] 3.5 Implement `Load/SaveAdmission` for `.speclane/runtime/admissions/<request_id>.yaml`
- [ ] 3.6 Implement `Load/SaveChange` for `.speclane/runtime/changes/<request_id>.yaml`
- [ ] 3.7 Implement `Load/SaveRunRecord` for `.speclane/runs/<request_id>.yaml`
- [ ] 3.8 Implement active-request resolver (exactly one active request for scoped commands)
- [ ] 3.9 Implement archive migration helpers for done/cancel (direct vs governed, including run-record archive path)
- [ ] 3.10 Enforce no runtime dependency on `.speclane/evidence/**` directories and no creation of those paths

## 4. Artifact Lifecycle (`internal/engine/artifact/`)

- [ ] 4.1 Scaffold governed bundle for L2/L3 only (`change.yaml`, `proposal.md`, `spec.md`, `design.md`, `tasks.md`, `assurance.md`)
- [ ] 4.2 Add L3-only `explore.md` scaffold and required section checks
- [ ] 4.3 Implement artifact staleness propagation DAG (`proposal -> spec -> design -> tasks -> assurance`, `explore -> design`)
- [ ] 4.4 Implement archive freeze + move (`aircraft/changes/<slug>/` -> `aircraft/changes/archived/<slug>/`)
- [ ] 4.5 Implement `assurance.md` ownership timing (`S7` updates scope/evidence/risks, `S8` updates verdict/archive-decision)

## 5. Routing Engine (`internal/engine/router/`)

- [ ] 5.1 Implement executable vs non-executable intake classification (`non_speclane` boundary)
- [ ] 5.2 Implement guardrail domain detection + canonicalization + risk floor
- [ ] 5.3 Implement auto-level routing (`L1|L2|L3`) and fixed-level path with conflict blocking
- [ ] 5.4 Persist route snapshot + intake assessment in admission state
- [ ] 5.5 Keep derived contract metadata read-time only (`required_artifacts/gates/checks`)
- [ ] 5.6 Implement omitted `--level` behavior split for interactive vs non-interactive mode using `defaults.level_mode`
- [ ] 5.7 Implement deterministic `new_project` / `major_refactor` signal derivation and auto-route integration

## 6. Action Workflow (`internal/engine/action/`)

- [ ] 6.1 Implement canonical states (`S0..S8`, `DONE`) and transition graph
- [ ] 6.2 Implement level paths (`L1: S0,S1,S6,S7,S8`; `L2: S0,S1,S4..S8`; `L3: S0,S1,S2..S8`)
- [ ] 6.3 Implement `speclane new` landing states (`L1->S6`, `L2->S4`, `L3->S2`)
- [ ] 6.4 Implement remediation loops (`S5->S4`, `S7->S6`, `S8->S6`, pivot analyze-first)
- [ ] 6.5 Implement L1 `S7/S8` behavior contract (summary-based lightweight review, lightweight verify, explicit `done` gate)

## 7. Gate + Check Engine (`internal/engine/gate/`)

- [ ] 7.1 Implement gates: `G_scope`, `G_plan`, `G_pivot`, `G_ship`
- [ ] 7.2 Implement check types: `command_check` and `human_confirmation`
- [ ] 7.3 Implement `G_scope` checks (`explore` sections, worktree authenticity) + `scope_confirmed`
- [ ] 7.4 Implement `G_plan` checks (`plan_artifacts_ready`, `openspec_validate_pass`) + `execute_ready`
- [ ] 7.5 Implement `G_ship` baseline check IDs (`tests_pass`, `lint_pass`, `tasks_all_checked`) + `review_done` + `ship_ready`
- [ ] 7.9 Implement deterministic gate-check catalog + gate-to-check mapping contract as single source for gate engine
- [ ] 7.6 Keep MVP `G_ship` fixed-baseline only (defer domain-specific extra ship checks to post-MVP)
- [ ] 7.7 Implement failed-check default block + explicit user override flow (`override=true`, note persisted in run record)
- [ ] 7.8 Remove gate dependency on governance-skill evidence/session_id comparators
- [ ] 7.10 Implement `G_pivot` as rule gate in MVP (entry-state/kind validity/analyze-first), without catalog check IDs

## 8. Wave / Review / Context (`internal/engine/wave`, `review`, `context`)

- [ ] 8.1 Implement DAG wave builder (topological layers + conflict split)
- [ ] 8.2 Implement L1 synthetic task normalization from direct brief
- [ ] 8.10 Implement `task_kind=other` isolation + manual-checkpoint execution behavior
- [ ] 8.12 Document and enforce `task_kind` semantics (`code|test|doc|ops` are reporting labels; only `other` changes scheduling)
- [ ] 8.3 Implement in-wave execution (parallel/sequential by config)
- [ ] 8.4 Implement post-wave changed-files overlap detection (`post_wave_file_conflict`)
- [ ] 8.5 Implement non-pass loop (`retry`, `skip`, `abort_wave`, `pivot`)
- [ ] 8.11 Implement retry guard enforcement (`max_retries_per_task`, default 2)
- [ ] 8.6 Implement checkpoint pause/resume with response persistence
- [ ] 8.7 Implement frozen wave summary snapshots in run record (`wave_summaries[]` append-only, `summary_version` monotonic, `latest_summary_version` pointer update)
- [ ] 8.8 Implement review engine to consume latest frozen summary from run record
- [ ] 8.13 Implement review entry preconditions (`S6/S8` require frozen summary and state constraints)
- [ ] 8.9 Implement compact context pack (intent/scope/blockers/next action/checks/confirmations)

## 9. CLI Commands (`cmd/`)

- [ ] 9.1 `init`: create minimal runtime layout (`.speclane/runs/` included, no evidence dirs)
- [ ] 9.2 `new`: intake/analyze/route/persist landing state + governed scaffold when needed
- [ ] 9.11 `new`: `non_speclane` returns successful classification contract (exit `0`, parse-stable payload, no runtime writes)
- [ ] 9.3 `do`: execute exactly one next action per invocation (including L1; no same-invocation `S6->S7/S8` auto-chain) and include checkpoint handling
- [ ] 9.4 `status`: default JSON, diagnostics mode for `0` or `>1` active requests
- [ ] 9.5 `context`: compact view with `text|yaml|json` formats
- [ ] 9.6 `done`: strict finalizer with checks/confirmations and optional explicit override
- [ ] 9.7 `cancel`: terminal cancel + in-flight preemption + archive (including run record)
- [ ] 9.8 `pivot`: analyze-first reroute/rescope semantics
- [ ] 9.9 `repair`: stale lock cleanup, malformed config recovery, partial archive repair
- [ ] 9.10 `analyze` and `review`: explicit override commands with strict preconditions

## 10. Tool Adapters (Optional Sidecar)

- [ ] 10.1 Keep four-target adapter registry (`claude/cursor/codex/opencode`)
- [ ] 10.2 Generate concise command wrappers only (no embedded governance policy)
- [ ] 10.3 Keep helper guides advisory-only and non-blocking
- [ ] 10.4 Ensure deterministic byte-identical generation on repeat runs
- [ ] 10.5 Enforce canonical CLI command name `speclane` in adapter wrappers (AI triggers route to canonical CLI)
- [ ] 10.6 Enforce `spl` short-name trigger family with tool-distinct structures:
  - claude `/spl:<command>`
  - cursor `/spl.<command>`
  - codex `/prompts:spl-<command>`
  - opencode `/spl-<command>`
- [ ] 10.7 Use English canonical confirmation prompt templates in generated artifacts; runtime localization remains AI-layer concern

## 11. Unit Tests

- [ ] 11.1 Routing tests: executable boundary + auto/fixed-level outcomes
- [ ] 11.10 Routing mode tests: omitted `--level` interactive/non-interactive behavior with config fallback
- [ ] 11.11 Routing signal tests: `new_project` / `major_refactor` derivation fixtures
- [ ] 11.2 State tests: admission/change/run-record round-trip + lock timeout + atomic writes
- [ ] 11.3 Gate tests: `G_scope/G_plan/G_ship/G_pivot` pass/block paths
- [ ] 11.4 Override tests: failed command check + explicit override continuation trace
- [ ] 11.12 RunRecord schema tests: `history[]` append behavior, ledger-only ownership, and `check_id` identity contracts
- [ ] 11.16 Override persistence tests: overridden command check stores `override=true`, optional `override_note`, and `override_at`
- [ ] 11.5 Wave tests: DAG layering, conflict split, overlap downgrade, retry loop
- [ ] 11.13 `task_kind=other` tests: isolated wave + manual checkpoint before pass
- [ ] 11.14 Retry guard tests: retries exhausted path blocks additional retry
- [ ] 11.6 Review/context tests: frozen-summary consumption + diagnostics payloads
- [ ] 11.7 CLI error-envelope and exit-code taxonomy tests
- [ ] 11.8 Archive tests: direct vs governed done/cancel migration and freeze behavior
- [ ] 11.9 Config-repair tests: malformed config backup + default rewrite
- [ ] 11.15 `do` single-step tests: L1 does not auto-chain `S6->S7/S8` in one invocation
- [ ] 11.17 Tool adapter trigger tests: all four tools use `spl` short-name family with distinct per-tool syntax (including Codex `/prompts:spl-*`)
- [ ] 11.18 Prompt-template tests: canonical prompts are English while runtime keeps check-id semantics stable under localization
- [ ] 11.19 `non_speclane` classification tests: `speclane new` exits `0` with parse-stable classification payload
- [ ] 11.20 Review precondition tests: `review` from `S6/S8` enforces frozen-summary/state preconditions
- [ ] 11.21 Assurance ownership tests: `S7` and `S8` update the required assurance sections

## 12. Integration Tests

- [ ] 12.1 `non_speclane` request creates no runtime/admission/change/run files
- [ ] 12.2 L1 end-to-end: `new -> do -> done` using admission + run record only
- [ ] 12.3 L2 end-to-end: governed scaffold + `G_plan/G_ship` command checks + confirmations
- [ ] 12.4 L3 end-to-end: discover/scope + `G_scope` + governed run to done
- [ ] 12.9 Routing behavior integration: omitted `--level` mode split and `new_project/major_refactor`-driven L3 routing
- [ ] 12.5 Failed check override flow: user approves continuation and done succeeds with override trace
- [ ] 12.6 Cancel in-flight wave: graceful-stop then force-kill then archive
- [ ] 12.10 Terminal archive integration: run record moves to `.speclane/archive/runs/<request_id>.yaml` on done/cancel
- [ ] 12.7 `status/context` diagnostics behavior under zero/multi-active contexts
- [ ] 12.8 `openspec validate` and `openspec validate --strict` pass after updates

## 13. Verification Commands

- [ ] 13.1 `openspec validate go-mvp-openspec-workflow`
- [ ] 13.2 `openspec validate --strict go-mvp-openspec-workflow`
- [ ] 13.3 `openspec status --change go-mvp-openspec-workflow`
- [ ] 13.4 Repo grep sweep confirms no runtime contract dependency on `.speclane/evidence/skills/`
