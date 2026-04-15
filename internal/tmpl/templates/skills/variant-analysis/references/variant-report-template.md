# Variant Hunt Report Template

Use as the deliverable when closing a variant hunt. The structure pins
the root cause, the patterns tried, the variants confirmed, and the FP
families the next hunt will see. Copy the scaffold below into the
tracking document; fill only the sections the hunt actually exercised.

## 1. Header

```markdown
# Variant Hunt: [Original Bug ID or CVE]

- **Seed finding:** [link]
- **Hunt owner:** [name]
- **Date range:** [YYYY-MM-DD] – [YYYY-MM-DD]
- **Scope:** [repo / component / language]
- **Status:** [draft | in triage | complete]
```

## 2. Root cause

One sentence in the form used in `methodology.md`:

> "This bug exists because [UNTRUSTED DATA] reaches [DANGEROUS
> OPERATION] without [REQUIRED PROTECTION]."

If you cannot state it in one sentence, stop and return to root-cause
analysis. A hunt without a clean root-cause statement is not a hunt.

## 3. Pattern ladder

Every pattern tried, in climb order. The value of this table is that the
*next* hunt reviews it before re-deriving patterns from scratch.

```markdown
| Step | Level | Tool      | Pattern excerpt           | Matches | TP | FP | Action |
|------|-------|-----------|---------------------------|---------|----|----|--------|
| 1    | 0     | rg        | [literal seed line]       | 1       | 1  | 0  | keep   |
| 2    | 1     | Semgrep   | $SINK(..., $VAR, ...)     | 4       | 3  | 1  | keep   |
| 3    | 2     | Semgrep   | + pattern-inside def $FN  | 17      | 9  | 8  | narrow |
| 4    | 3     | CodeQL    | TaintTracking + sanitizer | 6       | 6  | 0  | ship   |
```

Annotations to record per row:

- **Action**: `keep`, `narrow`, `widen`, `revert`, `ship`, or `drop`.
- **Notes**: one line on what the step *taught you* — the learning, not
  the syntax.

## 4. Confirmed variants

```markdown
| # | Location          | Severity | Fix status | Ticket | Notes |
|---|-------------------|----------|------------|--------|-------|
| 1 | path/to/file.py:L42 | High    | patched    | ABC-1  | Same copy-paste |
| 2 | path/to/other.go:L17 | Medium | in review  | ABC-2  | Missing sanitizer |
```

For each confirmed variant, link to the patch (or the open ticket) so a
reader can trace closure without rerunning the hunt.

## 5. False-positive families

This is the section that prevents the next hunt from repeating your
work. Record every FP class and *why* it is an FP:

```markdown
- **Guarded by `is_internal()`** — called only from the internal admin
  CLI, where the input is trusted. FP in all contexts.
- **Test fixtures** — `tests/fixtures/*.py` constructs literal payloads
  as part of the test setup. Exclude with `--glob '!**/tests/**'`.
- **Dead branch** — variants inside `if settings.LEGACY_V1:`. Legacy
  flag is disabled in prod and slated for removal.
```

A bare "FP list" without rationale is not useful. The rationale is what
lets a future hunter decide whether the FP is still an FP in their
context.

## 6. Pattern durability

Mark each surviving pattern's intended destination:

| Pattern | Destination | Precondition for promotion |
|---------|-------------|----------------------------|
| `variant-[slug]-literal` | delete | one-shot, served its purpose |
| `variant-[slug]-structural` | audit ruleset | keep at WARNING |
| `variant-[slug]-taint` | CI blocker | two releases at <5% FP |

A pattern that has no destination is noise and should be deleted in the
closeout commit.

## 7. Metrics

Close the hunt with concrete numbers:

```markdown
- Patterns written: N
- Patterns shipped: N
- Variants confirmed: N
- Variants fixed by end of hunt: N
- Cumulative FP rate on shipped patterns: N%
- Time invested: [author-hours]
```

Teams that aggregate these numbers across hunts start spotting classes
(e.g. "authorization-equality bypass" recurs every quarter), which is
where hunting turns into prevention.

## 8. Follow-ups

Explicitly list what was **not** done so it is not mistaken for closure:

- Variant classes deferred to a later hunt (and why now is not the time).
- Sibling repos or mirrors not covered.
- Tooling gaps surfaced (missing CodeQL dataflow library, missing
  Semgrep sanitizer, missing ripgrep mirror).

Each follow-up gets a ticket link. Unlinked follow-ups are not
follow-ups, they are intentions.

## 9. Cross-links

- Seed bug / CVE report.
- Merged patches for each confirmed variant.
- Rule / query files added to `sast-orchestration` or other rulesets.
- Related prior hunts (especially hunts with overlapping FP families).

When you hand this report to an author or a release gate, they should
need nothing else in hand to reproduce, extend, or defer the hunt.
