# Requirements

## Requirements

### Requirement: Binary-backed automatic hooks

REQ-001: The system MUST execute Slipway-owned automatic hook behavior through
compiled Slipway commands instead of generated shell business logic or
`bash "<hook>.sh"` settings.

#### Scenario: Session start uses compiled behavior

GIVEN a generated AI-tool adapter with SessionStart support
WHEN its hook runs
THEN lifecycle lookup, handoff-path resolution, and host output are produced by
`slipway hook session-start --tool <tool>`.

#### Scenario: Context pressure remains compiled

GIVEN a generated AI-tool adapter with PostToolUse support
WHEN context-pressure input is delivered to the hook
THEN context-pressure classification is produced by
`slipway hook context-pressure` and not by a shell implementation.

### Requirement: Platform-native thin launchers

REQ-002: The system MUST render platform-native hook launchers only as thin
binary dispatch adapters, and generated settings MUST NOT use `bash` or a
`.sh` file as the canonical registered command.

#### Scenario: Windows adapter generation

GIVEN Slipway generates hook-capable adapter surfaces on Windows
WHEN hook launchers and settings are rendered
THEN the registered launcher is Windows-native and contains no lifecycle,
handoff, JSON, GitHub, SARIF, or helper business logic.

#### Scenario: Missing binary in automatic hook

GIVEN an automatic hook launcher cannot find the `slipway` binary
WHEN the host AI tool invokes the hook
THEN the launcher exits successfully without blocking the host tool.

### Requirement: Binary-backed skill helpers with explicit manual dependencies

REQ-003: The system MUST replace supported generated skill helper scripts with
`slipway tool ...` commands and MUST NOT ship generated bash, Python, jq, or
shell helper payloads. Manual helpers MAY use explicit domain dependencies when
they are the best backend for the workflow, but those dependencies MUST be
documented, selected deliberately, and fail closed when unavailable.

#### Scenario: Local helper execution

GIVEN a generated skill needs SARIF merge, action pinning, or variant
scaffolding support
WHEN the skill instructions name the helper
THEN they invoke a `slipway tool ...` command and no generated
`skills/*/scripts/*` payload is required.

#### Scenario: GitHub helper execution

GIVEN a generated skill needs PR checks, review feedback, review requests, or a
review-thread reply helper
WHEN the helper runs
THEN `--backend auto` prefers authenticated `gh`, falls back to token-backed API
with `GH_TOKEN` or `GITHUB_TOKEN` when `gh` is unavailable or reports an
auth-required error, and fails closed with remediation when neither
authenticated backend is available.

#### Scenario: Explicit GitHub helper backend

GIVEN an operator selects `--backend gh`
WHEN the GitHub CLI is unavailable or unauthenticated
THEN the helper fails closed with a `gh`-specific remediation and does not
silently fall back to another backend.

#### Scenario: Token API GitHub helper backend

GIVEN an operator selects `--backend api`
WHEN `GH_TOKEN` and `GITHUB_TOKEN` are both absent
THEN the helper fails closed with a token-specific remediation and does not make
unauthenticated GitHub API or fetch requests.

#### Scenario: Domain-specific local helper dependency

GIVEN a generated skill invokes `slipway tool find-polluter-go`
WHEN the Go toolchain is unavailable
THEN the helper fails closed with installation remediation because the workflow
itself is defined in terms of `go list` and `go test`.

### Requirement: Generated-surface enforcement

REQ-004: The system MUST update tests, template inventory, and docs so legacy
script-based hook/helper contracts are rejected rather than preserved.

#### Scenario: Legacy hook command rejected

GIVEN generated tool settings are refreshed
WHEN tests inspect registered Slipway hook commands
THEN they fail if a command contains a canonical `bash`, `.sh`, Python, jq, or
GitHub CLI helper dependency in an automatic hook registration.

#### Scenario: Legacy skill scripts absent

GIVEN generated skill support files are refreshed
WHEN tests inspect generated skill payloads
THEN no Slipway-owned executable helper script is emitted under
`skills/*/scripts/*`.
