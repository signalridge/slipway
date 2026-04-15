# SARIF Merge Operator Guide

Synthesized reference. Consolidates operator-facing merge guidance from
`trailofbits/semgrep/scripts/merge_sarif.py` (authoritative merge semantics)
and `trailofbits/audit-augmentation/references/formats.md` (external report
interchange shape). The companion script ships as
`scripts/merge-sarif.sh` under this skill.

## When to merge

- Multi-tool runs: CodeQL + Semgrep + custom rules each emit their own SARIF.
  Downstream gates, triage, and `validate` expect a single file.
- Multi-profile runs: the same scanner executed against `important-only` and
  `run-all` suites. Keep them separate during authoring; merge immediately
  before handoff so rule provenance survives in `tool.driver.rules[]`.
- Multi-target runs: separate monorepo packages scanned in parallel.

## Invariants the merge must preserve

1. **Every run keeps its own `tool.driver`.** SARIF's `runs[]` is the correct
   boundary. Do not flatten `runs[]` into one run; doing that loses `driver`
   metadata and collapses rule IDs that legitimately collide across tools.
2. **Result indices stay consistent.** `results[].ruleIndex` points into the
   owning run's `tool.driver.rules[]`. If merging happens at the results
   level, reindex in lockstep.
3. **Artifact indices stay consistent.** `results[].locations[].physicalLocation
   .artifactLocation.index` points into the owning run's `artifacts[]`. Merge
   artifact tables per run, not globally, unless indices are rewritten.
4. **Deterministic ordering.** Sort `runs[]` by `tool.driver.name`, then by
   scan profile if embedded in `invocations[].properties`. Sort each run's
   `results[]` by `ruleId` then by first location's `artifactLocation.uri`
   then `startLine`. The output must be byte-stable across reruns so CI diffs
   stay signal, not noise.

## Operator recipe

```bash
# Collect per-tool SARIF outputs into one directory
mkdir -p raw-sarif
codeql database analyze db/ --format=sarif-latest --output raw-sarif/codeql.sarif
semgrep --config auto --sarif --output raw-sarif/semgrep.sarif

# Merge into one file before handoff
scripts/merge-sarif.sh raw-sarif merged.sarif
```

The authoritative merge path is the shipped shell + jq helper. It stays
offline and deterministic: sorted input filenames, first-seen tool metadata,
first-seen rule ordering, dedup by `(ruleId, uri, startLine)`, and a
sort-keys JSON output that stays byte-stable across reruns.

## When to collapse vs. keep separate

| Situation | Action |
|-----------|--------|
| Two runs of the same tool on the same target, same config | Drop the older; newer is authoritative. |
| Same tool, different scan profiles (`important-only` vs `run-all`) | Keep separate runs; tag via `invocations[].properties`. |
| Different tools (CodeQL + Semgrep) | Keep separate runs; never merge `driver`. |
| Same tool, different targets (monorepo packages) | Keep separate runs; tag target in `invocations[].workingDirectory`. |

## Common defects caught during merge

- **Rule ID collision across tools.** Semgrep emits numeric short IDs that
  can collide with CodeQL's path-style IDs. Keeping separate `runs[]` avoids
  the collision by preserving per-tool namespaces.
- **Missing artifact index.** Semgrep sometimes omits `artifacts[]` entirely.
  Re-emit a minimal artifacts table derived from
  `results[].locations[].physicalLocation.artifactLocation.uri` so downstream
  readers that assume indexed lookups do not NPE.
- **Version drift.** SARIF 2.1.0 is the working contract. Reject input runs
  with `version` != `2.1.0` fast; a silent upgrade/downgrade is a data
  corruption bug.

## Handoff contract to validate / report

- Consumer reads `merged.sarif` once and walks `runs[].results[]`.
- `ruleId` + tool driver name is the triage key.
- `level` maps to triage severity (`error | warning | note | none`).
- `properties.issue_severity` / `properties.security-severity` may refine
  severity; prefer tool-level property keys over cross-tool normalization
  during merge. Normalization happens at report time, not merge time.

## External interchange

When a downstream consumer expects CSV / JSON / GitHub code-scanning ingest
instead of raw SARIF, defer the translation to the report stage. The merge
step should leave the SARIF shape intact so audit trails survive. See
`trailofbits/audit-augmentation/references/formats.md` (upstream) for the
external-shape mappings.

## Anti-patterns

- Flattening `runs[]` into a single run to "simplify downstream code".
  Loses driver metadata and causes silent rule-id collisions.
- Rewriting `ruleIndex` without re-sorting `rules[]`. Off-by-one breaks
  triage tooling silently.
- Replacing the shipped merge helper with an ad-hoc jq one-liner in CI.
  Keep the deterministic merge contract in the checked-in script so fixture
  tests can pin byte-for-byte output.
