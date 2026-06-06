# Requirements
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Engine yields an obviously-not-real scaffold, not fabricated substance (#91)
REQ-001: The engine's default `requirements.md` and `tasks.md` seed (no `--from-doc`)
MUST NOT fabricate plausible normative requirements or tautological scenarios.
`seedRequirements`/`appendRequirementBlock` and `seedTasks` MUST emit an
obviously-not-real honest placeholder scaffold (headings plus guidance prose that
embeds the quality bar and explicit "replace with…" instructions), matching the
honesty of `seedDecision`/`seedResearch`. The default-seeded content MUST be
detected by `LooksLikeTemplatePlaceholder`. The `--from-doc` path MAY still derive
requirement/task titles from the user document but MUST NOT fabricate normative
bodies or tautology scenarios.

#### Scenario: Default seed is detectably non-substantive
GIVEN a change with no `--from-doc` source
WHEN the engine seeds `requirements.md` and `tasks.md`
THEN the seeded requirement body and scenario contain honest placeholder prose
that `LooksLikeTemplatePlaceholder` reports as a placeholder, and the prior
fabricated GIVEN/THEN tautology scaffold lines are no longer emitted.

### Requirement: Placeholder detection covers requirements tautologies (#91)
REQ-002: `LooksLikeTemplatePlaceholder` MUST return true for the requirements
scaffold sentinels — the legacy GIVEN/WHEN/THEN tautology scenario lines, the
honest-seed requirement and scenario placeholder markers, the placeholder
verification-objective marker, and the requirements fallback marker — in addition
to the existing decision/research/task sentinels. (The exact sentinel strings are
pinned in the artifact-package tests; this file paraphrases them to avoid
self-flagging.)

#### Scenario: Requirements tautology strings are recognized
GIVEN text containing a requirements scaffold tautology sentinel
WHEN `LooksLikeTemplatePlaceholder` evaluates it
THEN it returns true.

### Requirement: requirements.md must pass a substance gate, not only structure (#91)
REQ-003: `EvaluateRequirementsContract` MUST, beyond the existing structure checks,
return `invalid` when the requirements lack substance: each `REQ-*` body line MUST
contain an RFC-2119 strong-obligation keyword (`MUST`, `SHALL`, or the equivalent
`REQUIRED`); each requirement block MUST have at least one concrete `#### Scenario`
whose body is neither a tautology sentinel nor a requirements placeholder hit; and
overall placeholder-only content MUST be rejected. Placeholder detection MUST be
requirements-specific so legitimately-authored prose that merely contains a generic
sentinel substring (e.g. "pending investigation") is NOT rejected. Legitimately-authored
requirements (real `MUST`/`SHALL`/`REQUIRED` bodies and concrete scenarios) MUST still
return `valid`.

#### Scenario: Mechanical requirements are rejected
GIVEN a `requirements.md` consisting only of the engine's default placeholder
scaffold (or a requirement whose body has no MUST/SHALL, or only a tautology
scenario)
WHEN `EvaluateRequirementsContract` evaluates it
THEN the result is `invalid` with a message naming the missing substance.

#### Scenario: Authored requirements pass
GIVEN a `requirements.md` whose every `REQ-*` body contains MUST/SHALL and has a
concrete, non-tautology scenario
WHEN `EvaluateRequirementsContract` evaluates it
THEN the result is `valid`.

### Requirement: tasks.md must pass a substance gate (#91)
REQ-004: A tasks substance validator MUST reject a `tasks.md` whose task objectives
are the engine's seeded placeholder markers (the seeded task/verification objective
markers) or otherwise non-substantive, and MUST be wired into the same governed validation
path (`slipway validate` / plan-audit) that consumes the requirements contract.
A real authored task list MUST pass.

#### Scenario: Placeholder tasks are rejected
GIVEN a `tasks.md` whose only task is the engine placeholder objective
WHEN governed validation evaluates the bundle
THEN the tasks substance check reports the bundle invalid with an actionable
message.

#### Scenario: Authored tasks pass
GIVEN a `tasks.md` with real task objectives, target files, and covers mapping
WHEN governed validation evaluates the bundle
THEN the tasks substance check passes.

### Requirement: Placeholder requirements/tasks are rejected by the governed validation path (#91)
REQ-005: A placeholder/seed `requirements.md` or `tasks.md` MUST NOT be counted as
substantive: the progression substance gate (hard, from plan-audit onward) and the
`slipway validate` requirements/tasks contracts MUST reject it. The runtime
substantive-content helper (`artifactSectionHasSubstantiveContent` in
`internal/engine/governance/runtime_actions.go`) MUST stay scoped to `decision.md`
(its only invoked artifacts are `decision.md`/`assurance.md`); it MUST NOT carry a
dead generalization to `requirements.md`/`tasks.md`, whose substance is owned by the
governed validation path above.

#### Scenario: Placeholder requirements/tasks cannot pass governed validation
GIVEN a `requirements.md` or `tasks.md` whose body is placeholder scaffold prose
WHEN governed validation evaluates the bundle at or past plan-audit
THEN it is reported invalid and cannot reach done, while the runtime helper carries
no requirements/tasks placeholder branch.

### Requirement: A public instructions surface serves template + guidance (#91)
REQ-006: `slipway instructions <artifact>` MUST return the artifact template plus
authoring guidance (the quality bar) for a named governed artifact (at least
`requirements` and `tasks`), in both human text and `--json`, so an authoring
skill can read the template and substance bar before writing. An unknown artifact
name MUST produce an actionable error.

#### Scenario: instructions returns template and guidance
GIVEN `slipway instructions requirements` (and `--json`)
WHEN the command runs
THEN it outputs the requirements template and authoring guidance, exit 0.

#### Scenario: unknown artifact errors
GIVEN `slipway instructions not-an-artifact`
WHEN the command runs
THEN it exits non-zero with a message naming the valid artifact names.

### Requirement: Generated surfaces stay aligned with zero drift (#91)
REQ-007: After the source and surface changes, all generated skills, command
references, and docs MUST be regenerated so the toolgen self-loop reports zero
drift, and `go build ./... && go vet ./... && go test ./...` MUST pass.

#### Scenario: Toolgen self-loop is clean
GIVEN the change is implemented
WHEN toolgen regeneration and the drift check run
THEN there is zero drift and build, vet, and test all pass.
