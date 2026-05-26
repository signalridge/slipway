# Query Patterns

Starter patterns for turning a confirmed bug shape into a broader search. Keep
the pattern grounded in the anti-predicate from `methodology.md`; do not jump
to tool syntax before the bug statement is stable.

## CodeQL Starter Shape

- model the untrusted source, sink, and required guard separately
- validate the original bug site first, then generalize
- keep one language and one sink family per pass

Pseudo-shape:

```ql
from Source src, Sink sink
where src.flowsTo(sink)
  and not exists(Guard g | g.protects(src, sink))
select sink, "Untrusted data reaches sink without the required guard."
```

## Semgrep Starter Shape

- start structural before going taint-mode
- abstract one axis at a time: identifier, then literal, then surrounding control flow
- require the missing guard in the same rule so matches stay triageable

Pseudo-shape:

```yaml
rules:
  - id: missing-guard-variant
    patterns:
      - pattern: dangerous_call(...)
      - pattern-not-inside: |
          if required_guard(...):
            ...
    message: Dangerous call without the required guard
```

## Triage Reminders

- Exact-match clean means nothing unless the anti-predicate was written down.
- A large result set is not success; classify or tighten the pattern.
- When CodeQL and Semgrep disagree, treat that as a clue about abstraction
  level, not proof that one tool is wrong.
