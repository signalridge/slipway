# Decision
## Project Context
- Tech Stack: Go CLI, Cobra, Slipway governance runtime
- Conventions: command metadata in `internal/toolgen`, command wiring in `cmd/root.go`, compact YAML verification records, flat runtime task evidence JSON
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Approach 1: Public `slipway evidence task` command
- Add a top-level `evidence` command with `task` subcommand.
- The command resolves the active/bound change, computes freshness inputs, writes flat runtime task evidence JSON, and returns the path in JSON/text output.
- Tradeoffs: adds a CLI surface and toolgen/docs work, but directly removes host hand-writing of internal runtime paths and centralizes fail-closed validation.

### Approach 2: Internal helper only
- Add an internal Go helper to write task evidence and use it only in tests or future runtime code.
- Tradeoffs: smaller surface, but governed hosts still have no supported way to record task evidence; root cause A remains.

### Approach 3: Infer task evidence from wave-orchestration verification
- Extend `slipway run` or `repair` to parse wave-orchestration YAML notes into task evidence.
- Tradeoffs: avoids a new command, but relies on implicit prose parsing and creates fragile hidden behavior.

## Selected Approach
Use Approach 1. Add `slipway evidence task` as the first-class task evidence recording surface, update wave-orchestration guidance to call it, and keep execution-summary generation/repair consuming the same flat runtime JSON format already parsed by `progression.ParseTaskEvidence`.

For final-closeout reuse, keep `final-closeout.yaml` as required standard/strict evidence but permit a reuse branch only through explicit string references checked by the kernel. A final-closeout record that claims reuse must match a passing goal-verification record for the latest run version and fresh execution-summary state.

## Interfaces and Data Flow
- New command: `slipway evidence task --task-id <id> --run-summary-version <n> --task-kind <code|docs|verification|...> --verdict <pass|fail> --evidence-ref <ref> [--changed-file <path> ...] [--target-file <path> ...] [--blocker <code[:detail]> ...] [--captured-at <RFC3339Nano>] [--session-id <id>] [--json] [--change <slug>]`.
- Data flow: command input -> active change resolution -> `state.ExpectedExecutionTaskFreshnessInputs` -> JSON payload under `state.EvidenceTasksDir` -> `SyncGovernedWaveExecution` -> `execution-summary.yaml`.
- Reuse flow: `goal-verification.yaml` + fresh execution-summary context -> `final-closeout.yaml` references `closeout:goal_verification_reuse=pass` and `closeout:goal_verification_reuse_run_version=<n>` -> ship authority validation.
- Generated contract updates: wave-orchestration template, final-closeout template, command docs/toolgen metadata.

## Rollout and Rollback
- Rollout: add command/tests first, then template/docs, then closeout reuse validation.
- Rollback: revert code changes and generated contract text; existing task evidence files remain parseable because the command writes the current flat schema.
- Compatibility: no migration is needed for existing evidence; the new command is additive.

## Risk
- Risk: command writes stale/cross-change freshness inputs. Mitigation: do not accept caller-provided freshness inputs; compute them from current change/run/task.
- Risk: final-closeout reuse hides stale proof. Mitigation: require matching run version, passing goal-verification, and fresh execution-summary diagnostics at ship evaluation time.
- Risk: repair fabricates missing execution source evidence. Mitigation: test missing source evidence as non-repairable and keep summary rebuild derived only from valid task evidence.
- Risk: scope drift into Tier 3 planning recovery. Mitigation: exclude S3/S4 planning recovery and pivot/rescope changes from tasks.
