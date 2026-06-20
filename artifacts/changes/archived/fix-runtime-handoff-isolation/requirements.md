# Requirements

## Requirements

### Requirement: Per-change session handoff
REQ-001: The system MUST define, read, and report runtime session handoff notes
through a per-change runtime path under `.git/slipway/runtime/changes/<slug>/`
whenever an active governed change can be resolved.

#### Scenario: Session start in one worktree ignores another change's handoff
GIVEN two active governed changes bound to different worktrees
WHEN each change has its own session handoff note
THEN the session-start hook in each worktree reports only that change's handoff
path and does not report the other change's handoff.

#### Scenario: Handoff remains advisory
GIVEN a per-change session handoff file exists
WHEN lifecycle status, next action, validation, or evidence freshness is computed
THEN the handoff note is not treated as lifecycle authority, governed evidence,
freshness input, or a gate.

### Requirement: Legacy handoff and runtime hygiene
REQ-002: The system MUST detect and productize cleanup/reporting for legacy
repo-level runtime handoff files and old `.git/slipway/changes/` runtime
directories without silently discarding ambiguous operator context.

#### Scenario: Legacy handoff files are visible to operators
GIVEN `.git/slipway/runtime/handoff.md` or similarly named repo-level handoff
files exist
WHEN health or repair inspects local runtime state
THEN the operator receives a clear finding or repair summary that identifies the
legacy path and the per-change replacement contract.

#### Scenario: Old runtime change directory is no longer hidden
GIVEN `.git/slipway/changes/<slug>/` exists from a retired runtime layout
WHEN health or repair inspects local runtime state
THEN the stale legacy runtime directory is reported or safely removed when it is
unambiguous.

### Requirement: Safe empty lock-anchor cleanup
REQ-003: The system MUST preserve the workspace/scope-level create and repair
lock semantics while safely cleaning empty lock-anchor files that have no live
metadata and are not actively held.

#### Scenario: Create lock remains global
GIVEN two `slipway new` operations could race before either has a stable change
slug
WHEN change creation begins
THEN the global `change-create.lock` remains the coordination point for the
workspace/scope creation critical section.

#### Scenario: Empty lock anchor cleanup is safe
GIVEN a `.lock` file exists without a companion `.meta` file
WHEN repair evaluates local lock artifacts
THEN it only removes the lock anchor after a non-blocking lock check proves the
anchor is not currently held.

### Requirement: Generated guidance matches runtime contract
REQ-004: Generated workflow, run-command, and context-pressure guidance MUST
direct agents to the per-change handoff contract and MUST NOT instruct agents to
write repo-level session handoff files for active governed changes.

#### Scenario: Generated skills do not advertise repo-global handoff as current
GIVEN the generated workflow skill text is inspected
WHEN it describes runtime session handoff
THEN it names the per-change runtime handoff contract and preserves the advisory
non-authority warning.
