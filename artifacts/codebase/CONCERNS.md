# Concerns

Re-authored for change
`add-engine-enforced-fail-closed-safety-nets-for-shared-workt`.

- Public contract drift: `WaveRun.dispatch_mode` changes from `parallel` to
  `parallel_subagents`, and readiness surfaces four new blocker codes. Tests must
  pin the new reason-code taxonomy and dispatch parser behavior.
- Silent-safety regression: a started parallel wave without explicit
  `dispatch_mode` evidence must fail closed instead of inheriting the plan's
  `Parallel` flag.
- Scope-audit false negatives: if task evidence under-reports `changed_files`,
  the engine cannot detect every collision. The generated host surface must keep
  exhaustive `changed_files` as an explicit safety requirement.
- Scope-audit false positives: stale task evidence should not be judged against a
  changed plan. The safety-net blockers therefore stay behind the existing
  plan-drift suppression guard.
- Shared-worktree collision risk: same-wave parallel tasks writing the same
  canonical path can clobber each other. The overlap audit must bucket paths with
  the same normalization semantics as planner conflict detection.
- Evidence-completeness ceiling: `executor_agent` handles prove the host recorded
  per-task executor claims, not that the engine spawned those agents. The
  generated docs must avoid implying an engine runtime exists.
- View freshness risk: wave-plan narrowing advisories are useful for plan audit
  but must remain display-only; persisting them would churn `wave-plan.yaml` and
  freshness hashes.
