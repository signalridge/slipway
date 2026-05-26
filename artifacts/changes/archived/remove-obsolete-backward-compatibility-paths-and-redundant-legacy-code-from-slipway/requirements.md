# Requirements

## Project Context
- Tech Stack: Go
- Conventions: CLI commands live in `cmd/`; durable state helpers live in `internal/state`; generated-surface code lives in `internal/toolgen`.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Remove legacy runtime sidecar compatibility
REQ-001: Slipway MUST use `change.yaml` as the active change state authority and MUST NOT load, diagnose, repair, migrate, or delete `runtime-state.yaml` sidecars for old-version compatibility.

#### Scenario: Active change load
GIVEN an active change bundle contains `change.yaml`
WHEN Slipway loads the change
THEN loading depends on `change.yaml` only and does not merge fields from `runtime-state.yaml`.

#### Scenario: Health and repair
GIVEN a bundle contains a stale `runtime-state.yaml`
WHEN `health` or `repair` runs
THEN they do not emit legacy runtime-sidecar findings or migration summaries.

### Requirement: Remove old generated-surface upgrade cleanup
REQ-002: Tool generation refresh MUST stop carrying cleanup code whose only purpose is upgrading older Slipway-generated workspaces, including retired `slipway-sync`, old catalog root files, legacy Codex agent config blocks, legacy post-tool hooks, and legacy support-file provenance metadata.

#### Scenario: Refresh current generated tree
GIVEN `slipway init --refresh` regenerates current tool surfaces
WHEN the generated tree is refreshed
THEN current expected skills, commands, prompts, and hooks are written deterministically without old-version cleanup branches.

### Requirement: Update public docs and tests to the initial-version contract
REQ-003: Documentation and tests MUST stop advertising or asserting backward-compatibility behavior removed by this change, and MUST assert the new initial-version contract where behavior remains user-visible.

#### Scenario: Documentation
GIVEN a user reads README or command-contract docs
WHEN they inspect state authority and generated-surface behavior
THEN the docs describe current first-version behavior without promising old-version migration or cleanup.

### Requirement: Preserve current first-version command and agent handoff coherence
REQ-004: Current command surfaces, JSON output, generated host-skill handoff, and lifecycle semantics MUST remain internally coherent after backward-compatibility removal.

#### Scenario: Verification
GIVEN cleanup is complete
WHEN repo verification runs
THEN focused tests, `go test ./...`, and `go build ./...` pass.

### Requirement: external_api_contracts guardrail compliance
REQ-005: Any current external command or JSON behavior changed by the cleanup MUST be reflected in tests and docs in the same change.

#### Scenario: Guardrail compliance
GIVEN a user or agent consumes current Slipway output
WHEN the cleanup changes a current surface
THEN the new first-version contract is explicitly covered or documented.
