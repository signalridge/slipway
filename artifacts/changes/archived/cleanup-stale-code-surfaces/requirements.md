# Requirements

## Requirements

### Requirement: Current-main cleanup boundary
REQ-001: The change MUST clean only stale/dead/redundant code surfaces that were
confirmed against the current latest `main` in the new governed worktree, and it
MUST NOT use old worktrees as evidence or edit unrelated local dirt.
The canonical cleanup candidate inventory is
`redundancy-candidates.md`; pasted-report scope is not executable
unless captured there.

#### Scenario: cleanup uses current facts
GIVEN the governed worktree is based on the refreshed current `main`
WHEN a cleanup candidate is implemented
THEN the candidate is backed by current-worktree source references, focused tool
output, or both.

#### Scenario: old worktree evidence is excluded
GIVEN older worktrees exist in the repository
WHEN deciding whether a candidate is in scope
THEN those worktrees are not used as source evidence for deletion or retention.

#### Scenario: cleanup inventory is durable
GIVEN `redundancy-candidates.md` lists the cleanup candidates with
candidate IDs, source anchors or tool-output requirements, owning tasks,
expected actions, and allowed prove-still-live outcomes
WHEN implementation and final assurance are recorded
THEN every listed candidate has exactly one recorded disposition in the owning
task evidence or final assurance.

### Requirement: Remove confirmed dead and test-held surfaces
REQ-002: The system MUST remove or inline confirmed unused internal surfaces,
including production wrappers kept alive only by tests, exported internal helpers
with no production consumers, dead fields, and dead methods, while updating tests
to exercise the current production paths.

#### Scenario: test-held wrapper is removed
GIVEN a production wrapper has no production consumer and only test references
WHEN the cleanup is applied
THEN the wrapper is removed or tests are redirected to the production helper that
actually owns the behavior.

#### Scenario: dead model or state field is removed
GIVEN a struct field has no current writer or reader and is not part of a kept
public behavior
WHEN the cleanup is applied
THEN the field and any stale tests or snapshots that only preserve it are
removed.

### Requirement: Remove inert retired and compatibility wiring
REQ-003: The system MUST remove confirmed inert retired-feature wiring and
obsolete local-state reader shims that only preserve historical compatibility,
including retired workflow-state canonicalization and legacy handoff hygiene,
while preserving fail-closed defenses that reject retired inputs.

#### Scenario: inert closeout conditional wiring is removed
GIVEN `CloseoutConditional` is never set by the current skill registry
WHEN required-skill filtering and verification readiness are cleaned up
THEN the unused field, parameter threading, and tests that assert the old split
are removed without weakening live ship-verification enforcement.

#### Scenario: fail-closed retired input defense is preserved
GIVEN retired context origin tokens such as `goal` and `closeout` are rejected by
current validation
WHEN compatibility cleanup is applied
THEN those rejection paths remain active and are not treated as removable shims.

#### Scenario: retired workflow state compatibility is removed
GIVEN old lifecycle states such as `S2_EXECUTE` and `S4_VERIFY` are no longer
current workflow states
WHEN no-backward-compatibility cleanup is applied
THEN the reader no longer canonicalizes those states into current states, and
tests that preserved that compatibility are removed or rewritten. This is an
intentional behavior change: change loading no longer normalizes those retired
states, and source/test/doc/README surfaces do not keep those retired workflow
state tokens as compatibility fixtures.

### Requirement: Remove no-longer-emitted reason codes
REQ-004: The system MUST delete reason-code catalog, remediation, and snapshot
entries that current gates no longer emit, and it MUST update tests so reason
coverage reflects only live engine outputs or explicit negative contracts.

#### Scenario: stale reason code is removed
GIVEN a reason code exists only in the catalog/remediation tests and no current
gate emits it
WHEN the reason-code cleanup is applied
THEN the catalog, remediation table, frozen snapshot, and tests no longer retain
that code.

### Requirement: Resolve lint-confirmed cleanup and duplication
REQ-005: The system MUST resolve the lint-confirmed cleanup issues from the
focused `unused`, `unparam`, `staticcheck`, `ineffassign`, and `wastedassign`
passes and consolidate low-risk duplicate command wiring where behavior is
identical.

#### Scenario: focused lint passes are clean
GIVEN the cleanup has been implemented
WHEN the focused lint commands are run with `--tests=false`
THEN the previously confirmed cleanup findings no longer appear.

