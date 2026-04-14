---
skill_id: variant-analysis
domain: review-change-shape
function: hunt additional variants of a known bug across the codebase
tier: T1
primary_attachment: procedure
summary: "Use when a bug has been fixed in one place and variants elsewhere are plausible. Triggers on review or repair commands or user text asking for similar-bug hunts."
trigger_signals:
  - command: ["review", "repair"]
    reason: "Review or repair command invoked; hunt variants of the fix"
  - user_text_matches: ["variants", "similar bug", "elsewhere"]
    reason: "User text asks for variant hunting"
evidence_contract: artifact
bindings:
  - type: command-manual
    target: review
    attachment: procedure
  - type: command-manual
    target: repair
    attachment: procedure
provenance_ref: provenance.yaml
---

# Variant Analysis

```
IRON LAW: IF ONE SITE HAD THIS BUG, OTHER SITES PROBABLY DO TOO
```

## Purpose
A bug fixed in one place is often a bug pattern. Hunt the pattern across the
codebase before closing the ticket. The output is a list of callsites, each
labelled `affected`, `safe-with-reason`, or `needs-followup`.

## Procedure
1. Distill the fixed bug into a pattern: the anti-predicate that held at the
   bug site (e.g., "unvalidated user input reaches SQL builder").
2. Search the codebase for occurrences of the anti-predicate, not the exact
   code. Use structural or semantic queries (grep at minimum; CodeQL /
   Semgrep if available).
3. For each callsite, classify: `affected` (same bug), `safe-with-reason`
   (cite the guard that blocks it), `needs-followup` (cannot quickly tell).
4. Fix `affected` variants in the same change if scope allows; file followups
   for `needs-followup` with the callsite citation.
5. Record the pattern in the provenance artifact so future changes can check
   against it.

## Checklist
- [ ] Pattern written as an anti-predicate, not a string match.
- [ ] Every callsite classified.
- [ ] `safe-with-reason` cites the specific guard.
- [ ] `needs-followup` callsites are ticketed with citation.
- [ ] Pattern recorded in the change's provenance artifact.

## Anti-patterns
- Declaring "grep was clean" without writing down the pattern.
- Fixing one variant and ignoring the rest to keep the diff small without a
  ticket trail.
- Classifying callsites as `safe` without citing the guard.
