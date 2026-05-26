# Ruleset Catalog

## Official CodeQL Suites

| Suite | False Positives | Use Case |
|-------|-----------------|----------|
| `security-extended` | Low | **Default** - Security audits |
| `security-and-quality` | Medium | Comprehensive review (stable security + code quality) |
| `security-experimental` | Higher | Research, vulnerability hunting (stable security + experimental security) |

> **Suite hierarchy:** `security-and-quality` and `security-experimental` are complementary. `security-and-quality` excludes `experimental/` query paths. `security-experimental` includes them but excludes code quality queries. For maximum coverage (run-all mode), import both.

**Usage:** `codeql/<lang>-queries:codeql-suites/<lang>-security-extended.qls`

**Languages:** `cpp`, `csharp`, `go`, `java`, `javascript`, `python`, `ruby`, `swift`

---

## Trail of Bits Packs

| Pack | Language | Focus |
|------|----------|-------|
| `trailofbits/cpp-queries` | C/C++ | Memory safety, integer overflows |
| `trailofbits/go-queries` | Go | Concurrency, error handling |
| `trailofbits/java-queries` | Java | Security, code quality |

**Install:**
```bash
codeql pack download trailofbits/cpp-queries
codeql pack download trailofbits/go-queries
codeql pack download trailofbits/java-queries
```

---

## CodeQL Community Packs

| Pack | Language |
|------|----------|
| `GitHubSecurityLab/CodeQL-Community-Packs-JavaScript` | JavaScript/TypeScript |
| `GitHubSecurityLab/CodeQL-Community-Packs-Python` | Python |
| `GitHubSecurityLab/CodeQL-Community-Packs-Go` | Go |
| `GitHubSecurityLab/CodeQL-Community-Packs-Java` | Java |
| `GitHubSecurityLab/CodeQL-Community-Packs-CPP` | C/C++ |
| `GitHubSecurityLab/CodeQL-Community-Packs-CSharp` | C# |
| `GitHubSecurityLab/CodeQL-Community-Packs-Ruby` | Ruby |

**Install:**
```bash
codeql pack download GitHubSecurityLab/CodeQL-Community-Packs-<Lang>
```

**Source:** [github.com/GitHubSecurityLab/CodeQL-Community-Packs](https://github.com/GitHubSecurityLab/CodeQL-Community-Packs)

---

## Verify Installation

```bash
# List all installed packs
codeql resolve qlpacks

# Check specific packs
codeql resolve qlpacks | grep -E "(trailofbits|GitHubSecurityLab)"
```

## Language, Threat, and Build Notes

- Treat language selection and build recipe as part of ruleset choice.
  Compiled-language failures usually happen while building the database, not
  while running queries.
- Prefer the smallest suite that still matches the threat under review.
  Broader suites are for scheduled or high-risk passes, not every diff.
- On large repos, narrow scope before reaching for memory/time knobs. If you
  do tune execution, record the knob with the selected pack.
- When a build fails, capture the exact failing step and fix that first. A
  partial database is worse than no result because it looks authoritative while
  silently missing code.
