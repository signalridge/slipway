# Research

## Alternatives Considered

### Architecture
- Affected modules: generated AI-facing guidance under
  `internal/tmpl/templates/skills/`, shared template references under
  `internal/tmpl/templates/skills/_shared/references/`, governed artifact
  templates under `internal/tmpl/templates/artifacts/`, content tests in
  `internal/tmpl/templates_test.go`, and cross-adapter generation tests in
  `internal/toolgen/toolgen_test.go`.
- Amended affected modules: stale-evidence repair selection under
  `internal/engine/progression/evidence_repair.go` and its regression tests
  under `internal/engine/progression/evidence_repair_test.go`.
- Runtime touchpoint: `cmd/context_pressure_hook.go` may need wording that tells
  an agent where the handoff contract lives, but `cmd/session_start_hook.go`
  should keep surfacing only `session_handoff_present` and
  `session_handoff_path`.
- Dependency chains: template source -> `internal/tmpl` rendering -> generated
  host/command skills; hook runtime -> session/context-pressure output -> agent
  behavior.
- Blast radius: low to medium. The intended change is mostly prose plus tests,
  but the prose guides governed agents and can accidentally become
  pseudo-authority.
- Constraints: lifecycle transitions, gates, freshness, and next-host routing
  remain CLI-owned. `.git/slipway/runtime/handoff.md` is advisory context only,
  not evidence, not a gate, and not a source for lifecycle inference.

### Patterns
- Existing conventions: the workflow skill already states that the CLI owns
  transitions, gates, and command semantics, and that `next_skill.name` from
  `slipway next --json` is the governed-host handoff.
- Reusable abstractions: the existing shared checklist reference is a compact
  place for skill-template quality criteria; the existing `decision.md` template
  is the right place for supersession guidance.
- External references reviewed: local `mattpocock/skills` productivity skills
  support a targeted adoption model:
  - `handoff`: useful as a pure authoring contract, with reference-not-duplicate,
    redact-sensitive-data, and next-session-focus discipline.
  - `writing-great-skills`: useful as a review checklist for predictable skill
    prose: familiar leading words, reliable context pointers, checkable and
    where necessary exhaustive completion criteria, and no-op pruning.
  - `teach`: useful only for the narrow supersession discipline: mark replaced
    records as superseded rather than deleting history.
  - `grilling`: not in scope; Slipway's intake clarification already carries
    the useful "one question at a time, recommend an answer, inspect code first"
    behavior.
- Convention deviations: no new exported user skill should be added for
  `writing-great-skills` or `teach`; their useful pieces should be folded into
  existing Slipway authoring surfaces.
- Cross-project corroboration: local OpenAI and Anthropic skill-creator
  references support the same narrow direction: descriptions and pointers are
  trigger surfaces, reference material must say when it should be read, and
  skill changes need tests/evals that prove behavior changes. These references
  do not justify importing another skill system into Slipway.
- Engineering-skill boundary: `domain-modeling`, `prototype`, and `to-issues`
  are useful for product discovery, state-model exploration, and vertical-slice
  issue creation, but they are out of scope for this handoff-authoring change.

### Risks
- Medium: agents could treat a handoff narrative as lifecycle authority.
  Mitigation: write explicit negative guidance and add tests that reject bypass
  wording.
- Medium: stale historical intake evidence can create an impossible S1/audit
  replay path after plan-audit has certified current planning inputs.
  Mitigation: keep intake drift fail-closed before fresh plan-audit, then let
  current plan-audit freshness supersede historical S0 intake drift at S1/audit.
- Medium: importing full external productivity systems would increase context
  load and create parallel workflows. Mitigation: borrow only the narrow
  contract/checklist/supersession disciplines.
- Low: token-saving cleanup could remove useful project knowledge. Mitigation:
  limit cleanup to touched surfaces and only remove repetitive, contradictory,
  or behaviorally inert sentences.
- Low: adding a new shared reference could be missed by adapter generation.
  Mitigation: prefer existing referenced surfaces unless a new reference is
  clearly necessary and tested through rendered content.
- Guardrail domains: no auth, credentials, financial, schema migration,
  irreversible operation, or external API contract domain is touched.
- Reversibility: textual template changes and tests can be reverted cleanly.
  The runtime handoff file remains outside governed evidence.

### Test Strategy
- Existing coverage: `internal/tmpl/templates_test.go` already pins shared
  reference content, rendered command entries, and negative assertions against
  stale template prose.
- Required additions:
  - Assert rendered workflow guidance includes the complete handoff authoring
    contract, including when to write it, what to include, and what to reference
    by path; run and context-pressure surfaces may point to that contract but
    must not fork a second contract.
  - Assert handoff guidance includes suggested next skills from fresh
    `slipway next --json` output and redaction of secrets, credentials, and PII.
  - Assert generated guidance keeps `handoff.md` non-authoritative: no lifecycle
    bypass, no evidence gate, no freshness inference, and no standalone governed
    host skill.
  - Assert SessionStart remains path-only if touched: it may surface
    `session_handoff_present` and `session_handoff_path`, but it does not embed
    or interpret the handoff body.
  - Assert shared checklist prose includes leading words, checkable completion
    criteria, reliable context pointers, and no-op pruning without replacing
    Slipway contract tokens such as `next_skill.name`, `verification_dir`, or
    reason codes; if the shared prose grows beyond a compact scoped section,
    split it into a named shared reference and cover generation of that reference.
  - Assert decision guidance includes supersession behavior without creating a
    new artifact type or importing `teach` learning-record numbering.
