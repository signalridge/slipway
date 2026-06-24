# Requirements

## Requirements

### Requirement: Context-pressure CRITICAL message is an imperative write directive
REQ-001: At the CRITICAL context-pressure threshold (ratio >= 0.70) the hook
message produced by `contextPressureMessage` MUST be an imperative directive that
tells the agent to author the per-change handoff NOW, before its next action, via
`slipway handoff write --section <name>`, naming the judgment sections to write.
The WARNING threshold (ratio in [0.60, 0.70)) MUST remain a soft, non-imperative
suggestion.

#### Scenario: CRITICAL pressure emits an imperative write directive
GIVEN the context-usage ratio is at or above 0.70
WHEN the context-pressure hook renders its message
THEN the message imperatively directs the agent to author the handoff before its
next action via `slipway handoff write --section`
AND it names handoff judgment sections to write.

#### Scenario: WARNING pressure stays a soft suggestion
GIVEN the context-usage ratio is at or above 0.60 but below 0.70
WHEN the context-pressure hook renders its message
THEN the message remains a soft suggestion and is not an imperative "now" directive.

### Requirement: Generated slipway skill documents the fresh-session resume protocol
REQ-002: The generated slipway workflow skill (`SKILL.md`) MUST include a
"Continuing A Change In A Fresh Session" resume-protocol section that documents
the explicit read path: run `slipway status --json`, select `--change` when more
than one change is active, run `slipway handoff show --change <slug>` to read the
narrative first, then `slipway next`.

#### Scenario: Resume protocol is present in the generated skill
GIVEN the generated slipway workflow skill
WHEN its content is inspected
THEN it contains a "Continuing A Change In A Fresh Session" section
AND that section names the `slipway status --json` -> `--change` ->
`slipway handoff show --change` -> `slipway next` sequence.

### Requirement: A contract test guards the resume protocol against regression
REQ-003: The toolgen test suite MUST assert that the generated slipway workflow
skill ships the resume-protocol section, so the section cannot silently regress
out of the generated surface.

#### Scenario: Removing the resume section fails the guard
GIVEN the toolgen test suite
WHEN the generated slipway workflow skill does not contain the resume-protocol
section
THEN a toolgen test fails.

### Requirement: SessionStart hook stops auto-injecting change-state
REQ-004: The SessionStart hook MUST NOT auto-inject per-session change-state —
neither the active-worktree `next --json` view, nor the bound-elsewhere
`session_handoff_info: ... bound to <worktree>` pointer, nor the
`session_handoff_present` / `session_handoff_path` / handoff-brief summary. It
MUST still emit the `slipway_entry_skill` routing pointer and MUST remain
fail-silent (exit 0) on both the Claude XML and Codex JSON output paths.

#### Scenario: Bound-elsewhere session emits only the entry-skill pointer
GIVEN a session opens in a worktree with no active change of its own while a
change is bound to another worktree
WHEN the SessionStart hook runs
THEN its output contains the `slipway_entry_skill` routing pointer
AND its output contains no `session_handoff_info`, no active-change `next --json`
payload, and no `session_handoff_present` / `session_handoff_path` change-state.

#### Scenario: Hook stays fail-silent on internal error
GIVEN the SessionStart hook encounters an internal error (for example a root or
state-lock failure)
WHEN it runs
THEN it still exits 0 and never surfaces a blocking failure.

### Requirement: Handoff stays advisory and the engine never auto-writes its bytes
REQ-005: The change MUST NOT introduce any engine auto-writing of handoff bytes
or auto-bumping of handoff `updated_at`, so handoff freshness stays tied to real
agent authoring. The escalated CRITICAL message MUST retain the substrings the
hook contract asserts (`slipway handoff write`, `The handoff is advisory`,
`slipway status --json`, `slipway next --json`) and MUST avoid authority-elevating
wording (`lifecycle authority`, `governed evidence`, `freshness input`).

#### Scenario: Escalated message keeps required substrings and avoids forbidden ones
GIVEN the escalated CRITICAL context-pressure message
WHEN its text is inspected
THEN it contains `slipway handoff write`, `The handoff is advisory`,
`slipway status --json`, and `slipway next --json`
AND it contains none of `lifecycle authority`, `governed evidence`, or
`freshness input`.

#### Scenario: No new engine handoff auto-write path is introduced
GIVEN the implemented change
WHEN the handoff write surfaces are reviewed
THEN no engine code path writes handoff bytes or bumps handoff `updated_at`
automatically; handoff bytes are written only by an explicit
`slipway handoff write` invocation.
