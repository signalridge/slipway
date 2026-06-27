# Requirements

## Requirements

### Requirement: Built-Binary State Read Baseline
REQ-001: The change MUST keep a repeatable built-binary performance baseline for
state-read commands, including worktree count, `change.yaml` count,
verification record count, and `real/user/sys` timings.

#### Scenario: Before and after timings are comparable
GIVEN a synthetic fixture with at least 25 Git worktrees, 300 `change.yaml`
files, and 100 verification records
WHEN the before and after binaries are measured against the required command
matrix
THEN the verification artifacts record the fixture counts, command lines, and
`real/user/sys` timings for root status, bound status, bound next, bound
validate, and explicit `--change` scenarios.

### Requirement: Invocation-Scoped Read Context
REQ-002: The system MUST create and use a single-command state read context for
`status`, `next`, and `validate` so route resolution, current change loading,
path resolution, verification inventory, and status timeline reads are reused
inside one CLI invocation.

#### Scenario: One command reuses resolved state
GIVEN a command invocation has already resolved the active change and its
canonical paths
WHEN that command builds its JSON view and invocation route
THEN it reuses the already-loaded change and paths instead of loading the same
`change.yaml` or resolving the same paths again.

#### Scenario: No cross-command authority persists
GIVEN one CLI invocation has completed
WHEN a later invocation starts after filesystem state changes
THEN the later invocation reads fresh filesystem authority and cannot observe
stale data from the prior command context.

### Requirement: Explicit Change Fast Path
REQ-003: The system MUST use an authority-known fast path for explicit
`--change <slug>` success cases before falling back to global bundle scans for
missing, sibling, archived, or integrity diagnostics.

#### Scenario: Existing explicit bound slug avoids full discovery
GIVEN a change slug has a git-local worktree binding and a governed bundle in
that bound worktree
WHEN `status`, `next`, or `validate` runs with `--change <slug>`
THEN the command reads the bound authority directly and does not scan all
visible `change.yaml` bundles on the common success path.

#### Scenario: Missing explicit slug still fails closed
GIVEN no active or archived change exists for an explicit slug
WHEN a command runs with `--change <slug>`
THEN it exits with the existing `change_not_found` precondition behavior rather
than falling back to unscoped diagnostics.

### Requirement: Verification Inventory Reuse
REQ-004: The system MUST reuse verification records already read for the current
resolved change when rendering status/next/validate evidence views, while still
strictly validating malformed verification YAML.

#### Scenario: Status evidence pointers reuse resolved records
GIVEN status readiness or the invocation context has loaded verification records
for the current change
WHEN status builds evidence pointers
THEN it reuses that resolved-change inventory and does not re-run slug-based
verification directory resolution.

### Requirement: Tail-Oriented Status Timeline
REQ-005: The system MUST use a tail-oriented lifecycle JSONL read for status
timeline display so showing the last N events does not decode the full log.

#### Scenario: Status displays a bounded tail
GIVEN a lifecycle event log contains more events than the status display limit
WHEN status renders the timeline
THEN it decodes only the retained tail window needed for the display and returns
the same latest events as the full reader.

#### Scenario: Malformed retained line fails closed
GIVEN the retained lifecycle tail contains malformed JSON
WHEN status renders the timeline
THEN status surfaces the existing lifecycle-event-log unreadable diagnostic
instead of silently skipping the malformed line.

### Requirement: Existing Fail-Closed Semantics Stay Intact
REQ-006: The system MUST preserve existing bound worktree, archived change,
missing authority, multi-active, no-active, and malformed-state failure
semantics while removing duplicate reads.

#### Scenario: Existing route and integrity regressions remain covered
GIVEN the existing regression fixtures for missing explicit slug, archived
fallback, missing active authority, bound worktree routing, and malformed
lifecycle logs
WHEN the optimized read paths are exercised
THEN those tests continue to pass without compatibility shims or persistent
indexes.
