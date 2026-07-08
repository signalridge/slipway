# Integrations

## Internal Integrations
- CLI command views integrate with `internal/model.RecoverySummary` by passing canonical `ReasonCode` slices through `model.BuildRecovery`.
- Wave projection integrates `internal/wave.ParseTaskPlan`, `internal/wave.PlanWaves`, and `internal/state.MaterializeWavePlan*`.
- Readiness integrates workspace `git diff` / `git ls-files --others` scans with scope-contract evaluation.

## External Integrations
- No external services, APIs, databases, or network integrations are changed.
- The public integration surface is the CLI JSON/text contract for `fix`, `next`, `validate`, and `evidence`.

## File Formats
- `tasks.md` remains the planning authority for task IDs and wave projection.
- `wave-plan.yaml`, `execution-summary.yaml`, and verification YAML remain engine-owned state files.
- Task evidence is recorded through host-owned `slipway evidence task` flags; executor/subagent outputs remain factual reports consumed by the host.
