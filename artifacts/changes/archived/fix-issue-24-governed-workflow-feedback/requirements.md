# Requirements

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Supported Stale Execution Evidence Repair
REQ-001: `slipway repair --json` MUST safely rebuild a ready-but-stale
`verification/execution-summary.yaml` from current wave-backed runtime task
evidence when the stale cause is execution evidence drift and the current task
plan remains compatible. If planning evidence is stale or runtime task evidence
is invalid/missing, repair MUST leave the change unrepaired and report an
actionable target and next action.

#### Scenario: Ready summary is stale only against runtime task evidence
GIVEN an active governed change in S3 or S4 with a passing wave-orchestration
record and valid flat runtime task evidence
WHEN `execution-summary.yaml` has stale task timestamps or freshness fields
THEN `slipway repair --json` rebuilds the execution summary from runtime task
evidence and reports the rebuild as an applied repair.

#### Scenario: Planning evidence is stale
GIVEN a ready execution summary whose planning artifacts changed after
execution evidence
WHEN `slipway repair --json` runs
THEN the command does not overwrite execution evidence and reports unrepaired
planning drift.

### Requirement: Actionable Runtime Task Evidence Diagnostics
REQ-002: Missing run-summary and missing task-evidence blockers MUST name the
runtime task evidence directory and the required flat JSON fields so a human or
non-agent executor can produce the supported evidence envelope without code
spelunking.

### Requirement: Dirty Worktree Archive Warning
REQ-003: `slipway done --json` MUST warn when a worktree-bound change is
archived while source files still exist only as dirty or untracked workspace
changes, and MUST list the affected non-governance files. The warning MUST NOT
turn Git commit state into a hard governance requirement.

### Requirement: Review Checklist Coverage For Negative Paths And Toolchains
REQ-004: Generated governed review skills MUST explicitly require evidence for
requirement-named negative/error paths and dependency/toolchain compatibility
checks, including Rust workspace MSRV drift when dependency manifests or
lockfiles change.
