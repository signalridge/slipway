# Requirements

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: #28 per-task freshness excludes downstream summary timestamps
REQ-001: Execution task evidence freshness MUST NOT use `execution-summary.yaml`
`captured_at` or sibling task `captured_at` values as the latest relevant
upstream update for every task.

#### Scenario: Summary regenerated after task evidence
GIVEN an execution summary with multiple task records whose runtime task
evidence timestamps match their per-task `captured_at` values
AND the summary `captured_at` is later than those task timestamps
WHEN execution-summary freshness is evaluated
THEN the task evidence remains fresh unless a task structural input or upstream
planning artifact is newer than that task evidence.

### Requirement: Freshness failure paths remain fail-closed
REQ-002: Execution freshness MUST still return stale when an upstream planning
artifact needed for freshness evaluation exists but cannot be read.

#### Scenario: Unreadable planning artifact
GIVEN a ready execution summary
AND a freshness-relevant planning artifact path returns a non-not-exist read or
stat error
WHEN freshness is evaluated
THEN Slipway reports stale execution evidence rather than silently passing.

### Requirement: Validate failure paths remain read-only
REQ-003: `validate --json` MUST NOT create, rewrite, or repair files when it
returns no-active diagnostics, rejects an archived explicit slug, or encounters a
non-empty active bundle directory without `change.yaml`.

#### Scenario: Residual review artifact in orphan bundle
GIVEN an orphan active bundle directory containing only review artifacts
WHEN `validate --json` runs
THEN no new files are written and the residual directory contents are unchanged.

### Requirement: Active validate contract is documented
REQ-004: User-facing command docs and closeout/assurance templates MUST describe
`validate --json` as an active pre-`done` freshness/readiness gate, not as a
post-archive audit surface for frozen bundles.

#### Scenario: Archived slug after done
GIVEN a terminal archived change
WHEN an operator reads command or closeout guidance
THEN the guidance says active `validate --change <slug>` rejects archived slugs
and that archived audit would require a separate read-only surface.

### Requirement: Issue tracker closeout follows evidence
REQ-005: Issues #29, #30, #32, and #34 MUST be closed or commented according to
current evidence; #28 MUST remain tied to the implementation fix.

#### Scenario: Non-must-fix reports
GIVEN current HEAD does not support the reported root causes for #30 and #32
AND #34 is an orphan-bundle diagnostic enhancement rather than a core logic bug
WHEN local verification and final review pass
THEN the issues are closed with concise evidence-backed comments and deferred
enhancement framing where appropriate.
