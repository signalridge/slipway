# Catalog (target-indexed)

One row per Slipway catalog skill. 25 skills across 9 domains.

| # | Skill | Domain | Tier | Primary attachment | Primary bindings | Status |
|---|-------|--------|------|--------------------|------------------|--------|
| 1 | `scope-clarification` | intake | T1 | posture + checklist | host `intake-clarification`; technique-hint | shipped (B1) |
| 2 | `context-assembly` | intake | T1 | procedure + posture | hosts `research-orchestration`, `plan-audit`; technique-hint | shipped (B2) |
| 3 | `plan-authoring` | intake | T1 | procedure + checklist | host `plan-audit`; host-embedded; export-only | shipped (B1) |
| 4 | `tdd-proof` | execution | T1 | procedure | hosts `tdd-governance`, `wave-orchestration`; technique-hint | shipped (B1) |
| 5 | `parallel-executor-contract` | execution | T1 | procedure + checklist | host `wave-orchestration` | shipped (B2) |
| 6 | `fresh-verification-evidence` | execution | T1 | checklist + report-schema | hosts `goal-verification`, `final-closeout`, `tdd-governance` | shipped (B1) |
| 7 | `root-cause-tracing` | debugging | T1 | procedure | host `wave-orchestration`; command `repair`; technique-hint | shipped (B2) |
| 8 | `independent-review` | review-quality | T1 | procedure + checklist + report-schema | hosts `spec-compliance-review`, `code-quality-review`; command `review` | shipped (B1) |
| 9 | `multi-reviewer-calibration` | review-quality | T1 | procedure + checklist | host `code-quality-review`; command `review` | shipped (B4) |
| 10 | `security-review` | review-security | T1 | checklist | command `review`; hosts `spec-compliance-review`, `code-quality-review` | shipped (B2) |
| 11 | `threat-modeling` | review-security | T1 | procedure + report-schema | commands `review`, `validate`; export-only | shipped (B3) |
| 12 | `gha-security-review` | review-security | T2 | checklist + tool-recipe | commands `review`, `repair` | shipped (B3) |
| 13 | `supply-chain-audit` | review-security | T2 | checklist + tool-recipe | commands `review`, `repair`, `status` | shipped (B3) |
| 14 | `sast-orchestration` | review-security | T2 | tool-recipe | commands `review`, `validate`, `repair` | shipped (B3) |
| 15 | `differential-review` | review-change-shape | T1 | procedure + checklist | command `review` | shipped (B4) |
| 16 | `variant-analysis` | review-change-shape | T1 | procedure | commands `review`, `repair` | shipped (B4) |
| 17 | `spec-trace` | review-change-shape | T1 | checklist + report-schema | host `spec-compliance-review`; commands `validate`, `review` | shipped (B2) |
| 18 | `coverage-analysis` | verification | T1 | checklist + report-schema | commands `validate`; host `goal-verification` | shipped (B4) |
| 19 | `property-testing` | verification | T1 | procedure + checklist | command `validate`; host `goal-verification` | shipped (B4) |
| 20 | `mutation-testing` | verification | T1 | tool-recipe + report-schema | command `validate`; host `goal-verification` | shipped (B4) |
| 21 | `performance-profiling` | verification | T1 | procedure + report-schema | command `validate`; host `goal-verification`; command `status` | shipped (B4) |
| 22 | `ci-triage` | repair-ci | T2 | procedure + checklist | commands `repair`, `status` | shipped (B5) |
| 23 | `review-comment-triage` | repair-ci | T2 | procedure | command `repair` | shipped (B5) |
| 24 | `git-recovery` | repair-ci | T2 | procedure | commands `repair`, `status`; failure support for `worktree-preflight` | shipped (B5) |
| 25 | `incident-response` | ops-diagnostics | T3 | report-schema | commands `status`, `health`; export-only | shipped (B5) |

Status legend: `Bn` marks the rollout batch in which the skill lands. Upon
merge of that batch, Status column flips to `shipped`.

Attachment note:

- The table's `Primary attachment` column is a compact attachment profile.
- The first mode is the schema-level `primary_attachment`; any `+ ...` modes
  denote additional binding-level attachments for operator readability.

## Tier distribution

| Tier | Count |
|------|-------|
| T1 | 18 |
| T2 | 6 |
| T3 | 1 |
