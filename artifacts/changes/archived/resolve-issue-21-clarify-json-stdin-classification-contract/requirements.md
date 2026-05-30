# Requirements

## Project Context
- Tech Stack:
- Conventions:
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Requirements

### Requirement: WorkflowSkillTransportClarity
REQ-001: The primary exported workflow skill MUST state that explicit `slipway new --json` classification uses JSON stdin fields, not command-line flags.

### Requirement: MinimalJSONExample
REQ-002: At least one exported agent-facing surface MUST show a minimal `echo '{...}' | slipway new --json` example including `description`, `guardrail_domain`, `needs_discovery`, and `complexity`.

### Requirement: CommandPromptCompleteness
REQ-003: The `/slipway-new` command prompt surface MUST document the explicit JSON stdin classification path for AI callers that already know classification, while preserving guidance against unsupported manual routing labels.

### Requirement: CommandReferenceCompleteness
REQ-004: The generated workflow command reference MUST include stdin classification notes for `slipway new` in addition to CLI arguments.

### Requirement: RegressionCoverage
REQ-005: Generated-surface tests MUST fail if the JSON stdin classification contract, minimal example, or no-flag wording disappears from generated Codex and Claude surfaces.
