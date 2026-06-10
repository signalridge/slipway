# Concerns

Re-authored for change `resolve-github-issue-157-add-uncheckable-inconclusive-per-it`
(GitHub issue #157).

- Load-bearing invariant: review guidance must not allow unverifiable trace
  mappings to be converted into `pass` evidence. Current
  `spec-trace/CHECKLIST.tmpl:12` only names `covered`, `skipped`, and `drift`,
  which leaves no explicit bucket for a mapping the reviewer could not check.
- Guardrail risk: the change is classified as `external_api_contracts`; review
  evidence must fail closed. `spec-compliance-review/SKILL.md.tmpl:103` already
  has an R3 review layer for guardrail domains, so new guidance should align
  with that stage rather than bypass it.
- Compatibility risk: changing vocabulary too broadly could invalidate existing
  review habits. The safer path is to extend the matrix with `ambiguous` and
  `uncheckable` while keeping existing `covered`, `skipped`, and `drift`
  semantics.
- Overengineering risk: runtime row parsing would add engine behavior that
  Issue #157 says to avoid. Keep this change at the skill-output-contract layer.
- Testing risk: tests that only assert one phrase can miss the full contract.
  Regression assertions should cover the new statuses, required reasons,
  coverage-gap accounting, and pass-blocking semantics.
