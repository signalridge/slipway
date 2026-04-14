# By-source (source-indexed reverse index)

One row per authoritative source-corpus entry. The corpus contains 80 `SKILL.md`
files rooted at `skills_ref/`. `alirezarezvani/skill-tester/assets/sample-skill/
SKILL.md` is an embedded test fixture inside the `skill-tester` skill and is
**not** counted here. This document is manually maintained. It cites
`provenance.yaml` and does not replace per-skill provenance.

Disposition vocabulary:

- `standalone` — substantively feeds the named catalog skill
- `posture-only` — only stance/phrasing preserved; not a standalone promotion
- `partial-only` — only one subsection/template partial consumed
- `view-only` — kept as a diagnostics/view surface; no governed runtime method
- `route-only` — addressable via explicit command route or override
- `absorbed` — merged into another target's procedure/posture; no standalone node
- `deferred` — outside current rollout; listed for completeness

| Source | Disposition | Catalog skill(s) / landing | Status |
|--------|-------------|----------------------------|--------|
| alirezarezvani/adversarial-reviewer | standalone | multi-reviewer-calibration | B4 |
| alirezarezvani/agent-workflow-designer | absorbed | plan-authoring authoring guidance | B6 |
| alirezarezvani/code-reviewer | standalone | independent-review | B1 |
| alirezarezvani/dependency-auditor | standalone | supply-chain-audit | B3 |
| alirezarezvani/incident-commander | standalone | incident-response | B5 |
| alirezarezvani/incident-response | standalone | incident-response | B5 |
| alirezarezvani/performance-profiler | standalone | performance-profiling | B4 |
| alirezarezvani/pr-review-expert | standalone | differential-review | B4 |
| alirezarezvani/prompt-governance | deferred | future prompt-system governance | n/a |
| alirezarezvani/skill-security-auditor | view-only | health / validate diagnostics | B6 |
| alirezarezvani/skill-tester | view-only | validate diagnostics | B6 |
| getsentry/claude-settings-audit | view-only | health / validate diagnostics | B6 |
| getsentry/code-review | standalone | independent-review | B1 |
| getsentry/code-simplifier | absorbed | code-quality-review partial (simplification) | B4 |
| getsentry/find-bugs | standalone | differential-review | B4 |
| getsentry/gh-review-requests | view-only | status review queue view | B6 |
| getsentry/gha-security-review | standalone | gha-security-review | B3 |
| getsentry/iterate-pr | standalone | ci-triage + review-comment-triage | B5 |
| getsentry/security-review | standalone | security-review | B2 |
| getsentry/skill-scanner | view-only | health / validate diagnostics | B6 |
| openai/gh-address-comments | standalone | review-comment-triage | B5 |
| openai/gh-fix-ci | standalone | ci-triage | B5 |
| openai/security-best-practices | standalone | security-review | B2 |
| openai/security-ownership-map | standalone | threat-modeling | B3 |
| openai/security-threat-model | standalone | threat-modeling | B3 |
| openai/sentry | view-only | status / health observability view | B6 |
| sickn33/acceptance-orchestrator | absorbed | incident-response gate posture | B5 |
| sickn33/agent-orchestrator | posture-only | auto capability resolver heuristics | B6 |
| sickn33/antigravity-workflows | absorbed | distiller SOP / workflow routing | B6 |
| sickn33/audit-context-building | standalone | context-assembly | B2 |
| sickn33/code-review-ai-ai-review | standalone | multi-reviewer-calibration | B4 |
| spec-kitty/spec-kitty-charter-doctrine | absorbed | plan-authoring doctrine commentary | B6 |
| spec-kitty/spec-kitty-git-workflow | standalone | git-recovery | B5 |
| spec-kitty/spec-kitty-implement-review | standalone | parallel-executor-contract | B2 |
| spec-kitty/spec-kitty-mission-review | standalone | spec-trace | B2 |
| spec-kitty/spec-kitty-mission-system | posture-only | plan-authoring taxonomy commentary | B6 |
| spec-kitty/spec-kitty-runtime-next | posture-only | resolver constraints + conditional hydration | B6 |
| spec-kitty/spec-kitty-runtime-review | standalone | independent-review | B1 |
| superpowers/brainstorming | standalone | scope-clarification | B1 |
| superpowers/dispatching-parallel-agents | standalone | parallel-executor-contract | B2 |
| superpowers/executing-plans | posture-only | plan-authoring execution-contract | B6 |
| superpowers/receiving-code-review | standalone | independent-review | B1 |
| superpowers/requesting-code-review | standalone | independent-review | B1 |
| superpowers/subagent-driven-development | standalone | parallel-executor-contract | B2 |
| superpowers/systematic-debugging | standalone | root-cause-tracing | B2 |
| superpowers/test-driven-development | standalone | tdd-proof | B1 |
| superpowers/using-superpowers | posture-only | skill-first posture text | B6 |
| superpowers/verification-before-completion | standalone | fresh-verification-evidence | B1 |
| superpowers/writing-plans | standalone | plan-authoring | B1 |
| superpowers/writing-skills | absorbed | distiller SOP / adapter export guidance | B6 |
| trailofbits/agentic-actions-auditor | standalone | gha-security-review | B3 |
| trailofbits/ask-questions-if-underspecified | standalone | scope-clarification | B1 |
| trailofbits/audit-augmentation | standalone | sast-orchestration | B3 |
| trailofbits/audit-context-building | standalone | context-assembly | B2 |
| trailofbits/codeql | standalone | sast-orchestration (tool-recipe) | B3 |
| trailofbits/coverage-analysis | standalone | coverage-analysis | B4 |
| trailofbits/debug-buttercup | partial-only | root-cause-tracing triage posture | B2 |
| trailofbits/designing-workflow-skills | absorbed | distiller SOP | B6 |
| trailofbits/differential-review | standalone | differential-review | B4 |
| trailofbits/insecure-defaults | standalone | security-review | B2 |
| trailofbits/mutation-testing | standalone | mutation-testing | B4 |
| trailofbits/property-based-testing | standalone | property-testing | B4 |
| trailofbits/sarif-parsing | standalone | sast-orchestration | B3 |
| trailofbits/second-opinion | route-only | explicit `review` route/override | B6 |
| trailofbits/semgrep | standalone | sast-orchestration (tool-recipe) | B3 |
| trailofbits/sharp-edges | standalone | security-review | B2 |
| trailofbits/spec-to-code-compliance | standalone | spec-trace | B2 |
| trailofbits/supply-chain-risk-auditor | standalone | supply-chain-audit | B3 |
| trailofbits/variant-analysis | standalone | variant-analysis | B4 |
| wshobson/block-no-verify-hook | absorbed | git-recovery / policy guidance | B5 |
| wshobson/code-review-excellence | standalone | independent-review | B1 |
| wshobson/context-driven-development | standalone | context-assembly | B2 |
| wshobson/debugging-strategies | standalone | root-cause-tracing | B2 |
| wshobson/distributed-tracing | partial-only | performance-profiling checklist | B4 |
| wshobson/e2e-testing-patterns | standalone | coverage-analysis | B4 |
| wshobson/error-handling-patterns | posture-only | independent-review + code-quality-review partials | B6 |
| wshobson/git-advanced-workflows | standalone | git-recovery | B5 |
| wshobson/multi-reviewer-patterns | standalone | multi-reviewer-calibration | B4 |
| wshobson/parallel-debugging | standalone | root-cause-tracing (competing-hypothesis branch) | B2 |
| wshobson/workflow-patterns | standalone | plan-authoring + tdd-proof | B1 |

## Coverage snapshot

| Disposition | Count |
|-------------|-------|
| standalone | 56 |
| posture-only | 6 |
| partial-only | 2 |
| absorbed | 8 |
| view-only | 6 |
| route-only | 1 |
| deferred | 1 |
| **total** | **80** |

`Status` records the rollout batch where the source landing was implemented
(`B1`-`B6`). Deferred entries remain `n/a`.
