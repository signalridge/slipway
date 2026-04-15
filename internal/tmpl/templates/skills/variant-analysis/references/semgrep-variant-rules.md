# Semgrep for Variant Hunts

Use when the hunt is structural rather than semantic — you want a pattern
that matches call shapes, argument positions, or nesting, and you are
willing to trade dataflow precision for iteration speed. Defer to
`sast-orchestration/semgrep-*.md` for baseline rule mechanics; the
content below covers only what is specific to variant discovery.

## When Semgrep fits

Pick Semgrep when:

- The variant is **syntactic** — same call, different arguments.
- You need to iterate on the pattern in **minutes**, not hours.
- The codebase has no CodeQL database but has a regular source tree.
- You want **per-repo portability** — Semgrep rules are plain YAML and
  travel with the repo.

Reach for CodeQL instead when dataflow precision or interprocedural
tracking is load-bearing.

## Rule skeleton

```yaml
rules:
  - id: variant-[slug]
    message: |
      Variant of [root cause statement].
      Risk: [untrusted data → dangerous operation without required protection].
    languages: [python]  # or: javascript, typescript, go, java, ...
    severity: WARNING
    metadata:
      category: security
      confidence: HIGH
      references:
        - [link to original finding]
    patterns:
      - pattern: $SINK(..., $TAINT, ...)
      - pattern-inside: |
          def $FN(...):
            ...
      - pattern-not-inside: |
          if is_trusted($TAINT):
            ...
```

Keep `patterns` layered: positive shape first, then `pattern-inside` to
localize, then `pattern-not*` to subtract. The order is the triage
narrative.

## Mode selection

| Mode | When |
|------|------|
| `pattern:` / `patterns:` (search) | Variant is a single call shape |
| `taint:` (with `pattern-sources` / `pattern-sinks`) | Variant requires source-to-sink evidence and structural patterns are not enough |
| `pattern-either:` | Multiple equivalent shapes (e.g. `os.system` and `subprocess.call(..., shell=True)`) |

Avoid `pattern-regex:` for hunts — it bypasses the AST and tends to
produce FPs that no amount of filtering can recover.

## Authoring checklist

- [ ] `metadata.confidence` matches the actual FP budget you plan to run
      against — `HIGH` means production-safe.
- [ ] The rule has at least one positive test case that reproduces the
      original finding, and at least one negative case that mirrors a
      known-safe call.
- [ ] `pattern-not-inside` captures the project's real mitigations, not
      hypothetical ones. Missing a real sanitizer is the top FP source.
- [ ] Rule IDs are stable and namespaced (`variant-[root-cause-slug]`);
      do not rename once in CI.
- [ ] The rule runs in under a few seconds on the target repo — costly
      structural patterns should move to CodeQL.

## Iterative tightening

1. Start with an exact match derived from the original bug location. One
   hit is the correct baseline.
2. Abstract identifiers one at a time; rerun and review every new match.
3. Collapse equivalent shapes into `pattern-either:` once you have two or
   more families.
4. Add `pattern-not-*` subtractions for each FP family you confirm —
   never delete a true positive to silence the rule.
5. Measure FP rate against the rule's declared consumer context (see the
   budgets in the methodology reference). Demote severity or precision
   rather than accepting a budget overrun.

## Taint mode essentials

For variants where presence of a shape is not sufficient proof:

```yaml
rules:
  - id: variant-taint-[slug]
    mode: taint
    pattern-sources:
      - pattern: request.args.get(...)
      - pattern: request.form.get(...)
      - pattern: request.json.get(...)
    pattern-sinks:
      - pattern: cursor.execute($Q)
      - pattern: $CONN.query($Q)
    pattern-sanitizers:
      - pattern: sanitize(...)
      - pattern: bindparam(...)
```

Taint mode is the right default when the variant's essence is "untrusted
data reaches sink". Pure structural patterns are fine when the shape
itself is the bug.

## Common pitfalls

- **Forgetting metavariable constraints** — `$VAR` in `pattern-inside`
  and `$VAR` in `pattern` must match the same node; cross-pattern
  identity is Semgrep's defining feature, not a side effect.
- **Using `...` too liberally** — three wildcards in a row match
  unrelated call shapes. Scope with `pattern-inside` instead.
- **Shipping without a test pack** — a rule without positive/negative
  fixtures drifts the first time the codebase evolves.
- **Re-inventing sast-orchestration rules** — cross-link to the
  checked-in `sast-orchestration/semgrep-*.md` references for shared
  baselines and only capture the variant-specific structure here.

## Integration notes

- Run variant rules as a **separate Semgrep invocation** so their higher
  FP rate does not poison the CI baseline.
- Tag variant rules with `metadata.category: variant` to route their
  findings into the triage queue rather than the blocking queue, until
  FP rate proves they are CI-safe.
- Promote a rule to the blocking ruleset only after it meets the <5% FP
  budget on real PRs for two consecutive releases.
