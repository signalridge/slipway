# Research

## Research Findings

### Architecture
- Affected modules:
  - `cmd/root.go:139-168` wires top-level Cobra commands and root help groups.
  - `internal/toolgen/toolgen.go:143-197` is the adapter command metadata source of truth.
  - `internal/engine/progression/wave_sync.go:21-42` defines the flat task evidence JSON contract consumed by execution-summary sync.
  - `internal/engine/progression/wave_sync.go:68-168` synchronizes wave evidence, task evidence, wave runs, task checkboxes, and `verification/execution-summary.yaml`.
  - `internal/state/store.go:103-108` owns the canonical `.git/slipway/runtime/changes/<slug>/evidence/tasks` directory.
  - `internal/engine/progression/evidence.go:16-127` evaluates required verification records and currently checks run-version binding only.
  - `internal/engine/progression/authority.go:129-198` builds ship authority and enforces final-closeout readiness.
  - `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:58-84` currently asks for task results but not a supported task-evidence writer.
  - `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl:33-120` currently requires a full closeout rerun and has no reuse branch.
- Dependency chains:
  - `slipway run` -> `AdvanceGoverned` -> `SyncGovernedWaveExecution` -> `LoadExecutionTasksFromEvidence` / `BuildExecutionSummary` -> `state.SaveExecutionSummary`.
  - `slipway repair` -> `rebuildExecutionSummaries` -> `LoadExecutionTasksFromEvidence` / `SyncGovernedWaveExecution`.
  - `next`/`run`/`validate` -> `EvaluateRequiredSkillsForChange` -> required skill records -> `EvaluateShipAuthority`.
- Blast radius: CLI command surface, task evidence runtime state, execution-summary repair/reporting, generated skill templates, and ship-gate closeout evidence evaluation.
- Constraints: preserve flat runtime task evidence paths; keep evidence freshness fail-closed; do not weaken standard/strict final-closeout assurance attestation.

### Patterns
- Existing conventions:
  - Top-level commands are `make*Cmd` functions in `cmd/`, registered in `cmd/root.go`, described in `internal/toolgen/toolgen.go`, and usually covered by command-level tests using `commandForRoot`.
  - Verification evidence remains compact YAML with `verdict`, `blockers`, `timestamp`, `run_version`, string `references`, and `notes` (`internal/model/verification.go:16-23`).
  - Task evidence is flat JSON and already rejects legacy nested `task_run`, missing `captured_at`, missing `freshness_inputs`, invalid verdicts, and run-version mismatches (`internal/engine/progression/wave_sync.go:350-442`).
  - Fresh task inputs are deterministic: `change_id`, `run_summary_version`, `task_id`, and `guardrail_domain` (`internal/state/execution_summary.go:312-319`).
- Reusable abstractions:
  - Use `resolveActiveChangeRef`, `loadActiveChange`, `encodeJSONResponse`, and `newInvalidUsageError` style helpers for a new command.
  - Use `state.ExpectedExecutionTaskFreshnessInputs` rather than accepting caller-supplied freshness input fields.
  - Reuse `progression.ParseTaskEvidence` in tests to prove emitted evidence is consumer-compatible.
- Convention deviations: a nested command namespace like `slipway evidence task` is new, but it cleanly scopes runtime evidence recording without overloading `run`, `repair`, or host skills.

### Risks
- Technical risks:
  - High: allowing host-provided `freshness_inputs` would permit stale or cross-change evidence to pass. Mitigation: compute them in the command.
  - High: final-closeout reuse could become a rubber stamp if the kernel only trusts prose. Mitigation: enforce machine-readable reuse references against matching goal-verification run/version and fresh execution summary state.
  - Medium: a new command may broaden adapter/tool surfaces. Mitigation: add registry metadata, help text, and command tests.
  - Medium: `repair` must not fabricate task evidence. Mitigation: keep missing task evidence non-repairable and test the distinction.
- Guardrail domains: none of the sensitive guardrail domains apply; this is internal governance workflow/state behavior.
- Reversibility: changes are local CLI/runtime semantics and generated skill guidance; rollback is a code revert plus removal of generated command guidance.

### Test Strategy
- Existing coverage:
  - `internal/engine/progression/wave_sync_test.go` covers parsing and summary sync from task evidence.
  - `cmd/repair_test.go` covers execution-summary rebuild behavior and invalid task evidence.
  - `cmd/progression_next_test.go` covers missing task evidence blockers and closeout display behavior.
  - `internal/tmpl/templates_test.go` covers generated skill template contents.
- Infrastructure needs:
  - Add command tests for `evidence task`.
  - Add focused progression/repair tests for supported evidence -> summary rebuild and missing evidence -> non-repairable finding.
  - Add closeout reuse tests at the evidence/authority layer and template tests for the reuse branch.
- Verification approach:
  - Targeted command and progression tests first.
  - Then `go test ./...`, `go build ./...`, `go vet ./...`, `git diff --check`, and `slipway validate --json`.

## Alternatives Considered
- Approach 1: Add `slipway evidence task` as the supported runtime evidence writer and have wave-orchestration call it. Tradeoff: adds a command surface, but it centralizes controlled fields and gives hosts a stable contract. Selected.
- Approach 2: Add only an internal Go helper and update tests. Tradeoff: lower CLI surface area, but hosts would still hand-write internal runtime JSON and root cause A would remain user-visible. Rejected.
- Approach 3: Teach `slipway run` to infer task evidence from wave-orchestration YAML notes. Tradeoff: fewer host steps, but parsing prose/YAML notes into execution evidence is implicit and fragile. Rejected.
- Selected: Approach 1 plus a kernel-checked final-closeout reuse reference contract. It directly addresses A and F while preserving fail-closed runtime evidence semantics.

## Unknowns
- Resolved: command namespace -> Use `evidence task` because it names the domain contract and avoids overloading lifecycle commands.
- Resolved: task freshness inputs -> Compute from `state.ExpectedExecutionTaskFreshnessInputs(change, run_summary_version, task_id)`.
- Resolved: repair distinction -> Existing `rebuildExecutionSummaries` already calls `LoadExecutionTasksFromEvidence`; add tests and clearer findings rather than fabricating missing source evidence.
- Resolved: final-closeout reuse proof -> A final-closeout record may reuse goal-verification only when it carries explicit reuse references and the matching goal-verification record is passing for the same run version while current execution-summary freshness remains fresh.
- Remaining: None.

## Assumptions
- The task evidence file naming convention may remain `<task_id>.json` because current loaders scan the flat directory and choose latest per task. Evidence: `cmd/progression_next_test.go:3607-3638` and `internal/engine/progression/wave_sync.go:222-303`.
- `final-closeout` still writes its own verification record even when it reuses goal-verification proof; reuse reduces rerun requirements, not the presence of closeout evidence. Evidence: `internal/engine/progression/authority.go:141-163`.
- The generated skill templates are part of the user-facing contract and must be updated with code. Evidence: `internal/toolgen/toolgen.go:235-236` and `internal/tmpl/templates_test.go`.

## Canonical References
- `https://github.com/signalridge/slipway/issues/53#issuecomment-4604547893`
- `cmd/root.go:139-168`
- `internal/toolgen/toolgen.go:143-197`
- `internal/engine/progression/wave_sync.go:21-168`
- `internal/engine/progression/wave_sync.go:222-442`
- `internal/state/store.go:103-108`
- `internal/state/execution_summary.go:312-319`
- `internal/engine/progression/evidence.go:16-127`
- `internal/engine/progression/authority.go:129-198`
- `cmd/repair.go:687-771`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:58-84`
- `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl:33-120`
