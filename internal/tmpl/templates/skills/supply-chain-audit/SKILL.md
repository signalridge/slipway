---
skill_id: supply-chain-audit
domain: review-security
function: audit third-party dependencies for CVE, provenance, and pinning risk
tier: T2
primary_attachment: checklist
summary: "Use when dependency manifests or lockfiles change. Triggers on review, repair, or status commands or on changes to package/lock files."
trigger_signals:
  - changed_files_include:
      - "go.mod"
      - "go.sum"
      - "package.json"
      - "package-lock.json"
      - "pnpm-lock.yaml"
      - "yarn.lock"
      - "Cargo.toml"
      - "Cargo.lock"
      - "requirements*.txt"
      - "pyproject.toml"
      - "poetry.lock"
      - "uv.lock"
    reason: "Dependency manifest or lockfile changed"
  - command: ["review", "repair", "status"]
    reason: "Review/repair/status command invoked; dependency surface may apply"
evidence_contract: verdict
hydrate_references:
  - name: results-template.md
    reason: "Audit report schema for supply-chain findings"
  - name: dependency-management-best-practices.md
    reason: "Pinning, review cadence, and lockfile discipline"
  - name: vulnerability-assessment-guide.md
    reason: "CVE triage and severity assignment under time pressure"
  - name: license-compatibility-matrix.md
    reason: "License compatibility rules per distribution target"
bindings:
  - type: command-manual
    target: review
    attachment: checklist
  - type: command-manual
    target: repair
    attachment: tool-recipe
  - type: command-manual
    target: status
    attachment: checklist
provenance_ref: provenance.yaml
---

# Supply Chain Audit

```
IRON LAW: NEW DEPENDENCY IS A NEW ATTACK SURFACE
```

## Purpose
Audit changes to dependency manifests and lockfiles. Every added or updated
dependency must clear a provenance check, a CVE check, and a pinning check
before it ships.

## Checklist
- [ ] Added dependencies are listed with name, version, and rationale (why
      this package over existing alternatives).
- [ ] Each added or updated dependency has a CVE check against the resolved
      version, using a pinned scanner (osv-scanner, npm audit, govulncheck,
      cargo-audit, pip-audit).
- [ ] Provenance: the package is from the expected publisher; no typo-squat
      lookalikes; release is not brand-new (< 7 days) unless justified.
- [ ] Transitive additions are enumerated, not hidden. Unexpected transitives
      are called out.
- [ ] Lockfile is updated in the same change as the manifest; no drift.
- [ ] License is compatible with the project's license policy.
- [ ] For pinned-SHA ecosystems (actions, container images), pinning is
      verified end-to-end.

## Report schema
```yaml
verdict: pass | changes-requested | blocked
scanner: "<tool + version>"
additions:
  - name: "<package>"
    version: "<version>"
    transitive: true | false
    cve_status: clean | advisories_present
    provenance_status: verified | suspect
    license: "<spdx>"
    rationale: "<why this package>"
findings:
  - severity: blocker | major | minor
    category: cve | provenance | pinning | license | transitive
    location: "<manifest or lockfile:line>"
    remediation: "<concrete action>"
```

## Anti-patterns
- Accepting a dependency because "npm/pypi says it's popular".
- Skipping transitives because the direct add "looks fine".
- Running the scanner once locally without pinning the scanner version.