- Verification commands: targeted `go test ./internal/tmpl/...`; targeted
  `go test ./internal/toolgen/...`; add `go test ./cmd/...` if runtime hook text
  changes; targeted `go test ./internal/engine/progression/...` for the
  stale-intake lifecycle unblock; final `go test ./...`. A throwaway
  `slipway init --tools all` smoke can provide extra generated-surface evidence.

### Options
- Option A - targeted authoring-contract adoption: add handoff authoring
  guidance to existing generated workflow/session surfaces, fold skill-quality
  checks into the shared checklist, add supersession guidance to the decision
  template, and pin the contracts with template tests. Tradeoff: smallest
  mechanism change and strongest compatibility, but no first-class handoff
  command yet.
- Option A2 - standalone handoff helper command or user-invoked skill: add a
  direct surface to template or write `.git/slipway/runtime/handoff.md`.
  Tradeoff: could standardize authoring later, but it is premature before the
  missing writing contract exists and would widen this change.
- Option B - first-class handoff lifecycle mechanism: add a command, lifecycle
  gate, or evidence model for `.git/slipway/runtime/handoff.md`. Tradeoff:
  stronger enforcement, but it contradicts the intended advisory-only role and
  would create a second source of lifecycle truth.
- Option C - import the full productivity skill systems: add full
  `handoff`, `writing-great-skills`, and `teach`-style skills to Slipway.
  Tradeoff: more reusable vocabulary, but high context load, duplicate workflow
  concepts, and too much surface area for this change.
- Selected: Option A. It matches the user's explicit boundary: item 1 is the
  framework-level gap; items 2 and 3 are partial borrowings; broad deletion or
  full external-skill import is out of scope.
- Amendment: include the minimal stale-intake S1/audit unblock discovered while
  advancing this change. This is not a new lifecycle mechanism; it aligns stale
  repair target selection with the existing rule that later research/plan
  evidence owns durable intent freshness after intake.

## Unknowns
- Resolved: latest main was already at `532ed3250d5399a16ee01ca4a658ec5e989f7e21`
  before this change was recreated.
- Resolved: the stale prior change state was deleted before recreating
  `add-handoff-authoring-guidance`.
- Resolved: codebase map status is now populated and scoped to this template
  and hook surface.
- Resolved: local external references exist under
  `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/`.
- Resolved: explicit research confirmation has already advanced the change to
  `S1_PLAN/audit`.

## Assumptions
- The external productivity skills are references, not imported dependencies.
  Evidence: user asked to "参考" local ghq skills and separately constrained
  that only 1 is framework-needed while 2 and 3 are partial borrowings.
- The implementation should preserve generated-surface compatibility across
  Codex, Claude, and other adapters. Evidence:
  `internal/toolgen/toolgen_test.go:631-666`.
- A standalone handoff helper command can be reconsidered after this change if
  agents still write poor handoffs, but the current acceptance target is
  advisory guidance only. Evidence: the confirmed scope keeps handoff outside
  lifecycle authority and evidence.
- It is acceptable for the runtime context-pressure message to point agents at
  the handoff authoring contract, but not to embed or interpret handoff body
  content. Evidence: `cmd/session_start_hook.go:126-135` and
  `cmd/session_start_hook.go:166-170`.
- Plan audit should author the formal `decision.md`; research only selects the
  approach and records evidence. Evidence:
  `.codex/skills/slipway-research-orchestration/SKILL.md`.

## Canonical References
- `artifacts/changes/add-handoff-authoring-guidance/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/codebase/CONVENTIONS.md`
- `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`
- `internal/tmpl/templates/_partials/command-run-body.tmpl`
- `internal/tmpl/templates/skills/_shared/references/checklist-quality.md`
- `internal/tmpl/templates/artifacts/decision.md`
- `internal/tmpl/templates_test.go`
- `internal/engine/progression/evidence_repair.go`
- `internal/engine/progression/evidence_repair_test.go`
- `cmd/session_start_hook.go`
- `cmd/context_pressure_hook.go`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/handoff/SKILL.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/writing-great-skills/SKILL.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/writing-great-skills/GLOSSARY.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/teach/SKILL.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/productivity/teach/LEARNING-RECORD-FORMAT.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/engineering/domain-modeling/SKILL.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/engineering/prototype/SKILL.md`
- `/Users/yixianlu/ghq/github.com/mattpocock/skills/skills/engineering/to-issues/SKILL.md`
- `/Users/yixianlu/ghq/github.com/openai/skills/skills/.system/skill-creator/SKILL.md`
- `/Users/yixianlu/ghq/github.com/anthropics/skills/skills/skill-creator/SKILL.md`
