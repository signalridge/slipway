---
skill_id: sast-orchestration
domain: review-security
function: run SAST tooling (CodeQL/Semgrep) with SARIF triage
tier: T2
primary_attachment: tool-recipe
summary: "Use when running SAST tooling against the change. Triggers on review, validate, or repair commands and user text naming SAST tools."
trigger_signals:
  - command: ["review", "validate", "repair"]
    reason: "Review/validate/repair command invoked; SAST may apply"
  - user_text_matches: ["codeql", "semgrep", "sast", "sarif"]
    reason: "User text names a SAST tool"
evidence_contract: artifact
hydrate_references:
  - name: codeql-ruleset-catalog.md
    reason: "Pick a CodeQL query pack by threat model and language"
  - name: codeql-language-details.md
    reason: "Language-specific CodeQL build and analysis caveats"
  - name: codeql-threat-models.md
    reason: "Threat-model selection for CodeQL run scoping"
  - name: codeql-performance-tuning.md
    reason: "Scan-time / memory knobs for large repos"
  - name: codeql-build-fixes.md
    reason: "Common build failures that block the CodeQL database"
  - name: semgrep-rulesets.md
    reason: "Semgrep ruleset selection and risk coverage"
  - name: semgrep-scan-modes.md
    reason: "Full / diff / supply-chain scan-mode selection"
  - name: sarif-merge.md
    reason: "Deterministic multi-tool SARIF merge contract"
  - name: sarif-jq-queries.md
    reason: "Ad-hoc triage queries over SARIF output"
bindings:
  - type: command-manual
    target: review
    attachment: tool-recipe
  - type: command-manual
    target: validate
    attachment: tool-recipe
  - type: command-manual
    target: repair
    attachment: tool-recipe
provenance_ref: provenance.yaml
---

# SAST Orchestration

```
IRON LAW: RUN THE TOOL, TRIAGE SARIF, RECORD VERSION
```

## Purpose
Orchestrate static analysis (CodeQL, Semgrep, or equivalent) and triage the
SARIF output. The tool is load-bearing; the orchestrator is responsible for
reproducibility (versions, queries, scope) and for filtering false positives
with a written rationale.

## Tool recipe
- **Select** the SAST tool appropriate for the language(s) in scope. Default:
  CodeQL for compiled languages, Semgrep for scripting languages.
- **Pin** the tool and ruleset version; record both in the run manifest.
- **Scope** the scan to the diff plus its blast radius when possible.
- **Run** the scan; persist the SARIF file alongside the run artifact.
- **Triage** each SARIF result: confirmed, false-positive-with-reason,
  deferred-with-ticket. No result may be dropped silently.
- **Augment** Semgrep with custom rules only when a built-in ruleset does not
  cover the risk; cite the rule rationale.

## Report schema
```yaml
tool: codeql | semgrep | other
tool_version: "<semver>"
ruleset_version: "<semver or commit>"
scope: "<paths or query>"
results:
  - rule_id: "<id>"
    severity: high | medium | low
    location: "<path:line>"
    status: confirmed | false-positive | deferred
    rationale: "<why>"
```

## Anti-patterns
- Running SAST without pinning tool + ruleset version.
- Dismissing results without a written rationale.
- Custom rules without a stated risk that the built-in ruleset misses.
