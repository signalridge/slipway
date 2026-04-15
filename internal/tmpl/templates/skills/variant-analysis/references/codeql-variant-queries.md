# CodeQL for Variant Hunts

Use when the hunt needs dataflow precision a regex or Semgrep structural
pattern cannot provide — cross-file taint, interprocedural sinks, or
conditions that only matter when a specific predicate holds. Defer to
`sast-orchestration/codeql-*.md` for language-agnostic CodeQL foundations
and bring the variant-specific structure below on top.

## When CodeQL is the right tool

Pick CodeQL over ripgrep or Semgrep when:

- The variant requires **interprocedural dataflow** — source and sink are
  in different functions or files.
- You need **path-sensitivity** — the bug only triggers on certain
  branches or after a specific guard fails.
- You are building a **durable CI rule** and a <5% FP budget is required.
- The variant class is semantic (taint, type hierarchy, control
  dependence), not lexical.

Otherwise stay with the cheaper tools — CodeQL's authoring cost only pays
off for classes where precision matters.

## Query skeleton

All variant queries share the same three moving parts: a source
predicate, a sink predicate, and an absent-sanitizer predicate. Keep them
separate so each can evolve without rewriting the query.

```ql
/**
 * @name [Root cause, one line]
 * @description Variants of CVE-[id] / issue-[id]
 * @kind path-problem
 * @problem.severity warning
 * @precision high
 * @id [org]/[lang]/variant-[slug]
 * @tags security
 */

import [language]
import semmle.code.[language].dataflow.TaintTracking
import DataFlow::PathGraph

class VariantConfig extends TaintTracking::Configuration {
  VariantConfig() { this = "VariantConfig" }

  override predicate isSource(DataFlow::Node n) { /* ... */ }

  override predicate isSink(DataFlow::Node n) { /* ... */ }

  override predicate isSanitizer(DataFlow::Node n) { /* ... */ }
}

from VariantConfig cfg, DataFlow::PathNode src, DataFlow::PathNode snk
where cfg.hasFlowPath(src, snk)
select snk.getNode(), src, snk, "Untrusted data reaches [sink] from $@", src.getNode(), "this source"
```

## Authoring checklist

Before declaring a query "done":

- [ ] `@precision` matches the CI budget. `very-high` → CI blocker;
      `high` → developer warning; `medium` → audit triage.
- [ ] The `isSource` predicate matches the variant's original taint
      origin (request parameter, env var, deserialized input), not just
      "anything that looks like input".
- [ ] `isSanitizer` encodes every mitigation the codebase actually uses —
      missing sanitizers are the top FP source.
- [ ] At least one negative test case (a sanitized flow) is in the
      `test/` folder and fails the query as expected.
- [ ] At least one positive test case (the original bug, minimized) is in
      the `test/` folder and succeeds.

## Iterative tightening

Run the query on the same fixture after each change:

1. Start with the original bug as the only positive fixture.
2. Add sanitized analogs one at a time and confirm each fails to trigger.
3. Add near-miss cases (partial sanitation, wrong sanitizer) and confirm
   each *does* trigger — these are real variants.
4. Measure on the full repo. If FP rate exceeds the budget for
   `@precision`, demote precision or tighten the predicates, never the
   other way around.

## Query families to consider

| Family | When to use | CodeQL primitive |
|--------|-------------|------------------|
| Direct dataflow | Simple source-to-sink | `DataFlow::Configuration` |
| Taint tracking | Allows limited transformation | `TaintTracking::Configuration` |
| Control-flow reachability | Guard deleted or reordered | `ControlFlow::successorOf` |
| Type hierarchy | API consumers that skip a base validator | `Class.getASupertype*()` |
| Absent-method | Expected guard call missing | `not exists(MethodCall)` |

## Common pitfalls

- **Under-specified sources** — taint from *any* method on the request
  object, not just the user-controlled one. Narrow to the exact accessor
  path (e.g. `request.body.raw`, not `request`).
- **Over-broad sinks** — modeling "any call into library X" as a sink
  floods the hunt. Pin the actual dangerous API.
- **Skipping sanitizer modeling** — a query with no sanitizers will flag
  every real fix as a variant. Model the fix patterns before shipping.
- **Shared state between queries** — give each query its own
  configuration class; collisions silently merge predicates.

## Lifecycle

1. **Prototype** in the CodeQL VS Code extension against a minimized
   fixture — stay off the full repo until the query holds on fixtures.
2. **Measure on the real repo** only once the fixture pack passes.
3. **Commit the query plus its test pack** together; a query without
   fixtures rots within a release.
4. **Promote to CI** only after two consecutive merges with <5% FP on
   real PR diffs.

For ruleset packaging, invocation commands, and how CodeQL rules relate
to the rest of the SAST surface, defer to
`sast-orchestration/codeql-*.md`; duplicating that material here drifts
out of sync.
