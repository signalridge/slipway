# Requirements
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; generated skills/commands come from internal/tmpl/templates and toolgen; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Populated codebase map surfaces a non-blocking relevance self-check advisory
REQ-001: When a map-consuming planning skill (research-orchestration or plan-audit) is next and the codebase map status is `populated` (or `partial`), `slipway next`/`run` MUST surface a non-blocking advisory stating that the status reflects content presence, not scope relevance, and prompting the host to judge whether the map matches the current change and refresh stale sections inline. The advisory MUST be a `warnings` entry (never a blocker) and MUST NOT change any existing JSON field shape.

#### Scenario: Populated map consumed by a planning skill warns about relevance
GIVEN a `populated` codebase map and the next skill is research-orchestration or plan-audit
WHEN `slipway next --json` is rendered
THEN a non-blocking `warnings` advisory is present that names the populated status, states it reflects content presence rather than scope relevance, and prompts a host relevance self-check + inline refresh.

#### Scenario: Non-consuming skill gets no relevance advisory
GIVEN a `populated` codebase map and the next skill is not a map consumer
WHEN `slipway next --json` is rendered
THEN no relevance advisory is emitted.

### Requirement: Consuming skills instruct the host to self-check and refresh inline
REQ-002: The research-orchestration, plan-audit, and codebase-mapping skills MUST instruct the host that a `populated` map is not automatically relevant — the host MUST judge whether it matches the current change scope (affected seams, blast radius, concerns) and re-author stale/irrelevant sections inline before relying on it.

#### Scenario: Skills carry the populated-map self-check instruction
GIVEN the generated research-orchestration, plan-audit, and codebase-mapping skills
WHEN their codebase-map guidance is inspected
THEN each instructs the host to verify a populated map against the current change and refresh stale sections inline (populated ≠ relevant).

### Requirement: Staleness guidance is host-AI semantic relevance, not engine fingerprints
REQ-003: The codebase-map reference (`context-assembly/references/codebase-map.md`) MUST define populated-map staleness as a host-AI semantic relevance judgment with inline re-authoring, and MUST NOT define it via git-mtime distance, renamed entry points, or lockfile mismatch.

#### Scenario: Reference doc drops the fingerprint heuristics
GIVEN the codebase-map reference doc
WHEN its staleness section is inspected
THEN it describes re-reading the populated doc and judging scope relevance with inline re-author, and contains no git-mtime / entry-point-rename / lockfile-mismatch staleness rule.

### Requirement: Change is additive and generated surfaces stay aligned
REQ-004: The advisory MUST be additive (no existing JSON field shape changes; no engine fingerprint algorithm; no new metadata file; no blocking gate). Generated skills/commands MUST be regenerated so the host surfaces match, with only the intended additions appearing in the diff.

#### Scenario: Additive surface with aligned generation
GIVEN the implementation
WHEN `go run . init --refresh --tools all` runs and the project diff is checked
THEN the only generated changes are the intended skill/reference additions, the public JSON field shape is unchanged, and `go build/vet/test ./...` plus `golangci-lint run` are clean.
