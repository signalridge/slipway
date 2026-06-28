# Requirements

### Requirement: RouteAlgebraSurface
REQ-001: Public lifecycle commands MUST expose machine-readable invocation routes for successful, diagnostic, and precondition-blocked paths.

#### Scenario: Non-success paths expose route kind
GIVEN a command resolves no active change, multiple active changes, an explicit missing slug, or a change bound to another worktree
WHEN the command emits JSON or a CLIError payload
THEN the payload includes `invocation_route.kind` with `no_active`, `multi_active`, `explicit_missing`, or `bound_elsewhere` as appropriate.

#### Scenario: Archived local worktree is distinguishable
GIVEN an archived change still has a local worktree
WHEN status resolves that local archived authority
THEN the route kind is `archived_local` and lifecycle execution remains disabled.

### Requirement: FreshnessSurfaceParity
REQ-002: `next` and `done` MUST expose the same split freshness concepts already present on `status` and `validate`.

#### Scenario: Next freshness cannot hide blockers
GIVEN host capability or governance blockers are appended while building a next view
WHEN `next` or `run --diagnostics` renders JSON
THEN `execution_evidence_freshness`, `governance_evidence_freshness`, and `overall_readiness_freshness` are present and overall readiness is `blocked` when blockers are present.

#### Scenario: Done reports pre-archive readiness freshness
GIVEN a done-ready change is finalized
WHEN `done --json` succeeds
THEN the response reports execution, governance, and overall readiness freshness from the pre-archive state.

### Requirement: StatusFreshnessProse
REQ-003: Human `status` output MUST NOT present execution freshness as a single overall freshness value.

#### Scenario: Split human freshness
GIVEN status has separate execution, governance, and overall readiness freshness values
WHEN text status is rendered
THEN the output prints all three labels and does not print `Evidence Freshness: fresh` as a single-line overall claim.

### Requirement: HostCapabilityRegistry
REQ-004: Selected governance skill host capability requirements MUST be declared in machine-readable registry/template metadata instead of resolver-only skill-id hardcoding.

#### Scenario: Registry-owned independent-review capability
GIVEN `independent-review` is selected
WHEN the host capability resolver evaluates it
THEN the `subagent` requirement, explicit fallback modes, evidence requirement, and remediation come from registry metadata and mirror skill template frontmatter.

#### Scenario: Capability aliases stay bounded
GIVEN a future skill declares a non-subagent capability
WHEN the host reports only the `delegation` alias
THEN that alias does not satisfy unrelated capabilities.
