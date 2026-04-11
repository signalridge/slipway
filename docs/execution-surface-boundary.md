# Execution Surface Boundary

This record locks the product boundary introduced by the functional
optimization work.

## Identity

Slipway is a governance-first workflow runtime with explicit execution
surfaces and config-driven agent coordination.

## `next` Versus `run`

| Surface | Contract | Writes state by default | Primary flags |
|---|---|---|---|
| `next` | Show readiness, advance one step if able, and surface the next skill | yes, unless `--preview` is used | `--json`, `--preview`, `--context-guard`, `--change` |
| `run` | Advance governed execution until a skill, blocker, checkpoint, or done-ready outcome is surfaced | yes | `--json`, `--resume`, `--resume-response`, `--change` |

Design rules:

- `next` owns decision visibility and single-step advancement.
- `run` owns continuous execution control.
- Execution mutation is no longer hidden behind `next --auto`.
- Checkpoint continuation is no longer hidden behind `next --resume-response`.
- `run` is a first-class adapter-visible product command, not a CLI-only
  special case.

## Flag Migration

| Legacy form | Current form | Notes |
|---|---|---|
| `slipway next --auto` | `slipway run` | continuous governed execution moved to `run` |
| `slipway next --resume-response "<text>"` | `slipway run --resume-response "<text>"` | active checkpoint continuation moved to `run` |
| `slipway next --preview` | unchanged | remains the read-only inspection mode |
| `slipway next --context-guard` | unchanged | remains a `next` query surface |

## Resume Taxonomy

- `slipway run --resume-response "<text>"` is valid only when an active
  checkpoint exists. It validates and consumes the checkpoint payload.
- `slipway run --resume` is valid only when no active checkpoint exists and the
  current execution should continue from the latest incomplete wave.
- If an active checkpoint exists, plain `run` is rejected and the caller must
  use `--resume-response`.
- If resumable non-checkpoint execution exists, plain `run` is rejected and the
  caller must use `--resume`.
- Wave-backed execution artifacts are authoritative for resume on the current
  model. Missing or inconsistent wave artifacts must be repaired before resume.

## `abort` Versus `cancel`

| Surface | Contract | Archive change | Intended follow-up |
|---|---|---|---|
| `abort` | Stop only the in-flight execution session and preserve the active change | no | `status` or `health --doctor` -> `repair` if needed -> `run --resume` |
| `cancel` | Terminate the active change and archive terminal state | yes | none; the change is no longer active |

Rules:

- `abort` is only valid during `S2_EXECUTE`.
- `abort` clears the active checkpoint and preserves the change for later retry
  or replanning.
- `cancel` remains the only terminate-and-archive surface.

## Recovery Sequence

When execution truth looks inconsistent:

1. Run `slipway health --doctor`.
2. Run `slipway repair`.
3. Resume with `slipway run --resume` or `slipway run --resume-response`.
