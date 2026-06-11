# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/engine/artifact/decision_contract.go:34` evaluates `decision.md`
    substance and is the natural home for a structured decision contract result.
  - `internal/engine/artifact/manager.go:468` currently returns only selected
    decision strings from `decision.md`.
  - `cmd/next_skill.go:34` reads `decision.md` and surfaces parsed decisions as
    pending or locked skill constraints.
  - `internal/engine/progression/validation.go:587` enforces decision contract
    blockers at plan-audit and later.
  - `internal/model/reason_code.go` and `internal/model/recovery.go` must expose
    canonical diagnostics for any new fail-closed reason.
- Dependency chains:
  - `decision.md` -> `internal/engine/artifact` parser -> progression readiness
    blockers -> `validate` / `run` / `next` diagnostics.
  - `decision.md` -> shared parser -> `cmd/next_skill.go` -> pending or locked
    decisions in next-skill constraints.
- Blast radius:
  - Narrow: artifact parsing, progression decision blockers, next-skill
    constraints, reason-code taxonomy, and tests.
  - No runtime action, wave execution, state persistence, or archive flow changes
    are needed.
- Constraints:
  - Preserve existing #119 empty-floor behavior for missing, unreadable,
    structurally invalid, and template-only decisions.
  - Preserve issue #140 behavior: live decisions are pending before `G_plan` and
    locked only after `G_plan` is approved.
  - Missing status should remain compatible with existing authored decisions;
    explicit dead or unknown statuses should fail closed once status parsing is
    introduced.

### Patterns
- Existing conventions:
  - Contract checks return string reason specs such as
    `decision_structure_invalid:<detail>` before callers normalize them into
    `model.ReasonCode` values.
  - Artifact-level functions own markdown parsing and placeholder detection.
  - `cmd/next_skill.go` is a consumer of artifact parsing, not a markdown parser
    of record.
- Reusable abstractions:
  - Reuse `markdownSectionLines`, section-structure validation, and placeholder
    handling in `internal/engine/artifact`.
  - Add a typed parsed-decision result near the existing decision contract
    helpers, then keep `ParseDecisionLockedDecisions` as a compatibility wrapper
    over the parsed contract.
  - Add a `ShouldRejectDecisionStatus` helper modeled after GSD Core's
    `shouldRejectAdrStatus` pattern.
- Convention deviations:
  - The change introduces explicit status parsing to Slipway `decision.md`, whose
    current schema has required content sections but no status section.
  - It also introduces an explicit unknown-status blocker; this is stricter than
    today's status-free behavior but only applies when a status section exists.

### Risks
- Technical risks:
  - High: if dead status is parsed only in progression but not in next-skill
    constraints, downstream hosts can still receive a superseded selected
    approach.
  - Medium: if unknown statuses are treated as accepted, typoed or custom dead
    states can bypass the gate.
  - Medium: if missing status is blocked, existing governed bundles become
    incompatible for no issue-required benefit.
  - Low: status normalization can over-match prose; constrain it to explicit
    `Status`/`State`/`Lifecycle`/`Stage` sections.
- Guardrail domains:
  - None of the configured high-risk domains apply. This is governance integrity
    work, not auth, secrets, PII, financial, schema migration, irreversible
    operation, or external API work.
- Reversibility:
  - The code change is reversible. The planned behavior only adds blockers for
    explicit dead or unknown statuses and leaves status-free decisions
    compatible.

### Test Strategy
- Existing coverage:
  - `internal/engine/artifact/decision_contract_test.go` covers decision section
    substance and placeholder rejection.
  - `internal/engine/progression/validation_test.go:417` covers
    `DecisionContractBlockers` lifecycle timing.
  - `cmd/next_skill_constraints_test.go:52` covers selected decision extraction
    and pending-vs-locked routing.
  - `internal/model/reason_code_contract_test.go` freezes reason-code taxonomy.
- Infrastructure needs:
  - Artifact-level table tests for parsed status and selected decision content.
  - A fuzz/property-style test for status normalization and rejection.
  - Progression tests for superseded/deprecated/unknown explicit statuses.
  - Cmd tests for dead decisions not surfacing pending or locked constraints.
- Verification approach:
  - Prove `superseded` and `deprecated` block readiness even when all required
    decision sections are otherwise substantive.
  - Prove explicit unknown status blocks.
  - Prove missing status remains compatible.
  - Prove dead status produces no pending/locked decisions for host handoff.
  - Run targeted packages and then `go test -count=1 ./...`.

### Options
- Option A: Add status checks only to `DecisionContractBlockers`.
  - Tradeoff: plan readiness fails closed, but `cmd/next_skill.go` can still
    parse and surface a dead decision before or outside readiness validation.
  - Verdict: rejected because it does not cover all places that build on
    `decision.md`.
- Option B: Add status checks only to `cmd/next_skill.go`.
  - Tradeoff: host handoff avoids dead decisions, but `validate` and planning
    readiness do not get the fail-closed blocker that issue #163 asks for.
  - Verdict: rejected because it leaves the existing decision contract unaware
    of lifecycle status.
- Option C: Add a shared parsed decision contract in `internal/engine/artifact`,
  consumed by both progression readiness and next-skill constraints.
  - Tradeoff: slightly larger refactor, but it creates one parser/status
    taxonomy and prevents drift between validation and handoff.
  - Selected: implement Option C. Missing status stays compatible; explicit
    `superseded`, `deprecated`, and unknown statuses fail closed. `rejected`
    should also be rejected in the helper because the GSD reference includes it,
    but issue #163 acceptance specifically requires superseded/deprecated.

## Unknowns
- Resolved: exact integration point for refusing a dead decision -> use a shared
  parsed decision contract under `internal/engine/artifact`, then consume it from
  both `DecisionContractBlockers` and `cmd/next_skill.go`.
- Resolved: unknown decision status handling -> missing status is compatible;
  explicit unknown status blocks because issue #163 calls for defining a status
  taxonomy and fail-closed behavior.
- Remaining: None.

## Assumptions
- Missing status should remain compatible with existing decisions. Evidence:
  current `decision.md` schema requires five content sections and no status
  section in `internal/engine/artifact/schemas.yaml`.
- `rejected` belongs in the reject helper even though the issue names
  superseded/deprecated. Evidence: GSD Core's local reference reject set includes
  `superseded`, `rejected`, and `deprecated` in
  `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/adr-parser.cts:14`.
- The issue-number filename and append-only amendment ideas are follow-up scope.
  Evidence: GitHub issue #163 labels those items "Optionally" and the acceptance
  text only requires fail-closed superseded decisions plus parser unit/property
  tests.

## Canonical References
- `https://github.com/signalridge/slipway/issues/163`
- `artifacts/changes/resolve-issue-163-decisions-gate/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `internal/engine/artifact/decision_contract.go`
- `internal/engine/artifact/manager.go`
- `internal/engine/artifact/schemas.yaml`
- `internal/engine/progression/validation.go`
- `cmd/next_skill.go`
- `cmd/next_skill_constraints_test.go`
- `internal/engine/progression/validation_test.go`
- `internal/model/reason_code.go`
- `internal/model/recovery.go`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/src/adr-parser.cts`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/adr/README.md`
