# Routed surfaces

Frozen list of non-catalog surfaces. These are not catalog skills. They are
either view-only (`status` / `health` diagnostics), route-only (addressed via
explicit command routes), absorbed (merged posture/partial), or deferred
(future work).

## view-only

Surfaces mounted on `status` / `health` / `validate` as read-only diagnostics
landing zones. They do not carry progression authority, and only a subset are
currently exposed as explicit CLI selectors.

| Surface | Landing zone | Reason |
|---------|--------------|--------|
| `review-queue` | `status` view | queue aggregation wrapper |
| `observability-query` | `status` / `health` view | read-only inspection |
| `claude-settings-audit` | `health` / `validate` diagnostics | permission/config audit |
| `skill-scanner` | `health` / `validate` diagnostics | skill security audit report |
| `skill-security-auditor` | `health` / `validate` diagnostics | overlap with skill-scanner |
| `skill-tester` | `validate` diagnostics | quality gate / reporting |
| `gh-review-requests` | `status` review queue view | queue/query helper |
| `sentry` | `status` / `health` observability view | provider-specific read-only query |

### Minimum view schemas (B5 item 20)

Keep diagnostics views stylistically compatible with T3 `incident-response`.

| Surface | Minimum schema shape |
|---------|----------------------|
| `sentry` | `service`, `incident_hint`, `signal`, `severity`, `observed_at`, `evidence` |
| `skill-scanner` | `skill_id`, `risk_level`, `finding`, `severity`, `evidence`, `remediation` |
| `observability-query` | `service`, `metric_or_trace`, `window`, `anomaly`, `evidence`, `next_step` |
| `review-queue` | `queue_id`, `item_ref`, `age`, `priority`, `owner`, `action` |

## route-only

Addressable only via an explicit command route/override.

| Surface | Landing zone | Reason |
|---------|--------------|--------|
| `second-opinion` | `review` route override (`--mode=second-opinion`) | valuable review surface; not a reusable method |

## absorbed

Merged into a catalog skill's procedure/posture without a standalone node.

| Surface | Landing | Reason |
|---------|---------|--------|
| `agent-workflow-designer` | plan-authoring guidance | authoring meta-skill |
| `designing-workflow-skills` | distiller SOP | workflow-skill design rules |
| `writing-skills` | distiller SOP | TDD-for-skills process |
| `antigravity-workflows` | distiller SOP + workflow routing | orchestration meta-skill |
| `acceptance-orchestrator` | incident-response gate posture | preserves gate posture |
| `block-no-verify-hook` | git-recovery / policy guidance | hook-specific policy |
| `spec-kitty-charter-doctrine` | plan-authoring doctrine notes | already absorbed |
| `simplification-pass` | independent-review + code-quality-review partial | internal review technique |
| `review-request-response` | independent-review + review-comment-triage | spans two lifecycle points |
| `hypothesis-arbitration` | root-cause-tracing | cleaner as advanced branch |
| `code-simplifier` | code-quality-review partial | simplification posture |

## posture-only

Absorbed as stance only; no standalone promotion.

| Source | Landed into |
|--------|-------------|
| `superpowers/using-superpowers` | project- and agent-level skill-first posture |
| `superpowers/executing-plans` | plan-authoring execution-contract sections |
| `spec-kitty/mission-system` | plan-authoring taxonomy and procedure commentary |
| `spec-kitty/runtime-next` | resolver constraints + conditional hydration |
| `sickn33/agent-orchestrator` | auto capability resolver heuristics |
| `wshobson/error-handling-patterns` | independent-review + code-quality-review partials |

## deferred

| Surface | Reason |
|---------|--------|
| `skill-factory` | future repo-local `skill` command family |
| `prompt-governance` | future prompt-system governance surface |

## Command landing summary

| Command | Bound catalog skills | Diagnostics landing zones / shipped overrides |
|---------|---------------------|--------------------------|
| `review` | independent-review, multi-reviewer-calibration, security-review, threat-modeling, gha-security-review, supply-chain-audit, sast-orchestration, differential-review, variant-analysis, spec-trace; `second-opinion` override | — |
| `validate` | spec-trace, coverage-analysis, property-testing, mutation-testing, performance-profiling | skill-tester, skill-scanner, skill-security-auditor, claude-settings-audit |
| `repair` | root-cause-tracing, ci-triage, review-comment-triage, git-recovery, supply-chain-audit, gha-security-review, variant-analysis | — |
| `status` | incident-response, supply-chain-audit, ci-triage, performance-profiling | review-queue, observability-query, gh-review-requests, sentry |
| `health` | incident-response (T3 view) | observability-query, sentry, claude-settings-audit, skill-scanner, skill-security-auditor |

`--mode` / `--view` flags are shipped:
`review` / `validate` / `repair` support `--mode`,
`status` / `health` support `--view`.
Automatic `--view` routing applies only when a concrete active/selected change
context exists. In diagnostics fallback with no active change, `status` and
`health` keep `view` empty unless the operator passed an explicit `--view`.
Current non-catalog explicit `--view` overrides are
`review-queue` and `observability-query`.
For concrete active/selected changes, current auto-route selects
`incident-response` as the shipped catalog T3 view.
`validate` has no standalone `--view` flag; entries listed under `validate`
describe diagnostics/reporting landing zones rather than per-selector CLI
surfaces.
Other `view-only` entries in this table are documented diagnostics landing zones,
not guaranteed standalone `--view <id>` selectors.
`status` / `health` currently share one payload renderer: the selected
`--view` value is validated and preserved in output, but a bespoke per-view
payload is only guaranteed once that surface grows dedicated rendering logic.
