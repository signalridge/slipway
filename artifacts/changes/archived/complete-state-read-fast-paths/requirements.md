# Requirements

## Requirements

### Requirement: Invocation-scoped read context reuse
REQ-001: The system MUST reuse state-read facts discovered during one
`status`, `next`, or `validate` invocation for that invocation only, including
loaded change authority, resolved change paths, verification inventory, and
display timeline reads.

#### Scenario: Reuse within one command
GIVEN a command has already loaded a governed change and resolved its paths
WHEN the same command builds downstream status, next, or validate views for that change
THEN the command reuses the invocation-scoped read context instead of repeating equivalent state discovery.

#### Scenario: No cross-command cache
GIVEN a command has completed
WHEN a later command reads the same change
THEN it starts from fresh state authority and does not reuse a prior command's cached facts.

### Requirement: Explicit change fast path
REQ-002: The system MUST use a narrow authority-known fast path for successful
`--change <slug>` reads and MUST avoid scanning all change bundles on the
ordinary successful explicit-slug path.

#### Scenario: Successful explicit slug
GIVEN a valid active change slug is passed to `status`, `next`, or `validate`
WHEN the command can load the change from invocation-local or bound authority
THEN the command completes without enumerating every active change bundle.

#### Scenario: Missing explicit slug
GIVEN an explicit slug that does not name an active or archived change
WHEN `status`, `next`, or `validate` resolves that slug
THEN the command fails closed with stable `change_not_found` semantics and exit code 3.

### Requirement: Tail-oriented status timeline read
REQ-003: The system MUST use a bounded tail read for status timeline display
and MUST preserve fail-closed malformed-log behavior for retained lines and
full-log integrity surfaces.

#### Scenario: Status displays recent events
GIVEN a lifecycle event log with many historical entries
WHEN `status` renders the default bounded timeline
THEN it reads and decodes only the bounded tail plus required predecessor transition context.

#### Scenario: Malformed retained line
GIVEN the retained timeline tail or required predecessor context contains malformed JSON
WHEN `status` builds its timeline
THEN the command returns a lifecycle event log read error instead of silently skipping the malformed line.

### Requirement: Existing lifecycle semantics preserved
REQ-004: The system MUST preserve existing bound-worktree, archived-change,
missing explicit slug, no-active, and multi-active fail-closed semantics while
optimizing reads.

#### Scenario: Archived local worktree
GIVEN an invocation runs from an archived change worktree
WHEN status, next, or validate resolves lifecycle authority
THEN archived-local behavior remains preferred over unrelated active changes.

#### Scenario: Multi-active root
GIVEN the root workspace has multiple active changes
WHEN an unscoped command needs lifecycle authority
THEN the command continues to report the existing multi-active remediation rather than selecting one implicitly.
