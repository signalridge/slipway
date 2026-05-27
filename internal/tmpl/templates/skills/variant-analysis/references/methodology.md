# Variant-Analysis Methodology

Use when a single confirmed vulnerability needs to be generalized into a
search for related defects — copy-paste siblings, incomplete fixes,
framework-wide misuses. Anchor on the root cause before choosing a tool.

## Why variants cluster

Vulnerabilities co-occur because people and codebases repeat themselves:

- **Author habits** — the same engineer makes the same mistake elsewhere.
- **Copy-paste propagation** — boilerplate carries the defect across files.
- **API misuse** — complex APIs invite a consistent misunderstanding.
- **Framework idioms** — recurring patterns create predictable shapes.
- **Incomplete fixes** — only the reported site was patched.

Understanding *why* a variant exists tells you *where* to look next.

## Step 1 — Root-cause statement

Before writing any pattern, reduce the original finding to one sentence:

> "This bug exists because [UNTRUSTED DATA] reaches [DANGEROUS OPERATION]
> without [REQUIRED PROTECTION]."

Examples:
- "User input reaches `eval()` without sanitization."
- "Attacker-controlled size reaches `malloc()` without bounds check."
- "Untrusted path reaches `open()` without canonicalization."

That sentence *is* your pattern. If you cannot write it, you do not yet
understand the bug well enough to hunt for variants.

## Step 2 — Climb the abstraction ladder

Start specific, generalize one axis at a time.

| Level | What is abstract | Expected matches | FP rate | Typical tool |
|-------|------------------|------------------|---------|--------------|
| 0. Exact match | Nothing | 1 (the original) | 0% | `rg` literal |
| 1. Variable names | Identifiers | 3–5 | Low | `rg` regex, Semgrep `$VAR` |
| 2. Structural | Control flow / call shape | 10–30 | Medium | Semgrep `pattern-inside` |
| 3. Semantic | Source → sink | 50–100+ | High, requires triage | Semgrep taint, CodeQL dataflow |

Choose the level that matches your goal, not your tool preference.

| Goal | Stop at level |
|------|----------------|
| Verify a specific fix landed everywhere | 0 or 1 |
| Find copy-paste siblings | 1 |
| Audit one component | 2 |
| Full product assessment | 3 |

## Step 3 — One change at a time

The cardinal rule: do not abstract two axes in one step. Each refinement
is: *make one change → rerun → review every new match → decide keep or
revert.* Combined jumps hide which change introduced the false positives.

Decision prompts:

- **Abstract this identifier?** Yes if another name could carry the same
  bug; no if the name is a security-critical token (e.g. `isAdmin`).
- **Abstract this literal?** Yes if any value triggers the defect; no if
  the value itself (a shift width, a magic constant) is the hazard.
- **Use `...` wildcards?** Yes when argument position is irrelevant; no
  when only specific positions are sinks.
- **Switch to taint mode?** Yes when presence of the pattern is not proof
  the data actually flows; no when structural match already implies it.

## Step 4 — False-positive budgets

Cap false-positive rate by *context*, not by feel:

| Consumer | Max FP rate |
|----------|-------------|
| CI blocker | <5% |
| Developer warning | <20% |
| Audit triage queue | <50% |
| Research / exploration | <80% |

Common FP filters:

- **Dead code** — `pattern-not-inside: if False: ...`
- **Tests** — exclude `**/test*`, `**/*_test.*`
- **Already sanitized** — `pattern-not: sink(sanitize($X))`
- **Literals** — `pattern-not: sink("...")` to drop hard-coded constants

## Step 5 — Expand the class before stopping

One root cause usually has semantic cousins. Before closing the hunt:

1. **Synonyms** — `isAuthenticated` → also check `isActive`, `isAdmin`,
   `isVerified`; `userId` → also `ownerId`, `creatorId`, `authorId`.
2. **Boolean errors** — inverted conditions, wrong default return,
   short-circuit ordering bugs.
3. **Type edges** — null/None/undefined, empty string, zero, empty list.
4. **Doc mismatches** — function whose docstring contradicts its body
   (e.g. "returns True if access DENIED" but returns True for allowed).
5. **Null-equality bypass** — `a.id == b.id` is true when both sides are
   `None`; audit authorization equality checks for this.

## Multi-repo campaigns

Sequence for cross-repo hunts:

1. **Recon** — ripgrep across mirrors to surface hotspots.
2. **Deep analysis** — Semgrep or CodeQL on each hotspot.
3. **Refinement** — measure FP rate, narrow the pattern.
4. **Automation** — promote stable patterns to CI once FP < 5%.

## Tracking document (mandatory)

A hunt without notes is not reproducible. Keep a per-hunt tracker:

```markdown
## Variant Hunt: [Original Bug ID]

### Root cause
...single-sentence statement...

### Patterns tried
| Pattern | Level | Matches | TP | FP | Notes |

### Confirmed variants
| Location | Severity | Status | Notes |

### FP families
- Pattern X — always FP because ...
- Pattern Y — FP in [context] but TP in [context]
```

## Anti-patterns

- **Starting at Level 3** — you cannot triage 100 findings before you
  understand one.
- **Abstracting everything at once** — you lose the signal on which step
  broke the precision.
- **"I'll triage later"** — false positives are the feedback loop. Read
  them as they come.
- **Tool loyalty** — ripgrep for recon, Semgrep for iteration, CodeQL for
  precision. Use all three.
- **Pattern hoarding** — delete patterns that do not meet the FP budget.

## See also

For engine-specific starter shapes, use `query-patterns.md`. Keep the
anti-predicate workflow and callsite classification rules here.

## Output Schema

Every run should end with one row per callsite:
- `affected`
- `safe-with-reason`
- `needs-followup`

If a callsite is marked safe, cite the exact guard. If it needs follow-up,
record the missing observation that prevented a verdict.
