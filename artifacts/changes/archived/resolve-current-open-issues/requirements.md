# Requirements

## Requirements

### Requirement: Recover Invalid Selected-Review Evidence

REQ-001: The system MUST provide a public, CLI-driven recovery path for an
already passing selected-review skill record whose evidence is invalid only
because its `context_origin:stage=review=<handle>` reference is missing or
malformed, while continuing to reject arbitrary overwrites of current passing
evidence.

#### Scenario: Invalid context-origin can be replaced

GIVEN a governed change in S3 review has passing `goal-verification` evidence
without a valid selected-review context-origin handle
WHEN the operator reruns the selected reviewer and records replacement evidence
with a valid `context_origin:stage=review=<handle>` reference
THEN `slipway evidence skill` accepts the replacement and the context-origin
authority blocker can clear through normal validation.

#### Scenario: Current passing evidence is still protected

GIVEN a selected-review skill already has passing evidence with a valid
context-origin handle
WHEN the operator attempts to record unrelated replacement evidence
THEN the command rejects the write with the existing not-current guard and does
not weaken fail-closed review independence.

### Requirement: Transactional And Ownership-Safe Toolgen Refresh

REQ-002: The adapter generation path SHALL apply generated writes and removals
through a rollback-capable transaction and SHALL classify generated files by
path/checksum ownership before destructive cleanup.

#### Scenario: Failed refresh rolls back

GIVEN an existing generated adapter tree
WHEN a refresh fails after one generated file mutation succeeds
THEN prior generated files are restored and callers do not observe a partially
refreshed adapter tree.

#### Scenario: Unknown and modified files are preserved

GIVEN a refresh encounters an unknown user file or a generated file whose
checksum no longer matches the ownership manifest
WHEN stale cleanup runs
THEN unknown files are preserved and modified generated files are backed up or
refused rather than silently deleted.

### Requirement: Fail-Closed Install Profiles And Routers

REQ-003: Generated skill installation profiles and namespace routers MUST reduce
eager surface listing only by applying an explicit dependency closure that keeps
lifecycle-critical, gate-owning, and sensitive-domain skills installed and
reachable in every profile.

#### Scenario: Critical governance skills cannot be pruned

GIVEN the core or standard profile is selected
WHEN toolgen computes the generated skill set
THEN intake, planning, execution, review, evidence, done, and sensitive-domain
review skills remain installed and the generated tests prove they cannot be
removed by profile selection.

#### Scenario: Routers point to commands without replacing gates

GIVEN namespace router skills are generated
WHEN an agent follows a router
THEN the router directs the agent to Slipway command/host surfaces and does not
replace lifecycle progression, review, or evidence gates.

### Requirement: Diataxis Documentation With GSD Core And Trellis References

REQ-004: The documentation SHALL be organized into Diataxis tutorials, how-to,
reference, and explanation sections, using GSD Core as the structural reference
for Diataxis grouping and using Trellis as the onboarding-content reference for
task/spec/memory-centered first-run and real-world-scenario guidance. The docs
SHALL include guided tutorials for a first governed change and onboarding an
existing codebase.

#### Scenario: New user can follow a tutorial

GIVEN a new user opens the documentation
WHEN they follow the first governed change tutorial
THEN the tutorial shows the lifecycle from `slipway new` through governed
readiness using current command surfaces, mirrors GSD Core's one-guaranteed-path
tutorial style, borrows Trellis's task-centered first-task onboarding shape, and
does not document force/skip bypasses.

#### Scenario: Docs navigation reflects Diataxis

GIVEN the MkDocs navigation is built
WHEN the docs are rendered or checked
THEN tutorials, how-to, reference, and explanation sections are discoverable and
existing command/reference coverage remains linked, with the Start Here path
covering install, first governed change, how the workflow works, real-world
scenarios, and onboarding an existing codebase.

### Requirement: Go-Native Bad-Test Policy And Analyzer

REQ-005: The repository MUST document a delete-bad-tests policy and provide
Go-native analyzer coverage for source-grep and timing/elapsed test assertions,
with explicit exceptions for generated-surface, golden, and contract tests where
text output is the behavior under test.

#### Scenario: Vacuous source-grep test is flagged

GIVEN a Go test reads a `.go` source file and asserts only that the source
contains a string
WHEN the analyzer runs
THEN it reports the source-grep test pattern.

#### Scenario: Legitimate contract text tests are allowed

GIVEN a generated-surface or golden-output contract test asserts expected text
WHEN the analyzer runs
THEN the test is not reported if it matches the documented exception pattern.

### Requirement: Live Tracker Accuracy

REQ-006: The GSD-core borrowable-ideas tracker MUST be updated or closed from
fresh GitHub state after this batch so it does not continue to list closed issue
#258 as an open work item.

#### Scenario: Tracker reflects current open state

GIVEN the implementation and verification work for the current open issue set is
complete
WHEN the open issue list is refreshed
THEN #169 is updated or closed with the current live state and no longer ranks
#258 as open.

### Requirement: Guarded Verification And Rollback

REQ-007: The change MUST retain irreversible-operation guardrails by documenting
rollback, running domain and independent review, and using fresh Slipway
readiness evidence before any completion claim.

#### Scenario: Completion requires governed evidence

GIVEN implementation tasks are complete
WHEN the change approaches closeout
THEN targeted tests, full Go tests, surface manifest checks, domain review,
independent review, and fresh `validate` / `next --diagnostics` readiness output
are recorded before marking the work done-ready.
