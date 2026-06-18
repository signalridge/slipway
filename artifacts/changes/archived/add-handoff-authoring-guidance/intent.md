# Intent

## Summary
Add Slipway guidance for session handoff authoring, skill-template quality
checks, and decision supersession discipline.
## Complexity Assessment
complex
Rationale: this is a meta-profile governance-surface change touching generated
skill templates and their regression tests. The implementation should stay
mostly textual, but the affected surfaces guide future agents and therefore need
clear scope and compatibility checks.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Add an authoring contract for `.git/slipway/runtime/handoff.md` so agents know
  when and how to preserve fresh-session handoff context.
- Add narrowly scoped skill-template quality guidance based on predictable
  skill writing: familiar leading words where useful, reliable context
  pointers, checkable completion criteria, and no-op pruning.
- Add narrowly scoped decision/supersession guidance so replaced decisions are
  marked as superseded rather than left as conflicting live guidance.
- Repair the S1/audit stale-intake dead end discovered while advancing this
  change: once fresh `plan-audit` evidence certifies current planning inputs,
  historical S0 intake digest drift must not block forward-only advancement.
- Review existing generated skill template prose touched by this change and
  remove only obvious high-ROI verbosity where a sentence is repetitive,
  contradictory, or behaviorally inert.

## Out of Scope
- Do not add a new governed lifecycle state or evidence gate for handoff files.
- Do not add a standalone handoff command or governed host skill in this
  change; a pure helper command can be reconsidered later if guidance alone
  does not make agents write useful handoffs.
- Do not treat `handoff.md` as progression authority, freshness input, or skill
  evidence.
- Do not import the full `teach`, `grilling`, or `writing-great-skills` systems
  as user-facing Slipway skills.
- Do not broadly rewrite every generated skill.
- Do not redesign all stale-evidence recovery or weaken the existing
  fail-closed behavior for substantive intake drift before plan-audit is fresh.
- Do not delete useful project knowledge, contract tokens, substantive skill
  guidance, or locally useful prose merely to save tokens.

## Constraints
- Keep Slipway CLI behavior as lifecycle authority; handoff prose must be
  advisory and reverified by `slipway status --json` / `slipway next --json`.
- Preserve generated-surface compatibility across host adapters.
- Use template/content tests for AI-facing contract changes and negative checks
  where old contradictory guidance could reappear.

## Acceptance Signals
- Generated workflow/session guidance explains when to write
  `.git/slipway/runtime/handoff.md`, what to include, what to reference by path,
  how to redact sensitive information, how to name suggested next skills from
  `slipway next --json`, and what must not be inferred from it.
- Shared skill-template guidance names the allowed quality checks without
  replacing Slipway contract tokens such as `next_skill.name`,
  `verification_dir`, or reason codes: familiar leading words, reliable context
  pointers, checkable completion criteria, and no-op pruning.
- Decision guidance documents supersession behavior without adding a new
  artifact type.
- Template tests cover the new handoff contract and quality/supersession
  guidance, including at least one negative assertion against governance bypass
  wording.
- Progression tests prove stale S0 intake evidence still blocks before current
  planning evidence owns scope, but does not block S1/audit advancement after a
  fresh passing `plan-audit` certifies the current plan inputs.

## Open Questions
None

## Deferred Ideas
- A larger generated-skill prose audit can be done later if token profiling or
  user feedback points to specific expensive surfaces.
- A first-class handoff command can be considered later if authoring guidance
  alone proves insufficient.

## Approved Summary
Confirmed by user on 2026-06-18: Add Slipway guidance for
`.git/slipway/runtime/handoff.md` session handoff authoring, plus narrow
skill-template quality and decision supersession guidance. The handoff file
remains advisory context only: it is not lifecycle authority, evidence,
freshness input, a new governed state, or a standalone governed host skill. The
implementation may trim obvious high-ROI verbosity only in affected templates
where a sentence is repetitive, contradictory, or behaviorally inert; it must
not delete useful project knowledge, contract tokens, or substantive skill
guidance merely to save tokens.

Amended on 2026-06-19 after S1 plan-audit: this change also includes the
minimal lifecycle-surface repair needed to continue honestly. Stale historical
S0 intake evidence must not block S1/audit advancement after fresh passing
`plan-audit` evidence owns the current planning input freshness boundary. This
does not allow stale intake evidence to pass during S1/research or without a
fresh plan-audit digest.
