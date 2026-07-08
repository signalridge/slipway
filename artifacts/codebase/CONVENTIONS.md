# Conventions

## Command Surface Conventions
- Cobra command handlers produce typed view structs for JSON output and shared `CLIError` objects for blocked operations.
- User-visible recovery should come from canonical reason codes and `model.BuildRecovery` so `next`, `validate`, and command errors agree.
- Public command help, generated prompt surfaces, and docs should be updated together when a command's decision guard changes.

## State And Evidence Conventions
- Engine-owned freshness state is written by `internal/state` helpers and CLI evidence commands.
- Tests use command fixtures rather than manually forging final readiness where possible; when helpers write state directly, they name the exact scenario under test.

## Testing Conventions
- Behavioral regressions are covered with command-level tests in `cmd/`.
- Shared model behavior, especially recovery ordering and canonical reason codes, is covered in `internal/model` tests.
- Readiness and scope-contract behavior is covered near `internal/engine/progression` and `internal/engine/scopecontract`.