#### Scenario: duplicate command wiring is consolidated
GIVEN two command helpers only differ by concrete view type but perform the same
path-authority assignment
WHEN the cleanup is applied
THEN a shared helper owns the common behavior and public command output remains
covered by existing tests.

### Requirement: Verify lifecycle and generated surfaces
REQ-006: The change MUST finish with current-worktree verification evidence,
including Go tests, default lint, focused cleanup lint checks, generated surface
manifest verification when relevant, and Slipway lifecycle readiness.

#### Scenario: verification proves cleanup
GIVEN the cleanup code and artifacts are final
WHEN verification is run
THEN `go test ./...`, default `golangci-lint run ./...`, the focused cleanup
lint passes, and Slipway validation all provide current evidence for readiness.

#### Scenario: generated surfaces remain synchronized
GIVEN command surfaces or generated public metadata are touched
WHEN the surface manifest check is run
THEN `go run ./internal/toolgen/cmd/gen-surface-manifest --check` reports that
generated docs are up to date.

### Requirement: Remove dead config and public no-op surfaces
REQ-007: The system MUST remove confirmed no-op configuration, state, and
public command surfaces, including no-op validation flags, write-only review
drift counters, and no-op `done/validate --json` flags, while preserving live
artifact schema behavior.

#### Scenario: live artifact schemas are preserved
GIVEN current worktree facts show `core`, `expanded`, and `custom` artifact
schemas are decoded, validated, and wired into runtime artifact resolution
WHEN cleanup is applied
THEN the cleanup does not remove `custom_artifacts`, the `custom` schema, or the
current `core`/`expanded` behavior.

#### Scenario: validation no-op flags are removed
GIVEN validation config flags only round-trip in configuration and do not alter
requirements enforcement
WHEN cleanup is applied
THEN those no-op flags and related catalog/config-command tests are removed
rather than preserved as inert configuration.

#### Scenario: public no-op flags are retired
GIVEN `done --json` and `validate --json` are no-op compatibility tokens because
those commands already emit JSON
WHEN cleanup is applied
THEN the flags, generated command metadata, docs, and tests are updated to the
new public contract.

### Requirement: Consolidate confirmed redundant implementations
REQ-008: The system MUST consolidate the remaining confirmed redundant
implementations from `redundancy-candidates.md` in the same
governed change, including command route/freshness wiring,
`statusRoute` vs route-kind overlap,
`EvidenceFreshness` vs `ExecutionEvidenceFreshness` synchronization,
`cmd/tool_github` pagination/check-run/status extraction duplication, stale
evidence repair predicates, S3 review template text, artifact contract helper
boilerplate, strict YAML cache loaders, load-error wrappers,
`blockerRemediations` vs `canonicalReasonDefinitions` drift, test verification
helpers, and tiny command `findRepoRoot` duplication.

#### Scenario: command wiring duplication is removed
GIVEN command view helpers repeat route, path-authority, readiness freshness, or
remediation wiring with identical behavior
WHEN cleanup is applied
THEN the repeated wiring is owned by shared helpers or a single route source, and
covered command JSON/text behavior remains intentional, including explicit
disposition of `statusRoute` vs route-kind overlap and `EvidenceFreshness` vs
`ExecutionEvidenceFreshness` synchronization.

#### Scenario: GitHub helper duplication is removed
GIVEN `cmd/tool_github` carries duplicate pagination, check-run envelope, and
combined status extraction helpers
WHEN cleanup is applied
THEN the common backend-agnostic behavior is shared or otherwise reduced with
focused tests preserving GitHub output semantics.

#### Scenario: engine helper duplication is removed
GIVEN engine/state helpers repeat stale-evidence repair predicates, strict cache
loaders, load-error wrappers, artifact contract read/empty handling, or
reason/remediation key ownership
WHEN cleanup is applied
THEN the repeated behavior is consolidated without weakening strict decoding,
fail-closed recovery, or artifact contract validation, and
`blockerRemediations` vs `canonicalReasonDefinitions` drift is addressed by a
single ownership or completeness check.

#### Scenario: template and test helper duplication is removed
GIVEN S3 review skill templates and test packages repeat the same disk-handoff,
record-verification, or verification-writing helper contracts
WHEN cleanup is applied
THEN a shared template partial or test helper owns the repeated contract, and
generated skill/template tests remain synchronized.

#### Scenario: tiny binary root discovery duplication is removed
GIVEN tiny internal command binaries each carry their own `findRepoRoot`
implementation
WHEN cleanup is applied
THEN root discovery is shared through the internal filesystem helper layer and
both binaries keep their existing current-root behavior.
